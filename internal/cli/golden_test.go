package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// TestGolden runs the CLI against fixture workspaces at a fixed --now.
//
// Each row's stdout is compared with testdata/<fixture>/golden/<golden>.
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
	}
	for _, r := range rows {
		t.Run(r.fixture+"/"+r.golden, func(t *testing.T) {
			root := filepath.Join("..", "..", "testdata", r.fixture)
			var stdout, stderr bytes.Buffer
			code := Run(append([]string{"--now", "2026-10-21"}, r.args...), root, &stdout, &stderr)
			if code != r.exit {
				t.Errorf("exit = %d, want %d\nstderr: %s", code, r.exit, stderr.String())
			}
			compare(t, filepath.Join(root, "golden", r.golden), stdout.Bytes())
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
	if code := Run([]string{"--now", "2026-10-21", "--out", out, "lint", "--strict"}, root, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr %s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty with --out, got %q", stdout.String())
	}
	b, err := os.ReadFile(out)
	if err != nil || string(b) != "no problems\n" {
		t.Errorf("out file = %q, %v", b, err)
	}
}

func TestUsageErrors(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "spec-example")
	for _, args := range [][]string{
		{},
		{"bogus"},
		{"--now", "21/10/2026", "lint"},
		{"lint", "--format", "xml"},
		{"lint", "extra"},
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
