package mgmt.policy.ensek_test

import data.mgmt.policy.ensek

okr := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "OKRSet",
	"metadata": {"name": "sam-h2"},
	"spec": {
		"person": "sam",
		"cycle": "h2",
		"objectives": [{
			"title": "Ship the CDN",
			"keyResults": ["p95 under 200ms"],
			"competencies": ["scope.crossteam", "delivery.ownership"],
		}],
	},
}

cycle(milestones) := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "ReviewCycle",
	"metadata": {"name": "h2"},
	"spec": {"milestones": milestones},
}

h2 := cycle([
	{"name": "peer-feedback", "due": "2026-11-07"},
	{"name": "manager-draft", "due": "2026-11-14"},
])

status := {"sam": {
	"scope.crossteam": {"status": "unproven"},
	"delivery.ownership": {"status": "proven"},
}}

doc(now, c) := {
	"now": now,
	"resources": {"OKRSet": {"sam-h2": okr}, "ReviewCycle": {"h2": c}},
	"sources": {"OKRSet/sam-h2": {"file": "resources/okrs.yaml", "line": 1}},
}

_ids(fs) := {[f.id, f.person, f.file, f.line] | some f in fs}

warns(now, c) := fs if {
	fs := ensek.warn with input as doc(now, c) with data.mgmt.derive.status as status
}

test_fires_for_unproven_link_from_window_start if {
	fs := warns("2026-10-31", h2)
	_ids(fs) == {["okr-no-evidence", "sam", "resources/okrs.yaml", 1]}
	some f in fs
	contains(f.message, "scope.crossteam")
	contains(f.message, "2026-11-14")
}

test_fires_on_due_date if {
	count(warns("2026-11-14", h2)) == 1
}

test_silent_before_window if {
	count(warns("2026-10-30", h2)) == 0
}

test_silent_after_due if {
	count(warns("2026-11-15", h2)) == 0
}

test_silent_when_cycle_has_no_manager_draft if {
	count(warns("2026-11-01", cycle([{"name": "peer-feedback", "due": "2026-11-07"}]))) == 0
}

test_silent_for_unknown_cycle if {
	d := json.remove(doc("2026-11-01", h2), ["resources/ReviewCycle"])
	fs := ensek.warn with input as d with data.mgmt.derive.status as status
	count(fs) == 0
}

test_silent_for_unknown_person if {
	fs := ensek.warn with input as doc("2026-11-01", h2) with data.mgmt.derive.status as {}
	count(fs) == 0
}
