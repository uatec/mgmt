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

_roles(fw) := object.get(fw, ["spec", "roles"], [])

_levels(role) := object.get(role, "levels", [])

_has_role(fw, name) if {
	some r in _roles(fw)
	r.name == name
}

_has_level(fw, role, level) if {
	some r in _roles(fw)
	r.name == role
	some l in _levels(r)
	l.name == level
}

# --- ref-unknown ----------------------------------------------------------

deny contains f if {
	some name, p in _persons
	not _frameworks[p.spec.framework]
	f := _at("Person", name, "ref-unknown", name, sprintf("person %q: unknown framework %q", [name, p.spec.framework]))
}

deny contains f if {
	some name, p in _persons
	fw := _frameworks[p.spec.framework]
	not _has_role(fw, p.spec.role)
	f := _at("Person", name, "ref-unknown", name, sprintf("person %q: unknown role %q in framework %q", [name, p.spec.role, p.spec.framework]))
}

deny contains f if {
	some name, p in _persons
	fw := _frameworks[p.spec.framework]
	_has_role(fw, p.spec.role)
	not _has_level(fw, p.spec.role, p.spec.level)
	f := _at("Person", name, "ref-unknown", name, sprintf("person %q: unknown level %q for role %q", [name, p.spec.level, p.spec.role]))
}

deny contains f if {
	some name, c in _cycles
	not _frameworks[c.spec.framework]
	f := _at("ReviewCycle", name, "ref-unknown", "", sprintf("review cycle %q: unknown framework %q", [name, c.spec.framework]))
}

deny contains f if {
	some name, c in _cycles
	some p in object.get(c, ["spec", "selector", "people"], [])
	not _persons[p]
	f := _at("ReviewCycle", name, "ref-unknown", "", sprintf("review cycle %q: selector names unknown person %q", [name, p]))
}

deny contains f if {
	some name, fw in _frameworks
	some r in _roles(fw)
	some l in _levels(r)
	l.inherits
	not _has_level(fw, r.name, l.inherits)
	f := _at("CompetencyFramework", name, "ref-unknown", "", sprintf("role %q: level %q inherits unknown level %q", [r.name, l.name, l.inherits]))
}

deny contains f if {
	some name, fw in _frameworks
	ids := {c.id | some c in object.get(fw, ["spec", "competencies"], [])}
	some r in _roles(fw)
	some l in _levels(r)
	some field in ["expects", "removes"]
	some id in object.get(l, field, [])
	not id in ids
	f := _at("CompetencyFramework", name, "ref-unknown", "", sprintf("role %q: level %q %s unknown competency %q", [r.name, l.name, field, id]))
}

# _looping(r) is the set of levels in role r that reach themselves via inherits.
_looping(r) := {l.name |
	g := {x.name: {x.inherits | x.inherits} | some x in _levels(r)}
	some l in _levels(r)
	l.inherits
	l.name in graph.reachable(g, {l.inherits})
}

deny contains f if {
	some name, fw in _frameworks
	some r in _roles(fw)
	loop := _looping(r)
	count(loop) > 0
	f := _at("CompetencyFramework", name, "ref-unknown", "", sprintf("role %q: levels %s inherit in a loop", [r.name, concat(", ", sort(loop))]))
}

# --- token-duplicate ------------------------------------------------------

_suffix(id) := lower(regex.replace(id, `^.*\.`, ""))

_claims(fw) := {[lower(t), c.id] | some c in fw.spec.competencies; some t in object.get(c, "tokens", [])}

deny contains f if {
	some name, fw in _frameworks
	claims := _claims(fw)
	some [t, _] in claims
	ids := {id | some [t2, id] in claims; t2 == t} | {c.id | some c in fw.spec.competencies; _suffix(c.id) == t}
	count(ids) > 1
	f := _at("CompetencyFramework", name, "token-duplicate", "", sprintf("token %q is claimed by %s", [t, concat(", ", sort(ids))]))
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
