package cli

import (
	"fmt"
	"strings"

	"github.com/uatec/mgmt/internal/engine"
	"github.com/uatec/mgmt/internal/facts"
)

// lintGrouped renders findings as markdown for people to read (the default
// lint text): a header and summary, then the errors grouped by id (errors
// block reports, so they come first), then one section per person with
// warnings (by name; ids sorted, findings by file and line under each id),
// then warnings with no person under "Workspace". names maps a Person's
// name to their displayName. A message's leading "<person>: " is dropped
// under that person's heading. Findings with no file print no location
// (Ruling R17).
func lintGrouped(now string, names map[string]string, errs, warns []facts.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Lint — %s\n\n", now)
	if len(errs)+len(warns) == 0 {
		b.WriteString("no problems\n")
		return b.String()
	}
	b.WriteString(summary(len(errs), len(warns)) + "\n")
	if len(errs) > 0 {
		b.WriteString("\n## Errors\n\n")
		writeByID(&b, errs, func(f facts.Finding) string {
			if f.Person == "" {
				return f.Message
			}
			return f.Person + ": " + stripPerson(f)
		})
	}
	byPerson := map[string][]facts.Finding{}
	for _, f := range warns {
		byPerson[f.Person] = append(byPerson[f.Person], f)
	}
	for _, p := range sortedKeys(byPerson) {
		if p == "" {
			continue
		}
		heading := p
		if d := names[p]; d != "" && d != p {
			heading = fmt.Sprintf("%s (%s)", d, p)
		}
		fmt.Fprintf(&b, "\n## %s\n\n", heading)
		writeByID(&b, byPerson[p], stripPerson)
	}
	if ws := byPerson[""]; len(ws) > 0 {
		b.WriteString("\n## Workspace\n\n")
		writeByID(&b, ws, stripPerson)
	}
	return b.String()
}

// writeByID writes one "- **id**" bullet per id (sorted) with its findings,
// in file, line order, as sub-bullets; text gives a finding's message.
func writeByID(b *strings.Builder, fs []facts.Finding, text func(facts.Finding) string) {
	byID := map[string][]facts.Finding{}
	for _, f := range fs {
		byID[f.ID] = append(byID[f.ID], f)
	}
	for _, id := range sortedKeys(byID) {
		group := byID[id]
		engine.SortFindings(group)
		fmt.Fprintf(b, "- **%s**\n", id)
		for _, f := range group {
			fmt.Fprintf(b, "  - %s", text(f))
			if f.Suggest != "" {
				fmt.Fprintf(b, " (did you mean %q?)", f.Suggest)
			}
			switch {
			case f.File == "":
			case f.Line == 0:
				fmt.Fprintf(b, " (%s)", f.File)
			default:
				fmt.Fprintf(b, " (%s:%d)", f.File, f.Line)
			}
			b.WriteByte('\n')
		}
	}
}

// stripPerson returns the message without a redundant leading
// "<person>: ", which several core messages start with.
func stripPerson(f facts.Finding) string {
	if f.Person == "" {
		return f.Message
	}
	return strings.TrimPrefix(f.Message, f.Person+": ")
}

// personNames maps each Person's name to their displayName (or "").
func personNames(f facts.Facts) map[string]string {
	names := map[string]string{}
	for name, v := range f.Resources["Person"] {
		obj, _ := v.(map[string]any)
		spec, _ := obj["spec"].(map[string]any)
		names[name], _ = spec["displayName"].(string)
	}
	return names
}

// lintFlat renders findings one per line (lint --format flat), errors and warnings interleaved in
// file, line, id, person order, followed by a summary line. A finding with no
// file (a workspace policy emitting a plain string) prints without a
// location, as "warning <pkg>: <message>" (Ruling R17: the one accepted
// exception to every finding carrying file:line); one with a file but no line
// prints "<file>: ...".
func lintFlat(errs, warns []facts.Finding) string {
	if len(errs)+len(warns) == 0 {
		return "no problems\n"
	}
	all := append(append([]facts.Finding{}, errs...), warns...)
	engine.SortFindings(all)
	var b strings.Builder
	for _, f := range all {
		switch {
		case f.File == "":
		case f.Line == 0:
			fmt.Fprintf(&b, "%s: ", f.File)
		default:
			fmt.Fprintf(&b, "%s:%d: ", f.File, f.Line)
		}
		fmt.Fprintf(&b, "%s %s", f.Severity, f.ID)
		if f.Person != "" {
			fmt.Fprintf(&b, " [%s]", f.Person)
		}
		fmt.Fprintf(&b, ": %s", f.Message)
		if f.Suggest != "" {
			fmt.Fprintf(&b, " (did you mean %q?)", f.Suggest)
		}
		b.WriteByte('\n')
	}
	b.WriteString(summary(len(errs), len(warns)) + "\n")
	return b.String()
}

func summary(errs, warns int) string {
	return plural(errs, "error") + ", " + plural(warns, "warning")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
