package mgmt

// #OKRSet is Ensek's per-cycle objectives for one person. Each objective
// links the framework competencies it should produce evidence for.
#OKRSet: #Resource & {
	kind: "OKRSet"
	spec: {
		person: string
		cycle:  string
		objectives: [...{
			title: string
			keyResults: [...string]
			competencies: [...string]
		}]
	}
}
