package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, root, &out, &errb)
	return code, out.String(), errb.String()
}

func TestInitScaffoldLintsClean(t *testing.T) {
	root := t.TempDir()
	code, out, errs := runCLI(t, root, "init")
	if code != 0 {
		t.Fatalf("init exit %d: %s", code, errs)
	}
	for _, want := range []string{"resources/workspace.yaml", "notes/2026-W43.md", "policy/README.md", "policy/example/example.rego"} {
		if !strings.Contains(out, want+"\n") {
			t.Errorf("init output missing %s:\n%s", want, out)
		}
	}
	code, out, errs = runCLI(t, root, "--now", "2026-10-21", "lint")
	if code != 0 {
		t.Fatalf("lint exit %d: %s%s", code, out, errs)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, ": error ") || strings.HasPrefix(line, "error ") {
			t.Errorf("error line: %s", line)
		}
	}
	if !strings.Contains(out, "0 errors") {
		t.Errorf("summary: %s", out)
	}
}

// TestInitExamplePolicyIsLive: the scaffold's example policy loads and fires
// once you owe one person more than 10 open actions.
func TestInitExamplePolicyIsLive(t *testing.T) {
	root := t.TempDir()
	if code, _, errs := runCLI(t, root, "init"); code != 0 {
		t.Fatalf("init: %s", errs)
	}
	code, out, _ := runCLI(t, root, "--now", "2026-10-21", "lint")
	if code != 0 || strings.Contains(out, "example-me-backlog") {
		t.Fatalf("example policy fired on the scaffold (exit %d):\n%s", code, out)
	}
	var b strings.Builder
	b.WriteString("# 2026-10-20\n\n## 1:1 Robin\n")
	for i := range 11 {
		fmt.Fprintf(&b, "- [ ] follow up %d @30oct\n", i)
	}
	if err := os.WriteFile(filepath.Join(root, "notes", "2026-10-20.md"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	_, out, _ = runCLI(t, root, "--now", "2026-10-21", "lint", "--format", "flat")
	if !strings.Contains(out, "warning example-me-backlog [robin]: robin: you owe them 12 open actions (more than 10)") {
		t.Errorf("example policy did not fire:\n%s", out)
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "resources", "workspace.yaml")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errs := runCLI(t, root, "init")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(errs, "refusing to overwrite resources/workspace.yaml") {
		t.Errorf("stderr: %q", errs)
	}
	if out != "" {
		t.Errorf("stdout: %q", out)
	}
	if b, _ := os.ReadFile(existing); string(b) != "mine\n" {
		t.Errorf("existing file changed: %q", b)
	}
	var files []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if len(files) != 1 {
		t.Errorf("files written despite refusal: %v", files)
	}
}
