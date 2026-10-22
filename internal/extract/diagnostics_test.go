package extract

import (
	"reflect"
	"testing"
)

type wantDiag struct {
	Code    string
	Line    int
	Suggest string
}

func TestExtractDiagnostics(t *testing.T) {
	const w = "notes/2026-W43.md"
	rows := []struct {
		name, path, src string
		want            []wantDiag
		wantMsg         string
	}{
		{"token-unknown", w, "# 2026-W43\n\n## Sam\n- +plannig x\n", []wantDiag{{"token-unknown", 4, "planning"}}, `unknown competency token "plannig"`},
		{"person-unknown", w, "# 2026-W43\n\n- @samm +ownership x\n", []wantDiag{{"person-unknown", 3, "sam"}}, `unknown person "samm"`},
		{"date-unresolved", "notes/misc.md", "- +ownership x\n", []wantDiag{{"date-unresolved", 1, ""}}, ""},
		{"date-unresolved prose only", "notes/misc.md", "just prose\n", []wantDiag{{"date-unresolved", 1, ""}}, ""},
		{"blank dateless", "notes/misc.md", "\n  \n", nil, ""},
		{"dated by H1 after marks", "notes/misc.md", "## Sam\n- +ownership x\n\n# 2026-10-21\n", nil, ""},
		{"heading-multi-person", w, "# 2026-W43\n\n## Sam and Priya\n- +ownership x\n", []wantDiag{{"heading-multi-person", 4, ""}}, ""},
		{"heading-multi-person ok with @", w, "# 2026-W43\n\n## Sam and Priya\n- @sam +ownership x\n", nil, ""},
		{"evidence-in-action", w, "# 2026-W43\n\n## Sam\n- [ ] +ownership do it\n", []wantDiag{{"evidence-in-action", 4, ""}}, ""},
		{"evidence-in-action strong", w, "# 2026-W43\n\n## Sam\n- [ ] ++ownership do it\n", []wantDiag{{"evidence-in-action", 4, ""}}, `evidence token "++ownership" on an action; evidence belongs on its own line (use ~ to say what the action addresses)`},
		{"input-unknown typo", w, "# 2026-W43\n\n## Sam\n- !self-asessment\n", []wantDiag{{"input-unknown", 4, "self-assessment"}}, ""},
		{"input case-insensitive", w, "# 2026-W43\n\n## Sam\n- !Self-Assessment\n", nil, ""},
		{"input out of range", "notes/2027-06-01.md", "## Sam\n- !self-assessment\n", []wantDiag{{"input-unknown", 2, ""}}, ""},
		{"person-missing", w, "# 2026-W43\n\n## Platform sync\n- +ownership x\n", []wantDiag{{"person-missing", 4, ""}}, ""},
		{"checkbox without person ignored", w, "# 2026-W43\n\n## Platform sync\n- [ ] book room\n", nil, ""},
		{"line-multi-person", w, "# 2026-W43\n\n- @sam @priya +ownership x\n", []wantDiag{{"line-multi-person", 3, ""}}, ""},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			got := extractOne(t, r.path, r.src)
			var have []wantDiag
			for _, d := range got.Diagnostics {
				if d.Severity != "error" || d.File != r.path {
					t.Errorf("bad diagnostic %+v", d)
				}
				have = append(have, wantDiag{d.Code, d.Line, d.Suggest})
				if r.wantMsg != "" && d.Message != r.wantMsg {
					t.Errorf("message = %q, want %q", d.Message, r.wantMsg)
				}
			}
			if !reflect.DeepEqual(have, r.want) {
				t.Errorf("diagnostics = %+v, want %+v", have, r.want)
			}
		})
	}
}

func TestDiagnosticsDropRecords(t *testing.T) {
	got := extractOne(t, "notes/2026-W43.md", "# 2026-W43\n\n## Sam\n- +plannig x\n- !nope\n- @samm +ownership y\n- [ ] +ownership z\n")
	if len(got.Evidence)+len(got.Inputs) != 0 {
		t.Errorf("records not dropped: %+v %+v", got.Evidence, got.Inputs)
	}
}

func TestH1AfterMarksDatesThem(t *testing.T) {
	got := extractOne(t, "notes/misc.md", "## Sam\n- +ownership x\n\n# 2026-10-21\n")
	if len(got.Evidence) != 1 || got.Evidence[0].Date != "2026-10-21" {
		t.Errorf("evidence = %+v", got.Evidence)
	}
}

func TestSuggest(t *testing.T) {
	cands := []string{"planning", "ownership", "delivery"}
	cases := []struct{ word, want string }{
		{"plannig", "planning"},
		{"PLANNIG", "planning"},
		{"zzzzzz", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := Suggest(c.word, cands); got != c.want {
			t.Errorf("Suggest(%q) = %q, want %q", c.word, got, c.want)
		}
	}
	if got := Suggest("ab", []string{"ad", "ac"}); got != "ac" {
		t.Errorf("tie = %q, want ac", got)
	}
	if got := Suggest("x", nil); got != "" {
		t.Errorf("nil = %q", got)
	}
}
