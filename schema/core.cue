package mgmt

import "time"

// #Date is a real calendar date written YYYY-MM-DD (2026-09-31 is rejected).
#Date:     =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" & time.Format("2006-01-02")
#Duration: =~"^[0-9]+[dw]$"

#CompetencyID: =~"^[a-z][a-z0-9-]*\\.[a-z][a-z0-9-]*$"
#Token:        =~"^[a-z][a-z0-9-]*$"

// #Resource is the base of every kind. Kind definitions unify with it and
// tighten spec.
#Resource: {
	apiVersion: "mgmt/v1alpha1"
	kind:       string
	metadata: {
		name: =~"^[a-z0-9][a-z0-9-]*$"
	}
	spec: _
}

#Workspace: #Resource & {
	kind: "Workspace"
	spec: {
		notes:           string
		me:              [...string]
		defaultWindow:   #Duration | *"90d"
		oneToOneCadence: #Duration | *"14d"
		policy?: [string]: {...}
	}
}

#Level: =~"^[A-Za-z0-9][A-Za-z0-9-]*$"

// #CompetencyFramework is a competency x level matrix. A competency is
// required at a level when it has a descriptor there. includes pulls in the
// competencies of other frameworks (transitively); engine.Validate checks
// what CUE cannot: unknown includes, include cycles, ladders that differ
// from an include's, descriptors off the ladder and duplicate ids.
#CompetencyFramework: #Resource & {
	kind: "CompetencyFramework"
	spec: {
		includes?: [...string]
		levels: [#Level, ...#Level]
		competencies: [...{
			id:       #CompetencyID
			title:    string
			tokens?:  [...#Token]
			summary?: string
			levels?: [string]: string
		}]
	}
}

#Person: #Resource & {
	kind: "Person"
	spec: {
		displayName?: string
		aliases?: [...string]
		framework:        string
		level:            string
		since:            #Date
		levelSince?:      #Date
		oneToOneCadence?: #Duration
	}
}

// #Selector needs at least one of people and matchFrameworks. That rule
// cannot be expressed in CUE without leaving the value incomplete, so
// engine.Validate checks it.
#Selector: {
	people?: [...string]
	matchFrameworks?: [...string]
}

#ReviewCycle: #Resource & {
	kind: "ReviewCycle"
	spec: {
		period: {from: #Date, to: #Date}
		selector: #Selector
		evidence: {
			minPerCompetency:  int & >=1 | *1
			negativeOverrides: bool | *true
		}
		inputs?: [...{
			name:      string
			source?:   string
			required?: bool
			minCount?: int & >=1
		}]
		milestones: [...{name: string, due: #Date}] | *[]
		outcomes?: {
			ratings?: [...string]
			promotionNomination?: bool
		}
	}
}
