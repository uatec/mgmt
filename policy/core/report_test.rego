package mgmt.report_test

import data.mgmt.report

# --- fixtures -------------------------------------------------------------

framework := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "CompetencyFramework",
	"metadata": {"name": "eng"},
	"spec": {
		"competencies": [
			{"id": "delivery.ownership", "title": "Owns", "tokens": ["ownership"]},
			{"id": "collab.planning", "title": "Plans", "tokens": ["planning"]},
			{"id": "scope.crossteam", "title": "Spans", "tokens": ["crossteam"]},
		],
		"roles": [{
			"name": "platform-engineer",
			"levels": [
				{"name": "L2", "expects": ["delivery.ownership"]},
				{"name": "L3", "inherits": "L2", "expects": ["collab.planning", "scope.crossteam"]},
			],
		}],
	},
}

sam := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "Person",
	"metadata": {"name": "sam"},
	"spec": {"displayName": "Sam Example", "framework": "eng", "role": "platform-engineer", "level": "L3", "since": "2026-05-01"},
}

workspace := {"kind": "Workspace", "metadata": {"name": "ws"}, "spec": {"notes": "notes/*.md", "me": ["me"], "defaultWindow": "90d"}}

h2 := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "ReviewCycle",
	"metadata": {"name": "ensek-2026-h2"},
	"spec": {
		"framework": "eng",
		"period": {"from": "2026-04-01", "to": "2026-09-30"},
		"selector": {"people": ["sam"]},
		"inputs": [
			{"name": "self-assessment", "required": true},
			{"name": "peer-feedback", "minCount": 3},
			{"name": "okrs", "required": true},
		],
		"milestones": [{"name": "peer-nominations", "due": "2026-10-27"}],
	},
}

doc(people, cycles) := {
	"now": "2026-10-21",
	"options": {"cycle": ""},
	"resources": {"Workspace": {"ws": workspace}, "CompetencyFramework": {"eng": framework}, "Person": people, "ReviewCycle": cycles},
	"sources": {},
	"evidence": [],
	"touches": [],
	"actions": [],
	"inputs": [],
	"sections": [],
}

base := doc({"sam": sam}, {"ensek-2026-h2": h2})

action(owner, due, file, line, done) := {
	"id": sprintf("%s:%d", [file, line]), "person": "sam", "owner": owner, "due": due, "done": done,
	"addresses": [], "date": "2026-10-01", "file": file, "line": line, "text": sprintf("a%d", [line]),
}

with_records(key, records) := json.patch(base, [{"op": "replace", "path": sprintf("/%s", [key]), "value": records}])

locs(items) := [sprintf("%s:%d", [i.file, i.line]) | some i in items]

# --- person and empty data ------------------------------------------------

test_person_and_empty_lists if {
	p := report.prep.sam with input as base
	p.person == {"name": "sam", "displayName": "Sam Example", "role": "platform-engineer", "level": "L3"}
	p.window == {"from": "2026-05-01", "to": "2026-09-30", "cycle": "ensek-2026-h2"}
	p.actions == {"mine": [], "theirs": []}
	p.lastSection == null
	p.since == []
	p.gaps == [
		{"competency": "collab.planning", "title": "Plans", "status": "unproven", "addressedBy": []},
		{"competency": "delivery.ownership", "title": "Owns", "status": "unproven", "addressedBy": []},
		{"competency": "scope.crossteam", "title": "Spans", "status": "unproven", "addressedBy": []},
	]
}

test_display_name_defaults_to_name if {
	bare := json.patch(base, [{"op": "remove", "path": "/resources/Person/sam/spec/displayName"}])
	report.prep.sam.person.displayName == "sam" with input as bare
}

# R8: a person whose framework is missing still gets a total view-model.
test_missing_framework_is_total if {
	broken := json.patch(base, [{"op": "replace", "path": "/resources/Person/sam/spec/framework", "value": "nope"}])
	p := report.prep.sam with input as broken
	p.gaps == []
	p.focus == null
	p.actions == {"mine": [], "theirs": []}
}

# --- actions --------------------------------------------------------------

test_actions_overdue_then_dated_then_no_due if {
	acts := [
		action("me", "", "notes/a.md", 1, false),
		action("me", "2026-10-30", "notes/a.md", 2, false),
		action("me", "2026-10-20", "notes/b.md", 3, false),
		action("me", "2026-10-20", "notes/a.md", 4, false),
		action("me", "2026-10-01", "notes/a.md", 5, true),
		action("sam", "2026-10-23", "notes/a.md", 6, false),
	]
	p := report.prep.sam with input as with_records("actions", acts)
	locs(p.actions.mine) == ["notes/a.md:4", "notes/b.md:3", "notes/a.md:2", "notes/a.md:1"]
	[a.overdue | some a in p.actions.mine] == [true, true, false, false]
	locs(p.actions.theirs) == ["notes/a.md:6"]
	p.actions.theirs[0].overdue == false
	p.actions.theirs[0].text == "a6"
}

test_due_today_is_not_overdue if {
	p := report.prep.sam with input as with_records("actions", [action("me", "2026-10-21", "notes/a.md", 1, false)])
	p.actions.mine[0].overdue == false
}

# --- since last 1:1 -------------------------------------------------------

section(date, line) := {"person": "sam", "date": date, "file": "notes/s.md", "line": line, "tokenCount": 1}

test_last_section_is_latest_before_now if {
	secs := [section("2026-10-05", 1), section("2026-10-12", 2), section("2026-10-21", 3)]
	report.prep.sam.lastSection == "2026-10-12" with input as with_records("sections", secs)
}

mark(pol, date, file, line) := {"person": "sam", "competency": "collab.planning", "polarity": pol, "date": date, "file": file, "line": line, "text": "t"}

touch(date, file, line) := {"person": "sam", "competency": "scope.crossteam", "date": date, "file": file, "line": line, "text": "t"}

test_since_after_last_section_merged_and_sorted if {
	facts := json.patch(base, [
		{"op": "replace", "path": "/sections", "value": [section("2026-10-12", 1)]},
		{"op": "replace", "path": "/evidence", "value": [
			mark(1, "2026-10-12", "notes/a.md", 1),
			mark(-1, "2026-10-19", "notes/b.md", 2),
			mark(1, "2026-10-22", "notes/c.md", 3),
		]},
		{"op": "replace", "path": "/touches", "value": [touch("2026-10-19", "notes/a.md", 9), touch("2026-10-13", "notes/z.md", 1)]},
	])
	p := report.prep.sam with input as facts
	locs(p.since) == ["notes/z.md:1", "notes/a.md:9", "notes/b.md:2"]
	p.since[0] == {"kind": "touch", "polarity": 0, "competency": "scope.crossteam", "date": "2026-10-13", "text": "t", "file": "notes/z.md", "line": 1}
	p.since[2].kind == "mark"
	p.since[2].polarity == -1
}

test_since_without_section_starts_at_since_date if {
	facts := with_records("evidence", [mark(1, "2026-04-30", "notes/a.md", 1), mark(1, "2026-05-01", "notes/b.md", 1)])
	locs(report.prep.sam.since) == ["notes/b.md:1"] with input as facts
}

# --- gaps -----------------------------------------------------------------

test_gaps_list_open_addressing_actions_only if {
	open := object.union(action("me", "", "notes/a.md", 1, false), {"addresses": ["collab.planning"]})
	closed := object.union(action("me", "", "notes/a.md", 2, true), {"addresses": ["collab.planning"]})
	facts := json.patch(base, [
		{"op": "replace", "path": "/actions", "value": [closed, open]},
		{"op": "replace", "path": "/evidence", "value": [mark(-1, "2026-09-01", "notes/m.md", 1)]},
	])
	p := report.prep.sam with input as facts
	p.gaps[0] == {"competency": "collab.planning", "title": "Plans", "status": "disproven", "addressedBy": [{"file": "notes/a.md", "line": 1, "text": "a1"}]}
	p.gaps[1].addressedBy == []
}

# --- focus ----------------------------------------------------------------

test_focus_never_touched_first_by_id if {
	facts := with_records("touches", [object.union(touch("2026-08-03", "notes/a.md", 1), {"competency": "collab.planning"})])
	p := report.prep.sam with input as facts
	p.focus == {"competency": "delivery.ownership", "title": "Owns", "lastTouch": null}
}

test_focus_oldest_touch_when_all_touched if {
	facts := with_records("touches", [
		object.union(touch("2026-09-01", "notes/a.md", 1), {"competency": "collab.planning"}),
		object.union(touch("2026-08-03", "notes/a.md", 2), {"competency": "scope.crossteam"}),
		object.union(touch("2026-10-01", "notes/a.md", 3), {"competency": "delivery.ownership"}),
	])
	p := report.prep.sam with input as facts
	p.focus == {"competency": "scope.crossteam", "title": "Spans", "lastTouch": "2026-08-03"}
}

# --- cycle ----------------------------------------------------------------

test_cycle_missing_inputs_sorted_by_name if {
	inp := {"person": "sam", "input": "peer-feedback", "cycle": "ensek-2026-h2", "date": "2026-10-19", "file": "notes/a.md", "line": 1}
	p := report.prep.sam with input as with_records("inputs", [inp])
	p.cycle == {
		"name": "ensek-2026-h2",
		"nextMilestone": {"name": "peer-nominations", "due": "2026-10-27"},
		"missing": [
			{"name": "okrs", "received": 0, "needed": 1},
			{"name": "peer-feedback", "received": 1, "needed": 3},
			{"name": "self-assessment", "received": 0, "needed": 1},
		],
	}
}

test_cycle_without_milestones_has_null_next if {
	# Without milestones the cycle ends with its period, so select it explicitly.
	facts := json.patch(base, [
		{"op": "replace", "path": "/resources/ReviewCycle/ensek-2026-h2/spec/milestones", "value": []},
		{"op": "replace", "path": "/options/cycle", "value": "ensek-2026-h2"},
	])
	p := report.prep.sam with input as facts
	p.cycle.name == "ensek-2026-h2"
	p.cycle.nextMilestone == null
}

test_no_cycle_is_null if {
	p := report.prep.sam with input as doc({"sam": sam}, {})
	p.cycle == null
	p.window.cycle == null
}

# --- review ---------------------------------------------------------------

test_review_groups_required_by_theme_newest_first if {
	open := object.union(action("me", "", "notes/a.md", 1, false), {"addresses": ["collab.planning"], "date": "2026-09-30"})
	closed := object.union(action("sam", "", "notes/a.md", 2, true), {"addresses": ["collab.planning"], "date": "2026-05-01"})
	before := object.union(action("me", "", "notes/a.md", 7, true), {"addresses": ["collab.planning"], "date": "2026-04-30"})
	after := object.union(action("me", "", "notes/a.md", 8, false), {"addresses": ["collab.planning"]}) # 2026-10-01
	facts := json.patch(base, [
		{"op": "replace", "path": "/actions", "value": [open, closed, before, after]},
		{"op": "replace", "path": "/evidence", "value": [
			mark(1, "2026-08-10", "notes/b.md", 6),
			mark(-1, "2026-09-14", "notes/c.md", 6),
			mark(1, "2026-08-10", "notes/a.md", 3),
			mark(1, "2026-10-19", "notes/d.md", 1), # outside the window
		]},
	])
	r := report.review.sam with input as facts
	r.person.displayName == "Sam Example"
	r.cycle == "ensek-2026-h2"
	r.window == {"from": "2026-05-01", "to": "2026-09-30", "cycle": "ensek-2026-h2"}
	[t.theme | some t in r.themes] == ["collab", "delivery", "scope"]
	[c.id | some t in r.themes; some c in t.competencies] == ["collab.planning", "delivery.ownership", "scope.crossteam"]
	planning := r.themes[0].competencies[0]
	planning.title == "Plans"
	planning.status == "disproven"
	locs(planning.positive) == ["notes/b.md:6", "notes/a.md:3"]
	locs(planning.negative) == ["notes/c.md:6"]
	# R14: only actions dated within the window (2026-05-01..2026-09-30).
	locs(planning.actions) == ["notes/a.md:2", "notes/a.md:1"]
	r.themes[1].competencies[0].positive == []
	r.themes[1].competencies[0].actions == []
}

test_review_inputs_sorted_by_name if {
	inp := {"person": "sam", "input": "peer-feedback", "cycle": "ensek-2026-h2", "date": "2026-10-19", "file": "notes/a.md", "line": 1, "text": "t"}
	r := report.review.sam with input as with_records("inputs", [inp])
	[i.name | some i in r.inputs] == ["okrs", "peer-feedback", "self-assessment"]
	r.inputs[1].received == 1
	locs(r.inputs[1].records) == ["notes/a.md:1"]
}

# Decision L: without a cycle the rolling window applies and inputs are null.
test_review_without_cycle if {
	r := report.review.sam with input as doc({"sam": sam}, {})
	r.cycle == null
	r.inputs == null
	r.defaultWindow == "90d"
	r.window == {"from": "2026-07-23", "to": "2026-10-21", "cycle": null}
}

test_review_missing_framework_is_total if {
	broken := json.patch(base, [{"op": "replace", "path": "/resources/Person/sam/spec/framework", "value": "nope"}])
	r := report.review.sam with input as broken
	r.themes == []
}

# --- explain --------------------------------------------------------------

explain_marks := [
	mark(-1, "2026-09-14", "notes/c.md", 6),
	mark(1, "2026-08-10", "notes/b.md", 6),
	mark(1, "2026-09-01", "notes/b.md", 9),
	mark(-1, "2026-10-19", "notes/d.md", 6), # outside the window
	mark(1, "2026-04-02", "notes/e.md", 1), # before since: neither
]

test_explain_negative_overrides_latest_decides if {
	e := report.explain.sam["collab.planning"] with input as with_records("evidence", explain_marks)
	e.person.level == "L3"
	e.competency == {"id": "collab.planning", "title": "Plans"}
	e.required == true
	e.status == "disproven"
	e.rule == "negativeOverrides"
	e.settings == {"minPerCompetency": 1, "negativeOverrides": true}
	[e.positives, e.negatives] == [2, 1]
	e.window.from == "2026-05-01"
	locs(e.considered) == ["notes/c.md:6", "notes/b.md:9", "notes/b.md:6"]
	[c.decides | some c in e.considered] == [true, false, false]
	locs(e.outside) == ["notes/d.md:6"]
}

test_explain_counting_no_single_item_decides if {
	facts := json.patch(with_records("evidence", explain_marks), [{
		"op": "add", "path": "/resources/ReviewCycle/ensek-2026-h2/spec/evidence",
		"value": {"minPerCompetency": 2, "negativeOverrides": false},
	}])
	e := report.explain.sam["collab.planning"] with input as facts
	e.status == "proven"
	e.rule == "counting"
	e.settings.minPerCompetency == 2
	[c.decides | some c in e.considered] == [false, false, false]
}

test_explain_not_required_and_empty if {
	no_scope := json.patch(base, [{"op": "replace", "path": "/resources/Person/sam/spec/level", "value": "L2"}])
	e := report.explain.sam["scope.crossteam"] with input as no_scope
	e.required == false
	e.status == "unproven"
	e.considered == []
	e.outside == []
}
