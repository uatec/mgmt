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
	"github.com/uatec/mgmt/internal/facts"
	"github.com/uatec/mgmt/internal/render"
)

const usage = `usage: mgmt [--now YYYY-MM-DD] [--out FILE] <command> [args]

commands:
  facts                                print the facts document as JSON
  lint [--strict] [--format text|json] print errors and warnings
  prep <person>                        print the 1:1 prep sheet
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
	if _, err := time.Parse("2006-01-02", *now); err != nil {
		return usageError(stderr, fmt.Sprintf("--now %q is not a YYYY-MM-DD date", *now))
	}
	rest := global.Args()
	if len(rest) == 0 {
		return usageError(stderr, "no command given")
	}
	var buf bytes.Buffer
	var code int
	switch rest[0] {
	case "facts":
		code = runFacts(rest[1:], root, *now, &buf, stderr)
	case "lint":
		code = runLint(rest[1:], root, *now, &buf, stderr)
	case "prep":
		code = runPrep(rest[1:], root, *now, &buf, stderr)
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
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return 2
	}
	return code
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

func runPrep(args []string, root, now string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		return usageError(stderr, "prep takes exactly one person")
	}
	return runReport("prep", args[0], root, engine.Options{Now: now}, stdout, stderr)
}

// runReport runs the pipeline, refuses on errors (the lint text goes to
// stderr), resolves the person argument when given, and renders the report.
func runReport(name, personArg, root string, opts engine.Options, stdout, stderr io.Writer) int {
	res, err := engine.Run(root, opts)
	if err != nil {
		fmt.Fprintf(stderr, "mgmt: %v\n", err)
		return 2
	}
	if len(res.Errors) > 0 {
		_, _ = io.WriteString(stderr, lintText(res.Errors, res.Warnings))
		return 2
	}
	person := ""
	if personArg != "" {
		var ok bool
		if person, ok = resolvePerson(res.Facts, personArg); !ok {
			fmt.Fprintf(stderr, "mgmt: unknown person %q\n", personArg)
			return 2
		}
	}
	data, err := reportData(res, name, person, opts)
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

// reportData is the template data (Ruling R4): view is report[name][person]
// when a person is given, else report[name]; null when undefined.
func reportData(res *engine.Result, name, person string, opts engine.Options) (map[string]any, error) {
	factsDoc, err := plainJSON(res.Facts)
	if err != nil {
		return nil, err
	}
	var view any = res.Report[name]
	if person != "" {
		byPerson, _ := view.(map[string]any)
		view = byPerson[person]
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
// file, line, id, person order, followed by a summary line.
func lintText(errs, warns []facts.Finding) string {
	if len(errs)+len(warns) == 0 {
		return "no problems\n"
	}
	all := append(append([]facts.Finding{}, errs...), warns...)
	engine.SortFindings(all)
	var b strings.Builder
	for _, f := range all {
		fmt.Fprintf(&b, "%s:%d: %s %s", f.File, f.Line, f.Severity, f.ID)
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
