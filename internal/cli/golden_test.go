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
		{"team", []string{"explain", "sam", "plannig"}, 2, "", "explain-plannig.stderr"},
		{"team", []string{"explain", "sam", "zzzzzz"}, 2, "", "explain-zzzzzz.stderr"},
		{"team", []string{"status"}, 0, "status.md", ""},
		{"team", []string{"status", "--cycle", "retro-2026-h1"}, 0, "status-retro.md", ""},
		{"team", []string{"status", "--cycle", "nope"}, 2, "", "status-nope.stderr"},
		{"spec-example", []string{"status"}, 0, "status.md", ""},
		{"spec-example", []string{"--now", "2027-01-15", "status"}, 0, "status-no-cycle.md", ""},
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

func TestUsageErrors(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "spec-example")
	for _, args := range [][]string{
		{},
		{"bogus"},
		{"--now", "21/10/2026", "lint"},
		{"lint", "--format", "xml"},
		{"lint", "extra"},
		{"prep"},
		{"prep", "sam", "extra"},
		{"prep", "sam", "--cycle"},
		{"review"},
		{"review", "sam", "extra"},
		{"explain", "sam"},
		{"explain", "sam", "planning", "extra"},
		{"status", "sam"},
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
	if _, err := reportData(res, "prep", []string{"sam"}, engine.Options{Now: "2026-10-21"}); err == nil ||
		!strings.Contains(err.Error(), `no prep report for "sam"`) {
		t.Errorf("err = %v", err)
	}
	if _, err := reportData(res, "prep", []string{"priya"}, engine.Options{Now: "2026-10-21"}); err != nil {
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
		"person":     map[string]any{"name": "sam", "role": "platform-engineer", "level": "L3"},
		"competency": map[string]any{"id": "collab.planning", "title": "Plans"},
		"required":   false, "status": "proven", "rule": "counting", "positives": 2, "negatives": 1,
		"settings": map[string]any{"minPerCompetency": 2, "negativeOverrides": false},
		"window":   map[string]any{"from": "2026-07-23", "to": "2026-10-21", "cycle": nil},
		"considered": []any{map[string]any{
			"date": "2026-09-01", "polarity": -1, "text": "t", "file": "notes/a.md", "line": 3, "decides": false,
		}},
		"outside": []any{},
	}
	got, err := render.Render("explain", map[string]any{"view": view}, mgmt.CoreReports, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Required: no (platform-engineer L3)\n",
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
