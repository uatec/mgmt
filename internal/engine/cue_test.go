package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	mgmt "github.com/uatec/mgmt"
	"github.com/uatec/mgmt/internal/load"
)

const wsYAML = `apiVersion: mgmt/v1alpha1
kind: Workspace
metadata:
  name: default
spec:
  notes: notes/**/*.md
  me: [me]
`

func validateDir(t *testing.T, root string) (int, []string) {
	t.Helper()
	res, lf := load.LoadResources(root)
	if len(lf) != 0 {
		t.Fatalf("load findings: %+v", lf)
	}
	fs := Validate(res, mgmt.CoreSchema, filepath.Join(root, "schema"))
	var out []string
	for _, f := range fs {
		if f.ID != "schema-invalid" || f.Severity != "error" {
			t.Errorf("unexpected finding %+v", f)
		}
		out = append(out, f.File+":"+itoa(f.Line)+" "+f.Message)
	}
	return len(fs), out
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestValidateSpecExamples(t *testing.T) {
	n, out := validateDir(t, "../../testdata/schema/valid")
	if n != 0 {
		t.Fatalf("expected no findings, got %v", out)
	}
}

func TestValidateSchemaInvalid(t *testing.T) {
	person := func(extra string) string {
		return "apiVersion: mgmt/v1alpha1\nkind: Person\nmetadata:\n  name: sam\nspec:\n  framework: eng\n  role: r\n  level: L2\n  since: 2026-05-01\n" + extra
	}
	cycle := func(min string) string {
		return "apiVersion: mgmt/v1alpha1\nkind: ReviewCycle\nmetadata:\n  name: c\nspec:\n  framework: eng\n  period: {from: 2026-04-01, to: 2026-09-30}\n  selector: {people: [sam]}\n  evidence: {minPerCompetency: " + min + "}\n"
	}
	tests := []struct {
		name  string
		files map[string]string // relative to resources/
		file  string
		line  int
	}{
		{"bad date", map[string]string{"b.yaml": strings.Replace(person(""), "2026-05-01", "2026-5-1", 1)}, "resources/b.yaml", 1},
		{"unknown field", map[string]string{"b.yaml": person("  colour: red\n")}, "resources/b.yaml", 1},
		{"wrong apiVersion", map[string]string{"b.yaml": strings.Replace(person(""), "mgmt/v1alpha1", "mgmt/v2", 1)}, "resources/b.yaml", 1},
		{"min zero", map[string]string{"b.yaml": cycle("0")}, "resources/b.yaml", 1},
		{"empty selector", map[string]string{"b.yaml": strings.Replace(cycle("1"), "{people: [sam]}", "{}", 1)}, "resources/b.yaml", 1},
		{"unknown kind", map[string]string{"b.yaml": "apiVersion: mgmt/v1alpha1\nkind: Team\nmetadata:\n  name: t\nspec: {}\n"}, "resources/b.yaml", 1},
		{"second doc line", map[string]string{"b.yaml": person("") + "---\n" + strings.Replace(person(""), "sam", "BAD NAME", 1)}, "resources/b.yaml", 11},
		{"two workspaces", map[string]string{"b.yaml": wsYAML}, "resources/b.yaml", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "resources"), 0o755); err != nil {
				t.Fatal(err)
			}
			write := func(name, body string) {
				if err := os.WriteFile(filepath.Join(root, "resources", name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write("a.yaml", wsYAML)
			for n, b := range tc.files {
				write(n, b)
			}
			res, _ := load.LoadResources(root)
			fs := Validate(res, mgmt.CoreSchema, filepath.Join(root, "schema"))
			if len(fs) != 1 {
				t.Fatalf("want 1 finding, got %+v", fs)
			}
			f := fs[0]
			if f.ID != "schema-invalid" || f.Severity != "error" || f.File != tc.file || f.Line != tc.line || f.Message == "" {
				t.Errorf("got %+v, want file %s line %d", f, tc.file, tc.line)
			}
		})
	}
	t.Run("zero workspaces", func(t *testing.T) {
		fs := Validate(nil, mgmt.CoreSchema, filepath.Join(t.TempDir(), "schema"))
		if len(fs) != 1 || fs[0].ID != "schema-invalid" || !strings.Contains(fs[0].Message, "Workspace") {
			t.Fatalf("got %+v", fs)
		}
	})
}

func TestValidateWorkspaceKind(t *testing.T) {
	res, _ := load.LoadResources("../../testdata/schema/wskind")
	res = append(res, load.Resource{APIVersion: "mgmt/v1alpha1", Kind: "Workspace", Name: "default", File: "resources/ws.yaml", Line: 1,
		Object: map[string]any{"apiVersion": "mgmt/v1alpha1", "kind": "Workspace", "metadata": map[string]any{"name": "default"},
			"spec": map[string]any{"notes": "n/**/*.md", "me": []any{"me"}}}})
	if fs := Validate(res, mgmt.CoreSchema, "../../testdata/schema/wskind/schema"); len(fs) != 0 {
		t.Fatalf("with schema: %+v", fs)
	}
	fs := Validate(res, mgmt.CoreSchema, "../../testdata/schema/wskind/nonexistent")
	if len(fs) != 1 || fs[0].ID != "schema-invalid" || !strings.Contains(fs[0].Message, "no schema for kind OKRSet") || fs[0].File != "resources/okr.yaml" {
		t.Fatalf("without schema: %+v", fs)
	}
}

func TestKinds(t *testing.T) {
	got, err := Kinds(mgmt.CoreSchema, "../../testdata/schema/wskind/schema")
	if err != nil {
		t.Fatal(err)
	}
	want := "CompetencyFramework OKRSet Person ReviewCycle Workspace"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %v", got)
	}
}
