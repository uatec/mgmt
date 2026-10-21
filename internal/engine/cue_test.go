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

const (
	oneComp = "    - {id: a.x, title: X}\n"
	dupComp = oneComp + oneComp
	roleL1  = "    - name: r\n      levels:\n        - name: L1\n"
)

// framework is a CompetencyFramework document with the given competency and role list items.
func framework(comps, roles string) string {
	return "apiVersion: mgmt/v1alpha1\nkind: CompetencyFramework\nmetadata:\n  name: eng\nspec:\n  competencies:\n" + comps + "  roles:\n" + roles
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
		msg   string // substring of the message; "" checks only that it's set
	}{
		{"bad date", map[string]string{"b.yaml": strings.Replace(person(""), "2026-05-01", "2026-5-1", 1)}, "resources/b.yaml", 1, ""},
		{"impossible since", map[string]string{"b.yaml": strings.Replace(person(""), "2026-05-01", "2026-09-31", 1)}, "resources/b.yaml", 1, "since"},
		{"impossible period", map[string]string{"b.yaml": strings.Replace(cycle("1"), "2026-09-30", "2026-02-29", 1)}, "resources/b.yaml", 1, "period.to"},
		{"impossible milestone due", map[string]string{"b.yaml": cycle("1") + "  milestones:\n    - {name: m, due: 2026-11-31}\n"}, "resources/b.yaml", 1, "due"},
		{"unknown field", map[string]string{"b.yaml": person("  colour: red\n")}, "resources/b.yaml", 1, ""},
		{"wrong apiVersion", map[string]string{"b.yaml": strings.Replace(person(""), "mgmt/v1alpha1", "mgmt/v2", 1)}, "resources/b.yaml", 1, ""},
		{"min zero", map[string]string{"b.yaml": cycle("0")}, "resources/b.yaml", 1, ""},
		{"empty selector", map[string]string{"b.yaml": strings.Replace(cycle("1"), "{people: [sam]}", "{}", 1)}, "resources/b.yaml", 1, ""},
		{"unknown kind", map[string]string{"b.yaml": "apiVersion: mgmt/v1alpha1\nkind: Team\nmetadata:\n  name: t\nspec: {}\n"}, "resources/b.yaml", 1, ""},
		{"second doc line", map[string]string{"b.yaml": person("") + "---\n" + strings.Replace(person(""), "sam", "BAD NAME", 1)}, "resources/b.yaml", 11, ""},
		{"two workspaces", map[string]string{"b.yaml": wsYAML}, "resources/b.yaml", 1, ""},
		{"duplicate competency id", map[string]string{"b.yaml": framework(dupComp, roleL1)}, "resources/b.yaml", 1, `duplicate competency id "a.x"`},
		{"duplicate role name", map[string]string{"b.yaml": framework(oneComp, roleL1+roleL1)}, "resources/b.yaml", 1, `duplicate role name "r"`},
		{"duplicate resource name", map[string]string{"b.yaml": person("") + "---\n" + person("")}, "resources/b.yaml", 11, `duplicate Person "sam"`},
		{"duplicate level name", map[string]string{"b.yaml": framework(oneComp, "    - name: r\n      levels:\n        - name: L1\n        - name: L1\n")}, "resources/b.yaml", 1, `role "r": duplicate level name "L1"`},
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
			if f.ID != "schema-invalid" || f.Severity != "error" || f.File != tc.file || f.Line != tc.line || f.Message == "" || !strings.Contains(f.Message, tc.msg) {
				t.Errorf("got %+v, want file %s line %d", f, tc.file, tc.line)
			}
		})
	}
	t.Run("unique framework is valid", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "resources"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := wsYAML + "---\n" + framework(oneComp+"    - {id: a.y, title: Why}\n", roleL1+"    - name: s\n      levels:\n        - name: L1\n")
		if err := os.WriteFile(filepath.Join(root, "resources", "a.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		res, _ := load.LoadResources(root)
		if fs := Validate(res, mgmt.CoreSchema, filepath.Join(root, "schema")); len(fs) != 0 {
			t.Fatalf("got %+v", fs)
		}
	})
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

// TestValidateDefinitionWithoutLiteralKind: a workspace definition only
// registers a kind when it fixes kind to its own name; otherwise the
// resource's error says why.
func TestValidateDefinitionWithoutLiteralKind(t *testing.T) {
	dir := t.TempDir()
	src := "package mgmt\n\n#Widget: #Resource & {spec: {size: int}}\n"
	if err := os.WriteFile(filepath.Join(dir, "widget.cue"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := load.Resource{APIVersion: "mgmt/v1alpha1", Kind: "Workspace", Name: "default", File: "resources/ws.yaml", Line: 1,
		Object: map[string]any{"apiVersion": "mgmt/v1alpha1", "kind": "Workspace", "metadata": map[string]any{"name": "default"},
			"spec": map[string]any{"notes": "n/**/*.md", "me": []any{"me"}}}}
	widget := load.Resource{APIVersion: "mgmt/v1alpha1", Kind: "Widget", Name: "w", File: "resources/w.yaml", Line: 3,
		Object: map[string]any{"apiVersion": "mgmt/v1alpha1", "kind": "Widget", "metadata": map[string]any{"name": "w"},
			"spec": map[string]any{"size": 1}}}
	fs := Validate([]load.Resource{ws, widget}, mgmt.CoreSchema, dir)
	want := `no schema for kind Widget: definition #Widget does not set kind: "Widget" (write #Widget: #Resource & {kind: "Widget", ...})`
	if len(fs) != 1 || fs[0].File != "resources/w.yaml" || fs[0].Line != 3 || fs[0].Message != want {
		t.Fatalf("got %+v", fs)
	}
}

// TestValidateSchemaDirStatError: only a missing <root>/schema is ignored.
func TestValidateSchemaDirStatError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "schema")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{file, filepath.Join(file, "sub")} {
		fs := Validate(nil, mgmt.CoreSchema, dir)
		if len(fs) != 1 || fs[0].ID != "schema-invalid" || !strings.Contains(fs[0].Message, "schema does not compile: ") {
			t.Errorf("%s: got %+v", dir, fs)
		}
	}
}
