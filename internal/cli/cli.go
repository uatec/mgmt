// Package cli parses arguments and dispatches mgmt's subcommands.
package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	mgmt "github.com/uatec/mgmt"
	"github.com/uatec/mgmt/internal/engine"
	"github.com/uatec/mgmt/internal/extract"
	"github.com/uatec/mgmt/internal/facts"
	"github.com/uatec/mgmt/internal/load"
	"github.com/uatec/mgmt/internal/render"
)

const usage = `usage: mgmt [--now YYYY-MM-DD] [--out FILE] <command> [args]
       (--now and --out may also follow the command)

commands:
  init                                 scaffold a new workspace here (never overwrites)
  facts                                print the facts document as JSON
  lint [--strict] [--format text|json] print errors and warnings
  prep <person> [--cycle NAME]         print the 1:1 prep sheet
  review <person> [--cycle NAME]       print the review evidence pack
  explain <person> <competency> [--cycle NAME]
                                       explain why a competency has its status
  status [--cycle NAME]                print the team matrix and cycle conditions
  report <name> [<person>] [--cycle NAME]
                                       run a workspace report (reports/<name>.tmpl)
`

// Run executes one mgmt invocation against the workspace at root and returns
// the exit code. Command output goes to stdout, or to --out FILE.
func Run(args []string, root string, stdout, stderr io.Writer) int {
	global := flag.NewFlagSet("mgmt", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	now := global.String("now", time.Now().Format("2006-01-02"), "")
	out := global.String("out", "", "")
	if err := global.Parse(args); err != nil {
		return usageError(stderr, err.Error())
	}
	rest := global.Args()
	if len(rest) == 0 {
		return usageError(stderr, "no command given")
	}
	sub, err := hoistGlobals(rest[1:], global)
	if err != nil {
		return usageError(stderr, rest[0]+": "+err.Error())
	}
	rest = append([]string{rest[0]}, sub...)
	if _, err := time.Parse("2006-01-02", *now); err != nil {
		return usageError(stderr, fmt.Sprintf("--now %q is not a YYYY-MM-DD date", *now))
	}
	var buf bytes.Buffer
	var code int
	switch rest[0] {
	case "init":
		code = runInit(rest[1:], root, &buf, stderr)
	case "facts":
		code = runFacts(rest[1:], root, *now, &buf, stderr)
	case "lint":
		code = runLint(rest[1:], root, *now, &buf, stderr)
	case "prep":
		code = runReport("prep", rest[1:], "<person>", root, *now, &buf, stderr)
	case "review":
		code = runReport("review", rest[1:], "<person>", root, *now, &buf, stderr)
	case "explain":
		code = runReport("explain", rest[1:], "<person> <competency>", root, *now, &buf, stderr)
	case "status":
		code = runReport("status", rest[1:], "", root, *now, &buf, stderr)
	case "report":
		code = runCustomReport(rest[1:], root, *now, &buf, stderr)
	default:
		return usageError(stderr, fmt.Sprintf("unknown command %q", rest[0]))
	}
	if *out == "" {
		_, _ = stdout.Write(buf.Bytes())
		return code
	}
	if code == 2 && buf.Len() == 0 {
		return code // failed with nothing to write: leave --out untouched
	}
	path := *out
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return 2
	}
	return code
}

// hoistGlobals removes the global flags (--now, --out; "-" or "--", with the
// value as the next argument or after "=") from a subcommand's arguments and
// sets them on global, so they work after the subcommand too. A later
// occurrence overrides an earlier one.
func hoistGlobals(args []string, global *flag.FlagSet) ([]string, error) {
	var rest []string
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(args[i], "-"), "-"), "=")
		if !strings.HasPrefix(args[i], "-") || (name != "now" && name != "out") {
			rest = append(rest, args[i])
			continue
		}
		if !hasValue {
			if i+1 >= len(args) {
				return nil, fmt.Errorf("flag needs an argument: -%s", name)
			}
			i++
			value = args[i]
		}
		if err := global.Set(name, value); err != nil {
			return nil, err
		}
	}
	return rest, nil
}

func usageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "mgmt: %s\n%s", msg, usage)
	return 2
}

// pipeline runs the engine, reporting I/O failures on stderr.
func pipeline(root, now string, stderr io.Writer) (*engine.Result, bool) {
	res, err := engine.Run(root, engine.Options{Now: now})
	if err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return nil, false
	}
	return res, true
}

func runFacts(args []string, root, now string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		return usageError(stderr, "facts takes no arguments")
	}
	res, ok := pipeline(root, now, stderr)
	if !ok {
		return 2
	}
	if err := writeJSON(stdout, res.Facts); err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return 2
	}
	return 0
}

func runLint(args []string, root, now string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("lint", flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	strict := fl.Bool("strict", false, "")
	format := fl.String("format", "text", "")
	if err := fl.Parse(args); err != nil {
		return usageError(stderr, "lint: "+err.Error())
	}
	if fl.NArg() != 0 {
		return usageError(stderr, "lint takes no arguments")
	}
	if *format != "text" && *format != "json" {
		return usageError(stderr, fmt.Sprintf("lint: unknown format %q", *format))
	}
	res, ok := pipeline(root, now, stderr)
	if !ok {
		return 2
	}
	if *format == "json" {
		doc := map[string][]facts.Finding{"errors": res.Errors, "warnings": res.Warnings}
		if err := writeJSON(stdout, doc); err != nil {
			fmt.Fprintf(stderr, "mgmt: %v\n", err)
			return 2
		}
	} else {
		_, _ = io.WriteString(stdout, lintText(res.Errors, res.Warnings))
	}
	switch {
	case len(res.Errors) > 0:
		return 2
	case *strict && len(res.Warnings) > 0:
		return 1
	}
	return 0
}

// runReport parses "<args> [--cycle NAME]" (flags may come anywhere) for a
// built-in report and renders it; its view must exist.
func runReport(name string, args []string, argUsage, root, now string, stdout, stderr io.Writer) int {
	pos, cycle, code := parseReportArgs(name, args, stderr)
	if code != 0 {
		return code
	}
	if want := len(strings.Fields(argUsage)); len(pos) != want {
		if argUsage == "" {
			argUsage = "no arguments"
		}
		return usageError(stderr, fmt.Sprintf("%s takes %s", name, argUsage))
	}
	return renderReport(name, pos, cycle, true, root, now, stdout, stderr)
}

// builtinReports are the core reports with their own commands (Ruling R16):
// their views are keyed differently, so `report` refuses them and names the
// command instead. A workspace may still override their templates.
var builtinReports = map[string]string{
	"prep":    "prep <person>",
	"review":  "review <person>",
	"explain": "explain <person> <competency>",
	"status":  "status",
}

// runCustomReport runs "report <name> [<person>] [--cycle NAME]": any
// workspace template other than a built-in report's. Unlike the built-in commands, its
// view may be null (Ruling R4): custom templates can use facts and derive.
func runCustomReport(args []string, root, now string, stdout, stderr io.Writer) int {
	pos, cycle, code := parseReportArgs("report", args, stderr)
	if code != 0 {
		return code
	}
	if len(pos) < 1 || len(pos) > 2 {
		return usageError(stderr, "report takes <name> [<person>]")
	}
	if usage, ok := builtinReports[pos[0]]; ok {
		fmt.Fprintf(stderr, "mgmt: %s is a built-in report; run \"mgmt %s\"\n", pos[0], usage)
		return 2
	}
	ok, err := render.Exists(pos[0], mgmt.CoreReports, filepath.Join(root, "reports"))
	if err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return 2
	}
	if !ok {
		fmt.Fprintf(stderr, "mgmt: unknown report %q\n", pos[0])
		return 2
	}
	return renderReport(pos[0], pos[1:], cycle, false, root, now, stdout, stderr)
}

// parseReportArgs returns the positional arguments and --cycle; code is
// non-zero after a usage error.
func parseReportArgs(name string, args []string, stderr io.Writer) (pos []string, cycle string, code int) {
	fl := flag.NewFlagSet(name, flag.ContinueOnError)
	fl.SetOutput(io.Discard)
	c := fl.String("cycle", "", "")
	pos, err := parseInterspersed(fl, args)
	if err != nil {
		return nil, "", usageError(stderr, name+": "+err.Error())
	}
	return pos, *c, 0
}

// renderReport runs the pipeline, refuses on errors (the lint text goes to
// stderr), resolves pos[0] as the person and pos[1] as a competency of
// theirs, and renders report name. strict makes a missing view an error.
func renderReport(name string, pos []string, cycle string, strict bool, root, now string, stdout, stderr io.Writer) int {
	opts := engine.Options{Now: now, Cycle: cycle}
	res, err := engine.Run(root, opts)
	if err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return 2
	}
	if len(res.Errors) > 0 {
		_, _ = io.WriteString(stderr, lintText(res.Errors, res.Warnings))
		return 2
	}
	if _, ok := res.Facts.Resources["ReviewCycle"][opts.Cycle]; opts.Cycle != "" && !ok {
		fmt.Fprintf(stderr, "mgmt: unknown cycle %q\n", opts.Cycle)
		return 2
	}
	var keys []string // status takes no person: its view is unkeyed (Ruling R4)
	if len(pos) > 0 {
		person, ok := resolvePerson(res.Facts, pos[0])
		if !ok {
			fmt.Fprintf(stderr, "mgmt: unknown person %q\n", pos[0])
			return 2
		}
		if opts.Cycle != "" && !inCycle(res.Derive, opts.Cycle, person) {
			fmt.Fprintf(stderr, "mgmt: %s is not in cycle %q\n", person, opts.Cycle)
			return 2
		}
		keys = []string{person}
	}
	if len(pos) == 2 {
		id, suggest, ok := resolveCompetency(res.Facts, keys[0], pos[1])
		if !ok {
			fmt.Fprintf(stderr, "mgmt: unknown competency %q", pos[1])
			if suggest != "" {
				fmt.Fprintf(stderr, " (did you mean %q?)", suggest)
			}
			fmt.Fprintln(stderr)
			return 2
		}
		keys = append(keys, id)
	}
	data, err := reportData(res, name, keys, opts, strict)
	if err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return 2
	}
	text, err := render.Render(name, data, mgmt.CoreReports, filepath.Join(root, "reports"))
	if err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return 2
	}
	_, _ = io.WriteString(stdout, text)
	return 0
}

// inCycle reports whether the cycle selects the person
// (data.mgmt.derive.cycle_people), so an explicit --cycle never silently
// falls back to the rolling window.
func inCycle(derive map[string]any, cycle, person string) bool {
	byCycle, _ := derive["cycle_people"].(map[string]any)
	people, _ := byCycle[cycle].([]any)
	for _, p := range people {
		if p == person {
			return true
		}
	}
	return false
}

// parseInterspersed parses fl's flags wherever they appear in args and
// returns the positional arguments in order.
func parseInterspersed(fl *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fl.Parse(args); err != nil {
			return nil, err
		}
		if fl.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fl.Arg(0))
		args = fl.Args()[1:]
	}
}

// resolvePerson matches arg against Person names and aliases,
// case-insensitively, and returns the Person's metadata.name.
func resolvePerson(f facts.Facts, arg string) (string, bool) {
	names := make([]string, 0, len(f.Resources["Person"]))
	for name := range f.Resources["Person"] {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if strings.EqualFold(name, arg) {
			return name, true
		}
		obj, _ := f.Resources["Person"][name].(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		aliases, _ := spec["aliases"].([]any)
		for _, a := range aliases {
			if s, ok := a.(string); ok && strings.EqualFold(s, arg) {
				return name, true
			}
		}
	}
	return "", false
}

// resolveCompetency resolves arg through the person's framework (tokens,
// full ids, unambiguous suffixes) as the extractor does. When it doesn't
// resolve, it returns the closest token, id or suffix as a suggestion.
func resolveCompetency(f facts.Facts, person, arg string) (id, suggest string, ok bool) {
	var resources []load.Resource
	for _, kind := range sortedKeys(f.Resources) {
		for _, name := range sortedKeys(f.Resources[kind]) {
			obj, _ := f.Resources[kind][name].(map[string]any)
			resources = append(resources, load.Resource{Kind: kind, Name: name, Object: obj})
		}
	}
	idx := extract.NewIndex(resources)
	if id, res := idx.Competency(person, arg); res == extract.Resolved {
		return id, "", true
	}
	return "", extract.Suggest(arg, idx.TokenNames(person)), false
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// reportData is the template data (Ruling R4): view is report[name] indexed
// by keys (the person, then for explain the competency). When strict, a view
// missing for those keys is an error rather than an empty report; otherwise
// it is null.
func reportData(res *engine.Result, name string, keys []string, opts engine.Options, strict bool) (map[string]any, error) {
	factsDoc, err := plainJSON(res.Facts)
	if err != nil {
		return nil, err
	}
	view := res.Report[name]
	for _, k := range keys {
		m, _ := view.(map[string]any)
		if view = m[k]; view == nil {
			if !strict {
				break
			}
			return nil, fmt.Errorf("no %s report for %q", name, strings.Join(keys, " "))
		}
	}
	person := ""
	if len(keys) > 0 {
		person = keys[0]
	}
	return map[string]any{
		"now":    opts.Now,
		"person": nullable(person),
		"cycle":  nullable(opts.Cycle),
		"view":   view,
		"facts":  factsDoc,
		"derive": res.Derive,
	}, nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// plainJSON turns v into maps, slices and json.Numbers, as templates see
// policy results.
func plainJSON(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var out any
	err = dec.Decode(&out)
	return out, err
}

// lintText renders findings one per line, errors and warnings interleaved in
// file, line, id, person order, followed by a summary line. A finding with no
// file (a workspace policy emitting a plain string) prints without a
// location, as "warning <pkg>: <message>" (Ruling R17: the one accepted
// exception to every finding carrying file:line); one with a file but no line
// prints "<file>: ...".
func lintText(errs, warns []facts.Finding) string {
	if len(errs)+len(warns) == 0 {
		return "no problems\n"
	}
	all := append(append([]facts.Finding{}, errs...), warns...)
	engine.SortFindings(all)
	var b strings.Builder
	for _, f := range all {
		switch {
		case f.File == "":
		case f.Line == 0:
			fmt.Fprintf(&b, "%s: ", f.File)
		default:
			fmt.Fprintf(&b, "%s:%d: ", f.File, f.Line)
		}
		fmt.Fprintf(&b, "%s %s", f.Severity, f.ID)
		if f.Person != "" {
			fmt.Fprintf(&b, " [%s]", f.Person)
		}
		fmt.Fprintf(&b, ": %s", f.Message)
		if f.Suggest != "" {
			fmt.Fprintf(&b, " (did you mean %q?)", f.Suggest)
		}
		b.WriteByte('\n')
	}
	b.WriteString(summary(len(errs), len(warns)) + "\n")
	return b.String()
}

func summary(errs, warns int) string {
	return plural(errs, "error") + ", " + plural(warns, "warning")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// writeJSON writes v as 2-space indented JSON with sorted keys and a
// trailing newline, without HTML escaping.
func writeJSON(w io.Writer, v any) error {
	generic, err := plainJSON(v)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(generic)
}
