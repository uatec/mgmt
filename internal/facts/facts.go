// Package facts defines the facts document and finding types shared between
// the Go extractor and the Rego policies.
package facts

import "encoding/json"

// Source locates the start of a resource document.
type Source struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

// Evidence is a +x or -x mark against a competency.
type Evidence struct {
	Person     string `json:"person"`
	Competency string `json:"competency"`
	Polarity   int    `json:"polarity"`
	Date       string `json:"date"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Text       string `json:"text"`
}

// Touch is a ~x mark: a competency was discussed without a polarity.
type Touch struct {
	Person     string `json:"person"`
	Competency string `json:"competency"`
	Date       string `json:"date"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Text       string `json:"text"`
}

// Action is a checkbox item. Due is "" when none; Addresses is [] when none.
type Action struct {
	ID        string   `json:"id"`
	Person    string   `json:"person"`
	Owner     string   `json:"owner"`
	Due       string   `json:"due"`
	Done      bool     `json:"done"`
	Addresses []string `json:"addresses"`
	Date      string   `json:"date"`
	File      string   `json:"file"`
	Line      int      `json:"line"`
	Text      string   `json:"text"`
}

// Input is a !name mark resolved against a review cycle.
type Input struct {
	Person string `json:"person"`
	Input  string `json:"input"`
	Cycle  string `json:"cycle"`
	Date   string `json:"date"`
	File   string `json:"file"`
	Line   int    `json:"line"`
}

// Section is a heading that names exactly one person.
type Section struct {
	Person     string `json:"person"`
	Date       string `json:"date"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	TokenCount int    `json:"tokenCount"`
}

// Ignore is a <!-- mgmt:ignore ... --> comment.
type Ignore struct {
	File string   `json:"file"`
	Line int      `json:"line"`
	IDs  []string `json:"ids"`
}

// Diagnostic is a problem found by the extractor.
type Diagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Suggest  string `json:"suggest,omitempty"`
}

// Options carries run options into the policies.
type Options struct {
	Cycle string `json:"cycle"`
}

// Facts is the document handed to the policies.
type Facts struct {
	Now         string                    `json:"now"`
	Options     Options                   `json:"options"`
	Resources   map[string]map[string]any `json:"resources"` // kind -> metadata.name -> full object
	Sources     map[string]Source         `json:"sources"`   // "Kind/name" -> document start
	Evidence    []Evidence                `json:"evidence"`
	Touches     []Touch                   `json:"touches"`
	Actions     []Action                  `json:"actions"`
	Inputs      []Input                   `json:"inputs"`
	Sections    []Section                 `json:"sections"`
	Ignores     []Ignore                  `json:"ignores"`
	Diagnostics []Diagnostic              `json:"diagnostics"`
}

// Finding is a lint result.
type Finding struct {
	Severity string `json:"severity"`
	ID       string `json:"id"`
	Person   string `json:"person"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
	Suggest  string `json:"suggest,omitempty"`
}

// MarshalJSON guarantees [] and {} rather than null for empty collections.
func (f Facts) MarshalJSON() ([]byte, error) {
	type plain Facts
	p := plain(f)
	if p.Resources == nil {
		p.Resources = map[string]map[string]any{}
	}
	if p.Sources == nil {
		p.Sources = map[string]Source{}
	}
	if p.Evidence == nil {
		p.Evidence = []Evidence{}
	}
	if p.Touches == nil {
		p.Touches = []Touch{}
	}
	if p.Actions == nil {
		p.Actions = []Action{}
	}
	if p.Inputs == nil {
		p.Inputs = []Input{}
	}
	if p.Sections == nil {
		p.Sections = []Section{}
	}
	if p.Ignores == nil {
		p.Ignores = []Ignore{}
	}
	if p.Diagnostics == nil {
		p.Diagnostics = []Diagnostic{}
	}
	return json.Marshal(p)
}
