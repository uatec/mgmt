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
	// A realistic matrix: core has 6 competencies, eng includes core and adds
	// 4 more, and every competency has a descriptor at each of 7 levels.
	levels := []string{"IC1", "IC2", "IC3", "IC4", "IC5", "IC6", "IC7"}
	framework := func(name, includes, prefix string, n int) string {
		var b strings.Builder
		fmt.Fprintf(&b, "apiVersion: mgmt/v1alpha1\nkind: CompetencyFramework\nmetadata:\n  name: %s\nspec:\n%s  levels: [%s]\n  competencies:\n", name, includes, strings.Join(levels, ", "))
		for c := 1; c <= n; c++ {
			var ds []string
			for _, l := range levels {
				ds = append(ds, fmt.Sprintf("%s: does %s%d at %s", l, prefix, c, l))
			}
			fmt.Fprintf(&b, "    - { id: %s.c%d, title: %s %d, tokens: [%s%d], summary: s, levels: {%s} }\n", prefix, c, prefix, c, prefix, c, strings.Join(ds, ", "))
		}
		return b.String()
	}
	write("resources/framework.yaml", framework("core", "", "core", 6)+"---\n"+framework("eng", "  includes: [core]\n", "eng", 4))
	var people strings.Builder
	names := make([]string, 10)
	for i := range names {
		names[i] = fmt.Sprintf("person%d", i)
		fmt.Fprintf(&people, "---\napiVersion: mgmt/v1alpha1\nkind: Person\nmetadata:\n  name: %s\nspec:\n  displayName: Person %d\n  aliases: [P%d]\n  framework: eng\n  level: %s\n  since: 2026-01-01\n", names[i], i, i, levels[i%len(levels)])
	}
	write("resources/people.yaml", people.String()[4:])
	for w := 1; w <= 52; w++ {
		var b strings.Builder
		fmt.Fprintf(&b, "# 2026-W%02d\n\n", w)
		for i := range names {
			// Rotate through all 10 competencies across the weeks.
			core1, core2, eng1 := 1+(w+i)%6, 1+(w+i+3)%6, 1+(w+i)%4
			fmt.Fprintf(&b, "## 1:1 P%d\n- notes for the week\n- +core%d shipped it\n- -core%d was quiet\n- ++eng%d led the migration\n- --eng%d missed the review\n- +eng%d helped another team\n- ~core%d talked it through\n- [ ] follow up @fri ~core%d\n\n", i, core1, core2, eng1, 1+(eng1%4), eng1, core2, core2)
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
