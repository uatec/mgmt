package load

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/uatec/mgmt/internal/facts"
)

func TestLoadResourcesMultiDoc(t *testing.T) {
	res, finds := LoadResources("../../testdata/load")
	if len(finds) != 0 {
		t.Fatalf("unexpected findings: %+v", finds)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 resources, got %d", len(res))
	}
	if res[0].Line != 1 || res[1].Line != 12 {
		t.Errorf("lines = %d, %d; want 1, 12", res[0].Line, res[1].Line)
	}
	for i, name := range []string{"sam", "priya"} {
		if res[i].Kind != "Person" || res[i].Name != name {
			t.Errorf("res[%d] = %s/%s", i, res[i].Kind, res[i].Name)
		}
		if res[i].File != "resources/people.yaml" || res[i].APIVersion != "mgmt/v1alpha1" {
			t.Errorf("res[%d] file/apiVersion = %s %s", i, res[i].File, res[i].APIVersion)
		}
	}
}

func TestLoadResourcesDatesStayStrings(t *testing.T) {
	res, _ := LoadResources("../../testdata/load")
	spec := res[0].Object["spec"].(map[string]any)
	if spec["since"] != "2026-05-01" {
		t.Errorf("since = %#v", spec["since"])
	}
	period := spec["period"].(map[string]any)
	if period["from"] != "2026-04-01" || period["to"] != "2026-09-30" {
		t.Errorf("period = %#v", period)
	}
}

func TestLoadResourcesCRLFAndBOM(t *testing.T) {
	res, finds := LoadResources("../../testdata/load/crlf")
	if len(finds) != 0 {
		t.Fatalf("findings: %+v", finds)
	}
	var crlf, lf []Resource
	for _, r := range res {
		if strings.HasSuffix(r.File, "people-lf.yaml") {
			lf = append(lf, r)
		} else {
			crlf = append(crlf, r)
		}
	}
	if len(crlf) != 2 || len(lf) != 2 {
		t.Fatalf("crlf=%d lf=%d", len(crlf), len(lf))
	}
	for i := range crlf {
		if crlf[i].Name != lf[i].Name || crlf[i].Line != lf[i].Line || crlf[i].Kind != lf[i].Kind {
			t.Errorf("doc %d differs: %+v vs %+v", i, crlf[i], lf[i])
		}
	}
	if crlf[1].Line != 6 {
		t.Errorf("second doc line = %d, want 6", crlf[1].Line)
	}
}

func TestLoadResourcesBadYAML(t *testing.T) {
	res, finds := LoadResources("../../testdata/load/bad")
	want := facts.Finding{Severity: "error", ID: "schema-invalid", File: "resources/bad.yaml", Line: 6}
	if len(finds) != 1 {
		t.Fatalf("findings = %+v", finds)
	}
	got := finds[0]
	if got.Severity != want.Severity || got.ID != want.ID || got.File != want.File || got.Line != want.Line || got.Message == "" {
		t.Errorf("finding = %+v, want %+v", got, want)
	}
	if len(res) != 2 || res[0].Name != "ok" || res[1].Name != "after" {
		t.Errorf("resources = %+v", res)
	}
}

func TestLoadResourcesMissingKindOrName(t *testing.T) {
	res, finds := LoadResources("../../testdata/load/missing")
	if len(finds) != 2 {
		t.Fatalf("findings = %+v", finds)
	}
	for i, line := range []int{1, 5} {
		if finds[i].ID != "schema-invalid" || finds[i].Severity != "error" || finds[i].Line != line || finds[i].File != "resources/m.yaml" {
			t.Errorf("finding %d = %+v", i, finds[i])
		}
	}
	if len(res) != 1 || res[0].Name != "fine" {
		t.Errorf("resources = %+v", res)
	}
}

func TestLoadNotesGlob(t *testing.T) {
	notes, err := LoadNotes("../../testdata/load/notes", "notes/**/*.md")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, n := range notes {
		paths = append(paths, n.Path)
	}
	want := []string{"notes/2026-W43.md", "notes/2026/2026-10-22-1on1.md"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("paths = %v, want %v", paths, want)
	}
	none, err := LoadNotes("../../testdata/load/notes", "nothing/**/*.md")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("no-match = %#v, %v", none, err)
	}
}

func TestFactsEmptyListsMarshalAsArrays(t *testing.T) {
	b, err := json.Marshal(facts.Facts{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"evidence":[]`) || strings.Contains(s, "null") {
		t.Errorf("json = %s", s)
	}
	for _, k := range []string{`"touches":[]`, `"actions":[]`, `"inputs":[]`, `"sections":[]`, `"ignores":[]`, `"diagnostics":[]`, `"resources":{}`, `"sources":{}`, `"options":{"cycle":""}`} {
		if !strings.Contains(s, k) {
			t.Errorf("missing %s in %s", k, s)
		}
	}
}

func TestParseDocsIgnoreComments(t *testing.T) {
	src := "apiVersion: mgmt/v1alpha1\nkind: Person\nmetadata:\n  name: sam\n---\n" +
		"# mgmt:ignore oneonone-overdue\napiVersion: mgmt/v1alpha1\nkind: Person\nmetadata:\n  name: jo  # mgmt:ignore gap-unaddressed, competency-untouched\n" +
		"spec:\n  aliases: [Jo] # mgmt:ignore oneonone-overdue\n"
	res, finds := parseDocs("resources/p.yaml", []byte(src))
	if len(finds) != 0 || len(res) != 2 {
		t.Fatalf("res=%+v finds=%+v", res, finds)
	}
	if len(res[0].Ignores) != 0 {
		t.Errorf("sam ignores = %v, want none", res[0].Ignores)
	}
	want := []string{"competency-untouched", "gap-unaddressed", "oneonone-overdue"}
	if strings.Join(res[1].Ignores, ",") != strings.Join(want, ",") || res[1].Line != 6 {
		t.Errorf("jo ignores = %v line %d, want %v line 6", res[1].Ignores, res[1].Line, want)
	}
}
