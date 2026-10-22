package mgmt.derive_test

import data.mgmt.derive

# --- fixtures -------------------------------------------------------------

framework := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "CompetencyFramework",
	"metadata": {"name": "eng"},
	"spec": {
		"levels": ["IC2", "IC3", "IC4"],
		"competencies": [
			{"id": "delivery.ownership", "title": "Owns", "tokens": ["ownership"], "summary": "Owns results", "levels": {"IC2": "own2", "IC3": "own3", "IC4": "own4"}},
			{"id": "collab.planning", "title": "Plans", "tokens": ["planning"], "levels": {"IC3": "plan3", "IC4": "plan4"}},
			{"id": "scope.crossteam", "title": "Spans", "tokens": ["crossteam"], "levels": {"IC3": "span3", "IC4": "span4"}},
			{"id": "collab.mentoring", "title": "Mentors"},
		],
	},
}

# effective(fw) is the facts.frameworks entry for a framework without includes.
effective(fw) := {
	"levels": fw.spec.levels,
	"competencies": {c.id: {
		"id": c.id, "title": c.title, "theme": split(c.id, ".")[0],
		"summary": object.get(c, "summary", ""), "levels": object.get(c, "levels", {}),
	} |
		some c in fw.spec.competencies
	},
}

person(level, since) := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "Person",
	"metadata": {"name": "sam"},
	"spec": {"framework": "eng", "level": level, "since": since},
}

sam := person("IC3", "2026-01-01")

workspace := {"kind": "Workspace", "metadata": {"name": "ws"}, "spec": {"notes": "notes/*.md", "me": [], "defaultWindow": "90d"}}

h2 := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "ReviewCycle",
	"metadata": {"name": "ensek-2026-h2"},
	"spec": {
		"period": {"from": "2026-04-01", "to": "2026-09-30"},
		"selector": {"people": ["sam"]},
		"inputs": [
			{"name": "self-assessment", "required": true},
			{"name": "peer-feedback", "minCount": 3},
			{"name": "okrs"},
		],
		"milestones": [
			{"name": "peer-nominations", "due": "2026-10-27"},
			{"name": "conversations", "due": "2026-11-28"},
		],
	},
}

doc(fw, people, cycles) := {
	"now": "2026-10-21",
	"options": {"cycle": ""},
	"resources": {"Workspace": {"ws": workspace}, "CompetencyFramework": {"eng": fw}, "Person": people, "ReviewCycle": cycles},
	"frameworks": {"eng": effective(fw)},
	"sources": {},
	"evidence": [],
	"touches": [],
	"actions": [],
	"inputs": [],
	"sections": [],
}

no_cycle := doc(framework, {"sam": sam}, {})

with_cycle := doc(framework, {"sam": sam}, {"ensek-2026-h2": h2})

# ev(polarity, date, file, line) is an evidence record for sam's ownership.
ev(pol, date, file, line) := {
	"person": "sam", "competency": "delivery.ownership", "polarity": pol,
	"date": date, "file": file, "line": line, "text": "t",
}

with_evidence(d, es) := object.union(d, {"evidence": es})

# with_settings(min, neg) is no_cycle under a cycle (window 2026-01-01..2026-12-31)
# whose evidence settings are min and neg.
with_settings(min, neg) := doc(framework, {"sam": sam}, {"c": object.union(h2, {
	"metadata": {"name": "c"},
	"spec": {
		"period": {"from": "2026-01-01", "to": "2026-12-31"},
		"evidence": {"minPerCompetency": min, "negativeOverrides": neg},
	},
})})

own_status(d) := s if {
	s := derive.status.sam["delivery.ownership"] with input as d
}

# --- required -------------------------------------------------------------

test_required_at_level_is_competencies_with_a_descriptor if {
	derive.required.sam == ["collab.planning", "delivery.ownership", "scope.crossteam"] with input as no_cycle
	derive.required.sam == ["delivery.ownership"] with input as doc(framework, {"sam": person("IC2", "2026-01-01")}, {})
}

test_required_level_off_ladder_is_empty if {
	d := doc(framework, {"sam": person("IC9", "2026-01-01")}, {})
	derive.required.sam == [] with input as d
	not derive.next_level.sam with input as d
	not derive.descriptor.sam["delivery.ownership"] with input as d
	derive.status.sam["delivery.ownership"].status == "unproven" with input as d
}

test_ladder_level_next_level if {
	derive.ladder.sam == ["IC2", "IC3", "IC4"] with input as no_cycle
	derive.level.sam == "IC3" with input as no_cycle
	derive.next_level.sam == "IC4" with input as no_cycle
	not derive.next_level.sam with input as doc(framework, {"sam": person("IC4", "2026-01-01")}, {})
}

test_descriptor_at_current_level if {
	derive.descriptor.sam["collab.planning"] == "plan3" with input as no_cycle
	not derive.descriptor.sam["collab.mentoring"] with input as no_cycle
	derive.descriptor.sam == {"collab.planning": "plan3", "delivery.ownership": "own3", "scope.crossteam": "span3"} with input as no_cycle
}

# --- competencies ---------------------------------------------------------

test_competencies_have_theme_summary_and_levels if {
	derive.competencies.sam["collab.planning"] == {
		"id": "collab.planning", "title": "Plans", "theme": "collab", "summary": "",
		"levels": {"IC3": "plan3", "IC4": "plan4"},
	} with input as no_cycle
	derive.competencies.sam["delivery.ownership"].summary == "Owns results" with input as no_cycle
	count(derive.competencies.sam) == 4 with input as no_cycle
}

# --- window and settings --------------------------------------------------

test_window_cycle if {
	derive.window.sam == {"from": "2026-04-01", "to": "2026-09-30", "cycle": "ensek-2026-h2"} with input as with_cycle
}

test_window_rolling if {
	derive.window.sam == {"from": "2026-07-23", "to": "2026-10-21", "cycle": null} with input as no_cycle
}

test_window_raised_to_since if {
	d := doc(framework, {"sam": person("IC3", "2026-09-01")}, {})
	derive.window.sam == {"from": "2026-09-01", "to": "2026-10-21", "cycle": null} with input as d
}

test_window_cycle_raised_to_since if {
	d := doc(framework, {"sam": person("IC3", "2026-05-01")}, {"ensek-2026-h2": h2})
	derive.window.sam.from == "2026-05-01" with input as d
}

test_window_clamped_by_level_since if {
	promoted := object.union(sam, {"spec": {"levelSince": "2026-06-15"}})
	derive.window.sam == {"from": "2026-06-15", "to": "2026-09-30", "cycle": "ensek-2026-h2"} with input as doc(framework, {"sam": promoted}, {"ensek-2026-h2": h2})
	derive.window.sam.from == "2026-07-23" with input as doc(framework, {"sam": promoted}, {})
	late := object.union(sam, {"spec": {"levelSince": "2026-09-01"}})
	derive.window.sam.from == "2026-09-01" with input as doc(framework, {"sam": late}, {})
}

test_marks_before_level_since_are_outside if {
	promoted := object.union(sam, {"spec": {"levelSince": "2026-06-15"}})
	d := with_evidence(doc(framework, {"sam": promoted}, {"ensek-2026-h2": h2}), [ev(1, "2026-05-02", "n.md", 1), ev(-1, "2026-07-01", "n.md", 2)])
	[m.line | some m in derive.marks.sam["delivery.ownership"]] == [2] with input as d
	[m.line | some m in derive.outside.sam["delivery.ownership"]] == [1] with input as d
	own_status(d).status == "disproven"
}

test_settings_default_without_cycle if {
	derive.settings.sam == {"minPerCompetency": 1, "negativeOverrides": true} with input as no_cycle
}

test_settings_default_when_cycle_has_no_evidence if {
	derive.settings.sam == {"minPerCompetency": 1, "negativeOverrides": true} with input as with_cycle
}

test_settings_from_cycle if {
	derive.settings.sam == {"minPerCompetency": 2, "negativeOverrides": false} with input as with_settings(2, false)
}

# --- marks and outside ----------------------------------------------------

test_marks_in_window_sorted_outside_split if {
	d := with_evidence(with_cycle, [
		ev(1, "2026-10-19", "notes/b.md", 1),
		ev(1, "2026-05-02", "notes/b.md", 9),
		ev(-1, "2026-05-02", "notes/a.md", 3),
		ev(1, "2025-12-01", "notes/old.md", 1),
	])
	[m.line | some m in derive.marks.sam["delivery.ownership"]] == [3, 9] with input as d
	[m.file | some m in derive.outside.sam["delivery.ownership"]] == ["notes/b.md"] with input as d
	derive.marks.sam["collab.planning"] == [] with input as d
}

# --- status: negativeOverrides --------------------------------------------

test_status_no_marks_unproven if {
	own_status(no_cycle) == {"status": "unproven", "positives": 0, "negatives": 0, "decidedBy": null, "rule": "negativeOverrides"}
}

test_status_positive_proven if {
	own_status(with_evidence(no_cycle, [ev(1, "2026-10-01", "n.md", 4)])) == {
		"status": "proven", "positives": 1, "negatives": 0,
		"decidedBy": {"file": "n.md", "line": 4}, "rule": "negativeOverrides",
	}
}

test_status_positive_then_negative_disproven if {
	s := own_status(with_evidence(no_cycle, [ev(-1, "2026-10-02", "n.md", 8), ev(1, "2026-10-01", "n.md", 4)]))
	s.status == "disproven"
	s.decidedBy == {"file": "n.md", "line": 8}
	s.positives == 1
	s.negatives == 1
}

test_status_negative_then_positive_proven if {
	s := own_status(with_evidence(no_cycle, [ev(1, "2026-10-02", "n.md", 8), ev(-1, "2026-10-01", "n.md", 4)]))
	s.status == "proven"
	s.decidedBy == {"file": "n.md", "line": 8}
}

test_status_same_date_ordered_by_file if {
	s := own_status(with_evidence(no_cycle, [ev(-1, "2026-10-02", "a.md", 9), ev(1, "2026-10-02", "b.md", 1)]))
	s.status == "proven"
	s.decidedBy == {"file": "b.md", "line": 1}
}

test_status_same_date_and_file_ordered_by_line if {
	s := own_status(with_evidence(no_cycle, [ev(-1, "2026-10-02", "a.md", 10), ev(1, "2026-10-02", "a.md", 9)]))
	s.status == "disproven"
	s.decidedBy == {"file": "a.md", "line": 10}
}

test_status_marks_outside_window_ignored if {
	own_status(with_evidence(no_cycle, [ev(1, "2026-07-01", "n.md", 1)])).status == "unproven"
}

test_status_overrides_min2_single_positive_unproven if {
	s := own_status(with_evidence(with_settings(2, true), [ev(1, "2026-10-01", "n.md", 1)]))
	s.status == "unproven"
	s.decidedBy == null
	s.rule == "negativeOverrides"
}

# --- status: counting -----------------------------------------------------

test_status_counting_pos_pos_neg_proven if {
	s := own_status(with_evidence(with_settings(1, false), [
		ev(1, "2026-10-01", "n.md", 1), ev(1, "2026-10-02", "n.md", 1), ev(-1, "2026-10-03", "n.md", 1),
	]))
	s == {"status": "proven", "positives": 2, "negatives": 1, "decidedBy": null, "rule": "counting"}
}

test_status_counting_pos_neg_disproven if {
	s := own_status(with_evidence(with_settings(1, false), [ev(1, "2026-10-01", "n.md", 1), ev(-1, "2026-10-02", "n.md", 1)]))
	s.status == "disproven"
}

test_status_counting_neg_disproven if {
	own_status(with_evidence(with_settings(1, false), [ev(-1, "2026-10-02", "n.md", 1)])).status == "disproven"
}

test_status_counting_no_marks_unproven if {
	own_status(with_settings(1, false)).status == "unproven"
}

test_status_counting_min2_pos_neg_pos_proven if {
	s := own_status(with_evidence(with_settings(2, false), [
		ev(1, "2026-10-01", "n.md", 1), ev(-1, "2026-10-02", "n.md", 1), ev(1, "2026-10-03", "n.md", 1),
	]))
	s.status == "proven"
}

test_status_counting_min2_single_positive_unproven if {
	own_status(with_evidence(with_settings(2, false), [ev(1, "2026-10-01", "n.md", 1)])).status == "unproven"
}

# --- gaps -----------------------------------------------------------------

test_gaps_disproven_first_then_id if {
	d := with_evidence(no_cycle, [
		{"person": "sam", "competency": "scope.crossteam", "polarity": -1, "date": "2026-10-01", "file": "n.md", "line": 1, "text": "t"},
		ev(1, "2026-10-01", "n.md", 2),
	])
	derive.gaps.sam == [
		{"competency": "scope.crossteam", "status": "disproven"},
		{"competency": "collab.planning", "status": "unproven"},
	] with input as d
}

# --- last_touch -----------------------------------------------------------

test_last_touch_marks_and_touches if {
	d := object.union(with_evidence(with_cycle, [ev(1, "2026-10-01", "n.md", 1), ev(1, "2025-12-01", "n.md", 1)]), {"touches": [
		{"person": "sam", "competency": "delivery.ownership", "date": "2026-10-10", "file": "n.md", "line": 2, "text": "t"},
		{"person": "sam", "competency": "collab.planning", "date": "2026-11-01", "file": "n.md", "line": 3, "text": "t"},
	]})
	derive.last_touch.sam["delivery.ownership"] == "2026-10-10" with input as d
	not derive.last_touch.sam["collab.planning"] with input as d
}

# --- cycles ---------------------------------------------------------------

test_active_cycles_within_milestones if {
	derive.active_cycles == ["ensek-2026-h2"] with input as with_cycle
	derive.cycle.sam == "ensek-2026-h2" with input as with_cycle
}

test_active_cycles_after_last_milestone_empty if {
	d := object.union(with_cycle, {"now": "2026-12-01"})
	derive.active_cycles == [] with input as d
	not derive.cycle.sam with input as d
}

test_active_cycles_option_forces if {
	d := object.union(with_cycle, {"now": "2026-12-01", "options": {"cycle": "ensek-2026-h2"}})
	derive.active_cycles == ["ensek-2026-h2"] with input as d
	derive.cycle.sam == "ensek-2026-h2" with input as d
}

on(name, fw) := object.union(sam, {"metadata": {"name": name}, "spec": {"framework": fw}})

# matchFrameworks selects a person whose framework is listed or includes a
# listed framework, transitively; people still adds named people.
test_cycle_people_match_frameworks if {
	core := {"kind": "CompetencyFramework", "metadata": {"name": "core"}, "spec": {"levels": ["IC2"], "competencies": []}}
	base := {"kind": "CompetencyFramework", "metadata": {"name": "base"}, "spec": {"includes": ["core"], "levels": ["IC2"], "competencies": []}}
	se := {"kind": "CompetencyFramework", "metadata": {"name": "software-engineering"}, "spec": {"includes": ["base"], "levels": ["IC2"], "competencies": []}}
	other := {"kind": "CompetencyFramework", "metadata": {"name": "other"}, "spec": {"levels": ["IC2"], "competencies": []}}
	people := {"sam": on("sam", "software-engineering"), "co": on("co", "core"), "al": on("al", "other"), "jo": on("jo", "other")}
	c := object.union(h2, {"spec": {"selector": {"people": ["jo"], "matchFrameworks": ["core"]}}})
	d := json.patch(doc(framework, people, {"ensek-2026-h2": c}), [{"op": "replace", "path": "/resources/CompetencyFramework", "value": {
		"core": core, "base": base, "software-engineering": se, "other": other,
	}}])
	derive.cycle_people["ensek-2026-h2"] == ["co", "jo", "sam"] with input as d
	only_se := object.union(h2, {"spec": {"selector": {"matchFrameworks": ["software-engineering"]}}})
	derive.cycle_people["ensek-2026-h2"] == ["sam"] with input as json.patch(d, [{"op": "replace", "path": "/resources/ReviewCycle/ensek-2026-h2", "value": only_se}])
}

test_cycle_first_by_name if {
	d := doc(framework, {"sam": sam}, {"b": object.union(h2, {"metadata": {"name": "b"}}), "a": object.union(h2, {"metadata": {"name": "a"}})})
	derive.active_cycles == ["a", "b"] with input as d
	derive.cycle.sam == "a" with input as d
}

test_next_milestone if {
	derive.next_milestone["ensek-2026-h2"] == {"name": "peer-nominations", "due": "2026-10-27"} with input as with_cycle
	not derive.next_milestone["ensek-2026-h2"] with input as object.union(with_cycle, {"now": "2026-11-29"})
}

# --- inputs ---------------------------------------------------------------

rec(name, line) := {"person": "sam", "input": name, "cycle": "ensek-2026-h2", "date": "2026-10-01", "file": "n.md", "line": line}

test_inputs if {
	d := object.union(with_cycle, {"inputs": [
		rec("peer-feedback", 2), rec("peer-feedback", 1),
		object.union(rec("peer-feedback", 3), {"cycle": "other"}),
	]})
	ins := derive.inputs["ensek-2026-h2"].sam with input as d
	[i.name | some i in ins] == ["self-assessment", "peer-feedback", "okrs"]
	ins[0].required == true
	ins[0].complete == false
	ins[0].received == 0
	ins[1].complete == false
	ins[1].received == 2
	ins[1].minCount == 3
	ins[1].required == false
	[r.line | some r in ins[1].records] == [1, 2]
	ins[2].complete == true
	ins[2].records == []
}

# --- addressed_by ---------------------------------------------------------

act(addr, date, line) := {
	"id": "x", "person": "sam", "owner": "sam", "due": "", "done": false,
	"addresses": addr, "date": date, "file": "n.md", "line": line, "text": "t",
}

test_addressed_by if {
	d := object.union(no_cycle, {"actions": [
		act(["delivery.ownership"], "2026-10-02", 1),
		act(["delivery.ownership", "collab.planning"], "2026-10-01", 5),
		act([], "2026-10-01", 7),
	]})
	[a.line | some a in derive.addressed_by.sam["delivery.ownership"]] == [5, 1] with input as d
	derive.addressed_by.sam["scope.crossteam"] == [] with input as d
}

# --- totality -------------------------------------------------------------

test_empty_input_is_total if {
	derive.active_cycles == [] with input as {}
	count(derive.window) == 0 with input as {}
}

# A person whose framework is missing keeps a window, settings and cycle, but
# every framework-dependent rule is undefined for them (ref-unknown reports it).
test_missing_framework_person if {
	lost := object.union(sam, {"spec": {"framework": "nope"}})
	d := doc(framework, {"sam": lost}, {"ensek-2026-h2": h2})
	derive.window.sam.cycle == "ensek-2026-h2" with input as d
	derive.settings.sam.minPerCompetency == 1 with input as d
	derive.cycle.sam == "ensek-2026-h2" with input as d
	not derive.competencies.sam with input as d
	not derive.required.sam with input as d
	not derive.ladder.sam with input as d
	not derive.next_level.sam with input as d
	not derive.descriptor.sam with input as d
	not derive.marks.sam with input as d
	not derive.outside.sam with input as d
	not derive.status.sam with input as d
	not derive.gaps.sam with input as d
	not derive.last_touch.sam with input as d
	not derive.addressed_by.sam with input as d
	count(derive.inputs["ensek-2026-h2"].sam) == 3 with input as d
}
