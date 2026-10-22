package engine

import (
	"errors"
	"path/filepath"
	"strconv"

	"github.com/bmatcuk/doublestar/v4"

	mgmt "github.com/uatec/mgmt"
	"github.com/uatec/mgmt/internal/extract"
	"github.com/uatec/mgmt/internal/facts"
	"github.com/uatec/mgmt/internal/load"
)

// Options are the run-wide settings passed into facts.
type Options struct {
	Now   string // YYYY-MM-DD
	Cycle string // optional ReviewCycle name
}

// Result is everything a command needs from one pipeline run.
type Result struct {
	Facts            facts.Facts
	Derive, Report   map[string]any  // data.mgmt.derive and data.mgmt.report
	Errors, Warnings []facts.Finding // sorted by file, line, id, person
}

// Run loads, validates, extracts and evaluates the workspace at root. The
// error is reserved for I/O failures; data problems become Errors.
func Run(root string, opts Options) (*Result, error) {
	res := &Result{
		Facts:    facts.Facts{Now: opts.Now, Options: facts.Options{Cycle: opts.Cycle}},
		Derive:   map[string]any{},
		Report:   map[string]any{},
		Errors:   []facts.Finding{},
		Warnings: []facts.Finding{},
	}
	resources, finds := load.LoadResources(root)
	finds = append(finds, Validate(resources, mgmt.CoreSchema, filepath.Join(root, "schema"))...)
	schemaOK := len(finds) == 0
	if schemaOK {
		var err error
		if finds, err = extractFacts(root, resources, &res.Facts); err != nil {
			return nil, err
		}
	}
	res.Facts.Diagnostics = make([]facts.Diagnostic, 0, len(finds))
	for _, f := range finds {
		res.Facts.Diagnostics = append(res.Facts.Diagnostics, facts.Diagnostic{
			Severity: f.Severity, Code: f.ID, File: f.File, Line: f.Line, Message: f.Message, Suggest: f.Suggest,
		})
	}
	res.Errors = append(res.Errors, finds...)
	if schemaOK {
		modules, err := LoadModules(mgmt.CorePolicy, filepath.Join(root, "policy"))
		if err != nil {
			return nil, err
		}
		derive, report, deny, warn, err := Evaluate(res.Facts, modules)
		if err != nil {
			return nil, err
		}
		res.Derive, res.Report = derive, report
		res.Errors = append(res.Errors, deny...)
		res.Warnings = suppress(warn, res.Facts.Ignores)
	}
	SortFindings(res.Errors)
	return res, nil
}

// extractFacts fills f with resources, sources, effective frameworks and the records extracted
// from the Workspace's notes, and returns the extraction diagnostics as
// findings (Run copies them back into f.Diagnostics).
func extractFacts(root string, resources []load.Resource, f *facts.Facts) ([]facts.Finding, error) {
	f.Resources = map[string]map[string]any{}
	f.Sources = map[string]facts.Source{}
	var glob string
	var ws facts.Source
	for _, r := range resources {
		if f.Resources[r.Kind] == nil {
			f.Resources[r.Kind] = map[string]any{}
		}
		f.Resources[r.Kind][r.Name] = r.Object
		f.Sources[r.Kind+"/"+r.Name] = facts.Source{File: r.File, Line: r.Line}
		if r.Kind == "Workspace" {
			spec, _ := r.Object["spec"].(map[string]any)
			glob, _ = spec["notes"].(string)
			ws = facts.Source{File: r.File, Line: r.Line}
		}
	}
	f.Frameworks = extract.Frameworks(resources)
	notes, err := load.LoadNotes(root, glob)
	if errors.Is(err, doublestar.ErrBadPattern) {
		return []facts.Finding{{Severity: "error", ID: "schema-invalid", File: ws.File, Line: ws.Line,
			Message: "spec.notes: invalid glob " + strconv.Quote(glob)}}, nil
	}
	if err != nil {
		return nil, err
	}
	x := extract.Extract(notes, extract.NewIndex(resources))
	f.Evidence, f.Touches, f.Actions, f.Inputs = x.Evidence, x.Touches, x.Actions, x.Inputs
	f.Sections, f.Ignores = x.Sections, x.Ignores
	// "# mgmt:ignore" in a resource document anchors at the document's start,
	// where resource-anchored warnings point.
	for _, r := range resources {
		if len(r.Ignores) > 0 {
			f.Ignores = append(f.Ignores, facts.Ignore{File: r.File, Line: r.Line, IDs: r.Ignores})
		}
	}
	finds := make([]facts.Finding, 0, len(x.Diagnostics))
	for _, d := range x.Diagnostics {
		finds = append(finds, facts.Finding{Severity: d.Severity, ID: d.Code, File: d.File, Line: d.Line, Message: d.Message, Suggest: d.Suggest})
	}
	return finds, nil
}
