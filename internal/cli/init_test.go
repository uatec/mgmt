package cli

import (
	"bytes"
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
	for _, want := range []string{"resources/workspace.yaml", "notes/2026-W43.md", "policy/README.md"} {
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
