package engine

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/tester"

	mgmt "github.com/uatec/mgmt"
	"github.com/uatec/mgmt/internal/facts"
)

// TestRegoUnit runs every policy/core/*_test.rego, and every fixture
// workspace's testdata/*/policy/**/*_test.rego, through OPA's tester with
// the core modules and all fixture workspace modules loaded together.
func TestRegoUnit(t *testing.T) {
	mods := map[string]*ast.Module{}
	add := func(fsys fs.FS, root, prefix string) error {
		return fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".rego") {
				return err
			}
			src, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}
			m, err := ast.ParseModuleWithOpts(prefix+p, string(src), ast.ParserOptions{RegoVersion: ast.RegoV1})
			if err != nil {
				return err
			}
			mods[prefix+p] = m
			return nil
		})
	}
	if err := add(mgmt.CorePolicy, "policy/core", ""); err != nil {
		t.Fatal(err)
	}
	wsDirs, err := filepath.Glob("../../testdata/*/policy")
	if err != nil {
		t.Fatal(err)
	}
	wsTests := 0
	for _, dir := range wsDirs {
		if err := add(os.DirFS(dir), ".", filepath.ToSlash(dir)+"/"); err != nil {
			t.Fatal(err)
		}
	}
	for name := range mods {
		if strings.HasPrefix(name, "../") && strings.HasSuffix(name, "_test.rego") {
			wsTests++
		}
	}
	if wsTests == 0 {
		t.Fatal("no workspace *_test.rego found under testdata/*/policy")
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

// TestEvaluateDecodesConftestStrings: Conftest-style rules emitting plain
// strings, and objects missing id or message, become findings attributed to
// their package rather than blank lines.
func TestEvaluateDecodesConftestStrings(t *testing.T) {
	mod := `package mgmt.policy.acme
deny contains "OKRs must name a cycle"
warn contains "check the OKRs"
warn contains {"person": "sam", "file": "f", "line": 3, "message": "no id"}
warn contains {"id": "odd", "file": "f", "line": 4}`
	_, _, deny, warn, err := Evaluate(facts.Facts{}, map[string]string{"acme.rego": mod})
	if err != nil {
		t.Fatal(err)
	}
	wantDeny := []facts.Finding{{Severity: "error", ID: "acme", Message: "OKRs must name a cycle"}}
	if !reflect.DeepEqual(deny, wantDeny) {
		t.Errorf("deny = %+v", deny)
	}
	wantWarn := []facts.Finding{
		{Severity: "warning", ID: "acme", Message: "check the OKRs"},
		{Severity: "warning", ID: "acme", Person: "sam", File: "f", Line: 3, Message: "no id"},
		{Severity: "warning", ID: "odd", File: "f", Line: 4, Message: `{"file":"f","id":"odd","line":4}`},
	}
	if !reflect.DeepEqual(warn, wantWarn) {
		t.Errorf("warn = %+v\nwant %+v", warn, wantWarn)
	}
}

func TestLoadModulesStatError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// Stat of a path below a regular file fails with ENOTDIR, not ENOENT.
	if _, err := LoadModules(mgmt.CorePolicy, filepath.Join(file, "policy")); err == nil {
		t.Fatal("want an I/O error for an unreadable policy dir")
	}
	if _, err := LoadModules(mgmt.CorePolicy, file); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("policy is a file: err = %v", err)
	}
}
