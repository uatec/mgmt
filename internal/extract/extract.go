package extract

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"

	"github.com/uatec/mgmt/internal/facts"
	"github.com/uatec/mgmt/internal/load"
)

var (
	ignoreRe   = regexp.MustCompile(`<!--\s*mgmt:ignore\s+([^>]*?)\s*-->`)
	checkboxRe = regexp.MustCompile(`^\s*\[[ xX]\]\s*`)
	markdown   = goldmark.New(goldmark.WithExtensions(extension.TaskList))
)

// Extract turns notes into facts records, in file, line and token order.
// Resources, Sources, Now and Options are left for the caller to fill.
func Extract(files []load.NoteFile, idx *Index) facts.Facts {
	out := facts.Facts{
		Evidence:    []facts.Evidence{},
		Touches:     []facts.Touch{},
		Actions:     []facts.Action{},
		Inputs:      []facts.Input{},
		Sections:    []facts.Section{},
		Ignores:     []facts.Ignore{},
		Diagnostics: []facts.Diagnostic{},
	}
	for _, f := range files {
		x := newExtractor(f, idx, &out)
		x.run()
	}
	return out
}

// frame is one open heading on the context stack.
type frame struct {
	level   int
	persons []string
	date    time.Time
	section int // index into out.Sections, or -1
}

type extractor struct {
	idx        *Index
	out        *facts.Facts
	file       string
	src        []byte // BOM and CRs stripped; line numbers unchanged
	masked     []byte // src with everything but scannable prose replaced by maskByte
	lineStarts []int
	fileDate   time.Time
	stack      []frame
	first      int // index of this file's first diagnostic
}

func newExtractor(f load.NoteFile, idx *Index, out *facts.Facts) *extractor {
	src := bytes.TrimPrefix(f.Source, []byte("\xef\xbb\xbf"))
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	src = bytes.ReplaceAll(src, []byte("\r"), []byte("\n"))
	src = blankFrontMatter(src)
	x := &extractor{idx: idx, out: out, file: f.Path, src: src, lineStarts: []int{0}, first: len(out.Diagnostics)}
	for i, c := range src {
		if c == '\n' {
			x.lineStarts = append(x.lineStarts, i+1)
		}
	}
	x.fileDate, _ = FileDate(f.Path)
	return x
}

// blankFrontMatter replaces a leading YAML front matter block (a first line
// of exactly "---", closed by "---" or "...") with empty lines, so nothing in
// it is extracted while later line numbers stay put. src is already BOM- and
// CR-free. Without a closing delimiter the file is returned unchanged.
func blankFrontMatter(src []byte) []byte {
	isDelim := func(l []byte, set ...string) bool {
		l = bytes.TrimRight(l, " \t")
		for _, d := range set {
			if string(l) == d {
				return true
			}
		}
		return false
	}
	lines := bytes.Split(src, []byte("\n"))
	if len(lines) == 0 || !isDelim(lines[0], "---") {
		return src
	}
	for i := 1; i < len(lines); i++ {
		if isDelim(lines[i], "---", "...") {
			for j := 0; j <= i; j++ {
				lines[j] = nil
			}
			return bytes.Join(lines, []byte("\n"))
		}
	}
	return src
}

// diag records an extraction error.
func (x *extractor) diag(code string, line int, msg string) {
	x.diagSuggest(code, line, msg, "")
}

func (x *extractor) diagSuggest(code string, line int, msg, suggest string) {
	x.out.Diagnostics = append(x.out.Diagnostics, facts.Diagnostic{
		Severity: "error", Code: code, File: x.file, Line: line, Message: msg, Suggest: suggest,
	})
}

func (x *extractor) lineOf(off int) int {
	return sort.Search(len(x.lineStarts), func(i int) bool { return x.lineStarts[i] > off })
}

func (x *extractor) run() {
	doc := markdown.Parser().Parse(text.NewReader(x.src))
	x.mask(doc)
	x.h1Date(doc)
	defer x.finish()
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Heading:
			x.heading(n)
			return ast.WalkSkipChildren, nil
		case *ast.Paragraph, *ast.TextBlock:
			x.block(n)
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock, *ast.CodeBlock, *ast.HTMLBlock:
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
}

// h1Date lets the first dated H1 date the whole file, including lines that
// come before it (Ruling R2).
func (x *extractor) h1Date(doc ast.Node) {
	ref := x.fileDate
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		h, ok := c.(*ast.Heading)
		if !ok || h.Level != 1 {
			continue
		}
		if d, ok := HeadingDate(plainText(h, x.src), ref); ok && (!ref.IsZero() || d.Year() >= 1000) {
			x.fileDate = d
			return
		}
	}
}

// finish reports a file with content but no resolvable date, then orders the
// file's diagnostics by line.
func (x *extractor) finish() {
	if x.fileDate.IsZero() && len(bytes.TrimSpace(x.src)) > 0 {
		x.diag("date-unresolved", 1, "no date in the file name or its first-level heading")
	}
	mine := x.out.Diagnostics[x.first:]
	sort.SliceStable(mine, func(i, j int) bool { return mine[i].Line < mine[j].Line })
}

// maskByte replaces masked source bytes. It is neither whitespace nor '(',
// so a sigil right after a masked region is not at a token boundary.
const maskByte = 0

// mask builds x.masked (Decision D): only text outside code spans and images
// survives, so code, autolinks, raw HTML, link destinations and markup
// delimiters are masked. It records every mgmt:ignore comment on the way.
func (x *extractor) mask(doc ast.Node) {
	x.masked = make([]byte, len(x.src))
	for i, c := range x.src {
		if c == '\n' {
			x.masked[i] = '\n'
		} else {
			x.masked[i] = maskByte
		}
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.CodeSpan, *ast.FencedCodeBlock, *ast.CodeBlock, *ast.Image:
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			copy(x.masked[n.Segment.Start:n.Segment.Stop], x.src[n.Segment.Start:n.Segment.Stop])
		case *ast.RawHTML:
			if n.Segments.Len() > 0 {
				x.ignores(n.Segments.At(0).Start, n.Segments.At(n.Segments.Len()-1).Stop)
			}
		case *ast.HTMLBlock:
			if l := n.Lines(); l.Len() > 0 {
				x.ignores(l.At(0).Start, l.At(l.Len()-1).Stop)
			}
		}
		return ast.WalkContinue, nil
	})
}

func (x *extractor) ignores(start, stop int) {
	for _, m := range ignoreRe.FindAllSubmatchIndex(x.src[start:stop], -1) {
		ids := []string{}
		for _, id := range strings.Split(string(x.src[start+m[2]:start+m[3]]), ",") {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
		x.out.Ignores = append(x.out.Ignores, facts.Ignore{File: x.file, Line: x.lineOf(start + m[0]), IDs: ids})
	}
}

// heading updates the context stack (Decision B) and the date.
func (x *extractor) heading(n *ast.Heading) {
	for len(x.stack) > 0 && x.stack[len(x.stack)-1].level >= n.Level {
		x.stack = x.stack[:len(x.stack)-1]
	}
	var parent *frame
	if len(x.stack) > 0 {
		parent = &x.stack[len(x.stack)-1]
	}
	title := plainText(n, x.src)
	f := frame{level: n.Level, persons: x.idx.HeadingPersons(title), section: -1}

	ref, _ := x.date()
	if d, ok := HeadingDate(title, ref); ok && (!ref.IsZero() || d.Year() >= 1000) {
		f.date = d
		if n.Level == 1 {
			x.fileDate = d // an H1 date dates the whole file
		}
	} else {
		f.date = ref
	}

	if len(f.persons) == 1 && n.Lines().Len() > 0 {
		f.section = len(x.out.Sections)
		x.out.Sections = append(x.out.Sections, facts.Section{
			Person: f.persons[0], Date: fmtDate(f.date), File: x.file, Line: x.lineOf(n.Lines().At(0).Start),
		})
	}
	if len(f.persons) == 0 && parent != nil {
		f.persons = parent.persons
	}
	x.stack = append(x.stack, f)
}

func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := c.(*ast.Text); ok && entering {
			b.Write(t.Segment.Value(src))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// date is the entry date of the current line: innermost heading, else file.
func (x *extractor) date() (time.Time, bool) {
	d := x.fileDate
	if len(x.stack) > 0 {
		d = x.stack[len(x.stack)-1].date
	}
	return d, !d.IsZero()
}

// block scans each line of a paragraph or text block. The first line of a
// list item whose first child holds a TaskCheckBox is an action.
func (x *extractor) block(n ast.Node) {
	action, done := false, false
	if _, ok := n.Parent().(*ast.ListItem); ok && n.Parent().FirstChild() == n {
		if cb, ok := n.FirstChild().(*extast.TaskCheckBox); ok {
			action, done = true, cb.IsChecked
		}
	}
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		start := lines.At(i).Start
		end := bytes.IndexByte(x.src[start:], '\n')
		if end < 0 {
			end = len(x.src)
		} else {
			end += start
		}
		isAction := action && i == 0
		if isAction {
			// The checkbox is markup too; scan from just after it.
			start += len(checkboxRe.Find(x.src[start:end]))
		}
		x.line(start, end, isAction, done)
	}
}

func (x *extractor) line(start, end int, isAction, done bool) {
	toks := ScanLine(string(x.masked[start:end]))
	if len(toks) == 0 && !isAction {
		return
	}
	entry, ok := x.date()
	if !ok {
		return // finish reports date-unresolved once per file (Ruling R2)
	}
	lineNo := x.lineOf(start)

	person := ""
	due := ""
	unknown := false
	for _, t := range toks {
		if t.Sigil != '@' {
			continue
		}
		if d, ok := DueDate(t.Body, entry); ok {
			if isAction && due == "" {
				due = fmtDate(d)
			}
			continue
		}
		if p, ok := x.idx.Person(t.Body); ok {
			if person != "" && person != p {
				x.diag("line-multi-person", lineNo, fmt.Sprintf("line names two people: %q and %q", person, p))
				return
			}
			person = p
			continue
		}
		if isAction && x.idx.IsMe(t.Body) {
			continue
		}
		x.diagSuggest("person-unknown", lineNo, fmt.Sprintf("unknown person %q", t.Body),
			Suggest(t.Body, x.idx.PersonNames()))
		unknown = true
	}
	if unknown {
		return
	}
	marks := []string{}
	for _, t := range toks {
		if t.Sigil != '@' {
			marks = append(marks, t.Mark())
		}
	}
	attributed := person != ""
	if person == "" && len(x.stack) > 0 {
		switch ps := x.stack[len(x.stack)-1].persons; len(ps) {
		case 1:
			person = ps[0]
		case 0:
		default:
			if len(marks) > 0 {
				x.diag("heading-multi-person", lineNo, fmt.Sprintf(
					"heading names several people (%s); put @person on the line to say whose %s is",
					strings.Join(ps, ", "), marks[0]))
			}
			return
		}
	}
	if person == "" {
		if len(marks) > 0 { // Decision H: a checkbox with no marks is ignored
			x.diag("person-missing", lineNo, fmt.Sprintf("%s has no person: no @person on the line and no person heading above it", marks[0]))
		}
		return
	}

	txt := recordText(string(x.src[start:end]), toks)
	date := fmtDate(entry)

	if isAction {
		owner := "me"
		if attributed {
			owner = person
		}
		addresses := []string{}
		for _, t := range toks {
			switch t.Sigil {
			case '~':
				if id, ok := x.competency(person, t.Body, lineNo); ok && !contains(addresses, id) {
					addresses = append(addresses, id)
				}
			case '+', '-':
				x.diag("evidence-in-action", lineNo, fmt.Sprintf(
					"evidence token %q on an action; evidence belongs on its own line (use ~ to say what the action addresses)",
					t.Mark()))
			}
		}
		x.out.Actions = append(x.out.Actions, facts.Action{
			ID: fmt.Sprintf("%x", sha1.Sum([]byte(x.file+txt))), Person: person, Owner: owner, Due: due,
			Done: done, Addresses: addresses, Date: date, File: x.file, Line: lineNo, Text: txt,
		})
		x.count(person)
	}

	for _, t := range toks {
		switch t.Sigil {
		case '+', '-':
			if isAction {
				continue
			}
			if id, ok := x.competency(person, t.Body, lineNo); ok {
				pol, strength := 1, 1
				if t.Sigil == '-' {
					pol = -1
				}
				if t.Strong {
					strength = 2
				}
				x.out.Evidence = append(x.out.Evidence, facts.Evidence{
					Person: person, Competency: id, Polarity: pol, Strength: strength, Date: date, File: x.file, Line: lineNo, Text: txt,
				})
				x.count(person)
			}
		case '~':
			if isAction {
				continue
			}
			if id, ok := x.competency(person, t.Body, lineNo); ok {
				x.out.Touches = append(x.out.Touches, facts.Touch{
					Person: person, Competency: id, Date: date, File: x.file, Line: lineNo, Text: txt,
				})
				x.count(person)
			}
		case '!':
			cycle, name, ok := x.idx.Cycle(person, t.Body, date)
			if !ok {
				sug := Suggest(t.Body, x.idx.InputNames(person))
				if strings.EqualFold(sug, t.Body) {
					sug = "" // right name, wrong date: nothing to rename
				}
				x.diagSuggest("input-unknown", lineNo, fmt.Sprintf(
					"unknown input %q for %s on %s: no review cycle selecting them declares it for that date", t.Body, person, date), sug)
				continue
			}
			x.out.Inputs = append(x.out.Inputs, facts.Input{
				Person: person, Input: name, Cycle: cycle, Date: date, File: x.file, Line: lineNo,
			})
			x.count(person)
		}
	}
}

// competency resolves a token (Decision E). Only Unknown is an extraction
// error; Duplicate and NoFramework are reported by Rego.
func (x *extractor) competency(person, token string, line int) (string, bool) {
	id, res := x.idx.Competency(person, token)
	switch res {
	case Resolved:
		return id, true
	case Unknown:
		x.diagSuggest("token-unknown", line, fmt.Sprintf("unknown competency token %q", token),
			Suggest(token, x.idx.TokenNames(person)))
	}
	return "", false
}

// count adds one record about person to the innermost person section, but
// only when that section is person's: an @-attributed line inside someone
// else's 1:1 does not mark that 1:1.
func (x *extractor) count(person string) {
	for i := len(x.stack) - 1; i >= 0; i-- {
		if s := x.stack[i].section; s >= 0 {
			if x.out.Sections[s].Person == person {
				x.out.Sections[s].TokenCount++
			}
			return
		}
	}
}

// recordText applies Decision A to a line from the original source; list,
// blockquote and checkbox markers are already cut.
func recordText(line string, toks []RawToken) string {
	b := []byte(line)
	for _, t := range toks {
		for i := t.Start; i < t.End; i++ {
			b[i] = ' '
		}
	}
	s := ignoreRe.ReplaceAllString(string(b), "")
	return strings.Join(strings.Fields(s), " ")
}

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func contains(list []string, s string) bool {
	for _, e := range list {
		if e == s {
			return true
		}
	}
	return false
}
