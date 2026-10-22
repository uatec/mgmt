package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	mgmt "github.com/uatec/mgmt"
	"github.com/uatec/mgmt/internal/engine"
	"github.com/uatec/mgmt/internal/facts"
	"github.com/uatec/mgmt/internal/render"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestGolden runs the CLI against fixture workspaces at a fixed --now.
//
// Each row's stdout is compared with testdata/<fixture>/golden/<golden>;
// rows with no golden require stdout to be empty.
// Rows that set stderr compare stderr with testdata/<fixture>/golden/<stderr>;
// rows that leave it empty require stderr to be empty. -update rewrites both.
func TestGolden(t *testing.T) {
	rows := []struct {
		fixture string
		args    []string
		exit    int
		golden  string
		stderr  string
	}{
		{"spec-example", []string{"facts"}, 0, "facts.json", ""},
		{"spec-example", []string{"lint"}, 0, "lint.txt", ""},
		{"errors", []string{"lint"}, 2, "lint.txt", ""},
		{"errors", []string{"lint", "--format", "json"}, 2, "lint.json", ""},
		{"schema-errors", []string{"lint"}, 2, "lint.txt", ""},
		{"warnings", []string{"lint", "--strict"}, 1, "lint.txt", ""},
		{"team", []string{"lint"}, 0, "lint.txt", ""},
		{"spec-example", []string{"lint", "--format", "flat"}, 0, "lint-flat.txt", ""},
		{"errors", []string{"lint", "--format", "flat"}, 2, "lint-flat.txt", ""},
		{"schema-errors", []string{"lint", "--format=flat"}, 2, "lint-flat.txt", ""},
		{"warnings", []string{"lint", "--strict", "--format", "flat"}, 1, "lint-flat.txt", ""},
		{"team", []string{"lint", "--format", "flat"}, 0, "lint-flat.txt", ""},
		{"ensek", []string{"lint", "--format", "flat"}, 0, "lint-flat.txt", ""},
		{"ensek", []string{"--now", "2026-11-01", "lint", "--format", "flat"}, 0, "lint-2026-11-01-flat.txt", ""},
		{"team", []string{"prep", "sam"}, 0, "prep-sam.md", ""},
		{"team", []string{"prep", "Samuel"}, 0, "prep-sam.md", ""},
		{"team", []string{"prep", "PRIYA"}, 0, "prep-priya.md", ""},
		{"team", []string{"prep", "jo"}, 0, "prep-jo.md", ""},
		{"team", []string{"prep", "nobody"}, 2, "", "prep-nobody.stderr"},
		{"errors", []string{"prep", "sam"}, 2, "", "prep-refused.stderr"},
		{"team", []string{"prep", "sam", "--cycle", "retro-2026-h1"}, 0, "prep-sam-retro.md", ""},
		{"team", []string{"prep", "jo", "--cycle", "retro-2026-h1"}, 2, "", "prep-jo-not-in-cycle.stderr"},
		{"team", []string{"prep", "alex"}, 0, "prep-alex.md", ""},
		{"team", []string{"review", "sam"}, 0, "review-sam.md", ""},
		{"team", []string{"review", "sam", "--cycle", "retro-2026-h1"}, 0, "review-sam-retro.md", ""},
		{"team", []string{"review", "--cycle", "retro-2026-h1", "jo"}, 2, "", "review-jo-not-in-cycle.stderr"},
		{"team", []string{"explain", "jo", "planning", "--cycle", "retro-2026-h1"}, 2, "", "explain-jo-not-in-cycle.stderr"},
		{"team", []string{"review", "alex"}, 0, "review-alex.md", ""},
		{"team", []string{"review", "jo"}, 0, "review-jo.md", ""},
		{"team", []string{"review", "Priya"}, 0, "review-priya.md", ""},
		{"team", []string{"review", "sam", "--cycle", "nope"}, 2, "", "review-nope.stderr"},
		{"team", []string{"explain", "sam", "planning"}, 0, "explain-sam-planning.md", ""},
		{"team", []string{"explain", "sam", "delivery.ownership"}, 0, "explain-sam-ownership.md", ""},
		{"team", []string{"explain", "jo", "PLANNING"}, 0, "explain-jo-planning.md", ""},
		{"team", []string{"explain", "priya", "crossteam"}, 0, "explain-priya-crossteam.md", ""},
		{"team", []string{"explain", "sam", "plannig"}, 2, "", "explain-plannig.stderr"},
		{"team", []string{"explain", "sam", "zzzzzz"}, 2, "", "explain-zzzzzz.stderr"},
		{"team", []string{"status"}, 0, "status.md", ""},
		{"team", []string{"status", "--cycle", "retro-2026-h1"}, 0, "status-retro.md", ""},
		{"team", []string{"status", "--cycle", "nope"}, 2, "", "status-nope.stderr"},
		{"spec-example", []string{"status"}, 0, "status.md", ""},
		{"spec-example", []string{"--now", "2027-01-15", "status"}, 0, "status-no-cycle.md", ""},
		{"team", []string{"status", "--format", "yaml"}, 0, "status-yaml.md", ""},
		{"team", []string{"status", "--format=markdown"}, 0, "status.md", ""},
		{"spec-example", []string{"status", "--format", "yaml"}, 0, "status-yaml.md", ""},
		{"spec-example", []string{"--now", "2027-01-15", "status", "--format", "yaml"}, 0, "status-no-cycle.md", ""},
		{"ensek", []string{"lint"}, 0, "lint.txt", ""},
		{"ensek", []string{"--now", "2026-11-01", "lint"}, 0, "lint-2026-11-01.txt", ""},
		{"ensek", []string{"prep", "sam"}, 0, "prep-sam.md", ""},
		{"ensek", []string{"report", "okrs", "sam"}, 0, "report-okrs-sam.md", ""},
		{"ensek", []string{"report", "okrs"}, 0, "report-okrs.md", ""},
		{"ensek", []string{"report", "okrs", "priya"}, 0, "report-okrs-priya.md", ""},
		{"ensek", []string{"report", "okrs", "nobody"}, 2, "", "report-okrs-nobody.stderr"},
		{"ensek", []string{"report", "okrs", "jo", "--cycle", "retro-2026-h1"}, 2, "", "report-okrs-jo-not-in-cycle.stderr"},
		{"team", []string{"report", "okrs"}, 2, "", "report-okrs.stderr"},
	}
	for _, r := range rows {
		t.Run(r.fixture+"/"+strings.Join(r.args, "_"), func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", r.fixture)
			var stdout, stderr bytes.Buffer
			code := Run(append([]string{"--now", "2026-10-21"}, r.args...), root, &stdout, &stderr)
			if code != r.exit {
				t.Errorf("exit = %d, want %d\nstderr: %s", code, r.exit, stderr.String())
			}
			if r.golden != "" {
				compare(t, filepath.Join(root, "golden", r.golden), stdout.Bytes())
			} else if stdout.Len() != 0 {
				t.Errorf("unexpected stdout: %s", stdout.String())
			}
			if r.stderr != "" {
				compare(t, filepath.Join(root, "golden", r.stderr), stderr.Bytes())
			} else if stderr.Len() != 0 {
				t.Errorf("unexpected stderr: %s", stderr.String())
			}
		})
	}
}

func compare(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

func TestLintStrictAndOut(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join("..", "..", "testdata", "spec-example")
	out := filepath.Join(dir, "lint.txt")
	var stdout, stderr bytes.Buffer
	// spec-example has warnings only: --strict turns them into exit 1.
	if code := Run([]string{"--now", "2026-10-21", "--out", out, "lint", "--strict"}, root, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, stderr %s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty with --out, got %q", stdout.String())
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	compare(t, filepath.Join(root, "golden", "lint.txt"), b)
}

// TestOutUntouchedOnFailure: a command that fails without output must not
// create or truncate the --out file.
func TestOutUntouchedOnFailure(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		args    []string
	}{
		{"team", []string{"prep", "nobody"}}, // unknown person
		{"errors", []string{"prep", "sam"}},  // report refused on errors
		{"", []string{"facts"}},              // policy fails to compile
	} {
		root := filepath.Join("..", "..", "testdata", tc.fixture)
		if tc.fixture == "" {
			root = brokenPolicyWorkspace(t)
		}
		out := filepath.Join(t.TempDir(), "out.md")
		if err := os.WriteFile(out, []byte("keep me\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		args := append([]string{"--now", "2026-10-21", "--out", out}, tc.args...)
		if code := Run(args, root, &stdout, &stderr); code != 2 {
			t.Fatalf("%v: exit = %d, want 2", tc.args, code)
		}
		if b, _ := os.ReadFile(out); string(b) != "keep me\n" {
			t.Errorf("%v: --out file changed to %q", tc.args, b)
		}
		missing := filepath.Join(t.TempDir(), "new.md")
		Run(append([]string{"--now", "2026-10-21", "--out", missing}, tc.args...), root, &stdout, &stderr)
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Errorf("%v: --out file created on failure", tc.args)
		}
	}
}

// brokenPolicyWorkspace is spec-example plus a workspace policy that does
// not parse, so every command fails in the engine.
func brokenPolicyWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "testdata", "spec-example"))); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "policy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "policy", "bad.rego"), []byte("package mgmt.policy.bad\ndeny contains"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestPrepOut(t *testing.T) {
	out := filepath.Join(t.TempDir(), "prep.md")
	root := filepath.Join("..", "..", "testdata", "team")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--now", "2026-10-21", "--out", out, "prep", "sam"}, root, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr %s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty with --out, got %q", stdout.String())
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	compare(t, filepath.Join(root, "golden", "prep-sam.md"), b)
}

// TestGlobalFlagsAfterCommand: --now and --out are accepted after the
// subcommand too, in either "--flag value" or "--flag=value" form.
func TestGlobalFlagsAfterCommand(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		args    []string
		exit    int
		golden  string
	}{
		{"team", []string{"review", "sam", "--out", "OUT", "--now", "2026-10-21"}, 0, "review-sam.md"},
		{"team", []string{"prep", "--now=2026-10-21", "sam", "--out=OUT"}, 0, "prep-sam.md"},
		{"spec-example", []string{"lint", "--now", "2026-10-21", "--strict", "--out", "OUT"}, 1, "lint.txt"},
		{"spec-example", []string{"--now", "1999-01-01", "lint", "-now", "2026-10-21", "-out", "OUT"}, 0, "lint.txt"},
	} {
		root := filepath.Join("..", "..", "testdata", tc.fixture)
		out := filepath.Join(t.TempDir(), "out")
		args := make([]string, len(tc.args))
		for i, a := range tc.args {
			args[i] = strings.Replace(a, "OUT", out, 1)
		}
		var stdout, stderr bytes.Buffer
		if code := Run(args, root, &stdout, &stderr); code != tc.exit {
			t.Fatalf("%v: exit = %d, want %d; stderr %s", tc.args, code, tc.exit, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("%v: stdout should be empty with --out, got %q", tc.args, stdout.String())
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		compare(t, filepath.Join(root, "golden", tc.golden), b)
	}
}

// TestOutFileMode: reports carry personal data, so a new --out file is
// readable by its owner only.
func TestOutFileMode(t *testing.T) {
	out := filepath.Join(t.TempDir(), "prep.md")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--now", "2026-10-21", "--out", out, "prep", "sam"}, filepath.Join("..", "..", "testdata", "team"), &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr %s", code, stderr.String())
	}
	st, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want -rw-------", st.Mode().Perm())
	}
}

func TestUsageErrors(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "spec-example")
	for _, args := range [][]string{
		{},
		{"bogus"},
		{"--now", "21/10/2026", "lint"},
		{"lint", "--format", "xml"},
		{"lint", "--now"},
		{"lint", "--now", "21/10/2026"},
		{"prep", "sam", "--out"},
		{"lint", "extra"},
		{"prep"},
		{"prep", "sam", "extra"},
		{"prep", "sam", "--cycle"},
		{"review"},
		{"review", "sam", "extra"},
		{"explain", "sam"},
		{"explain", "sam", "planning", "extra"},
		{"status", "sam"},
		{"status", "--format", "json"},
		{"status", "--format"},
		{"prep", "sam", "--format", "yaml"},
		{"lint", "--format", "yaml"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, root, &stdout, &stderr); code != 2 {
			t.Errorf("%v: exit = %d, want 2", args, code)
		}
		if stderr.Len() == 0 {
			t.Errorf("%v: no usage message", args)
		}
	}
}

func TestPlural(t *testing.T) {
	cases := map[[2]int]string{
		{1, 1}: "1 error, 1 warning",
		{2, 0}: "2 errors, 0 warnings",
		{0, 3}: "0 errors, 3 warnings",
	}
	for in, want := range cases {
		if got := summary(in[0], in[1]); got != want {
			t.Errorf("summary%v = %q, want %q", in, got, want)
		}
	}
}

// TestReviewNeverRates: no review pack states a rating (the cycle's outcomes
// list below/meets/exceeds, and the fixture notes avoid those words).
func TestReviewNeverRates(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "*", "golden", "review-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 4 {
		t.Fatalf("found %d review goldens, want at least 4", len(files))
	}
	rating := regexp.MustCompile(`(?i)rating|below|meets|exceeds`)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if rating.MatchString(line) {
				t.Errorf("%s:%d states a rating: %q", f, i+1, line)
			}
		}
	}
}

// TestReportDataMissingView: a person argument whose view is undefined is an
// error, not an empty report.
func TestReportDataMissingView(t *testing.T) {
	res := &engine.Result{Report: map[string]any{"prep": map[string]any{"priya": map[string]any{}}}}
	if _, err := reportData(res, "prep", []string{"sam"}, engine.Options{Now: "2026-10-21"}, true); err == nil ||
		!strings.Contains(err.Error(), `no prep report for "sam"`) {
		t.Errorf("err = %v", err)
	}
	if _, err := reportData(res, "prep", []string{"priya"}, engine.Options{Now: "2026-10-21"}, true); err != nil {
		t.Errorf("priya: %v", err)
	}
}

// TestPrepTemplateBranches covers prep.tmpl branches no team golden reaches.
func TestPrepTemplateBranches(t *testing.T) {
	view := map[string]any{
		"person":      map[string]any{"displayName": "Sam Example"},
		"window":      map[string]any{"from": "2026-07-23", "to": "2026-10-21", "cycle": nil},
		"actions":     map[string]any{"mine": []any{}, "theirs": []any{}},
		"lastSection": nil,
		"since":       []any{},
		"toRaise":     []any{},
		"gaps": []any{map[string]any{
			"competency": "collab.planning", "title": "Plans", "status": "disproven",
			"addressedBy": []any{
				map[string]any{"file": "notes/a.md", "line": 5, "text": "x"},
				map[string]any{"file": "notes/b.md", "line": 6, "text": "y"},
			},
		}},
		"focus": nil,
		"cycle": map[string]any{"name": "retro", "nextMilestone": nil, "missing": []any{}},
	}
	got, err := render.Render("prep", map[string]any{"now": "2026-10-21", "view": view}, mgmt.CoreReports, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Window: 2026-07-23 to 2026-10-21 (rolling)\n",
		"## To raise\n- none\n",
		"## Since last 1:1 (none)\n- none\n",
		"addressed by notes/a.md:5, notes/b.md:6\n",
		"## Suggested focus\n- none\n",
		"- Next milestone: none\n- Missing: none\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

// TestExplainCountingRule covers the counting rule line, which no team
// cycle uses.
func TestExplainCountingRule(t *testing.T) {
	view := map[string]any{
		"person":     map[string]any{"name": "sam", "framework": "eng", "level": "IC3"},
		"competency": map[string]any{"id": "collab.planning", "title": "Plans"},
		"descriptor": nil,
		"required":   false, "status": "proven", "rule": "counting", "positives": 2, "negatives": 1,
		"settings": map[string]any{"minPerCompetency": 2, "negativeOverrides": false},
		"window":   map[string]any{"from": "2026-07-23", "to": "2026-10-21", "cycle": nil},
		"considered": []any{map[string]any{
			"date": "2026-09-01", "polarity": -1, "strength": 1, "tag": "-", "text": "t", "file": "notes/a.md", "line": 3, "decides": false,
		}},
		"outside": []any{},
	}
	got, err := render.Render("explain", map[string]any{"view": view}, mgmt.CoreReports, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Required: no (eng IC3)\nBar (IC3): not required at IC3\n",
		"Rule: counting: proven when positives exceed negatives and reach 2 (2 positive, 1 negative)\n",
		"Window: 2026-07-23 to 2026-10-21 (rolling)\n",
		"- 2026-09-01 - · t (notes/a.md:3)\n\n## Outside window\n- none\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

func TestStatusRejectsArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := filepath.Join("..", "..", "testdata", "team")
	if code := Run([]string{"status", "sam"}, root, &stdout, &stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.HasPrefix(stderr.String(), "mgmt: status takes no arguments\n") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// TestEnsekGoldenFacts pins the brief's facts in the ensek goldens, so an
// -update cannot silently accept the wrong output.
func TestEnsekGoldenFacts(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ensek", "golden", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	okr := regexp.MustCompile(`(?m)^.*okr-no-evidence.*$`)
	if got := okr.FindAllString(read("lint-flat.txt"), -1); len(got) != 0 {
		t.Errorf("lint at 2026-10-21 has okr-no-evidence: %q", got)
	}
	got := okr.FindAllString(read("lint-2026-11-01-flat.txt"), -1)
	if len(got) != 1 || !strings.HasPrefix(got[0], "resources/okrs.yaml:1: warning okr-no-evidence [sam]: ") ||
		!strings.Contains(got[0], "scope.crossteam") {
		t.Errorf("lint at 2026-11-01: okr-no-evidence lines = %q, want one for scope.crossteam", got)
	}
	if first, _, _ := strings.Cut(read("prep-sam.md"), "\n"); first != "# Ensek 1:1 prep" {
		t.Errorf("prep first line = %q", first)
	}
	for _, want := range []string{"- scope.crossteam: unproven\n", "- delivery.ownership: proven\n"} {
		if !strings.Contains(read("report-okrs-sam.md"), want) {
			t.Errorf("report okrs sam lacks %q", want)
		}
	}
}

// TestExtensionsNeedNoRebuild edits a copy of the ensek workspace between
// runs of the same binary: workspace policies, kinds and templates are read
// at run time.
func TestExtensionsNeedNoRebuild(t *testing.T) {
	src := filepath.Join("..", "..", "testdata", "ensek")
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(src)); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, string, int) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := Run(append([]string{"--now", "2026-11-01"}, args...), root, &stdout, &stderr)
		return stdout.String(), stderr.String(), code
	}
	policyDir := filepath.Join(root, "policy", "ensek")
	rego := filepath.Join(policyDir, "okr.rego")
	orig, err := os.ReadFile(rego)
	if err != nil {
		t.Fatal(err)
	}

	if out, _, _ := run("lint"); !strings.Contains(out, "okr-no-evidence") {
		t.Fatalf("baseline lint lacks okr-no-evidence:\n%s", out)
	}

	if err := os.RemoveAll(policyDir); err != nil {
		t.Fatal(err)
	}
	if out, _, _ := run("lint"); strings.Contains(out, "okr-no-evidence") {
		t.Errorf("okr-no-evidence still reported without policy/ensek:\n%s", out)
	}

	if err := os.MkdirAll(policyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(orig), "which is unproven", "which still needs evidence", 1)
	if edited == string(orig) {
		t.Fatal("policy message not found in okr.rego")
	}
	if err := os.WriteFile(rego, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, _, _ := run("lint"); !strings.Contains(out, "links scope.crossteam, which still needs evidence") {
		t.Errorf("edited policy message not shown:\n%s", out)
	}

	// Kinds: tighten the OKRSet schema and the resource becomes invalid.
	cue := filepath.Join(root, "schema", "ensek", "okr.cue")
	b, err := os.ReadFile(cue)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cue, []byte(strings.Replace(string(b), "person: string", `person: "priya"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, _, code := run("lint", "--format", "flat"); code != 2 || !strings.Contains(out, "resources/okrs.yaml:1: error schema-invalid") {
		t.Errorf("tightened schema: exit %d\n%s", code, out)
	}
	if err := os.WriteFile(cue, b, 0o644); err != nil {
		t.Fatal(err)
	}

	// Templates: a new report is picked up immediately.
	if err := os.WriteFile(filepath.Join(root, "reports", "hello.tmpl"), []byte("hello {{.person}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, errOut, code := run("report", "hello", "Samuel"); code != 0 || out != "hello sam\n" {
		t.Errorf("report hello: exit %d out %q err %q", code, out, errOut)
	}
}

func TestReportUsage(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "ensek")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"report"}, "mgmt: report takes <name> [<person>]\n"},
		{[]string{"report", "okrs", "sam", "extra"}, "mgmt: report takes <name> [<person>]\n"},
		{[]string{"report", "../okrs"}, "mgmt: unknown report \"../okrs\"\n"},
		{[]string{"report", "nope", "sam"}, "mgmt: unknown report \"nope\"\n"},
		{[]string{"report", "status", "sam"}, "mgmt: status is a built-in report; run \"mgmt status\"\n"},
		{[]string{"report", "status-yaml"}, "mgmt: status-yaml is a built-in report; run \"mgmt status --format yaml\"\n"},
		{[]string{"report", "explain", "sam"}, "mgmt: explain is a built-in report; run \"mgmt explain <person> <competency>\"\n"},
		{[]string{"report", "prep"}, "mgmt: prep is a built-in report; run \"mgmt prep <person>\"\n"},
		{[]string{"report", "review", "sam"}, "mgmt: review is a built-in report; run \"mgmt review <person>\"\n"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"--now", "2026-10-21"}, tc.args...), root, &stdout, &stderr); code != 2 {
			t.Errorf("%v: exit = %d, want 2", tc.args, code)
		}
		if !strings.HasPrefix(stderr.String(), tc.want) {
			t.Errorf("%v: stderr = %q, want prefix %q", tc.args, stderr.String(), tc.want)
		}
		if stdout.Len() != 0 {
			t.Errorf("%v: stdout = %q", tc.args, stdout.String())
		}
	}
}

// TestLintTextWithoutLocation: Conftest-style string findings carry no file
// and print without a location rather than as ":0:".
func TestLintTextWithoutLocation(t *testing.T) {
	got := lintFlat(
		[]facts.Finding{{Severity: "error", ID: "acme", Message: "OKRs must name a cycle"}},
		[]facts.Finding{{Severity: "warning", ID: "odd", File: "resources/x.yaml", Message: "m"}},
	)
	want := "error acme: OKRs must name a cycle\nresources/x.yaml: warning odd: m\n1 error, 1 warning\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestLintGrouped(t *testing.T) {
	names := map[string]string{"jo": "Jo Kim", "sam": "Sam Lee"}
	errs := []facts.Finding{
		{Severity: "error", ID: "token-unknown", Person: "sam", File: "notes/b.md", Line: 3, Message: "unknown token +plannig", Suggest: "planning"},
		{Severity: "error", ID: "acme", Message: "OKRs must name a cycle"},
		{Severity: "error", ID: "token-unknown", Person: "jo", File: "notes/a.md", Line: 7, Message: "jo: unknown token +x"},
	}
	warns := []facts.Finding{
		{Severity: "warning", ID: "gap-unaddressed", Person: "sam", File: "resources/people.yaml", Line: 1, Message: "sam: gap x (unproven) has no open action"},
		{Severity: "warning", ID: "action-no-due", Person: "sam", File: "notes/b.md", Line: 9, Message: "action \"b\" has no due date"},
		{Severity: "warning", ID: "action-no-due", Person: "sam", File: "notes/a.md", Line: 12, Message: "action \"a\" has no due date"},
		{Severity: "warning", ID: "odd", File: "resources/x.yaml", Message: "no line"},
		{Severity: "warning", ID: "acme-note", Message: "plain string"},
		{Severity: "warning", ID: "custom", Person: "ghost", File: "notes/c.md", Line: 1, Message: "ghost: unnamed"},
		{Severity: "warning", ID: "oneonone-overdue", Person: "jo", File: "resources/people.yaml", Line: 26, Message: "no 1:1 section for jo at all"},
	}
	got := lintGrouped("2026-10-21", names, errs, warns)
	want := `# Lint — 2026-10-21

3 errors, 7 warnings

## Errors

- **acme**
  - OKRs must name a cycle
- **token-unknown**
  - jo: unknown token +x (notes/a.md:7)
  - sam: unknown token +plannig (did you mean "planning"?) (notes/b.md:3)

## ghost

- **custom**
  - unnamed (notes/c.md:1)

## Jo Kim (jo)

- **oneonone-overdue**
  - no 1:1 section for jo at all (resources/people.yaml:26)

## Sam Lee (sam)

- **action-no-due**
  - action "a" has no due date (notes/a.md:12)
  - action "b" has no due date (notes/b.md:9)
- **gap-unaddressed**
  - gap x (unproven) has no open action (resources/people.yaml:1)

## Workspace

- **acme-note**
  - plain string
- **odd**
  - no line (resources/x.yaml)
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestLintGroupedNoProblems(t *testing.T) {
	if got, want := lintGrouped("2026-10-21", nil, nil, nil), "# Lint — 2026-10-21\n\nno problems\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLintGroupedErrorsOnly(t *testing.T) {
	got := lintGrouped("2026-10-21", nil, []facts.Finding{{Severity: "error", ID: "e", Person: "sam", File: "f.md", Line: 1, Message: "sam: m"}}, nil)
	want := "# Lint — 2026-10-21\n\n1 error, 0 warnings\n\n## Errors\n\n- **e**\n  - sam: m (f.md:1)\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
