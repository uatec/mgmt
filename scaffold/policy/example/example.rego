# Example workspace policy: edit it or delete it. Every *.rego under policy/
# loads alongside the core policies, with no rebuild. A policy is a package
# mgmt.policy.<name>; it reads the facts document (input, see `mgmt facts`)
# and data.mgmt.derive, and adds findings to deny (errors) or warn
# (warnings). Silence a warning with <!-- mgmt:ignore example-me-backlog -->
# (notes) or "# mgmt:ignore example-me-backlog" (in the Person's YAML).
package mgmt.policy.example

import data.mgmt.lib

# Warn when you owe one person more than _max_open open actions.
_max_open := 10

warn contains f if {
	some p, _ in object.get(input.resources, "Person", {})
	open := [a | some a in input.actions; a.person == p; a.owner == "me"; not a.done]
	count(open) > _max_open
	src := input.sources[sprintf("Person/%s", [p])]
	msg := sprintf("%s: you owe them %d open actions (more than %d)", [p, count(open), _max_open])
	f := lib.finding("example-me-backlog", p, src.file, src.line, msg)
}
