package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mgmt "github.com/uatec/mgmt"
)

func runHelpArgs(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, t.TempDir(), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestHelpPrintsUsage(t *testing.T) {
	code, out, errOut := runHelpArgs(t, "help")
	if code != 0 || out != usage || errOut != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if !strings.Contains(usage, "\n  help [notes] ") {
		t.Errorf("usage does not list help:\n%s", usage)
	}
}

func TestHelpNotesPrintsCheatSheet(t *testing.T) {
	code, out, errOut := runHelpArgs(t, "help", "notes")
	if code != 0 || out != mgmt.NotesCheatSheet || errOut != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if n := strings.Count(out, "\n"); n > 40 {
		t.Errorf("cheat sheet is %d lines, want at most 40", n)
	}
	for _, want := range []string{"++x", "--x", "~x", "- [ ]", "@person", "@date", "!input", "<!-- mgmt:ignore id -->",
		"A negative is neutralised only when an action addressing it is completed."} {
		if !strings.Contains(out, want) {
			t.Errorf("cheat sheet lacks %q", want)
		}
	}
}

func TestHelpUnknownTopic(t *testing.T) {
	code, out, errOut := runHelpArgs(t, "help", "marks")
	if code != 2 || out != "" || !strings.HasPrefix(errOut, "mgmt: unknown help topic \"marks\"\nusage: ") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, _, errOut = runHelpArgs(t, "help", "notes", "extra")
	if code != 2 || !strings.HasPrefix(errOut, "mgmt: help takes [topic]\n") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

// TestReadmeHasCheatSheet keeps the README's "Notes cheat sheet" section
// and the embedded text from drifting apart.
func TestReadmeHasCheatSheet(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	section := "### Notes cheat sheet\n\n`mgmt help notes` prints this.\n\n```text\n" + mgmt.NotesCheatSheet + "```\n"
	if !strings.Contains(string(readme), section) {
		t.Errorf("README.md lacks the notes cheat sheet verbatim; want:\n%s", section)
	}
}
