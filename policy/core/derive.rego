# Package mgmt.derive turns the facts into the core model: active cycles,
# evidence windows, required competencies, statuses, gaps and cycle inputs.
# Every rule is total on schema-valid facts: optional fields go through
# object.get, and missing references leave a rule undefined rather than error.
package mgmt.derive

import data.mgmt.lib

_now := object.get(input, "now", "")

_persons := object.get(input, ["resources", "Person"], {})

# _frameworks holds the raw CompetencyFramework resources (only their
# includes are read here); competency data comes from input.frameworks, the
# effective frameworks resolved in Go (Decision N).
_frameworks := object.get(input, ["resources", "CompetencyFramework"], {})

_effective := object.get(input, "frameworks", {})

_cycles := object.get(input, ["resources", "ReviewCycle"], {})

# default_window is Workspace.spec.defaultWindow, used without a cycle.
default_window := _default_window

_default_window := w if {
	ws := [object.get(x, ["spec", "defaultWindow"], "90d") | some x in object.get(input, ["resources", "Workspace"], {})]
	count(ws) > 0
	w := ws[0]
} else := "90d"

# _sorted_by_loc sorts records ascending by [date, file, line].
_sorted_by_loc(records) := [r | some pair in sort([[[r.date, r.file, r.line], r] | some r in records]); r := pair[1]]

_framework(p) := _effective[_persons[p].spec.framework]

_since(p) := object.get(_persons[p], ["spec", "since"], "")

# level_since[p] is when the person reached their level (Decision Q),
# clamped to since: nothing before since counts, as in every window.
level_since[p] := _level_since(p) if some p in object.keys(_persons)

_level_since(p) := lib.max_date(object.get(_persons[p], ["spec", "levelSince"], _since(p)), _since(p))

# _includes maps each framework to the frameworks it includes directly.
_includes := {name: object.get(fw, ["spec", "includes"], []) | some name, fw in _frameworks}

# --- cycles ---------------------------------------------------------------

# _active_end is the later of period.to and the last milestone's due date.
_active_end(c) := max({c.spec.period.to} | {m.due | some m in object.get(c, ["spec", "milestones"], [])})

# active_end[cy] is the last day of the cycle's active range.
active_end[cy] := _active_end(c) if some cy, c in _cycles

_option_cycle := object.get(input, ["options", "cycle"], "")

active_cycles := [_option_cycle] if _option_cycle != ""

active_cycles := sort([name |
	some name, c in _cycles
	c.spec.period.from <= _now
	_now <= _active_end(c)
]) if _option_cycle == ""

cycle_people[cy] := sort([p |
	some p, person in _persons
	_selects(c, p, person)
]) if some cy, c in _cycles

_selects(c, p, _) if p in object.get(c, ["spec", "selector", "people"], [])

# matchFrameworks selects a person whose framework, or a framework it
# includes transitively, is listed (Decision R).
_selects(c, _, person) if {
	some f in graph.reachable(_includes, {person.spec.framework})
	f in object.get(c, ["spec", "selector", "matchFrameworks"], [])
}

cycle[p] := cys[0] if {
	some p in object.keys(_persons)
	cys := [cy | some cy in active_cycles; p in object.get(cycle_people, cy, [])]
	count(cys) > 0
}

next_milestone[cy] := {"name": ms[0][1], "due": ms[0][0]} if {
	some cy, c in _cycles
	ms := sort([[m.due, m.name] | some m in object.get(c, ["spec", "milestones"], []); m.due >= _now])
	count(ms) > 0
}

inputs[cy][p] := [_input_status(cy, p, i) | some i in object.get(c, ["spec", "inputs"], [])] if {
	some cy, c in _cycles
	some p in cycle_people[cy]
}

_input_status(cy, p, i) := {
	"name": i.name,
	"required": required,
	"minCount": min_count,
	"received": count(records),
	"complete": _complete(i, count(records)),
	"records": records,
} if {
	required := object.get(i, "required", false)
	min_count := object.get(i, "minCount", 0)
	records := _sorted_by_loc([r |
		some r in object.get(input, "inputs", [])
		r.cycle == cy
		r.person == p
		r.input == i.name
	])
}

_complete(i, received) := received >= max({object.get(i, "minCount", 0), 1}) if {
	_counted(i)
} else := true

_counted(i) if object.get(i, "required", false) == true

_counted(i) if "minCount" in object.keys(i)

# --- per-person window and settings ---------------------------------------

window[p] := {"from": lib.max_date(from, _level_since(p)), "to": to, "cycle": cy} if {
	some p in object.keys(_persons)
	[from, to, cy] := _window_range(p)
}

_window_range(p) := [c.spec.period.from, c.spec.period.to, cycle[p]] if {
	c := _cycles[cycle[p]]
} else := [lib.add_days(_now, 0 - lib.days(_default_window)), _now, null]

settings[p] := {
	"minPerCompetency": object.get(ev, "minPerCompetency", 1),
	"scoring": object.get(ev, "scoring", "weighted"),
} if {
	some p in object.keys(_persons)
	ev := _cycle_evidence(p)
}

_cycle_evidence(p) := object.get(_cycles[cycle[p]], ["spec", "evidence"], {}) if {
	_cycles[cycle[p]]
} else := {}

# --- competencies and requirements ----------------------------------------

# competencies[p] is the person's effective framework's competencies, by id:
# {id, title, theme, summary, levels: {level: descriptor}}.
competencies[p] := object.get(fw, "competencies", {}) if {
	some p in object.keys(_persons)
	fw := _framework(p)
}

# ladder[p] is the person's framework's ordered levels, lowest first.
ladder[p] := object.get(fw, "levels", []) if {
	some p in object.keys(_persons)
	fw := _framework(p)
}

level[p] := person.spec.level if some p, person in _persons

# next_level[p] is the ladder entry after the person's level; undefined at
# the top or when the level is not on the ladder.
next_level[p] := l[i + 1] if {
	some p, l in ladder
	some i, x in l
	x == level[p]
	i + 1 < count(l)
}

# descriptor[p][c] is competency c's descriptor at the person's level.
descriptor[p][c] := d if {
	some p, comps in competencies
	some c, comp in comps
	d := comp.levels[level[p]]
}

# A competency is required when it has a descriptor at the person's level.
required[p] := sort([c | some c in object.keys(object.get(descriptor, p, {}))]) if {
	some p in object.keys(competencies)
}

# --- evidence -------------------------------------------------------------

# Per-person indexes over the input records, built once so the rules below
# do not rescan every record for every person and competency.
evidence_by[p] := [e | some e in object.get(input, "evidence", []); e.person == p] if some p in object.keys(_persons)

touches_by[p] := [t | some t in object.get(input, "touches", []); t.person == p] if some p in object.keys(_persons)

actions_by[p] := [a | some a in object.get(input, "actions", []); a.person == p] if some p in object.keys(_persons)

sections_by[p] := [s | some s in object.get(input, "sections", []); s.person == p] if some p in object.keys(_persons)

_evidence_for(p, c) := [e | some e in evidence_by[p]; e.competency == c]

marks[p][c] := _sorted_by_loc([e |
	some e in _evidence_for(p, c)
	window[p].from <= e.date
	e.date <= window[p].to
]) if {
	some p, comps in competencies
	some c in object.keys(comps)
}

outside[p][c] := _sorted_by_loc([e |
	some e in _evidence_for(p, c)
	e.date >= _since(p)
	not _in_window(window[p], e.date)
]) if {
	some p, comps in competencies
	some c in object.keys(comps)
}

_in_window(w, date) if {
	w.from <= date
	date <= w.to
}

# scored[p][c] is the in-window marks, newest first, each with its weight
# (+1, +2, -1, -2 by polarity and strength; 0 when neutralised) and the
# completed action that neutralised it (or null).
#
# A negative is neutralised only by a completed action addressing it. For
# the k completed actions of p addressing c dated on or before now (in or
# after the window), the k most recent eligible negatives (those with a
# completed action dated on or after them) are neutralised, the i-th most
# recent paired with the i-th most recent action.
scored[p][c] := [object.union(m, {"weight": _weight(m, nb), "neutralisedBy": nb}) |
	some i, m in newest
	nb := object.get(paired, i, null)
] if {
	some p, comps in marks
	some c, ms in comps
	newest := array.reverse(ms)
	done := _completed(p, c)
	eligible := [i | some i, m in newest; m.polarity < 0; _eligible(m, done)]
	paired := {i: _action_ref(done[j]) | some j, i in eligible; j < count(done)}
}

# _completed(p, c) is p's completed actions addressing c, dated up to now,
# newest first by [date, file, line].
_completed(p, c) := array.reverse(_sorted_by_loc([a |
	some a in actions_by[p]
	a.done == true
	c in a.addresses
	a.date <= _now
]))

_eligible(m, done) if {
	count(done) > 0
	m.date <= done[0].date
}

_action_ref(a) := {"file": a.file, "line": a.line, "text": a.text}

_weight(_, nb) := 0 if nb != null

_weight(m, null) := m.polarity * object.get(m, "strength", 1)

status[p][c] := {
	"status": decision[0],
	"positives": pos,
	"negatives": neg,
	"score": score,
	"decidedBy": decision[1],
	"rule": s.scoring,
} if {
	some p, comps in scored
	some c, ms in comps
	s := settings[p]
	pos := count([m | some m in ms; m.polarity > 0])
	neg := count([m | some m in ms; m.polarity < 0])
	score := sum([m.weight | some m in ms])
	decision := _decide(ms, pos, score, s.minPerCompetency, s.scoring)
}

# _decide returns [status, decidedBy] for the newest-first scored marks.
# latest: the most recent mark decides.
_decide(ms, pos, _, minimum, "latest") := ["unproven", null] if {
	count(ms) == 0
} else := ["disproven", _loc(ms[0])] if {
	ms[0].polarity < 0
} else := ["proven", _loc(ms[0])] if {
	pos >= minimum
} else := ["unproven", null]

# weighted: proven above 0 with enough positive marks; disproven below 0.
_decide(_, pos, score, minimum, "weighted") := ["proven", null] if {
	score > 0
	pos >= minimum
} else := ["disproven", null] if {
	score < 0
} else := ["unproven", null]

_loc(m) := {"file": m.file, "line": m.line}

_gap_rank := {"disproven": 0, "unproven": 1}

gaps[p] := [g |
	some pair in sort([[[_gap_rank[st], c], {"competency": c, "status": st}] |
		some c in reqs
		st := status[p][c].status
		st != "proven"
	])
	g := pair[1]
] if some p, reqs in required

last_touch[p][c] := max(dates) if {
	some p, comps in competencies
	some c in object.keys(comps)
	records := array.concat(_evidence_for(p, c), [t | some t in touches_by[p]; t.competency == c])
	dates := {r.date | some r in records; r.date >= _since(p); r.date <= _now}
	count(dates) > 0
}

addressed_by[p][c] := _sorted_by_loc([a |
	some a in actions_by[p]
	c in a.addresses
]) if {
	some p, comps in competencies
	some c in object.keys(comps)
}
