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

# evs(polarity, strength, date, file, line) is ev with an explicit strength
# (2 for ++ and --).
evs(pol, strength, date, file, line) := object.union(ev(pol, date, file, line), {"strength": strength})

# with_settings(min, scoring) is no_cycle under a cycle (window
# 2026-01-01..2026-12-31) whose evidence settings are min and scoring.
with_settings(min, scoring) := doc(framework, {"sam": sam}, {"c": object.union(h2, {
	"metadata": {"name": "c"},
	"spec": {
		"period": {"from": "2026-01-01", "to": "2026-12-31"},
		"evidence": {"minPerCompetency": min, "scoring": scoring},
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
	derive.settings.sam == {"minPerCompetency": 1, "scoring": "weighted"} with input as no_cycle
}

test_settings_default_when_cycle_has_no_evidence if {
	derive.settings.sam == {"minPerCompetency": 1, "scoring": "weighted"} with input as with_cycle
}

test_settings_from_cycle if {
	derive.settings.sam == {"minPerCompetency": 2, "scoring": "latest"} with input as with_settings(2, "latest")
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

# --- status: latest -------------------------------------------------------

# latest(es) is sam's ownership status under the latest rule (min 1).
latest(es) := own_status(with_evidence(with_settings(1, "latest"), es))

test_status_latest_no_marks_unproven if {
	latest([]) == {"status": "unproven", "positives": 0, "negatives": 0, "score": 0, "decidedBy": null, "rule": "latest"}
}

test_status_latest_positive_proven if {
	latest([ev(1, "2026-10-01", "n.md", 4)]) == {
		"status": "proven", "positives": 1, "negatives": 0, "score": 1,
		"decidedBy": {"file": "n.md", "line": 4}, "rule": "latest",
	}
}

test_status_latest_positive_then_negative_disproven if {
	s := latest([ev(-1, "2026-10-02", "n.md", 8), ev(1, "2026-10-01", "n.md", 4)])
	s.status == "disproven"
	s.decidedBy == {"file": "n.md", "line": 8}
	s.positives == 1
	s.negatives == 1
	s.score == 0
}

test_status_latest_negative_then_positive_proven if {
	s := latest([ev(1, "2026-10-02", "n.md", 8), ev(-1, "2026-10-01", "n.md", 4)])
	s.status == "proven"
	s.decidedBy == {"file": "n.md", "line": 8}
}

# Ten negatives and one later positive: latest says proven, weighted -9.
test_status_latest_ten_negatives_then_positive_proven if {
	es := array.concat([ev(-1, sprintf("2026-10-%02d", [i]), "n.md", i) | some i in numbers.range(1, 10)], [ev(1, "2026-10-11", "n.md", 11)])
	s := latest(es)
	s.status == "proven"
	s.score == -9
}

test_status_latest_same_date_ordered_by_file if {
	s := latest([ev(-1, "2026-10-02", "a.md", 9), ev(1, "2026-10-02", "b.md", 1)])
	s.status == "proven"
	s.decidedBy == {"file": "b.md", "line": 1}
}

test_status_latest_same_date_and_file_ordered_by_line if {
	s := latest([ev(-1, "2026-10-02", "a.md", 10), ev(1, "2026-10-02", "a.md", 9)])
	s.status == "disproven"
	s.decidedBy == {"file": "a.md", "line": 10}
}

test_status_latest_min2_single_positive_unproven if {
	s := own_status(with_evidence(with_settings(2, "latest"), [ev(1, "2026-10-01", "n.md", 1)]))
	s.status == "unproven"
	s.decidedBy == null
	s.rule == "latest"
}

# --- status: weighted -----------------------------------------------------

# done(addr, date, line) is a completed action of sam's.
done(addr, date, line) := object.union(act(addr, date, line), {"done": true})

own := ["delivery.ownership"]

# weighted(es, acts) is sam's ownership status by default settings (no
# cycle: weighted, min 1) with evidence es and actions acts.
weighted(es, acts) := own_status(object.union(with_evidence(no_cycle, es), {"actions": acts}))

scored(es, acts) := s if {
	s := derive.scored.sam["delivery.ownership"] with input as object.union(with_evidence(no_cycle, es), {"actions": acts})
}

test_status_weighted_no_marks_unproven if {
	weighted([], []) == {"status": "unproven", "positives": 0, "negatives": 0, "score": 0, "decidedBy": null, "rule": "weighted"}
}

test_status_weighted_ten_negatives_one_positive_disproven if {
	es := array.concat([ev(-1, sprintf("2026-10-%02d", [i]), "n.md", i) | some i in numbers.range(1, 10)], [ev(1, "2026-10-11", "n.md", 11)])
	s := weighted(es, [])
	s.status == "disproven"
	s.score == -9
	s.positives == 1
	s.negatives == 10
	s.decidedBy == null
}

plus_plus_strong_minus := [
	ev(1, "2026-10-01", "n.md", 1),
	ev(1, "2026-10-02", "n.md", 2),
	evs(-1, 2, "2026-10-03", "n.md", 3),
]

test_status_weighted_completed_action_neutralises if {
	s := weighted(plus_plus_strong_minus, [done(own, "2026-10-04", 9)])
	s.status == "proven"
	s.score == 2
}

test_status_weighted_open_action_does_not_neutralise if {
	s := weighted(plus_plus_strong_minus, [act(own, "2026-10-04", 9)])
	s.status == "unproven"
	s.score == 0
}

test_status_weighted_plus_then_strong_minus if {
	es := [ev(1, "2026-10-01", "n.md", 1), evs(-1, 2, "2026-10-03", "n.md", 3)]
	open := weighted(es, [act(own, "2026-10-04", 9)])
	open.status == "disproven"
	open.score == -1
	ticked := weighted(es, [done(own, "2026-10-04", 9)])
	ticked.status == "proven"
	ticked.score == 1
}

test_status_weighted_strong_positive_alone_proven if {
	s := weighted([evs(1, 2, "2026-10-01", "n.md", 1)], [])
	s.status == "proven"
	s.score == 2
}

test_status_weighted_min2_single_strong_positive_unproven if {
	s := own_status(with_evidence(with_settings(2, "weighted"), [evs(1, 2, "2026-10-01", "n.md", 1)]))
	s.status == "unproven"
	s.score == 2
	s.rule == "weighted"
}

test_status_weighted_action_before_negative_does_not_neutralise if {
	s := weighted([evs(-1, 2, "2026-10-05", "n.md", 5)], [done(own, "2026-10-04", 9)])
	s.status == "disproven"
	s.score == -2
	[m.neutralisedBy | some m in scored([evs(-1, 2, "2026-10-05", "n.md", 5)], [done(own, "2026-10-04", 9)])] == [null]
}

test_status_weighted_one_action_neutralises_most_recent_eligible if {
	es := [ev(-1, "2026-10-02", "n.md", 2), ev(-1, "2026-10-06", "n.md", 6)]
	after_both := scored(es, [done(own, "2026-10-08", 9)])
	[[m.line, m.weight] | some m in after_both] == [[6, 0], [2, -1]]
	weighted(es, [done(own, "2026-10-08", 9)]).score == -1

	# Only the older negative is eligible for an action between the two.
	between := scored(es, [done(own, "2026-10-04", 9)])
	[[m.line, m.weight] | some m in between] == [[6, -1], [2, 0]]
}

test_status_weighted_action_after_window_before_now_neutralises if {
	d := object.union(with_evidence(with_cycle, [ev(-1, "2026-09-01", "n.md", 1)]), {"actions": [done(own, "2026-10-10", 9)]})
	s := own_status(d)
	s.score == 0
	s.status == "unproven"
	later := object.union(d, {"actions": [done(own, "2026-10-25", 9)]})
	own_status(later).score == -1
}

test_status_weighted_action_for_other_competency_or_person_ignored if {
	other := done(["collab.planning"], "2026-10-08", 9)
	theirs := object.union(done(own, "2026-10-08", 9), {"person": "jo"})
	weighted([ev(-1, "2026-10-02", "n.md", 2)], [other, theirs]).score == -1
}

test_scored_newest_first_with_weights_and_pairing if {
	es := [
		ev(1, "2026-10-01", "n.md", 1),
		ev(-1, "2026-10-02", "n.md", 2),
		evs(-1, 2, "2026-10-06", "n.md", 6),
		evs(1, 2, "2026-10-07", "n.md", 7),
	]
	acts := [
		object.union(done(own, "2026-10-09", 19), {"text": "second"}),
		object.union(done(own, "2026-10-07", 17), {"text": "first"}),
	]
	s := scored(es, acts)
	[[m.line, m.weight] | some m in s] == [[7, 2], [6, 0], [2, 0], [1, 1]]
	[m.neutralisedBy | some m in s] == [
		null,
		{"file": "n.md", "line": 19, "text": "second"},
		{"file": "n.md", "line": 17, "text": "first"},
		null,
	]
	s[0].strength == 2
	weighted(es, acts).score == 3
}

# Same-date actions pair in [date, file, line] order, newest first.
test_scored_pairing_ties_by_file_and_line if {
	es := [ev(-1, "2026-10-02", "n.md", 2), ev(-1, "2026-10-03", "n.md", 3)]
	acts := [done(own, "2026-10-09", 4), done(own, "2026-10-09", 8)]
	[m.neutralisedBy.line | some m in scored(es, acts)] == [8, 4]
}

# Ruling L4: each completed action neutralises at most one negative dated on
# or before it, newest first. An action dated before a negative never
# neutralises it, even when a newer action has already been used.
l4_negatives := [
	evs(-1, 2, "2026-09-20", "n.md", 20),
	ev(-1, "2026-10-02", "n.md", 2),
	ev(-1, "2026-10-06", "n.md", 6),
]

l4_actions := [done(own, "2026-10-01", 101), done(own, "2026-10-08", 108)]

test_scored_l4_date_valid_matching if {
	s := scored(l4_negatives, l4_actions)
	[[m.line, m.weight] | some m in s] == [[6, 0], [2, -1], [20, 0]]
	[_by_line(m) | some m in s] == [108, null, 101]
}

test_status_l4_plus_plus_with_matching_proven if {
	es := array.concat([ev(1, "2026-08-01", "n.md", 30), ev(1, "2026-08-02", "n.md", 31)], l4_negatives)
	s := weighted(es, l4_actions)
	s.status == "proven"
	s.score == 1
}

# _by_line(m) is the line of the action that neutralised m, or null.
_by_line(m) := m.neutralisedBy.line if m.neutralisedBy != null else := null

# _action_date(acts, ref) is the date of the action ref points at.
_action_date(acts, ref) := a.date if {
	some a in acts
	a.line == ref.line
}

# Every neutralising action is dated on or after the negative it neutralises.
test_scored_pairings_are_date_valid if {
	cases := [
		[l4_negatives, l4_actions],
		[
			[ev(-1, "2026-10-01", "n.md", 1), ev(-1, "2026-10-03", "n.md", 3), ev(-1, "2026-10-05", "n.md", 5), ev(-1, "2026-10-07", "n.md", 7)],
			[done(own, "2026-10-02", 102), done(own, "2026-10-04", 104), done(own, "2026-10-08", 108)],
		],
		[
			[ev(-1, "2026-10-05", "n.md", 5), ev(-1, "2026-10-06", "n.md", 6)],
			[done(own, "2026-10-01", 101), done(own, "2026-10-02", 102), done(own, "2026-10-07", 107)],
		],
	]
	every cs in cases {
		every m in scored(cs[0], cs[1]) {
			_valid_pair(cs[1], m)
		}
	}
	# Second case: 10-07 <- 10-08, 10-03 <- 10-04, 10-01 <- 10-02; 10-05 stays.
	[_by_line(m) | some m in scored(cases[1][0], cases[1][1])] == [108, null, 104, 102]
	# Third: only 10-07 can neutralise, and it takes the newest negative.
	[_by_line(m) | some m in scored(cases[2][0], cases[2][1])] == [107, null]
}

_valid_pair(_, m) if m.neutralisedBy == null

_valid_pair(acts, m) if _action_date(acts, m.neutralisedBy) >= m.date

test_status_weighted_outside_window_ignored if {
	weighted([ev(1, "2026-07-01", "n.md", 1)], []).status == "unproven"
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
