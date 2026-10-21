// Package engine validates resources, evaluates policies and runs the pipeline.
package engine

import (
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
	"cuelang.org/go/cue/load"

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
	if st, err := os.Stat(wsSchemaDir); err == nil && st.IsDir() {
		if err := walk(os.DirFS(wsSchemaDir), "ws/"); err != nil {
			return nil, err
		}
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
	for _, r := range resources {
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
			finds = append(finds, invalid(r.File, r.Line, strings.TrimPrefix(firstLine(err.Error()), "#"+r.Kind+".")))
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
			for _, msg := range frameworkDuplicates(r.Object) {
				finds = append(finds, invalid(r.File, r.Line, msg))
			}
		}
	}
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

// selectorProblem enforces that ReviewCycle.spec.selector names people or roles.
func selectorProblem(obj map[string]any) string {
	spec, _ := obj["spec"].(map[string]any)
	sel, ok := spec["selector"].(map[string]any)
	if !ok {
		return "" // CUE reports the missing or mistyped selector
	}
	_, p := sel["people"]
	_, m := sel["matchRoles"]
	if !p && !m {
		return "spec.selector: needs at least one of people and matchRoles"
	}
	return ""
}

// frameworkDuplicates enforces unique competency ids and role names per
// framework and unique level names per role, which CUE lists cannot express.
// Call it only on a CUE-valid framework.
func frameworkDuplicates(obj map[string]any) []string {
	spec, _ := obj["spec"].(map[string]any)
	var msgs []string
	msgs = append(msgs, uniqueNames(spec["competencies"], "id", "spec.competencies: duplicate competency id %q")...)
	msgs = append(msgs, uniqueNames(spec["roles"], "name", "spec.roles: duplicate role name %q")...)
	roles, _ := spec["roles"].([]any)
	for _, r := range roles {
		role, _ := r.(map[string]any)
		name, _ := role["name"].(string)
		msgs = append(msgs, uniqueNames(role["levels"], "name", fmt.Sprintf("spec.roles: role %q: duplicate level name %%q", name))...)
	}
	return msgs
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
