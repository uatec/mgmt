package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestComplete(t *testing.T) {
	team := filepath.Join("..", "..", "testdata", "team")
	ensek := filepath.Join("..", "..", "testdata", "ensek")
	empty := t.TempDir()
	rows := []struct {
		name  string
		root  string
		words string // the command line after "mgmt"; the last word is the one being completed
		want  string // space-separated values, in order; ":files" asks for file names
	}{
		{"commands", team, "", "init facts lint prep review explain status report completion"},
		{"command prefix", team, "re", "review report"},
		{"global flag before command", team, "--n", "--now"},
		{"people", team, "prep ", "alex jo priya sam Samuel"},
		{"people by prefix, any case", team, "prep S", "sam Samuel"},
		{"alias is offered once, not per casing", team, "prep Sam", "sam Samuel"},
		{"review takes a person", team, "review pr", "priya"},
		{"person is only the first positional", team, "prep sam ", ""},
		{"competencies need a person first", team, "explain ", "alex jo priya sam Samuel"},
		{"competencies", ensek, "explain sam own", "own ownership"},
		{"competency id by prefix", ensek, "explain sam delivery.", "delivery.ownership"},
		{"competencies of an unknown person", team, "explain nobody ", ""},
		{"competencies after flags", team, "explain --cycle retro-2026-h1 sam pl", "plan planning"},
		{"cycle value", team, "prep sam --cycle ", "ensek-2026-h2 retro-2026-h1"},
		{"cycle value joined with =", team, "status --cycle=re", "--cycle=retro-2026-h1"},
		{"lint format", team, "lint --format ", "text flat json"},
		{"lint format joined with =", team, "lint --format=f", "--format=flat"},
		{"status format", team, "status --format ", "markdown yaml"},
		{"flag names", team, "lint --", "--strict --format --now --out"},
		{"flags of a command", team, "status --c", "--cycle"},
		{"flag after positionals", team, "prep sam --", "--cycle --now --out"},
		{"out takes files", team, "lint --out ", ":files"},
		{"global flag value is not a positional", team, "--now 2026-10-21 pr", "prep"},
		{"report names", ensek, "report ", "okrs"},
		{"report then person", ensek, "report okrs s", "sam Samuel"},
		{"completion shells", team, "completion ", "zsh bash fish"},
		{"no data outside a workspace", empty, "prep ", ""},
		{"commands outside a workspace", empty, "st", "status"},
		{"nothing for facts", team, "facts ", ""},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			words := strings.Split(r.words, " ")
			var out bytes.Buffer
			if code := Run(append([]string{"__complete"}, words...), r.root, &out, &out); code != 0 {
				t.Fatalf("exit = %d", code)
			}
			var got []string
			for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
				if line != "" {
					got = append(got, strings.SplitN(line, "\t", 2)[0])
				}
			}
			if g := strings.Join(got, " "); g != r.want {
				t.Errorf("complete %q = %q, want %q", r.words, g, r.want)
			}
		})
	}
}

func TestCompleteDescriptions(t *testing.T) {
	var out bytes.Buffer
	Run([]string{"__complete", "prep", "sa"}, filepath.Join("..", "..", "testdata", "team"), &out, &out)
	if !strings.HasPrefix(out.String(), "sam\t") {
		t.Errorf("want a tab-separated description after the person, got %q", out.String())
	}
}

func TestCompletionScripts(t *testing.T) {
	for _, shell := range completionShells {
		var out, errb bytes.Buffer
		if code := Run([]string{"completion", shell}, t.TempDir(), &out, &errb); code != 0 {
			t.Errorf("completion %s: exit %d: %s", shell, code, errb.String())
		}
		if !strings.Contains(out.String(), "__complete") {
			t.Errorf("completion %s: script doesn't call __complete", shell)
		}
	}
	var out, errb bytes.Buffer
	if code := Run([]string{"completion", "powershell"}, t.TempDir(), &out, &errb); code != 2 {
		t.Errorf("unknown shell: exit %d, want 2", code)
	}
}
