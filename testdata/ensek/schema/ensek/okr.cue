package mgmt

// #OKRSet is Ensek's per-cycle objectives for one person. Each objective
// links the framework competencies it should produce evidence for. Key
// results and linked competencies are optional; the okrs report and the
// okr-no-evidence policy treat a missing list as empty.
#OKRSet: #Resource & {
	kind: "OKRSet"
	spec: {
		person: string
		cycle:  string
		objectives: [...{
			title: string
			keyResults?: [...string]
			competencies?: [...string]
		}]
	}
}
