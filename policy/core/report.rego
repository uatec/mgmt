# Package mgmt.report builds the view-models the report templates format
# (Decision K). Every rule is total for every Person: data that is missing,
# including derive rules left undefined by a missing framework (R8), becomes
# null or [] so templates never meet an absent key.
package mgmt.report

import data.mgmt.derive

_now := object.get(input, "now", "")

_persons := object.get(input, ["resources", "Person"], {})

_since(p) := object.get(_persons[p], ["spec", "since"], "")

# _by_key sorts [key, value] pairs by key and returns the values.
_by_key(pairs) := [v | some pair in sort(pairs); v := pair[1]]

_title(p, c) := object.get(derive.competencies, [p, c, "title"], c)

# --- prep -----------------------------------------------------------------

prep[p] := {
	"person": _person(p),
	"window": object.get(derive.window, p, null),
	"actions": {"mine": _open_actions(p, true), "theirs": _open_actions(p, false)},
	"lastSection": last,
	"since": since,
	"toRaise": _newest_first([i | some i in since; _strong(i)]),
	"gaps": _gaps(p),
	"focus": _focus(p),
	"cycle": _cycle(p),
} if {
	some p in object.keys(_persons)
	last := _last_section(p)
	since := _since_items(p, last)
}

_person(p) := {
	"name": p,
	"displayName": object.get(_persons[p], ["spec", "displayName"], p),
	"framework": _persons[p].spec.framework,
	"level": _persons[p].spec.level,
}

# Open actions, overdue first, then by due date, then undated; ties by
# file and line.
_open_actions(p, mine) := _by_key([[[_due_key(a.due), a.file, a.line], object.union(a, {"overdue": _overdue(a.due)})] |
	some a in derive.actions_by[p]
	not a.done
	equal(a.owner, "me") == mine
])

_due_key("") := "9999-99-99"

_due_key(due) := due if due != ""

_overdue(due) := true if {
	due != ""
	due < _now
} else := false

_last_section(p) := max(dates) if {
	dates := {s.date | some s in derive.sections_by[p]; s.date < _now}
	count(dates) > 0
} else := null

# Marks and touches after the previous 1:1 (or from the since date when
# there was none) up to now. The ++ and -- marks among them are "To raise"
# (Decision T; Ruling L1: since-based, not levelSince).
_since_items(p, last) := _by_key([[[i.date, i.file, i.line, i.kind, i.competency], i] |
	some i in array.concat(
		[object.union(_item("mark", e.polarity, e), {"strength": _strength(e), "tag": _tag(e)}) | some e in derive.evidence_by[p]],
		[object.union(_item("touch", 0, t), {"strength": 0, "tag": "~"}) | some t in derive.touches_by[p]],
	)
	_after(i.date, last, _since(p))
	i.date <= _now
])

_item(kind, polarity, r) := {
	"kind": kind, "polarity": polarity, "competency": r.competency,
	"date": r.date, "text": r.text, "file": r.file, "line": r.line,
}

_strong(r) if r.strength == 2

_strength(r) := object.get(r, "strength", 1)

# _tag is a mark as written in the notes: ++, +, - or --.
_tag(r) := concat("", [_sign(r.polarity), _sign(r.polarity)]) if {
	_strength(r) == 2
} else := _sign(r.polarity)

_sign(polarity) := "+" if polarity > 0

_sign(polarity) := "-" if polarity < 0

# _tagged adds strength (default 1) and tag to each evidence record.
_tagged(records) := [object.union(r, {"strength": _strength(r), "tag": _tag(r)}) | some r in records]

_after(date, null, since) if date >= since

_after(date, last, _) if {
	last != null
	date > last
}

_gaps(p) := [{
	"competency": g.competency,
	"title": _title(p, g.competency),
	"status": g.status,
	"addressedBy": [{"file": a.file, "line": a.line, "text": a.text} |
		some a in object.get(derive.addressed_by, [p, g.competency], [])
		not a.done
	],
} |
	some g in object.get(derive.gaps, p, [])
]

# The required competency touched least recently; never-touched ones sort
# first (""), ties by id.
_focus(p) := f if {
	ranked := sort([[object.get(derive.last_touch, [p, c], ""), c] | some c in object.get(derive.required, p, [])])
	count(ranked) > 0
	c := ranked[0][1]
	f := {"competency": c, "title": _title(p, c), "lastTouch": object.get(derive.last_touch, [p, c], null)}
} else := null

_cycle(p) := {
	"name": cy,
	"nextMilestone": object.get(derive.next_milestone, cy, null),
	"missing": missing,
} if {
	cy := derive.cycle[p]
	missing := _by_key([[i.name, {"name": i.name, "received": i.received, "needed": max({i.minCount, 1})}] |
		some i in object.get(derive.inputs, [cy, p], [])
		not i.complete
	])
} else := null

# --- review ---------------------------------------------------------------

# The evidence pack: required competencies grouped by theme, each with the
# bar at the person's level. It carries no rating and none of the cycle's
# outcomes.
review[p] := {
	"person": _person(p),
	"window": object.get(derive.window, p, null),
	"cycle": object.get(derive.cycle, p, null),
	"defaultWindow": derive.default_window,
	"themes": _themes(p),
	"inputs": _review_inputs(p),
} if some p in object.keys(_persons)

_theme(p, c) := object.get(derive.competencies, [p, c, "theme"], split(c, ".")[0])

_themes(p) := [{"theme": t, "competencies": _by_key([[c, _review_item(p, c)] | some c in reqs; _theme(p, c) == t])} |
	reqs := object.get(derive.required, p, [])
	some t in sort({_theme(p, c) | some c in reqs})
]

_review_item(p, c) := {
	"id": c,
	"title": _title(p, c),
	"descriptor": object.get(derive.descriptor, [p, c], null),
	"status": object.get(derive.status, [p, c, "status"], "unproven"),
	"positive": _newest_first(_tagged([m | some m in ms; m.polarity > 0])),
	"negative": _newest_first(_tagged([m | some m in ms; m.polarity < 0])),
	"actions": [a | some a in object.get(derive.addressed_by, [p, c], []); _in_actions_range(p, a.date)],
} if {
	ms := object.get(derive.marks, [p, c], [])
}

# Ruling R14 (amended): the pack shows actions dated from the window start
# to the cycle's active end (the later of period.to and the last milestone),
# or to the window end without a cycle, so actions agreed after the period
# but during the review still show.
_in_actions_range(p, date) if {
	derive.window[p].from <= date
	date <= _actions_end(p)
}

_actions_end(p) := derive.active_end[derive.cycle[p]] if {
	derive.cycle[p]
} else := derive.window[p].to

# _newest_first sorts records descending by [date, file, line].
_newest_first(records) := array.reverse(_by_key([[[r.date, r.file, r.line], r] | some r in records]))

_review_inputs(p) := _by_key([[i.name, i] | some i in object.get(derive.inputs, [cy, p], [])]) if {
	cy := derive.cycle[p]
} else := null

# --- explain --------------------------------------------------------------

# Why a competency has its status: the bar at the person's level (null when
# the competency is not required there), the rule, the window, and every mark
# considered, newest first, flagging the one that decided (negativeOverrides
# only: under counting the totals decide, not a single mark).
explain[p][c] := {
	"person": _person(p),
	"competency": {"id": c, "title": _title(p, c)},
	"descriptor": object.get(derive.descriptor, [p, c], null),
	"required": c in object.get(derive.required, p, []),
	"status": st.status,
	"positives": st.positives,
	"negatives": st.negatives,
	"rule": st.rule,
	"settings": derive.settings[p],
	"window": derive.window[p],
	"considered": _newest_first([object.union(m, {"decides": _decides(st.decidedBy, m)}) | some m in _tagged(derive.marks[p][c])]),
	"outside": _newest_first(_tagged(derive.outside[p][c])),
} if {
	some p, comps in derive.competencies
	some c in object.keys(comps)
	st := derive.status[p][c]
}

_decides(d, m) := true if {
	d.file == m.file
	d.line == m.line
} else := false

# --- promo ----------------------------------------------------------------

# The draft promotion packet (Decision U): for each competency with a
# descriptor at the next level, the current and next bars and the person's
# ++ marks since they reached their level (Review Focus 2: not bounded by the
# review window, so it spans cycles), newest first. Competencies with no ++
# yet are the gaps. At the top of the ladder (or off it) nextLevel is null
# and both lists are empty. The packet never rates or judges readiness.
promo[p] := {
	"person": _person(p),
	"nextLevel": object.get(derive.next_level, p, null),
	"levelSince": derive.level_since[p],
	"competencies": [_promo_item(p, c, ev) |
		some c in _next_level_competencies(p)
		ev := _promo_evidence(p, c)
		count(ev) > 0
	],
	"gaps": [{"id": c, "title": _title(p, c), "next": _next_descriptor(p, c)} |
		some c in _next_level_competencies(p)
		count(_promo_evidence(p, c)) == 0
	],
} if some p in object.keys(_persons)

_next_descriptor(p, c) := derive.competencies[p][c].levels[derive.next_level[p]]

# Ids of the competencies with a descriptor at the next level, sorted.
_next_level_competencies(p) := sort([c |
	some c, comp in object.get(derive.competencies, p, {})
	comp.levels[object.get(derive.next_level, p, "")]
])

_promo_evidence(p, c) := _newest_first([{"date": e.date, "text": e.text, "file": e.file, "line": e.line} |
	some e in derive.evidence_by[p]
	e.competency == c
	e.polarity > 0
	_strong(e)
	e.date >= derive.level_since[p]
	e.date <= _now
])

_promo_item(p, c, ev) := {
	"id": c,
	"title": _title(p, c),
	"current": object.get(derive.descriptor, [p, c], null),
	"next": _next_descriptor(p, c),
	"evidence": ev,
}

# --- status ---------------------------------------------------------------

# The team matrix (every Person, by name; "" where a competency is not
# required) and, per active cycle, the status conditions over the people the
# cycle selects. With an explicit --cycle the matrix lists only the people
# that cycle selects (Ruling R15). Not keyed by person.
status := {
	"competencies": _status_competencies,
	"rows": [{"person": p, "cells": [{"competency": c, "status": _cell(p, c)} | some c in _status_competencies]} |
		some p in _status_people
	],
	"cycles": [{
		"name": cy,
		"conditions": _conditions(cy),
		"nextMilestone": object.get(derive.next_milestone, cy, null),
		"gaps": _check(_gap_detail(cy), _plural(_count_items(_gap_detail(cy)), "gap", "gaps"), "no gaps"),
		"inputs": _check(_input_detail(cy), sprintf("%d missing", [_count_items(_input_detail(cy))]), "all received"),
		"overdue": _check(_overdue_detail(cy), sprintf("%d overdue", [_count_items(_overdue_detail(cy))]), "none overdue"),
	} |
		some cy in derive.active_cycles
	],
}

_status_people := sort(object.keys(_persons)) if _option_cycle == ""

_status_people := sort(_cycle_people(_option_cycle)) if _option_cycle != ""

_status_competencies := sort({c | some p in _status_people; some c in object.get(derive.required, p, [])})

_cell(p, c) := object.get(derive.status, [p, c, "status"], "unproven") if c in object.get(derive.required, p, [])

_cell(p, c) := "" if not c in object.get(derive.required, p, [])

_conditions(cy) := [_evidence_condition(cy), _inputs_condition(cy), _actions_condition(cy)]

_plural(1, one, _) := sprintf("1 %s", [one])

_plural(n, _, many) := sprintf("%d %s", [n, many]) if n != 1

_people(n) := _plural(n, "person", "people")

_option_cycle := object.get(input, ["options", "cycle"], "")

_cycle_people(cy) := object.get(derive.cycle_people, cy, [])

_evidence_condition(cy) := {"type": "EvidenceComplete", "status": "True", "reason": "NoGaps", "message": sprintf("no gaps for %s", [_people(count(_cycle_people(cy)))])} if {
	count(_gap_people(cy)) == 0
}

_evidence_condition(cy) := {
	"type": "EvidenceComplete", "status": "False", "reason": "GapsRemain",
	"message": sprintf("%s across %s", [_plural(sum([count(object.get(derive.gaps, p, [])) | some p in _gap_people(cy)]), "gap", "gaps"), _people(count(_gap_people(cy)))]),
} if {
	count(_gap_people(cy)) > 0
}

_gap_people(cy) := {p | some p in _cycle_people(cy); count(object.get(derive.gaps, p, [])) > 0}

_missing(cy, p) := count([i | some i in object.get(derive.inputs, [cy, p], []); not i.complete])

_inputs_people(cy) := {p | some p in _cycle_people(cy); _missing(cy, p) > 0}

_inputs_condition(cy) := {"type": "InputsComplete", "status": "True", "reason": "AllInputsReceived", "message": "all inputs received"} if {
	count(_inputs_people(cy)) == 0
}

_inputs_condition(cy) := {
	"type": "InputsComplete", "status": "False", "reason": "InputsMissing",
	"message": sprintf("%s missing across %s", [_plural(sum([_missing(cy, p) | some p in _inputs_people(cy)]), "input", "inputs"), _people(count(_inputs_people(cy)))]),
} if {
	count(_inputs_people(cy)) > 0
}

_overdue_actions(cy) := [a |
	some a in object.get(input, "actions", [])
	a.person in _cycle_people(cy)
	not a.done
	_overdue(a.due)
]

_actions_condition(cy) := {"type": "ActionsCurrent", "status": "True", "reason": "NoOverdueActions", "message": "no overdue actions"} if {
	count(_overdue_actions(cy)) == 0
}

_actions_condition(cy) := {
	"type": "ActionsCurrent", "status": "False", "reason": "ActionsOverdue",
	"message": sprintf("%s overdue", [_plural(count(_overdue_actions(cy)), "open action", "open actions")]),
} if {
	count(_overdue_actions(cy)) > 0
}

# --- status detail: who has what, per check -------------------------------

# _check summarises one check's per-person detail ([{person, items}], people
# by name, only people with items) for the bullet list.
_check(people, summary, _) := {"ok": false, "summary": summary, "people": people} if count(people) > 0

_check(people, _, clear) := {"ok": true, "summary": clear, "people": []} if count(people) == 0

_count_items(people) := sum([count(p.items) | some p in people])

_detail(cy, f) := [{"person": p, "items": items} |
	some p in _cycle_people(cy)
	items := f[p]
	count(items) > 0
]

# Gaps in derive.gaps order (disproven first, then by id).
_gap_detail(cy) := _detail(cy, {p: object.get(derive.gaps, p, []) | some p in _cycle_people(cy)})

# Incomplete inputs in the cycle's order.
_input_detail(cy) := _detail(cy, {p: [{"name": i.name, "received": i.received, "needed": max({i.minCount, 1})} |
	some i in object.get(derive.inputs, [cy, p], [])
	not i.complete
] | some p in _cycle_people(cy)})

# Overdue open actions, most recently due first, then by file and line.
_overdue_detail(cy) := _detail(cy, {p: _newest_due_first([a | some a in _overdue_actions(cy); a.person == p]) | some p in _cycle_people(cy)})

_newest_due_first(actions) := [a |
	some due in array.reverse(sort({x.due | some x in actions}))
	some a in _by_key([[[x.file, x.line], x] | some x in actions; x.due == due])
]
