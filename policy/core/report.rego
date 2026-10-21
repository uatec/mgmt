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
	"since": _since_items(p, last),
	"gaps": _gaps(p),
	"focus": _focus(p),
	"cycle": _cycle(p),
} if {
	some p in object.keys(_persons)
	last := _last_section(p)
}

_person(p) := {
	"name": p,
	"displayName": object.get(_persons[p], ["spec", "displayName"], p),
	"role": _persons[p].spec.role,
	"level": _persons[p].spec.level,
}

# Open actions, overdue first, then by due date, then undated; ties by
# file and line.
_open_actions(p, mine) := _by_key([[[_due_key(a.due), a.file, a.line], object.union(a, {"overdue": _overdue(a.due)})] |
	some a in object.get(input, "actions", [])
	a.person == p
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
	dates := {s.date | some s in object.get(input, "sections", []); s.person == p; s.date < _now}
	count(dates) > 0
} else := null

# Marks and touches after the previous 1:1 (or from the since date when
# there was none) up to now.
_since_items(p, last) := _by_key([[[i.date, i.file, i.line, i.kind, i.competency], i] |
	some i in array.concat(
		[_item("mark", e.polarity, e) | some e in object.get(input, "evidence", []); e.person == p],
		[_item("touch", 0, t) | some t in object.get(input, "touches", []); t.person == p],
	)
	_after(i.date, last, _since(p))
	i.date <= _now
])

_item(kind, polarity, r) := {
	"kind": kind, "polarity": polarity, "competency": r.competency,
	"date": r.date, "text": r.text, "file": r.file, "line": r.line,
}

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

# The evidence pack: required competencies grouped by theme. It carries no
# rating and none of the cycle's outcomes.
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
	"status": object.get(derive.status, [p, c, "status"], "unproven"),
	"positive": _newest_first([m | some m in ms; m.polarity > 0]),
	"negative": _newest_first([m | some m in ms; m.polarity < 0]),
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

# Why a competency has its status: the rule, the window, and every mark
# considered, newest first, flagging the one that decided (negativeOverrides
# only: under counting the totals decide, not a single mark).
explain[p][c] := {
	"person": _person(p),
	"competency": {"id": c, "title": _title(p, c)},
	"required": c in object.get(derive.required, p, []),
	"status": st.status,
	"positives": st.positives,
	"negatives": st.negatives,
	"rule": st.rule,
	"settings": derive.settings[p],
	"window": derive.window[p],
	"considered": _newest_first([object.union(m, {"decides": _decides(st.decidedBy, m)}) | some m in derive.marks[p][c]]),
	"outside": _newest_first(derive.outside[p][c]),
} if {
	some p, comps in derive.competencies
	some c in object.keys(comps)
	st := derive.status[p][c]
}

_decides(d, m) := true if {
	d.file == m.file
	d.line == m.line
} else := false

# --- status ---------------------------------------------------------------

# The team matrix (every Person, by name; "" where a competency is not
# required) and, per active cycle, the status conditions over the people the
# cycle selects. Not keyed by person.
status := {
	"competencies": _status_competencies,
	"rows": [{"person": p, "cells": [{"competency": c, "status": _cell(p, c)} | some c in _status_competencies]} |
		some p in sort(object.keys(_persons))
	],
	"cycles": [{"name": cy, "conditions": _conditions(cy)} | some cy in derive.active_cycles],
}

_status_competencies := sort({c | some cs in derive.required; some c in cs})

_cell(p, c) := object.get(derive.status, [p, c, "status"], "unproven") if c in object.get(derive.required, p, [])

_cell(p, c) := "" if not c in object.get(derive.required, p, [])

_conditions(cy) := [_evidence_condition(cy), _inputs_condition(cy), _actions_condition(cy)]

_plural(1, one, _) := sprintf("1 %s", [one])

_plural(n, _, many) := sprintf("%d %s", [n, many]) if n != 1

_people(n) := _plural(n, "person", "people")

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
