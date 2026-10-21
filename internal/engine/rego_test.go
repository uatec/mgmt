package engine

import (
	"context"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/tester"

	mgmt "github.com/uatec/mgmt"
	"github.com/uatec/mgmt/internal/facts"
)

// TestRegoUnit runs every policy/core/*_test.rego through OPA's tester.
func TestRegoUnit(t *testing.T) {
	mods := map[string]*ast.Module{}
	err := fs.WalkDir(mgmt.CorePolicy, "policy/core", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".rego") {
			return err
		}
		src, err := fs.ReadFile(mgmt.CorePolicy, p)
		if err != nil {
			return err
		}
		m, err := ast.ParseModuleWithOpts(p, string(src), ast.ParserOptions{RegoVersion: ast.RegoV1})
		if err != nil {
			return err
		}
		mods[p] = m
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ch, err := tester.NewRunner().RaiseBuiltinErrors(true).Run(context.Background(), mods)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for r := range ch {
		n++
		if r.Error != nil {
			t.Errorf("%s.%s: error: %v", r.Package, r.Name, r.Error)
		} else if r.Fail {
			t.Errorf("%s.%s: FAIL", r.Package, r.Name)
		}
	}
	if n == 0 {
		t.Fatal("no Rego tests ran")
	}
}

func TestCapabilitiesBlockNetwork(t *testing.T) {
	mod := `package mgmt.policy.net
deny contains x if { x := http.send({"method": "GET", "url": "http://example.com"}) }`
	_, _, _, _, err := Evaluate(facts.Facts{}, map[string]string{"net.rego": mod})
	if err == nil || !strings.Contains(err.Error(), "http.send") {
		t.Fatalf("err = %v, want compile error mentioning http.send", err)
	}
	for _, name := range []string{"net.lookup_ip_addr", "opa.runtime", "time.now_ns", "rand.intn", "uuid.rfc4122"} {
		for _, b := range capabilities().Builtins {
			if b.Name == name {
				t.Errorf("builtin %s still allowed", name)
			}
		}
	}
}

func TestEvaluateDecodesFindings(t *testing.T) {
	mod := `package mgmt.policy.x
deny contains {"id": "b", "person": "", "file": "f", "line": 2, "msg": "from msg"}
deny contains {"id": "a", "person": "sam", "file": "f", "line": 2, "message": "m", "suggest": "s"}
warn contains {"id": "w", "person": "jo", "file": "e", "line": 9, "message": "w"}`
	derive, report, deny, warn, err := Evaluate(facts.Facts{}, map[string]string{"x.rego": mod})
	if err != nil {
		t.Fatal(err)
	}
	if derive == nil || report == nil {
		t.Errorf("derive/report should be empty maps, got %v %v", derive, report)
	}
	wantDeny := []facts.Finding{
		{Severity: "error", ID: "a", Person: "sam", File: "f", Line: 2, Message: "m", Suggest: "s"},
		{Severity: "error", ID: "b", File: "f", Line: 2, Message: "from msg"},
	}
	if !reflect.DeepEqual(deny, wantDeny) {
		t.Errorf("deny = %+v\nwant %+v", deny, wantDeny)
	}
	wantWarn := []facts.Finding{{Severity: "warning", ID: "w", Person: "jo", File: "e", Line: 9, Message: "w"}}
	if !reflect.DeepEqual(warn, wantWarn) {
		t.Errorf("warn = %+v", warn)
	}
}

func TestEvaluateRuntimeErrorIsStrict(t *testing.T) {
	mod := `package mgmt.policy.x
deny contains {"id": "a", "person": "", "file": "f", "line": n, "message": "m"} if { n := to_number("x") }`
	if _, _, _, _, err := Evaluate(facts.Facts{}, map[string]string{"x.rego": mod}); err == nil {
		t.Fatal("want builtin error")
	}
}

func TestLoadModulesSkipsTests(t *testing.T) {
	mods, err := LoadModules(mgmt.CorePolicy, "../../testdata/does-not-exist/policy")
	if err != nil {
		t.Fatal(err)
	}
	for n := range mods {
		if strings.HasSuffix(n, "_test.rego") || !strings.HasSuffix(n, ".rego") {
			t.Errorf("loaded %s", n)
		}
	}
	if len(mods) == 0 {
		t.Fatal("no core modules loaded")
	}
}

func TestSuppressIgnored(t *testing.T) {
	warn := []facts.Finding{
		{ID: "a", File: "n.md", Line: 3},
		{ID: "b", File: "n.md", Line: 3},
		{ID: "a", File: "n.md", Line: 4},
	}
	ign := []facts.Ignore{{File: "n.md", Line: 3, IDs: []string{"a"}}}
	got := suppress(warn, ign)
	want := []facts.Finding{{ID: "b", File: "n.md", Line: 3}, {ID: "a", File: "n.md", Line: 4}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("suppress = %+v", got)
	}
}

func TestRunSpecExample(t *testing.T) {
	res, err := Run("../../testdata/spec-example", Options{Now: "2026-10-21"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("errors = %+v", res.Errors)
	}
	f := res.Facts
	if f.Now != "2026-10-21" || len(f.Evidence) != 3 || len(f.Resources["Person"]) != 3 {
		t.Errorf("facts not filled: now=%q evidence=%d persons=%d", f.Now, len(f.Evidence), len(f.Resources["Person"]))
	}
	if s := f.Sources["Person/priya"]; s.File != "resources/people.yaml" || s.Line != 14 {
		t.Errorf("source = %+v", s)
	}
}

func TestRunSchemaErrorsSkipExtraction(t *testing.T) {
	res, err := Run("../../testdata/schema-errors", Options{Now: "2026-10-21"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) == 0 {
		t.Fatal("want schema errors")
	}
	for _, e := range res.Errors {
		if e.ID != "schema-invalid" {
			t.Errorf("unexpected %+v", e)
		}
	}
	if len(res.Facts.Diagnostics) != len(res.Errors) {
		t.Errorf("diagnostics %d != errors %d", len(res.Facts.Diagnostics), len(res.Errors))
	}
}
