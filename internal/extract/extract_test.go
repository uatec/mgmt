package extract

import (
	"crypto/sha1"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/uatec/mgmt/internal/facts"
	"github.com/uatec/mgmt/internal/load"
)

const specRoot = "../../testdata/spec-example"

func loadSpec(t *testing.T) ([]load.Resource, []load.NoteFile) {
	t.Helper()
	res, finds := load.LoadResources(specRoot)
	if len(finds) != 0 {
		t.Fatalf("resource findings: %+v", finds)
	}
	notes, err := load.LoadNotes(specRoot, "notes/**/*.md")
	if err != nil {
		t.Fatal(err)
	}
	return res, notes
}

func actionID(file, text string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(file+text)))
}

func specExpected() facts.Facts {
	const f = "notes/2026-W43.md"
	return facts.Facts{
		Evidence: []facts.Evidence{
			{Person: "sam", Competency: "delivery.ownership", Polarity: 1, Date: "2026-10-19", File: f, Line: 5, Text: "led the incident review well"},
			{Person: "sam", Competency: "collab.planning", Polarity: -1, Date: "2026-10-19", File: f, Line: 6, Text: "quiet in refinement again"},
			{Person: "priya", Competency: "scope.crossteam", Polarity: 1, Date: "2026-10-19", File: f, Line: 13, Text: "unblocked the SRE handover"},
		},
		Touches: []facts.Touch{
			{Person: "sam", Competency: "scope.crossteam", Date: "2026-10-19", File: f, Line: 7, Text: "discussed the Zoa access ask"},
		},
		Actions: []facts.Action{
			{ID: actionID(f, "send him the L4 section"), Person: "sam", Owner: "me", Due: "2026-10-28", Done: false,
				Addresses: []string{"collab.planning"}, Date: "2026-10-19", File: f, Line: 8, Text: "send him the L4 section"},
			{ID: actionID(f, "draft the CDN provisioning plan"), Person: "sam", Owner: "sam", Due: "2026-10-23", Done: false,
				Addresses: []string{}, Date: "2026-10-19", File: f, Line: 9, Text: "draft the CDN provisioning plan"},
		},
		Inputs: []facts.Input{
			{Person: "sam", Input: "self-assessment", Cycle: "ensek-2026-h2", Date: "2026-10-19", File: f, Line: 10},
		},
		Sections: []facts.Section{
			{Person: "sam", Date: "2026-10-19", File: f, Line: 3, TokenCount: 6},
		},
		Ignores:     []facts.Ignore{},
		Diagnostics: []facts.Diagnostic{},
	}
}

func assertFacts(t *testing.T, got, want facts.Facts) {
	t.Helper()
	check := func(name string, g, w any) {
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s:\n got  %+v\n want %+v", name, g, w)
		}
	}
	check("Evidence", got.Evidence, want.Evidence)
	check("Touches", got.Touches, want.Touches)
	check("Actions", got.Actions, want.Actions)
	check("Inputs", got.Inputs, want.Inputs)
	check("Sections", got.Sections, want.Sections)
	check("Ignores", got.Ignores, want.Ignores)
	check("Diagnostics", got.Diagnostics, want.Diagnostics)
}

func TestExtractSpecExample(t *testing.T) {
	res, notes := loadSpec(t)
	got := Extract(notes, NewIndex(res))
	assertFacts(t, got, specExpected())
}

func TestCRLFNotes(t *testing.T) {
	res, notes := loadSpec(t)
	for i := range notes {
		crlf := strings.ReplaceAll(string(notes[i].Source), "\n", "\r\n")
		notes[i].Source = append([]byte("\xef\xbb\xbf"), crlf...)
	}
	got := Extract(notes, NewIndex(res))
	assertFacts(t, got, specExpected())
}

// extractOne runs Extract on a single inline file against the spec resources.
func extractOne(t *testing.T, path, src string) facts.Facts {
	t.Helper()
	res, _ := loadSpec(t)
	return Extract([]load.NoteFile{{Path: path, Source: []byte(src)}}, NewIndex(res))
}

func TestHeadingStack(t *testing.T) {
	src := "# 2026-W43\n\n## 1:1 Sam\n\n### Career\n- +ownership grew\n\n## Platform sync\n- [ ] book room\n"
	got := extractOne(t, "notes/2026-W43.md", src)
	if len(got.Evidence) != 1 || got.Evidence[0].Person != "sam" || got.Evidence[0].Line != 6 {
		t.Errorf("evidence = %+v", got.Evidence)
	}
	if len(got.Actions) != 0 {
		t.Errorf("actions = %+v, want none", got.Actions)
	}
	want := []facts.Section{{Person: "sam", Date: "2026-10-19", File: "notes/2026-W43.md", Line: 3, TokenCount: 1}}
	if !reflect.DeepEqual(got.Sections, want) {
		t.Errorf("sections = %+v", got.Sections)
	}
	if len(got.Diagnostics) != 0 {
		t.Errorf("diagnostics = %+v", got.Diagnostics)
	}
}

func TestHeadingDateOverride(t *testing.T) {
	got := extractOne(t, "notes/2026-W43.md", "# 2026-W43\n\n## Sam (Wed 22 Oct)\n- +ownership good\n")
	if len(got.Evidence) != 1 || got.Evidence[0].Date != "2026-10-22" {
		t.Errorf("evidence = %+v", got.Evidence)
	}
	if len(got.Sections) != 1 || got.Sections[0].Date != "2026-10-22" {
		t.Errorf("sections = %+v", got.Sections)
	}

	got = extractOne(t, "notes/misc.md", "# 2026-10-21\n\n## Sam\n- +ownership good\n\n## Priya\n- ~planning chat\n")
	if len(got.Evidence) != 1 || got.Evidence[0].Date != "2026-10-21" {
		t.Errorf("misc evidence = %+v", got.Evidence)
	}
	if len(got.Touches) != 1 || got.Touches[0].Date != "2026-10-21" || got.Touches[0].Person != "priya" {
		t.Errorf("misc touches = %+v", got.Touches)
	}
}

func TestPersonWholeWord(t *testing.T) {
	got := extractOne(t, "notes/2026-W43.md", "## Jo's growth\n- +ownership a\n\n## Joanne\n- +ownership b\n\n## Major\n- +ownership c\n")
	if len(got.Evidence) != 1 || got.Evidence[0].Person != "jo" || got.Evidence[0].Text != "a" {
		t.Errorf("evidence = %+v", got.Evidence)
	}
	if len(got.Sections) != 1 || got.Sections[0].Person != "jo" || got.Sections[0].Line != 1 {
		t.Errorf("sections = %+v", got.Sections)
	}
}

func TestMaskedRegions(t *testing.T) {
	src := "## Sam\n" +
		"- see `+ownership` here\n" +
		"- run `git +ownership now` here\n" +
		"\n" +
		"```\n+ownership\n```\n" +
		"\n" +
		"- link <https://x.com/+ownership>\n" +
		"- other [link](https://x/+ownership)\n" +
		"- spaced [link](<https://x/ +ownership>)\n" +
		"- note <!-- +ownership --> done\n" +
		"- mail a@b.com and https://x/@foo\n" +
		"- a `cfg`-planning approach\n" +
		"- see [doc](u)-planning\n" +
		"- *x*+ownership\n" +
		"- the **ownership**-related work\n" +
		"- pic ![+ownership](img.png)\n" +
		"\n" +
		"<!-- +ownership -->\n"
	got := extractOne(t, "notes/2026-W43.md", src)
	n := len(got.Evidence) + len(got.Touches) + len(got.Actions) + len(got.Inputs)
	if n != 0 {
		t.Errorf("records: %+v", got)
	}
	if len(got.Diagnostics) != 0 {
		t.Errorf("diagnostics = %+v", got.Diagnostics)
	}
	// "-related" after emphasis resolves to nothing, so check that it isn't
	// even a token (Task 6 would report it as token-unknown).
	x := newExtractor(load.NoteFile{Path: "notes/2026-W43.md", Source: []byte("- the **ownership**-related work\n")}, NewIndex(nil), &facts.Facts{})
	x.run()
	if toks := ScanLine(string(x.masked)); len(toks) != 0 {
		t.Errorf("tokens after emphasis = %+v", toks)
	}
}

func TestIgnoreComment(t *testing.T) {
	got := extractOne(t, "notes/2026-W43.md", "## Sam\n\n- +ownership ok <!-- mgmt:ignore negative-unaddressed,action-no-due -->\n")
	wantIgn := []facts.Ignore{{File: "notes/2026-W43.md", Line: 3, IDs: []string{"negative-unaddressed", "action-no-due"}}}
	if !reflect.DeepEqual(got.Ignores, wantIgn) {
		t.Errorf("ignores = %+v", got.Ignores)
	}
	if len(got.Evidence) != 1 || got.Evidence[0].Text != "ok" {
		t.Errorf("evidence = %+v", got.Evidence)
	}
}

func TestMultipleTokensOneLine(t *testing.T) {
	got := extractOne(t, "notes/2026-W43.md", "## Sam\n- +ownership +planning both\n")
	want := []facts.Evidence{
		{Person: "sam", Competency: "delivery.ownership", Polarity: 1, Date: "2026-10-19", File: "notes/2026-W43.md", Line: 2, Text: "both"},
		{Person: "sam", Competency: "collab.planning", Polarity: 1, Date: "2026-10-19", File: "notes/2026-W43.md", Line: 2, Text: "both"},
	}
	if !reflect.DeepEqual(got.Evidence, want) {
		t.Errorf("evidence = %+v", got.Evidence)
	}
	if got.Sections[0].TokenCount != 2 {
		t.Errorf("tokenCount = %d", got.Sections[0].TokenCount)
	}
}

func TestActionLines(t *testing.T) {
	src := "## Sam\n- [x] closed it ~planning\n  then +ownership after\n- [ ]  @sam spaced out\n"
	got := extractOne(t, "notes/2026-W43.md", src)
	if len(got.Actions) != 2 {
		t.Fatalf("actions = %+v", got.Actions)
	}
	a := got.Actions[0]
	if !a.Done || a.Text != "closed it" || a.Line != 2 || !reflect.DeepEqual(a.Addresses, []string{"collab.planning"}) {
		t.Errorf("closed action = %+v", a)
	}
	if b := got.Actions[1]; b.Done || b.Owner != "sam" || b.Text != "spaced out" {
		t.Errorf("open action = %+v", b)
	}
	want := []facts.Evidence{{Person: "sam", Competency: "delivery.ownership", Polarity: 1, Date: "2026-10-19",
		File: "notes/2026-W43.md", Line: 3, Text: "then after"}}
	if !reflect.DeepEqual(got.Evidence, want) {
		t.Errorf("continuation evidence = %+v", got.Evidence)
	}
}

func TestOneOnOneMonthNamedPerson(t *testing.T) {
	res, _ := loadSpec(t)
	for _, r := range res {
		if r.Kind == "Person" && r.Name == "priya" {
			spec := r.Object["spec"].(map[string]any)
			spec["aliases"] = []any{"Priya", "May"}
		}
	}
	src := "# 2026-W43\n\n## 1:1 May\n- +crossteam helped\n"
	got := Extract([]load.NoteFile{{Path: "notes/2026-W43.md", Source: []byte(src)}}, NewIndex(res))
	if len(got.Evidence) != 1 || got.Evidence[0].Person != "priya" || got.Evidence[0].Date != "2026-10-19" {
		t.Errorf("evidence = %+v", got.Evidence)
	}
	if len(got.Sections) != 1 || got.Sections[0].Date != "2026-10-19" {
		t.Errorf("sections = %+v", got.Sections)
	}
}
