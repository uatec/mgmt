package mgmt.policy.core_warnings_test

import data.mgmt.policy.core

# --- fixtures -------------------------------------------------------------

framework := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "CompetencyFramework",
	"metadata": {"name": "eng"},
	"spec": {
		"levels": ["IC2", "IC3"],
		"competencies": [
			{"id": "delivery.ownership", "title": "Owns", "tokens": ["ownership"], "levels": {"IC2": "o2", "IC3": "o3"}},
			{"id": "collab.planning", "title": "Plans", "tokens": ["planning"], "levels": {"IC3": "p3"}},
		],
	},
}

# effective is framework as facts.frameworks carries it (Decision N).
effective := {
	"levels": framework.spec.levels,
	"competencies": {c.id: {
		"id": c.id, "title": c.title, "theme": split(c.id, ".")[0],
		"summary": "", "levels": c.levels,
	} |
		some c in framework.spec.competencies
	},
}

sam := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "Person",
	"metadata": {"name": "sam"},
	"spec": {"framework": "eng", "level": "IC3", "since": "2026-05-01"},
}

workspace := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "Workspace",
	"metadata": {"name": "ws"},
	"spec": {"notes": "notes/*.md", "me": ["me"], "defaultWindow": "90d", "oneToOneCadence": "14d"},
}

h2 := {
	"apiVersion": "mgmt/v1alpha1",
	"kind": "ReviewCycle",
	"metadata": {"name": "h2"},
	"spec": {
		"period": {"from": "2026-04-01", "to": "2026-09-30"},
		"selector": {"people": ["sam"]},
		"inputs": [{"name": "self-assessment", "required": true}, {"name": "okrs"}],
		"milestones": [
			{"name": "peer-nominations", "due": "2026-10-27"},
			{"name": "manager-draft", "due": "2026-11-14"},
			{"name": "conversations", "due": "2026-11-28"},
		],
	},
}

ev(c, pol, date, line) := {
	"person": "sam", "competency": c, "polarity": pol,
	"date": date, "file": "notes/n.md", "line": line, "text": "t",
}

# sev is a strong (++ / --) evidence record.
sev(c, pol, date, line) := object.union(ev(c, pol, date, line), {"strength": 2})

act(text, date, due, done, addresses, line) := {
	"id": sprintf("a%d", [line]), "person": "sam", "owner": "me", "due": due, "done": done,
	"addresses": addresses, "date": date, "file": "notes/n.md", "line": line, "text": text,
}

section(date, tokens, line) := {"person": "sam", "date": date, "file": "notes/n.md", "line": line, "tokenCount": tokens}

# quiet fires no warning: both required competencies were proven a week ago
# in a section with tokens, and there are no actions or cycles.
quiet := {
	"now": "2026-10-21",
	"options": {"cycle": ""},
	"resources": {"Workspace": {"ws": workspace}, "CompetencyFramework": {"eng": framework}, "Person": {"sam": sam}, "ReviewCycle": {}},
	"frameworks": {"eng": effective},
	"sources": {
		"Person/sam": {"file": "resources/people.yaml", "line": 1},
		"ReviewCycle/h2": {"file": "resources/cycle.yaml", "line": 1},
	},
	"evidence": [ev("delivery.ownership", 1, "2026-10-14", 2), ev("collab.planning", 1, "2026-10-14", 3)],
	"touches": [],
	"actions": [],
	"inputs": [],
	"sections": [section("2026-10-14", 2, 1)],
	"ignores": [],
}

patched(d, patch) := object.union(d, patch)

with_policy(d, pol) := patched(d, {"resources": {"Workspace": {"ws": {"spec": {"policy": pol}}}}})

with_sam(d, spec) := patched(d, {"resources": {"Person": {"sam": {"spec": spec}}}})

with_cycle(d) := patched(d, {"resources": {"ReviewCycle": {"h2": h2}}})

# planning_evidence replaces sam's planning evidence with es.
planning_evidence(d, es) := patched(d, {"evidence": array.concat([ev("delivery.ownership", 1, "2026-10-14", 2)], es)})

fired(d, id) := {f | some f in core.warn with input as d; f.id == id}

# --- quiet ----------------------------------------------------------------

test_quiet_fires_nothing if {
	count(core.warn) == 0 with input as quiet
}

test_empty_input_fires_nothing if {
	count(core.warn) == 0 with input as {}
}

test_every_warning_has_person_and_msg if {
	d := patched(planning_evidence(quiet, []), {"actions": [act("x", "2026-10-14", "", false, [], 5)]})
	fs := core.warn with input as d
	count(fs) > 0
	every f in fs {
		f.person == "sam"
		f.msg == f.message
	}
}

# --- section-unmarked -----------------------------------------------------

test_section_unmarked_fires if {
	fs := fired(patched(quiet, {"sections": [section("2026-10-14", 2, 1), section("2026-10-16", 0, 20)]}), "section-unmarked")
	count(fs) == 1
	some f in fs
	f.file == "notes/n.md"
	f.line == 20
	f.person == "sam"
}

test_section_unmarked_quiet if {
	count(fired(quiet, "section-unmarked")) == 0
}

# --- negative-unaddressed -------------------------------------------------

old_negative := planning_evidence(quiet, [ev("collab.planning", 1, "2026-10-01", 3), ev("collab.planning", -1, "2026-10-10", 4)])

test_negative_unaddressed_fires if {
	fs := fired(old_negative, "negative-unaddressed")
	count(fs) == 1
	some f in fs
	f.line == 4
	contains(f.message, "collab.planning")
}

test_negative_addressed_by_closed_action_same_day if {
	d := patched(old_negative, {"actions": [act("talk", "2026-10-10", "", true, ["collab.planning"], 9)]})
	count(fired(d, "negative-unaddressed")) == 0
}

test_negative_action_before_mark_does_not_address if {
	d := patched(old_negative, {"actions": [act("talk", "2026-10-09", "", true, ["collab.planning"], 9)]})
	count(fired(d, "negative-unaddressed")) == 1
}

test_negative_action_for_other_competency_does_not_address if {
	d := patched(old_negative, {"actions": [act("talk", "2026-10-12", "", false, ["delivery.ownership"], 9)]})
	count(fired(d, "negative-unaddressed")) == 1
}

test_negative_within_grace if {
	d := planning_evidence(quiet, [ev("collab.planning", -1, "2026-10-20", 4)])
	count(fired(d, "negative-unaddressed")) == 0
}

test_negative_grace_override if {
	d := planning_evidence(quiet, [ev("collab.planning", -1, "2026-10-20", 4)])
	count(fired(with_policy(d, {"negative-unaddressed": {"graceDays": 0}}), "negative-unaddressed")) == 1
}

test_negative_bad_threshold_uses_default if {
	d := planning_evidence(quiet, [ev("collab.planning", -1, "2026-10-20", 4)])
	count(fired(with_policy(d, {"negative-unaddressed": {"graceDays": "soon"}}), "negative-unaddressed")) == 0
}

test_negative_after_cycle_period_while_cycle_active if {
	# h2's period ended 2026-09-30 but its milestones run to 2026-11-28:
	# a recent negative still needs addressing (Ruling R11)
	d := with_cycle(old_negative)
	fs := fired(d, "negative-unaddressed")
	count(fs) == 1
	some f in fs
	f.line == 4
}

test_negative_before_since_ignored if {
	d := planning_evidence(quiet, [ev("collab.planning", -1, "2026-04-20", 4)])
	count(fired(d, "negative-unaddressed")) == 0
}

test_negative_after_now_ignored if {
	d := planning_evidence(quiet, [ev("collab.planning", -1, "2026-12-01", 4)])
	count(fired(d, "negative-unaddressed")) == 0
}

test_negative_exactly_grace_days_old if {
	d := planning_evidence(quiet, [ev("collab.planning", -1, "2026-10-14", 4)])
	count(fired(d, "negative-unaddressed")) == 0
}

test_strong_negative_unaddressed_fires if {
	d := planning_evidence(quiet, [sev("collab.planning", -1, "2026-10-10", 4)])
	fs := fired(d, "negative-unaddressed")
	count(fs) == 1
	some f in fs
	f.line == 4
}

# --- no-next-level --------------------------------------------------------

top_strong := planning_evidence(quiet, [sev("collab.planning", 1, "2026-10-14", 4)])

test_no_next_level_fires_at_top if {
	fs := fired(top_strong, "no-next-level")
	count(fs) == 1
	some f in fs
	f.person == "sam"
	f.file == "notes/n.md"
	f.line == 4
	contains(f.message, "IC3")
}

test_no_next_level_not_for_plain_positive if {
	count(fired(quiet, "no-next-level")) == 0
}

test_no_next_level_not_for_strong_negative if {
	d := planning_evidence(quiet, [sev("collab.planning", -1, "2026-10-20", 4)])
	count(fired(d, "no-next-level")) == 0
}

test_no_next_level_not_below_top if {
	count(fired(with_sam(top_strong, {"level": "IC2"}), "no-next-level")) == 0
}

test_no_next_level_level_off_ladder_silent if {
	count(fired(with_sam(top_strong, {"level": "IC9"}), "no-next-level")) == 0
}

test_no_next_level_still_counts_positive if {
	derive_status := data.mgmt.derive.status.sam["collab.planning"] with input as top_strong
	derive_status.positives == 1
}

# --- gap-unaddressed ------------------------------------------------------

gap := planning_evidence(quiet, [])

test_gap_unaddressed_fires if {
	fs := fired(gap, "gap-unaddressed")
	count(fs) == 1
	some f in fs
	f.file == "resources/people.yaml"
	f.line == 1
	contains(f.message, "collab.planning")
	contains(f.message, "unproven")
}

test_gap_addressed_by_open_action if {
	d := patched(gap, {"actions": [act("talk", "2026-10-14", "", false, ["collab.planning"], 9)]})
	count(fired(d, "gap-unaddressed")) == 0
}

test_gap_closed_action_does_not_address if {
	d := patched(gap, {"actions": [act("talk", "2026-10-14", "", true, ["collab.planning"], 9)]})
	count(fired(d, "gap-unaddressed")) == 1
}

# --- competency-untouched -------------------------------------------------

test_untouched_never if {
	fs := fired(gap, "competency-untouched")
	count(fs) == 1
	some f in fs
	f.line == 1
	contains(f.message, "collab.planning")
}

test_untouched_stale if {
	d := planning_evidence(quiet, [ev("collab.planning", 1, "2026-09-01", 3)])
	count(fired(d, "competency-untouched")) == 1
}

test_untouched_touch_counts if {
	d := patched(planning_evidence(quiet, []), {"touches": [{
		"person": "sam", "competency": "collab.planning", "date": "2026-10-01",
		"file": "notes/n.md", "line": 7, "text": "t",
	}]})
	count(fired(d, "competency-untouched")) == 0
}

test_untouched_quiet if {
	count(fired(quiet, "competency-untouched")) == 0
}

test_untouched_new_report if {
	# managed for 20 days only: below the 42-day threshold (Decision M)
	d := with_sam(gap, {"since": "2026-10-01"})
	count(fired(d, "competency-untouched")) == 0
}

test_untouched_days_override if {
	d := planning_evidence(quiet, [ev("collab.planning", 1, "2026-09-15", 3)])
	count(fired(d, "competency-untouched")) == 0
	count(fired(with_policy(d, {"competency-untouched": {"days": 30}}), "competency-untouched")) == 1
}

test_untouched_last_touch_exactly_limit_ago if {
	# 2026-09-09 is exactly 42 days before now: still inside the window
	d := planning_evidence(quiet, [ev("collab.planning", 1, "2026-09-09", 3)])
	count(fired(d, "competency-untouched")) == 0
}

test_untouched_since_exactly_limit_ago if {
	# managed for exactly 42 days: the threshold is reached (Decision M)
	d := with_sam(gap, {"since": "2026-09-09"})
	count(fired(d, "competency-untouched")) == 1
}

# --- action-overdue and action-no-due -------------------------------------

test_action_overdue if {
	d := patched(quiet, {"actions": [
		act("late", "2026-10-14", "2026-10-20", false, [], 5),
		act("today", "2026-10-14", "2026-10-21", false, [], 6),
		act("late but done", "2026-10-14", "2026-10-20", true, [], 7),
	]})
	fs := fired(d, "action-overdue")
	count(fs) == 1
	some f in fs
	f.line == 5
	contains(f.message, "late")
}

test_action_no_due if {
	d := patched(quiet, {"actions": [
		act("someday", "2026-10-14", "", false, [], 5),
		act("done someday", "2026-10-14", "", true, [], 6),
		act("dated", "2026-10-14", "2026-10-30", false, [], 7),
	]})
	fs := fired(d, "action-no-due")
	count(fs) == 1
	some f in fs
	f.line == 5
	contains(f.message, "someday")
}

# --- action-duplicate -----------------------------------------------------

test_action_duplicate_every_copy if {
	d := patched(quiet, {"actions": [
		act("Send the doc", "2026-10-14", "2026-10-30", false, [], 5),
		object.union(act("send  the DOC", "2026-10-14", "2026-10-30", false, [], 5), {"file": "notes/m.md"}),
		act("send the doc", "2026-10-14", "2026-10-30", true, [], 8),
	]})
	fs := fired(d, "action-duplicate")
	{[f.file, f.line] | some f in fs} == {["notes/n.md", 5], ["notes/m.md", 5]}
}

test_action_duplicate_needs_two_open if {
	d := patched(quiet, {"actions": [
		act("send the doc", "2026-10-14", "2026-10-30", false, [], 5),
		act("send the doc", "2026-10-14", "2026-10-30", true, [], 8),
	]})
	count(fired(d, "action-duplicate")) == 0
}

test_action_duplicate_per_person if {
	# same text for two people is two separate actions (Ruling R12)
	d := patched(quiet, {"actions": [
		act("book 1:1 room", "2026-10-14", "2026-10-30", false, [], 5),
		object.union(act("book 1:1 room", "2026-10-14", "2026-10-30", false, [], 6), {"person": "priya"}),
	]})
	count(fired(d, "action-duplicate")) == 0
}

# --- oneonone-overdue -----------------------------------------------------

test_oneonone_quiet if {
	count(fired(quiet, "oneonone-overdue")) == 0
}

test_oneonone_overdue_workspace_cadence if {
	fs := fired(patched(quiet, {"sections": [section("2026-10-01", 2, 1)]}), "oneonone-overdue")
	count(fs) == 1
	some f in fs
	f.file == "resources/people.yaml"
	contains(f.message, "2026-10-01")
}

test_oneonone_person_cadence_overrides if {
	d := with_sam(patched(quiet, {"sections": [section("2026-10-01", 2, 1)]}), {"oneToOneCadence": "30d"})
	count(fired(d, "oneonone-overdue")) == 0
}

test_oneonone_workspace_cadence_override if {
	d := patched(quiet, {
		"sections": [section("2026-10-10", 2, 1)],
		"resources": {"Workspace": {"ws": {"spec": {"oneToOneCadence": "1w"}}}},
	})
	count(fired(d, "oneonone-overdue")) == 1
}

test_oneonone_no_sections_since_old if {
	count(fired(patched(quiet, {"sections": []}), "oneonone-overdue")) == 1
}

test_oneonone_no_sections_since_recent if {
	d := with_sam(patched(quiet, {"sections": []}), {"since": "2026-10-10"})
	count(fired(d, "oneonone-overdue")) == 0
}

test_oneonone_exactly_one_cadence_ago if {
	count(fired(patched(quiet, {"sections": [section("2026-10-07", 2, 1)]}), "oneonone-overdue")) == 0
}

# --- cycle-input-missing --------------------------------------------------

test_cycle_input_missing if {
	fs := fired(with_cycle(quiet), "cycle-input-missing")
	count(fs) == 1
	some f in fs
	f.person == "sam"
	f.file == "resources/cycle.yaml"
	contains(f.message, "self-assessment")
	contains(f.message, "peer-nominations")
}

test_cycle_input_received if {
	d := patched(with_cycle(quiet), {"inputs": [{
		"person": "sam", "input": "self-assessment", "cycle": "h2",
		"date": "2026-10-14", "file": "notes/n.md", "line": 4,
	}]})
	count(fired(d, "cycle-input-missing")) == 0
}

test_cycle_input_milestone_far if {
	count(fired(patched(with_cycle(quiet), {"now": "2026-10-15"}), "cycle-input-missing")) == 0
}

test_cycle_input_days_before_override if {
	d := with_policy(with_cycle(quiet), {"cycle-input-missing": {"daysBefore": 5}})
	count(fired(d, "cycle-input-missing")) == 0
}

# --- cycle-evidence-thin --------------------------------------------------

test_cycle_evidence_thin_fires_near_draft if {
	# the cycle window ends 2026-09-30, so the October evidence leaves gaps
	fs := fired(patched(with_cycle(quiet), {"now": "2026-11-05"}), "cycle-evidence-thin")
	count(fs) == 1
	some f in fs
	f.file == "resources/cycle.yaml"
	contains(f.message, "collab.planning")
	contains(f.message, "2026-11-14")
}

test_cycle_evidence_thin_draft_far if {
	count(fired(with_cycle(quiet), "cycle-evidence-thin")) == 0
}

test_cycle_evidence_thin_days_before_override if {
	d := with_policy(with_cycle(quiet), {"cycle-evidence-thin": {"daysBefore": 30}})
	count(fired(d, "cycle-evidence-thin")) == 1
}

test_cycle_evidence_thin_no_gaps if {
	d := patched(with_cycle(quiet), {"now": "2026-11-05", "evidence": [
		ev("delivery.ownership", 1, "2026-09-20", 2),
		ev("collab.planning", 1, "2026-09-20", 3),
	]})
	count(fired(d, "cycle-evidence-thin")) == 0
}

test_threshold_accepts_duration if {
	d := planning_evidence(quiet, [ev("collab.planning", 1, "2026-09-15", 3)])
	count(fired(with_policy(d, {"competency-untouched": {"days": "4w"}}), "competency-untouched")) == 1
}

test_cycle_input_milestone_exactly_days_before if {
	# peer-nominations is due 2026-10-27, exactly 7 days after 2026-10-20
	count(fired(patched(with_cycle(quiet), {"now": "2026-10-20"}), "cycle-input-missing")) == 1
}

test_cycle_evidence_thin_draft_exactly_days_before if {
	# manager-draft is due 2026-11-14, exactly 14 days after 2026-10-31
	count(fired(patched(with_cycle(quiet), {"now": "2026-10-31"}), "cycle-evidence-thin")) == 1
}
