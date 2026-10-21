package mgmt

#OKRSet: #Resource & {
	kind: "OKRSet"
	spec: {
		person: string
		cycle:  string
		objectives: [...{
			title: string
			keyResults: [...{title: string, competencies?: [...string]}]
		}]
	}
}
