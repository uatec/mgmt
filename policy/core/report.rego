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
