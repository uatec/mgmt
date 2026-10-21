# Ensek's OKR policy (spec worked example): in the 14 days up to the
# manager-draft milestone, warn about every competency an OKR objective
# links that still has no evidence for the person.
package mgmt.policy.ensek

import data.mgmt.derive
import data.mgmt.lib

_lead_days := 14

warn contains f if {
	some name, okr in object.get(input, ["resources", "OKRSet"], {})
	p := okr.spec.person
	cycle := object.get(input, ["resources", "ReviewCycle", okr.spec.cycle], {})
	some m in object.get(cycle, ["spec", "milestones"], [])
	m.name == "manager-draft"
	lib.add_days(m.due, 0 - _lead_days) <= input.now
	input.now <= m.due
	some o in okr.spec.objectives
	some c in o.competencies
	derive.status[p][c].status == "unproven"
	src := input.sources[sprintf("OKRSet/%s", [name])]
	msg := sprintf("%s: OKR %q links %s, which is unproven; manager-draft is due %s", [p, o.title, c, m.due])
	f := lib.finding("okr-no-evidence", p, src.file, src.line, msg)
}
