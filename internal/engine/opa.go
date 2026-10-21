package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"

	"github.com/uatec/mgmt/internal/facts"
)

// blockedBuiltins reach the network, the wall clock, randomness or the host.
var blockedBuiltins = map[string]bool{
	"http.send":          true,
	"net.lookup_ip_addr": true,
	"opa.runtime":        true,
	"time.now_ns":        true,
	"rand.intn":          true,
	"uuid.rfc4122":       true,
}

// capabilities is this OPA version's builtins minus blockedBuiltins.
func capabilities() *ast.Capabilities {
	caps := ast.CapabilitiesForThisVersion()
	kept := make([]*ast.Builtin, 0, len(caps.Builtins))
	for _, b := range caps.Builtins {
		if !blockedBuiltins[b.Name] {
			kept = append(kept, b)
		}
	}
	caps.Builtins = kept
	return caps
}

// LoadModules reads every *.rego (excluding *_test.rego) under policy/core in
// core and, when it exists, under wsPolicyDir. Keys are slash paths:
// "core/<path>" for embedded modules and "policy/<path>" for workspace ones.
func LoadModules(core fs.FS, wsPolicyDir string) (map[string]string, error) {
	mods := map[string]string{}
	read := func(fsys fs.FS, dir, prefix string) error {
		return fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".rego") || strings.HasSuffix(p, "_test.rego") {
				return err
			}
			b, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}
			mods[path.Join(prefix, p)] = string(b)
			return nil
		})
	}
	if err := read(core, "policy/core", "core"); err != nil {
		return nil, err
	}
	st, err := os.Stat(wsPolicyDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return mods, nil // workspace policies are optional
	case err != nil:
		return nil, err
	case !st.IsDir():
		return nil, fmt.Errorf("%s is not a directory", wsPolicyDir)
	}
	if err := read(os.DirFS(wsPolicyDir), ".", "policy"); err != nil {
		return nil, err
	}
	return mods, nil
}

// Evaluate runs data.mgmt over the facts document. It returns
// data.mgmt.derive and data.mgmt.report (empty maps when undefined) and the
// deny and warn sets of every data.mgmt.policy.<pkg>, each sorted.
//
// Policy packages are one level below mgmt.policy, as the spec defines
// (`mgmt.policy.<name>`). Rules in a nested package such as
// mgmt.policy.ensek.okr appear at data.mgmt.policy.ensek.okr and are not
// read: put them in package mgmt.policy.ensek instead.
func Evaluate(f facts.Facts, modules map[string]string) (derive, report map[string]any, deny, warn []facts.Finding, err error) {
	input, err := toInput(f)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	opts := []func(*rego.Rego){
		rego.Query("data.mgmt"),
		rego.Capabilities(capabilities()),
		rego.StrictBuiltinErrors(true),
	}
	names := make([]string, 0, len(modules))
	for n := range modules {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		opts = append(opts, rego.Module(n, modules[n]))
	}
	ctx := context.Background()
	pq, err := rego.New(opts...).PrepareForEval(ctx)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("policy: %w", err)
	}
	rs, err := pq.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("policy: %w", err)
	}
	var root map[string]any
	if len(rs) > 0 && len(rs[0].Expressions) > 0 {
		root, _ = rs[0].Expressions[0].Value.(map[string]any)
	}
	derive = asMap(root["derive"])
	report = asMap(root["report"])
	pkgs := asMap(root["policy"])
	for _, pkg := range sortedKeys(pkgs) {
		p := asMap(pkgs[pkg])
		deny = append(deny, decodeFindings(p["deny"], "error", pkg)...)
		warn = append(warn, decodeFindings(p["warn"], "warning", pkg)...)
	}
	SortFindings(deny)
	SortFindings(warn)
	return derive, report, deny, warn, nil
}

// toInput turns facts into plain JSON values, keeping numbers exact.
func toInput(f facts.Facts) (any, error) {
	b, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	err = dec.Decode(&v)
	return v, err
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// decodeFindings reads a deny or warn set of package pkg. Each entry is
// normally {id, person, file, line, message} (message falls back to msg).
// Conftest-style entries are accepted too: a plain string becomes a finding
// with id = pkg and that string as message and no location (lint prints it
// as "error <pkg>: <message>"; Ruling R17 accepts this exception to the
// file:line rule, since a plain string has no location to give). An object without id gets id = pkg, and one
// without message or msg gets its own compact JSON as message, so no
// finding prints blank.
func decodeFindings(v any, severity, pkg string) []facts.Finding {
	list, _ := v.([]any)
	out := make([]facts.Finding, 0, len(list))
	for _, e := range list {
		m, isObj := e.(map[string]any)
		if !isObj {
			msg, ok := e.(string)
			if !ok {
				msg = compactJSON(e)
			}
			out = append(out, facts.Finding{Severity: severity, ID: pkg, Message: msg})
			continue
		}
		f := facts.Finding{
			Severity: severity,
			ID:       str(m["id"]),
			Person:   str(m["person"]),
			File:     str(m["file"]),
			Line:     num(m["line"]),
			Message:  str(m["message"]),
			Suggest:  str(m["suggest"]),
		}
		if f.Message == "" {
			f.Message = str(m["msg"])
		}
		if f.Message == "" {
			f.Message = compactJSON(e)
		}
		if f.ID == "" {
			f.ID = pkg
		}
		out = append(out, f)
	}
	return out
}

// compactJSON renders a policy value for a message; keys come out sorted.
func compactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) int {
	switch n := v.(type) {
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// SortFindings orders by file, line, id, person, then message.
func SortFindings(fs []facts.Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Person != b.Person {
			return a.Person < b.Person
		}
		return a.Message < b.Message
	})
}

// suppress drops warnings whose file and line carry an ignore for their id.
func suppress(warn []facts.Finding, ignores []facts.Ignore) []facts.Finding {
	type key struct {
		file string
		line int
		id   string
	}
	ignored := map[key]bool{}
	for _, ig := range ignores {
		for _, id := range ig.IDs {
			ignored[key{ig.File, ig.Line, id}] = true
		}
	}
	out := []facts.Finding{}
	for _, w := range warn {
		if !ignored[key{w.File, w.Line, w.ID}] {
			out = append(out, w)
		}
	}
	return out
}
