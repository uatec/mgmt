package mgmt.policy.core_test

import data.mgmt.lib
import data.mgmt.policy.core

# --- fixtures -------------------------------------------------------------

framework := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "CompetencyFramework",
	"metadata": {"name": "eng"},
	"spec": {
		"levels": ["IC2", "IC3"],
		"competencies": [
			{"id": "delivery.ownership", "title": "Owns", "tokens": ["ownership", "own"], "levels": {"IC2": "o2", "IC3": "o3"}},
			{"id": "collab.planning", "title": "Plans", "tokens": ["planning", "plan"], "levels": {"IC3": "p3"}},
			{"id": "scope.crossteam", "title": "Spans", "tokens": ["crossteam", "xteam"]},
		],
	},
}

person(name, aliases, fw, level) := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "Person",
	"metadata": {"name": name},
	"spec": {
		"aliases": aliases,
		"framework": fw,
		"level": level,
		"since": "2026-05-01",
	},
}

cycle_with(selector) := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "ReviewCycle",
	"metadata": {"name": "h2"},
	"spec": {
		"period": {"from": "2026-04-01", "to": "2026-09-30"},
		"selector": selector,
		"milestones": [],
	},
}

cycle(people) := cycle_with({"people": people})

sources := {
	"CompetencyFramework/eng": {"file": "resources/framework.yaml", "line": 1},
	"CompetencyFramework/core": {"file": "resources/core.yaml", "line": 1},
	"Person/sam": {"file": "resources/people.yaml", "line": 1},
	"Person/priya": {"file": "resources/people.yaml", "line": 12},
	"ReviewCycle/h2": {"file": "resources/cycle.yaml", "line": 3},
}

sam := person("sam", ["Sam", "Samuel"], "eng", "IC3")

priya := person("priya", ["Priya"], "eng", "IC2")

doc(fw, people, cycles) := {
	"now": "2026-10-21",
	"resources": {"CompetencyFramework": fw, "Person": people, "ReviewCycle": cycles},
	"sources": sources,
}

valid := doc({"eng": framework}, {"sam": sam, "priya": priya}, {"h2": cycle(["sam", "priya"])})

ids(findings) := {f.id | some f in findings}

with_comp(fw, i, comp) := json.patch(fw, [{"op": "replace", "path": sprintf("/spec/competencies/%d", [i]), "value": comp}])

# --- valid input fires nothing --------------------------------------------

test_valid_has_no_denies if {
	count(core.deny) == 0 with input as valid
}

test_empty_input_has_no_denies if {
	count(core.deny) == 0 with input as {}
}

# --- ref-unknown ----------------------------------------------------------

test_ref_unknown_person_framework if {
	d := doc({"eng": framework}, {"sam": person("sam", [], "nope", "IC3")}, {})
	fs := core.deny with input as d
	some f in fs
	f.id == "ref-unknown"
	f.person == "sam"
	f.file == "resources/people.yaml"
	f.line == 1
	contains(f.message, `"nope"`)
	f.msg == f.message
	count(fs) == 1
}

test_ref_unknown_person_level_off_ladder if {
	d := doc({"eng": framework}, {"priya": person("priya", [], "eng", "IC9")}, {})
	fs := core.deny with input as d
	some f in fs
	f.id == "ref-unknown"
	f.person == "priya"
	f.line == 12
	f.message == `person "priya": level "IC9" is not on the ladder of framework "eng" (IC2, IC3)`
	count(fs) == 1
}

test_ref_unknown_cycle_match_frameworks if {
	d := doc({"eng": framework}, {"sam": sam}, {"h2": cycle_with({"matchFrameworks": ["eng", "nope"]})})
	fs := core.deny with input as d
	some f in fs
	f.id == "ref-unknown"
	f.file == "resources/cycle.yaml"
	f.line == 3
	f.message == `review cycle "h2": selector names unknown framework "nope"`
	count(fs) == 1
}

test_ref_unknown_cycle_selector_person if {
	d := doc({"eng": framework}, {"sam": sam}, {"h2": cycle(["sam", "ghost"])})
	fs := core.deny with input as d
	some f in fs
	f.id == "ref-unknown"
	f.file == "resources/cycle.yaml"
	contains(f.message, `person "ghost"`)
	count(fs) == 1
}

test_ref_unknown_missing_source_still_reported if {
	d := json.patch(doc({"eng": framework}, {"zed": person("zed", [], "nope", "y")}, {}), [{"op": "replace", "path": "/sources", "value": {}}])
	fs := core.deny with input as d
	some f in fs
	f.id == "ref-unknown"
	f.person == "zed"
}

# --- token-duplicate ------------------------------------------------------

test_token_duplicate_two_competencies_one_token if {
	fw := with_comp(framework, 2, {"id": "scope.crossteam", "title": "Spans", "tokens": ["crossteam", "plan"]})
	fs := core.deny with input as doc({"eng": fw}, {}, {})
	some f in fs
	f.id == "token-duplicate"
	f.file == "resources/framework.yaml"
	f.line == 1
	contains(f.message, `"plan"`)
	contains(f.message, "collab.planning, scope.crossteam")
	count(fs) == 1
}

test_token_duplicate_token_equals_other_suffix if {
	fw := with_comp(framework, 2, {"id": "scope.crossteam", "title": "Spans", "tokens": ["crossteam", "ownership"]})
	fs := core.deny with input as doc({"eng": fw}, {}, {})
	some f in fs
	f.id == "token-duplicate"
	contains(f.message, `"ownership"`)
	contains(f.message, "delivery.ownership, scope.crossteam")
	count(fs) == 1
}

test_token_duplicate_not_for_own_suffix if {
	# delivery.ownership declares its own suffix "ownership" as a token
	count(core.deny) == 0 with input as valid
}

test_token_duplicate_not_for_shared_suffix_alone if {
	fw := with_comp(framework, 2, {"id": "scope.planning", "title": "Spans", "tokens": ["crossteam"]})
	fw2 := with_comp(fw, 1, {"id": "collab.planning", "title": "Plans", "tokens": ["plan"]})
	count(core.deny) == 0 with input as doc({"eng": fw2}, {}, {})
}

# Token collisions across includes are token-duplicate at the including
# framework; a collision inside an included framework is reported there only.
core_fw := {
	"kind": "CompetencyFramework",
	"metadata": {"name": "core"},
	"spec": {"levels": ["IC2", "IC3"], "competencies": [{"id": "core.comms", "title": "Talks", "tokens": ["comms"]}]},
}

test_token_duplicate_across_includes if {
	eng := json.patch(framework, [
		{"op": "add", "path": "/spec/includes", "value": ["core"]},
		{"op": "replace", "path": "/spec/competencies/2/tokens", "value": ["crossteam", "comms"]},
	])
	fs := core.deny with input as doc({"eng": eng, "core": core_fw}, {}, {})
	some f in fs
	f.id == "token-duplicate"
	f.file == "resources/framework.yaml"
	f.message == `token "comms" is claimed by core.comms, scope.crossteam`
	count(fs) == 1
}

test_token_duplicate_inside_include_reported_once if {
	dup := json.patch(core_fw, [{"op": "add", "path": "/spec/competencies/-", "value": {"id": "core.chat", "title": "Chats", "tokens": ["comms"]}}])
	eng := json.patch(framework, [{"op": "add", "path": "/spec/includes", "value": ["core"]}])
	fs := core.deny with input as doc({"eng": eng, "core": dup}, {}, {})
	some f in fs
	f.id == "token-duplicate"
	f.file == "resources/core.yaml"
	count(fs) == 1
}

test_includes_without_collision_are_clean if {
	eng := json.patch(framework, [{"op": "add", "path": "/spec/includes", "value": ["core"]}])
	count(core.deny) == 0 with input as doc({"eng": eng, "core": core_fw}, {"sam": sam}, {"h2": cycle_with({"matchFrameworks": ["core"]})})
}

# --- alias-duplicate ------------------------------------------------------

test_alias_duplicate_alias if {
	p := person("priya", ["Priya", "SAM"], "eng", "IC2")
	fs := core.deny with input as doc({"eng": framework}, {"sam": sam, "priya": p}, {})
	some f in fs
	f.id == "alias-duplicate"
	f.person == "priya"
	f.file == "resources/people.yaml"
	f.line == 12
	contains(f.message, `"sam"`)
	contains(f.message, "priya, sam")
	count(fs) == 1
}

test_alias_duplicate_not_for_own_name if {
	# sam's alias "Sam" equals its own name
	count(core.deny) == 0 with input as valid
}

# --- lib ------------------------------------------------------------------

test_lib_add_days if {
	lib.add_days("2026-12-29", 5) == "2027-01-03"
	lib.add_days("2026-10-21", -21) == "2026-09-30"
}

test_lib_days if {
	lib.days("90d") == 90
	lib.days("2w") == 14
}

test_lib_days_between if {
	lib.days_between("2026-10-19", "2026-10-21") == 2
	lib.days_between("2026-10-21", "2026-10-19") == -2
	lib.days_between("2026-03-28", "2026-03-30") == 2
}

test_lib_min_max_date if {
	lib.max_date("2026-10-19", "2026-10-21") == "2026-10-21"
	lib.min_date("2026-10-19", "2026-10-21") == "2026-10-19"
}

test_lib_finding if {
	lib.finding("x", "sam", "f.md", 3, "m") == {"id": "x", "person": "sam", "file": "f.md", "line": 3, "message": "m", "msg": "m"}
}
