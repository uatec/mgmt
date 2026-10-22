// Package engine validates resources, evaluates policies and runs the pipeline.
package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"

	"github.com/uatec/mgmt/internal/extract"
	"github.com/uatec/mgmt/internal/facts"
	mload "github.com/uatec/mgmt/internal/load"
)

var kindNameRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

// schemaFiles gathers core and workspace .cue sources, sorted by origin path.
func schemaFiles(core fs.FS, wsSchemaDir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	walk := func(fsys fs.FS, prefix string) error {
		return fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".cue") {
				return err
			}
			b, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}
			files[prefix+p] = b
			return nil
		})
	}
	if sub, err := fs.Sub(core, "schema"); err == nil {
		if err := walk(sub, "core/"); err != nil {
			return nil, err
		}
	}
	st, err := os.Stat(wsSchemaDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return files, nil // workspace schemas are optional
	case err != nil:
		return nil, err
	case !st.IsDir():
		return nil, fmt.Errorf("%s is not a directory", wsSchemaDir)
	}
	if err := walk(os.DirFS(wsSchemaDir), "ws/"); err != nil {
		return nil, err
	}
	return files, nil
}

// build compiles all schema files as one package from memory only.
func build(ctx *cue.Context, core fs.FS, wsSchemaDir string) (cue.Value, error) {
	files, err := schemaFiles(core, wsSchemaDir)
	if err != nil {
		return cue.Value{}, err
	}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	dir := string(filepath.Separator) + "mgmt-schema"
	overlay := map[string]load.Source{}
	var args []string
	for i, n := range names {
		p := filepath.Join(dir, fmt.Sprintf("%03d_%s", i, strings.ReplaceAll(path.Clean(n), "/", "_")))
		overlay[p] = load.FromBytes(files[n])
		args = append(args, p)
	}
	if len(args) == 0 {
		return cue.Value{}, fmt.Errorf("no schema files found")
	}
	insts := load.Instances(args, &load.Config{Dir: dir, Overlay: overlay, Package: "mgmt"})
	if len(insts) != 1 {
		return cue.Value{}, fmt.Errorf("unexpected schema instance count %d", len(insts))
	}
	if insts[0].Err != nil {
		return cue.Value{}, fmt.Errorf("%s", firstLine(insts[0].Err.Error()))
	}
	v := ctx.BuildInstance(insts[0])
	if err := v.Err(); err != nil {
		return cue.Value{}, fmt.Errorf("%s", firstLine(err.Error()))
	}
	return v, nil
}

// kindDefs maps kind name to its definition value. A top-level definition
// #Name registers kind Name only when its kind field is the literal "Name"
// (as in #Name: #Resource & {kind: "Name", ...}); every other definition
// name (helpers such as #Date, or a kind definition that forgot its kind
// literal) is returned in others so Validate can explain the miss.
func kindDefs(v cue.Value) (kinds map[string]cue.Value, others map[string]bool) {
	kinds, others = map[string]cue.Value{}, map[string]bool{}
	it, err := v.Fields(cue.Definitions(true))
	if err != nil {
		return kinds, others
	}
	for it.Next() {
		name := strings.TrimPrefix(it.Selector().String(), "#")
		if name == "Resource" {
			continue
		}
		k := it.Value().LookupPath(cue.ParsePath("kind"))
		if s, err := k.String(); err == nil && s == name {
			kinds[name] = it.Value()
		} else {
			others[name] = true
		}
	}
	return kinds, others
}

// Kinds lists the resource kinds defined by the core and workspace schemas, sorted.
func Kinds(core fs.FS, wsSchemaDir string) ([]string, error) {
	v, err := build(cuecontext.New(), core, wsSchemaDir)
	if err != nil {
		return nil, err
	}
	defs, _ := kindDefs(v)
	kinds := make([]string, 0, len(defs))
	for k := range defs {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds, nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// cueMessage is the first line of a validation error. When that is only a
// disjunction summary ("N errors in empty disjunction"), as for a defaulted
// list such as milestones, the most deeply nested error is more useful.
func cueMessage(err error) string {
	msg := firstLine(err.Error())
	if !strings.Contains(msg, "empty disjunction") {
		return msg
	}
	depth := 0
	for _, e := range cerrors.Errors(err) {
		if d := len(e.Path()); d > depth {
			depth, msg = d, firstLine(e.Error())
		}
	}
	return msg
}

func invalid(file string, line int, msg string) facts.Finding {
	return facts.Finding{Severity: "error", ID: "schema-invalid", File: file, Line: line, Message: msg}
}

// Validate checks every resource against its CUE definition and requires
// exactly one Workspace. wsSchemaDir (<root>/schema) may not exist.
func Validate(resources []mload.Resource, core fs.FS, wsSchemaDir string) []facts.Finding {
	ctx := cuecontext.New()
	schema, err := build(ctx, core, wsSchemaDir)
	if err != nil {
		return []facts.Finding{invalid("schema", 1, "schema does not compile: "+err.Error())}
	}
	defs, others := kindDefs(schema)
	var finds []facts.Finding
	seenWS := 0
	seenName := map[string]bool{}
	var fws []mload.Resource // valid frameworks, checked together for includes
	allFWs := map[string]bool{}
	for _, r := range resources {
		if r.Kind == "CompetencyFramework" {
			allFWs[r.Name] = true
		}
		if r.Kind == "Workspace" {
			seenWS++
			if seenWS > 1 {
				finds = append(finds, invalid(r.File, r.Line, "only one Workspace is allowed"))
			}
		}
		def, ok := defs[r.Kind]
		if !ok || !kindNameRE.MatchString(r.Kind) {
			msg := "no schema for kind " + r.Kind
			if others[r.Kind] && kindNameRE.MatchString(r.Kind) {
				msg += fmt.Sprintf(": definition #%[1]s does not set kind: %[2]q (write #%[1]s: #Resource & {kind: %[2]q, ...})", r.Kind, r.Kind)
			}
			finds = append(finds, invalid(r.File, r.Line, msg))
			continue
		}
		if r.Kind == "ReviewCycle" {
			if msg := selectorProblem(r.Object); msg != "" {
				finds = append(finds, invalid(r.File, r.Line, msg))
				continue
			}
		}
		v := def.Unify(ctx.Encode(intify(r.Object)))
		if err := v.Validate(cue.Concrete(true)); err != nil {
			finds = append(finds, invalid(r.File, r.Line, strings.TrimPrefix(cueMessage(err), "#"+r.Kind+".")))
			continue
		}
		// Workspaces are already limited to one above.
		key := r.Kind + "/" + r.Name
		if r.Kind != "Workspace" && seenName[key] {
			finds = append(finds, invalid(r.File, r.Line, fmt.Sprintf("duplicate %s %q: metadata.name must be unique per kind", r.Kind, r.Name)))
			continue
		}
		seenName[key] = true
		if r.Kind == "CompetencyFramework" {
			msgs := frameworkDuplicates(r.Object)
			for _, msg := range msgs {
				finds = append(finds, invalid(r.File, r.Line, msg))
			}
			if len(msgs) == 0 {
				fws = append(fws, r)
			}
		}
	}
	finds = append(finds, frameworkIncludes(fws, allFWs)...)
	if seenWS == 0 {
		finds = append(finds, invalid("resources", 1, "no Workspace resource found; exactly one is required"))
	}
	return finds
}

// intify turns whole-number float64 values (as produced by YAML-to-JSON
// decoding) into int64 so they satisfy CUE int constraints.
func intify(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = intify(e)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = intify(e)
		}
		return out
	case float64:
		if t == math.Trunc(t) && math.Abs(t) < 1<<53 {
			return int64(t)
		}
	}
	return v
}

// selectorProblem enforces that ReviewCycle.spec.selector names people or frameworks.
func selectorProblem(obj map[string]any) string {
	spec, _ := obj["spec"].(map[string]any)
	sel, ok := spec["selector"].(map[string]any)
	if !ok {
		return "" // CUE reports the missing or mistyped selector
	}
	_, p := sel["people"]
	_, m := sel["matchFrameworks"]
	if !p && !m {
		return "spec.selector: needs at least one of people and matchFrameworks"
	}
	return ""
}

// frameworkDuplicates enforces unique competency ids and ladder levels per
// framework and descriptors keyed by ladder levels, which CUE lists cannot
// express. Call it only on a CUE-valid framework.
func frameworkDuplicates(obj map[string]any) []string {
	spec, _ := obj["spec"].(map[string]any)
	msgs := uniqueNames(spec["competencies"], "id", "spec.competencies: duplicate competency id %q")
	onLadder := map[string]bool{}
	for _, l := range strList(spec["levels"]) {
		if onLadder[l] {
			msgs = append(msgs, fmt.Sprintf("spec.levels: duplicate level %q", l))
		}
		onLadder[l] = true
	}
	comps, _ := spec["competencies"].([]any)
	for _, c := range comps {
		cm, _ := c.(map[string]any)
		id, _ := cm["id"].(string)
		levels, _ := cm["levels"].(map[string]any)
		for _, l := range sortedKeys(levels) {
			if !onLadder[l] {
				msgs = append(msgs, fmt.Sprintf("spec.competencies: competency %q: descriptor for level %q, which is not on the ladder", id, l))
			}
		}
	}
	return msgs
}

// frameworkIncludes checks includes across the valid frameworks fws (in
// resource order): every include names a framework in all, includes do not
// loop, an included framework has the same ladder, and no competency id is
// defined twice in a framework's include closure. A duplicate already
// present in a direct include's closure is reported there only.
func frameworkIncludes(fws []mload.Resource, all map[string]bool) []facts.Finding {
	type fw struct {
		includes, levels, ids []string
	}
	specs := map[string]map[string]any{}
	byName := map[string]fw{}
	for _, r := range fws {
		spec, _ := r.Object["spec"].(map[string]any)
		specs[r.Name] = spec
		f := fw{includes: strList(spec["includes"]), levels: strList(spec["levels"])}
		comps, _ := spec["competencies"].([]any)
		for _, c := range comps {
			cm, _ := c.(map[string]any)
			id, _ := cm["id"].(string)
			f.ids = append(f.ids, id)
		}
		byName[r.Name] = f
	}
	// dups lists the ids defined by more than one framework in name's closure.
	dups := func(name string) map[string][]string {
		owners := map[string][]string{}
		for _, n := range extract.IncludeClosure(specs, name) {
			for _, id := range byName[n].ids {
				owners[id] = append(owners[id], n)
			}
		}
		out := map[string][]string{}
		for id, ns := range owners {
			if len(ns) > 1 {
				sort.Strings(ns)
				out[id] = ns
			}
		}
		return out
	}
	var finds []facts.Finding
	for _, r := range fws {
		f := byName[r.Name]
		for _, inc := range f.includes {
			if !all[inc] {
				finds = append(finds, invalid(r.File, r.Line, fmt.Sprintf("spec.includes: unknown framework %q", inc)))
			}
		}
		if path := includePath(r.Name, r.Name, func(n string) []string { return byName[n].includes }, map[string]bool{}); path != nil {
			finds = append(finds, invalid(r.File, r.Line, "spec.includes: include cycle "+strings.Join(append([]string{r.Name}, path...), " -> ")))
		}
		for _, inc := range f.includes {
			if g, ok := byName[inc]; ok && strings.Join(g.levels, "\x00") != strings.Join(f.levels, "\x00") {
				finds = append(finds, invalid(r.File, r.Line, fmt.Sprintf("spec.levels: ladder [%s] differs from included framework %q [%s]",
					strings.Join(f.levels, ", "), inc, strings.Join(g.levels, ", "))))
			}
		}
		inherited := map[string]bool{}
		for _, inc := range f.includes {
			for id := range dups(inc) {
				inherited[id] = true
			}
		}
		d := dups(r.Name)
		for _, id := range sortedKeys(d) {
			if !inherited[id] {
				finds = append(finds, invalid(r.File, r.Line, fmt.Sprintf("spec.competencies: duplicate competency id %q across included frameworks: %s", id, strings.Join(d[id], ", "))))
			}
		}
	}
	return finds
}

// includePath returns a path of includes from "from" to target (ending at
// target), following includes in declared order, or nil if there is none.
func includePath(from, target string, includes func(string) []string, seen map[string]bool) []string {
	for _, n := range includes(from) {
		if n == target {
			return []string{n}
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		if p := includePath(n, target, includes, seen); p != nil {
			return append([]string{n}, p...)
		}
	}
	return nil
}

func strList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, e := range list {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// uniqueNames reports each repeated item[key] in a list once per repeat, using format.
func uniqueNames(items any, key, format string) []string {
	var msgs []string
	seen := map[string]bool{}
	list, _ := items.([]any)
	for _, it := range list {
		m, _ := it.(map[string]any)
		v, _ := m[key].(string)
		if seen[v] {
			msgs = append(msgs, fmt.Sprintf(format, v))
		}
		seen[v] = true
	}
	return msgs
}
