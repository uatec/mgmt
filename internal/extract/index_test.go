package extract

import (
	"reflect"
	"slices"
	"testing"

	"github.com/uatec/mgmt/internal/facts"
	"github.com/uatec/mgmt/internal/load"
)

func res(kind, name string, spec map[string]any) load.Resource {
	return load.Resource{APIVersion: "mgmt/v1alpha1", Kind: kind, Name: name, File: "resources/x.yaml", Line: 1,
		Object: map[string]any{"apiVersion": "mgmt/v1alpha1", "kind": kind, "metadata": map[string]any{"name": name}, "spec": spec}}
}

func comp(id, title string, tokens []any, levels map[string]any) map[string]any {
	c := map[string]any{"id": id, "title": title, "tokens": tokens}
	if levels != nil {
		c["levels"] = levels
	}
	return c
}

// levelled is a core framework, a software-engineering framework that
// includes it, a person on the latter and a cycle selecting by core.
func levelled() []load.Resource {
	ladder := []any{"IC1", "IC2"}
	return []load.Resource{
		res("Workspace", "default", map[string]any{"notes": "notes/*.md", "me": []any{"me"}}),
		res("CompetencyFramework", "core", map[string]any{"levels": ladder, "competencies": []any{
			comp("core.communication", "Communication", []any{"comms"}, map[string]any{"IC1": "says things", "IC2": "says things well"}),
		}}),
		res("CompetencyFramework", "software-engineering", map[string]any{"includes": []any{"core"}, "levels": ladder, "competencies": []any{
			comp("eng.code", "Code", []any{"code"}, map[string]any{"IC2": "writes code"}),
		}}),
		res("Person", "sam", map[string]any{"framework": "software-engineering", "level": "IC1", "since": "2026-01-01", "aliases": []any{"Sam"}}),
		res("ReviewCycle", "h2", map[string]any{
			"period":   map[string]any{"from": "2026-07-01", "to": "2026-12-31"},
			"selector": map[string]any{"matchFrameworks": []any{"core"}},
			"inputs":   []any{map[string]any{"name": "self-assessment"}},
		}),
	}
}

func TestIncludedTokenResolves(t *testing.T) {
	idx := NewIndex(levelled())
	if id, r := idx.Competency("sam", "comms"); id != "core.communication" || r != Resolved {
		t.Errorf("comms = %q %v", id, r)
	}
	if id, r := idx.Competency("sam", "code"); id != "eng.code" || r != Resolved {
		t.Errorf("code = %q %v", id, r)
	}
	if id, r := idx.Competency("sam", "communication"); id != "core.communication" || r != Resolved {
		t.Errorf("suffix = %q %v", id, r)
	}
	if !slices.Contains(idx.TokenNames("sam"), "comms") {
		t.Errorf("token names = %v", idx.TokenNames("sam"))
	}
	src := "# 2026-W43\n\n## Sam\n- +comms ran the demo\n- +comns typo\n- !self-assessment in\n"
	got := Extract([]load.NoteFile{{Path: "notes/2026-W43.md", Source: []byte(src)}}, idx)
	if len(got.Evidence) != 1 || got.Evidence[0].Competency != "core.communication" {
		t.Errorf("evidence = %+v", got.Evidence)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != "token-unknown" || got.Diagnostics[0].Suggest != "comms" {
		t.Errorf("diagnostics = %+v", got.Diagnostics)
	}
	if len(got.Inputs) != 1 || got.Inputs[0].Cycle != "h2" {
		t.Errorf("matchFrameworks [core] should select sam: inputs = %+v", got.Inputs)
	}
}

func TestTokenCollisionAcrossIncludes(t *testing.T) {
	rs := levelled()
	spec := rs[2].Object["spec"].(map[string]any)
	spec["competencies"] = append(spec["competencies"].([]any), comp("eng.chat", "Chat", []any{"comms"}, nil))
	if _, r := NewIndex(rs).Competency("sam", "comms"); r != Duplicate {
		t.Errorf("comms = %v, want Duplicate", r)
	}
}

func TestMatchFrameworksNotIncluded(t *testing.T) {
	rs := levelled()
	rs[4].Object["spec"].(map[string]any)["selector"] = map[string]any{"matchFrameworks": []any{"other"}}
	if names := NewIndex(rs).InputNames("sam"); len(names) != 0 {
		t.Errorf("unselected person has inputs %v", names)
	}
}

func TestFrameworks(t *testing.T) {
	got := Frameworks(levelled())
	comm := facts.Competency{ID: "core.communication", Title: "Communication", Theme: "core", Summary: "",
		Levels: map[string]string{"IC1": "says things", "IC2": "says things well"}}
	code := facts.Competency{ID: "eng.code", Title: "Code", Theme: "eng", Summary: "", Levels: map[string]string{"IC2": "writes code"}}
	want := map[string]facts.Framework{
		"core":                 {Levels: []string{"IC1", "IC2"}, Competencies: map[string]facts.Competency{"core.communication": comm}},
		"software-engineering": {Levels: []string{"IC1", "IC2"}, Competencies: map[string]facts.Competency{"core.communication": comm, "eng.code": code}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Frameworks =\n%+v\nwant\n%+v", got, want)
	}
}
