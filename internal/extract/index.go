package extract

import (
	"sort"
	"strings"
	"unicode"

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
	persons   map[string]string // lower-case name or alias -> person name
	names     []string          // lower-case names and aliases, sorted
	framework map[string]string // person -> framework name
	role      map[string]string // person -> role
	tokens    map[string]*tokenMap
	cycles    []cycle // sorted by name
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
	matchRoles map[string]bool
	inputs     map[string]bool
}

// NewIndex builds the lookups from validated resources.
func NewIndex(resources []load.Resource) *Index {
	idx := &Index{
		persons:   map[string]string{},
		framework: map[string]string{},
		role:      map[string]string{},
		tokens:    map[string]*tokenMap{},
		me:        map[string]bool{},
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
			idx.role[r.Name], _ = spec["role"].(string)
		case "CompetencyFramework":
			idx.tokens[r.Name] = newTokenMap(spec)
		case "ReviewCycle":
			idx.cycles = append(idx.cycles, newCycle(r.Name, spec))
		}
	}
	for n := range idx.persons {
		idx.names = append(idx.names, n)
	}
	sort.Strings(idx.names)
	sort.Slice(idx.cycles, func(i, j int) bool { return idx.cycles[i].name < idx.cycles[j].name })
	return idx
}

func newTokenMap(spec map[string]any) *tokenMap {
	tm := &tokenMap{explicit: map[string]string{}, full: map[string]string{}, suffix: map[string]string{}}
	add := func(m map[string]string, key, id string) {
		key = strings.ToLower(key)
		if prev, ok := m[key]; ok && prev != id {
			m[key] = ""
			return
		}
		m[key] = id
	}
	comps, _ := spec["competencies"].([]any)
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
	c := cycle{name: name, people: map[string]bool{}, matchRoles: map[string]bool{}, inputs: map[string]bool{}}
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
	for _, r := range strs(sel["matchRoles"]) {
		c.matchRoles[r] = true
	}
	ins, _ := spec["inputs"].([]any)
	for _, in := range ins {
		im, _ := in.(map[string]any)
		if n, _ := im["name"].(string); n != "" {
			c.inputs[n] = true
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

// HeadingPersons returns the sorted, distinct persons whose name or alias
// appears in text as a whole word (bounded by non-letters and non-digits).
func (idx *Index) HeadingPersons(text string) []string {
	lower := strings.ToLower(text)
	seen := map[string]bool{}
	out := []string{}
	for _, n := range idx.names {
		if p := idx.persons[n]; !seen[p] && containsWord(lower, n) {
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
// the input and whose active range contains date (YYYY-MM-DD).
func (idx *Index) Cycle(person, input, date string) (string, bool) {
	for _, c := range idx.cycles {
		if !c.people[person] && !c.matchRoles[idx.role[person]] {
			continue
		}
		if c.inputs[input] && c.from <= date && date <= c.to {
			return c.name, true
		}
	}
	return "", false
}
