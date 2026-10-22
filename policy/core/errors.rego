# Core deny rules over resources: references, tokens and aliases.
# Extraction errors (token-unknown, person-unknown, ...) come from Go.
package mgmt.policy.core

import data.mgmt.lib

_frameworks := object.get(input, ["resources", "CompetencyFramework"], {})

_persons := object.get(input, ["resources", "Person"], {})

_cycles := object.get(input, ["resources", "ReviewCycle"], {})

# _src locates a resource's document; unknown sources report at line 0.
_src(kind, name) := object.get(input, ["sources", sprintf("%s/%s", [kind, name])], {"file": "", "line": 0})

_at(kind, name, id, person, message) := lib.finding(id, person, s.file, s.line, message) if {
	s := _src(kind, name)
}

_ladder(fw) := object.get(fw, ["spec", "levels"], [])

# --- ref-unknown ----------------------------------------------------------

deny contains f if {
	some name, p in _persons
	not _frameworks[p.spec.framework]
	f := _at("Person", name, "ref-unknown", name, sprintf("person %q: unknown framework %q", [name, p.spec.framework]))
}

deny contains f if {
	some name, p in _persons
	fw := _frameworks[p.spec.framework]
	not p.spec.level in _ladder(fw)
	f := _at("Person", name, "ref-unknown", name, sprintf("person %q: level %q is not on the ladder of framework %q (%s)", [name, p.spec.level, p.spec.framework, concat(", ", _ladder(fw))]))
}

deny contains f if {
	some name, c in _cycles
	some p in object.get(c, ["spec", "selector", "people"], [])
	not _persons[p]
	f := _at("ReviewCycle", name, "ref-unknown", "", sprintf("review cycle %q: selector names unknown person %q", [name, p]))
}

deny contains f if {
	some name, c in _cycles
	some fw in object.get(c, ["spec", "selector", "matchFrameworks"], [])
	not _frameworks[fw]
	f := _at("ReviewCycle", name, "ref-unknown", "", sprintf("review cycle %q: selector names unknown framework %q", [name, fw]))
}

# --- token-duplicate ------------------------------------------------------

# A framework's tokens are those of its own competencies and of every
# framework it includes, transitively.
_includes := {name: object.get(fw, ["spec", "includes"], []) | some name, fw in _frameworks}

_closure(name) := graph.reachable(_includes, {name})

_comps(name) := [c | some n in _closure(name); some c in object.get(_frameworks, [n, "spec", "competencies"], [])]

_suffix(id) := lower(regex.replace(id, `^.*\.`, ""))

_claims(name) := {[lower(t), c.id] | some c in _comps(name); some t in object.get(c, "tokens", [])}

# _claimants(name, t) is the ids that token t could mean in framework name.
_claimants(name, t) := {id | some [t2, id] in _claims(name); t2 == t} | {c.id | some c in _comps(name); _suffix(c.id) == t}

deny contains f if {
	some name in object.keys(_frameworks)
	some [t, _] in _claims(name)
	ids := _claimants(name, t)
	count(ids) > 1
	# a collision already inside an included framework is reported there
	not _collides_in_include(name, t)
	f := _at("CompetencyFramework", name, "token-duplicate", "", sprintf("token %q is claimed by %s", [t, concat(", ", sort(ids))]))
}

_collides_in_include(name, t) if {
	some inc in _closure(name)
	inc != name
	count(_claimants(inc, t)) > 1
}

# --- alias-duplicate ------------------------------------------------------

_keys(name, p) := {lower(name)} | {lower(a) | some a in object.get(p, ["spec", "aliases"], [])}

deny contains f if {
	some key in {k | some n, p in _persons; some k in _keys(n, p)}
	owners := {n | some n, p in _persons; key in _keys(n, p)}
	count(owners) > 1
	# report at the owner declared last
	last := max({[s.file, s.line, n] | some n in owners; s := _src("Person", n)})
	f := _at("Person", last[2], "alias-duplicate", last[2], sprintf("name or alias %q is shared by %s", [key, concat(", ", sort(owners))]))
}
