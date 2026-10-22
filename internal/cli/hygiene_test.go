package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// normalise folds whitespace runs and typographic hyphens to ASCII so that
// reformatted copies of the private text are still caught.
func normalise(s string) string {
	s = strings.NewReplacer("‑", "-", "‐", "-", " ", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// TestNoCompanyFrameworkTextInRepo fails if any line of the private company
// framework source appears in a git-tracked file. Skips when the source or git
// is unavailable.
func TestNoCompanyFrameworkTextInRepo(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	src, err := os.ReadFile(filepath.Join(home, "code", "mgmt-demo", "framework-source", "software-engineering.txt"))
	if err != nil {
		t.Skip("private framework source not present")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	rootOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not inside a git repository")
	}
	root := strings.TrimSpace(string(rootOut))

	lines := strings.Split(string(src), "\n")
	if len(lines) > 3 {
		lines = lines[3:] // the file's own header
	}
	var phrases []string
	for _, l := range lines {
		if p := normalise(l); len(p) > 40 {
			phrases = append(phrases, p)
		}
	}
	if len(phrases) == 0 {
		t.Skip("no candidate phrases in source")
	}

	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git ls-files failed: %v", err)
	}
	for _, name := range bytes.Split(out, []byte{0}) {
		if len(name) == 0 {
			continue
		}
		path := filepath.Join(root, string(name))
		if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := normalise(string(data))
		for _, p := range phrases {
			if strings.Contains(text, p) {
				shown := p
				if len(shown) > 60 {
					shown = shown[:60] + "..."
				}
				t.Errorf("company framework text found in tracked file %s: %q", name, shown)
				break
			}
		}
	}
}
