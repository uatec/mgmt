package extract

import (
	"sort"
	"strings"
	"unicode"

	"github.com/uatec/mgmt/internal/facts"
	"github.com/uatec/mgmt/internal/load"
)

// Resolution is the outcome of resolving a competency token.
type Resolution int

const (
	Resolved    Resolution = iota
	Unknown                // no competency matches: token-unknown
	Duplicate              // declared by two competencies: Rego reports token-duplicate
	NoFramework            // the person's framework doesn't exist: Rego reports ref-unknown
)

// Index holds the lookups the extractor needs, built from resources.
type Index struct {
	persons   map[string]string          // lower-case name or alias -> person name
	names     []string                   // lower-case names and aliases, sorted
	headings  map[string]string          // persons, plus lower-case displayName -> person name
	headNames []string                   // keys of headings, sorted
	framework map[string]string          // person -> framework name
	closure   map[string]map[string]bool // framework -> itself and its includes, transitively
	tokens    map[string]*tokenMap       // framework -> tokens of its effective competencies
	cycles    []cycle                    // sorted by name
	me        map[string]bool
}

// tokenMap resolves tokens for one framework. An empty id marks a duplicate.
type tokenMap struct {
	explicit, full, suffix map[string]string
}

type cycle struct {
	name       string
	from, to   string // active range, YYYY-MM-DD inclusive
	people     map[string]bool
	frameworks map[string]bool   // selector.matchFrameworks
	inputs     map[string]string // lower-case name -> declared name
}

// NewIndex builds the lookups from validated resources.
func NewIndex(resources []load.Resource) *Index {
	idx := &Index{
		persons:   map[string]string{},
		headings:  map[string]string{},
		framework: map[string]string{},
		closure:   map[string]map[string]bool{},
		tokens:    map[string]*tokenMap{},
		me:        map[string]bool{},
	}
	specs := frameworkSpecs(resources)
	for name := range specs {
		names := IncludeClosure(specs, name)
		idx.closure[name] = map[string]bool{}
		var comps []any
		for _, n := range names {
			idx.closure[name][n] = true
			c, _ := specs[n]["competencies"].([]any)
			comps = append(comps, c...)
		}
		idx.tokens[name] = newTokenMap(comps)
	}
	for _, r := range resources {
		spec, _ := r.Object["spec"].(map[string]any)
		switch r.Kind {
		case "Workspace":
			for _, m := range strs(spec["me"]) {
				idx.me[strings.ToLower(m)] = true
			}
		case "Person":
			idx.persons[strings.ToLower(r.Name)] = r.Name
			for _, a := range strs(spec["aliases"]) {
				idx.persons[strings.ToLower(a)] = r.Name
			}
			idx.framework[r.Name], _ = spec["framework"].(string)
			if d, _ := spec["displayName"].(string); strings.TrimSpace(d) != "" {
				idx.headings[strings.ToLower(strings.TrimSpace(d))] = r.Name
			}
		case "ReviewCycle":
			idx.cycles = append(idx.cycles, newCycle(r.Name, spec))
		}
	}
	for n, p := range idx.persons {
		idx.names = append(idx.names, n)
		idx.headings[n] = p // a name or alias wins over another's displayName
	}
	sort.Strings(idx.names)
	for n := range idx.headings {
		idx.headNames = append(idx.headNames, n)
	}
	sort.Strings(idx.headNames)
	sort.Slice(idx.cycles, func(i, j int) bool { return idx.cycles[i].name < idx.cycles[j].name })
	return idx
}

// frameworkSpecs maps each CompetencyFramework name to its spec.
func frameworkSpecs(resources []load.Resource) map[string]map[string]any {
	specs := map[string]map[string]any{}
	for _, r := range resources {
		if r.Kind == "CompetencyFramework" {
			specs[r.Name], _ = r.Object["spec"].(map[string]any)
		}
	}
	return specs
}

// IncludeClosure lists name and then every framework it includes, depth
// first in declared order, each once; specs maps framework name to spec.
// Unknown includes are skipped (Validate reports them).
func IncludeClosure(specs map[string]map[string]any, name string) []string {
	seen := map[string]bool{}
	var out []string
	var visit func(string)
	visit = func(n string) {
		if _, ok := specs[n]; !ok || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
		for _, inc := range strs(specs[n]["includes"]) {
			visit(inc)
		}
	}
	visit(name)
	return out
}

// Frameworks builds the facts' effective frameworks (Decision N): each
// framework's ladder with its own competencies and those of everything it
// includes, transitively.
func Frameworks(resources []load.Resource) map[string]facts.Framework {
	specs := frameworkSpecs(resources)
	out := make(map[string]facts.Framework, len(specs))
	for name, spec := range specs {
		fw := facts.Framework{Levels: strs(spec["levels"]), Competencies: map[string]facts.Competency{}}
		for _, n := range IncludeClosure(specs, name) {
			comps, _ := specs[n]["competencies"].([]any)
			for _, c := range comps {
				cm, _ := c.(map[string]any)
				id, _ := cm["id"].(string)
				if _, dup := fw.Competencies[id]; dup || id == "" {
					continue
				}
				fc := facts.Competency{ID: id, Theme: strings.SplitN(id, ".", 2)[0], Levels: map[string]string{}}
				fc.Title, _ = cm["title"].(string)
				fc.Summary, _ = cm["summary"].(string)
				levels, _ := cm["levels"].(map[string]any)
				for l, d := range levels {
					fc.Levels[l], _ = d.(string)
				}
				fw.Competencies[id] = fc
			}
		}
		out[name] = fw
	}
	return out
}

func newTokenMap(comps []any) *tokenMap {
	tm := &tokenMap{explicit: map[string]string{}, full: map[string]string{}, suffix: map[string]string{}}
	add := func(m map[string]string, key, id string) {
		key = strings.ToLower(key)
		if prev, ok := m[key]; ok && prev != id {
			m[key] = ""
			return
		}
		m[key] = id
	}
	for _, c := range comps {
		cm, _ := c.(map[string]any)
		id, _ := cm["id"].(string)
		if id == "" {
			continue
		}
		for _, tok := range strs(cm["tokens"]) {
			add(tm.explicit, tok, id)
		}
		add(tm.full, id, id)
		if i := strings.LastIndexByte(id, '.'); i >= 0 {
			add(tm.suffix, id[i+1:], id)
		}
	}
	return tm
}

func newCycle(name string, spec map[string]any) cycle {
	c := cycle{name: name, people: map[string]bool{}, frameworks: map[string]bool{}, inputs: map[string]string{}}
	period, _ := spec["period"].(map[string]any)
	c.from, _ = period["from"].(string)
	c.to, _ = period["to"].(string)
	ms, _ := spec["milestones"].([]any)
	for _, m := range ms {
		mm, _ := m.(map[string]any)
		if due, _ := mm["due"].(string); due > c.to {
			c.to = due
		}
	}
	sel, _ := spec["selector"].(map[string]any)
	for _, p := range strs(sel["people"]) {
		c.people[p] = true
	}
	for _, f := range strs(sel["matchFrameworks"]) {
		c.frameworks[f] = true
	}
	ins, _ := spec["inputs"].([]any)
	for _, in := range ins {
		im, _ := in.(map[string]any)
		if n, _ := im["name"].(string); n != "" {
			c.inputs[strings.ToLower(n)] = n
		}
	}
	return c
}

func strs(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, e := range list {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Person resolves an @-token body to a person name, case-insensitively.
func (idx *Index) Person(body string) (string, bool) {
	p, ok := idx.persons[strings.ToLower(body)]
	return p, ok
}

// IsMe reports whether an @-token body is one of Workspace.spec.me.
func (idx *Index) IsMe(body string) bool { return idx.me[strings.ToLower(body)] }

// HeadingPersons returns the sorted, distinct persons whose name, alias or
// displayName appears in text as a whole word (bounded by non-letters and
// non-digits), so "## [[Craig Bartram]]" names the person displayed so.
func (idx *Index) HeadingPersons(text string) []string {
	lower := strings.ToLower(text)
	seen := map[string]bool{}
	out := []string{}
	for _, n := range idx.headNames {
		if p := idx.headings[n]; !seen[p] && containsWord(lower, n) {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func containsWord(s, w string) bool {
	if w == "" {
		return false
	}
	for from := 0; ; {
		i := strings.Index(s[from:], w)
		if i < 0 {
			return false
		}
		start, end := from+i, from+i+len(w)
		if !wordRuneBefore(s, start) && !wordRuneAt(s, end) {
			return true
		}
		from = start + 1
	}
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func wordRuneBefore(s string, i int) bool {
	if i == 0 {
		return false
	}
	r := []rune(s[:i])
	return isWordRune(r[len(r)-1])
}

func wordRuneAt(s string, i int) bool {
	for _, r := range s[i:] {
		return isWordRune(r)
	}
	return false
}

// Competency resolves a token for a person: explicit tokens, then full id,
// then unambiguous id suffix, all case-insensitive.
func (idx *Index) Competency(person, token string) (string, Resolution) {
	tm, ok := idx.tokens[idx.framework[person]]
	if !ok {
		return "", NoFramework
	}
	key := strings.ToLower(token)
	if id, ok := tm.explicit[key]; ok {
		if id == "" {
			return "", Duplicate
		}
		return id, Resolved
	}
	if id, ok := tm.full[key]; ok && id != "" {
		return id, Resolved
	}
	if id, ok := tm.suffix[key]; ok && id != "" {
		return id, Resolved
	}
	return "", Unknown
}

// Cycle returns the first cycle (by name) that selects the person, declares
// the input (case-insensitively) and whose active range contains date
// (YYYY-MM-DD), together with the input's declared name.
func (idx *Index) Cycle(person, input, date string) (cycle, name string, ok bool) {
	key := strings.ToLower(input)
	for _, c := range idx.cycles {
		if !c.selects(idx, person) {
			continue
		}
		if n, has := c.inputs[key]; has && c.from <= date && date <= c.to {
			return c.name, n, true
		}
	}
	return "", "", false
}

// selects reports whether the cycle names the person, or lists their
// framework or a framework it includes (Decision R).
func (c cycle) selects(idx *Index, person string) bool {
	if c.people[person] {
		return true
	}
	for f := range idx.closure[idx.framework[person]] {
		if c.frameworks[f] {
			return true
		}
	}
	return false
}

// InputNames lists the inputs of every cycle that selects the person, sorted.
func (idx *Index) InputNames(person string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, c := range idx.cycles {
		if !c.selects(idx, person) {
			continue
		}
		for _, n := range c.inputs {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	sort.Strings(out)
	return out
}

// PersonNames lists every person name and alias, lower-case and sorted.
func (idx *Index) PersonNames() []string { return idx.names }

// TokenNames lists the tokens, ids and unambiguous id suffixes that resolve
// for the person, lower-case and sorted.
func (idx *Index) TokenNames(person string) []string {
	tm, ok := idx.tokens[idx.framework[person]]
	if !ok {
		return nil
	}
	out := []string{}
	for _, m := range []map[string]string{tm.explicit, tm.full, tm.suffix} {
		for k, id := range m {
			if id != "" {
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}
