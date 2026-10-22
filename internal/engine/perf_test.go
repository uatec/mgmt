package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestLintPerformance checks the spec's budget: 52 weekly files x 10 people
// lint in under a second (median of 3 runs after a warm-up).
func TestLintPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("performance test skipped under -short")
	}
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("resources/workspace.yaml", "apiVersion: mgmt/v1alpha1\nkind: Workspace\nmetadata:\n  name: default\nspec:\n  notes: notes/**/*.md\n  me: [me]\n  defaultWindow: 90d\n  oneToOneCadence: 14d\n")
	write("resources/framework.yaml", `apiVersion: mgmt/v1alpha1
kind: CompetencyFramework
metadata:
  name: eng
spec:
  includes: [core]
  levels: [IC1]
  competencies:
    - { id: delivery.ownership, title: Ownership, tokens: [ownership], levels: {IC1: owns it} }
    - { id: scope.crossteam, title: Cross team, tokens: [crossteam], levels: {IC1: spans teams} }
---
apiVersion: mgmt/v1alpha1
kind: CompetencyFramework
metadata:
  name: core
spec:
  levels: [IC1]
  competencies:
    - { id: collab.planning, title: Planning, tokens: [planning], levels: {IC1: plans} }
`)
	var people strings.Builder
	names := make([]string, 10)
	for i := range names {
		names[i] = fmt.Sprintf("person%d", i)
		fmt.Fprintf(&people, "---\napiVersion: mgmt/v1alpha1\nkind: Person\nmetadata:\n  name: %s\nspec:\n  displayName: Person %d\n  aliases: [P%d]\n  framework: eng\n  level: IC1\n  since: 2026-01-01\n", names[i], i, i)
	}
	write("resources/people.yaml", people.String()[4:])
	for w := 1; w <= 52; w++ {
		var b strings.Builder
		fmt.Fprintf(&b, "# 2026-W%02d\n\n", w)
		for i := range names {
			fmt.Fprintf(&b, "## 1:1 P%d\n- notes for the week\n- +ownership shipped it\n- -planning was quiet\n- +crossteam helped another team\n- ~planning talked it through\n- [ ] follow up on planning @fri ~planning\n\n", i)
		}
		write(fmt.Sprintf("notes/2026-W%02d.md", w), b.String())
	}

	run := func() time.Duration {
		start := time.Now()
		res, err := Run(root, Options{Now: "2026-12-20"})
		d := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Errors) > 0 {
			t.Fatalf("generated workspace has errors: %v", res.Errors[0])
		}
		return d
	}
	run() // warm-up
	runs := []time.Duration{run(), run(), run()}
	sort.Slice(runs, func(i, j int) bool { return runs[i] < runs[j] })
	t.Logf("runs: %v", runs)
	if runs[1] >= time.Second {
		t.Fatalf("median lint time %v, want < 1s", runs[1])
	}
}
