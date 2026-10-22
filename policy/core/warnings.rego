# Core warn rules: the data is valid but needs the manager's attention.
# Thresholds come from Workspace.spec.policy.<id>.<param>, else the default.
# Warnings on a line carrying <!-- mgmt:ignore id --> are dropped in Go.
package mgmt.policy.core

import data.mgmt.derive
import data.mgmt.lib

_now := object.get(input, "now", "")

_workspace := ws if {
	wss := [w | some w in object.get(input, ["resources", "Workspace"], {})]
	count(wss) > 0
	ws := wss[0]
} else := {}

# _param reads a whole-number threshold, or a duration such as "2w", and
# falls back to the default for anything else so rules stay total.
_param(id, name, default_value) := n if {
	v := object.get(_workspace, ["spec", "policy", id, name], null)
	n := _days_value(v)
} else := default_value

_days_value(v) := v if {
	is_number(v)
	v >= 0
	v == floor(v)
}

_days_value(v) := lib.days(v) if {
	is_string(v)
	regex.match(`^[0-9]+[dw]$`, v)
}

_open(a) if a.done == false

_actions := object.get(input, "actions", [])

# --- section-unmarked -----------------------------------------------------

warn contains f if {
	some s in object.get(input, "sections", [])
	s.tokenCount == 0
	f := lib.finding("section-unmarked", s.person, s.file, s.line, sprintf("%s's section on %s has no tokens", [s.person, s.date]))
}

# --- negative-unaddressed -------------------------------------------------

_addresses_since(p, c, date) if {
	some a in derive.actions_by[p]
	c in a.addresses
	a.date >= date
}

# Not windowed to the cycle period: a negative mark stays the manager's
# concern from the person's since date up to now (Ruling R11).
warn contains f if {
	grace := _param("negative-unaddressed", "graceDays", 7)
	some m in object.get(input, "evidence", [])
	m.polarity < 0
	p := m.person
	c := m.competency
	derive.competencies[p][c]
	object.get(_persons[p], ["spec", "since"], "") <= m.date
	m.date <= _now
	_now > lib.add_days(m.date, grace)
	not _addresses_since(p, c, m.date)
	f := lib.finding("negative-unaddressed", p, m.file, m.line, sprintf("negative %s mark for %s on %s has no ~%s action dated on or after it", [c, p, m.date, c]))
}

# --- no-next-level --------------------------------------------------------

# A ++ mark says "ready for more" (Decision S); at the top of the ladder
# there is nothing to be ready for. The mark still counts as positive. Only
# marks since the person reached their level, up to now, count (Decision Q).
warn contains f if {
	some m in object.get(input, "evidence", [])
	m.polarity > 0
	m.strength == 2
	p := m.person
	derive.level_since[p] <= m.date
	m.date <= _now
	derive.level[p] in derive.ladder[p]
	not derive.next_level[p]
	f := lib.finding("no-next-level", p, m.file, m.line, sprintf("++ %s mark for %s on %s, but %s is the top level of the ladder", [m.competency, p, m.date, derive.level[p]]))
}

# --- gap-unaddressed ------------------------------------------------------

_has_open_action(p, c) if {
	some a in object.get(derive.addressed_by, [p, c], [])
	_open(a)
}

warn contains f if {
	some p, gs in derive.gaps
	some g in gs
	not _has_open_action(p, g.competency)
	f := _at("Person", p, "gap-unaddressed", p, sprintf("%s: gap %s (%s) has no open action addressing it", [p, g.competency, g.status]))
}

# --- competency-untouched -------------------------------------------------

_touched_since(p, c, date) if derive.last_touch[p][c] >= date

_last_touch_text(p, c) := sprintf("last touched %s", [derive.last_touch[p][c]]) if {
	derive.last_touch[p][c]
} else := "never touched"

warn contains f if {
	limit := _param("competency-untouched", "days", 42)
	some p, reqs in derive.required
	lib.days_between(_persons[p].spec.since, _now) >= limit
	cutoff := lib.add_days(_now, 0 - limit)
	some c in reqs
	not _touched_since(p, c, cutoff)
	f := _at("Person", p, "competency-untouched", p, sprintf("%s: required competency %s %s; nothing in the last %d days", [p, c, _last_touch_text(p, c), limit]))
}

# --- actions --------------------------------------------------------------

warn contains f if {
	some a in _actions
	_open(a)
	a.due != ""
	a.due < _now
	f := lib.finding("action-overdue", a.person, a.file, a.line, sprintf("action %q was due %s", [a.text, a.due]))
}

warn contains f if {
	some a in _actions
	_open(a)
	a.due == ""
	f := lib.finding("action-no-due", a.person, a.file, a.line, sprintf("action %q has no due date", [a.text]))
}

# Duplicates are per person (Ruling R12): the same text for two people is
# two separate actions.
_action_key(a) := [a.person, regex.replace(lower(trim_space(a.text)), `\s+`, " ")]

_open_by_key[_action_key(a)] contains i if {
	some i, a in _actions
	_open(a)
}

warn contains f if {
	some a in _actions
	_open(a)
	n := count(_open_by_key[_action_key(a)])
	n > 1
	f := lib.finding("action-duplicate", a.person, a.file, a.line, sprintf("open action %q appears %d times", [a.text, n]))
}

# --- oneonone-overdue -----------------------------------------------------

_cadence(p) := object.get(_persons[p], ["spec", "oneToOneCadence"], object.get(_workspace, ["spec", "oneToOneCadence"], "14d"))

warn contains f if {
	some p in object.keys(_persons)
	cadence := _cadence(p)
	dates := {s.date | some s in derive.sections_by[p]}
	count(dates) > 0
	latest := max(dates)
	latest < lib.add_days(_now, 0 - lib.days(cadence))
	f := _at("Person", p, "oneonone-overdue", p, sprintf("no 1:1 section for %s since %s (cadence %s)", [p, latest, cadence]))
}

warn contains f if {
	some p, person in _persons
	cadence := _cadence(p)
	not _has_section(p)
	lib.add_days(person.spec.since, lib.days(cadence)) < _now
	f := _at("Person", p, "oneonone-overdue", p, sprintf("no 1:1 section for %s at all; managed since %s (cadence %s)", [p, person.spec.since, cadence]))
}

_has_section(p) if count(derive.sections_by[p]) > 0

# --- cycle-input-missing --------------------------------------------------

warn contains f if {
	before := _param("cycle-input-missing", "daysBefore", 7)
	some cy in derive.active_cycles
	next := derive.next_milestone[cy]
	left := lib.days_between(_now, next.due)
	0 <= left
	left <= before
	some p in object.get(derive.cycle_people, cy, [])
	some i in object.get(derive.inputs, [cy, p], [])
	i.complete == false
	f := _at("ReviewCycle", cy, "cycle-input-missing", p, sprintf("%s: input %s not recorded for cycle %s (%d received); %s is due %s", [p, i.name, cy, i.received, next.name, next.due]))
}

# --- cycle-evidence-thin --------------------------------------------------

warn contains f if {
	before := _param("cycle-evidence-thin", "daysBefore", 14)
	some cy in derive.active_cycles
	some m in object.get(_cycles, [cy, "spec", "milestones"], [])
	m.name == "manager-draft"
	_now <= m.due
	m.due <= lib.add_days(_now, before)
	some p in object.get(derive.cycle_people, cy, [])
	gs := object.get(derive.gaps, p, [])
	count(gs) > 0
	f := _at("ReviewCycle", cy, "cycle-evidence-thin", p, sprintf("%s: gaps remain (%s) and manager-draft for %s is due %s", [p, concat(", ", [g.competency | some g in gs]), cy, m.due]))
}
