package cli

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/uatec/mgmt/internal/extract"
	"github.com/uatec/mgmt/internal/load"
)

//go:embed completion
var completionScripts embed.FS

// completionShells are the shells `mgmt completion` can print a script for.
var completionShells = []string{"zsh", "bash", "fish"}

// filesDirective is the last line of __complete output when the shell should
// fall back to completing file names.
const filesDirective = ":files"

type candidate struct{ value, desc string }

// commandDescs lists the commands in usage order.
var commandDescs = []candidate{
	{"init", "scaffold a new workspace here"},
	{"facts", "print the facts document as JSON"},
	{"lint", "print errors and warnings"},
	{"prep", "print the 1:1 prep sheet"},
	{"review", "print the review evidence pack"},
	{"explain", "explain why a competency has its status"},
	{"status", "print the team matrix and cycle checks"},
	{"report", "run a workspace report"},
	{"completion", "print a shell completion script"},
}

var (
	globalFlags = []candidate{{"--now", "fix today's date (YYYY-MM-DD)"}, {"--out", "write the output to a file"}}
	cycleFlag   = candidate{"--cycle", "review cycle to use"}
	formatFlag  = candidate{"--format", "output format"}
	// commandFlags are each command's own flags.
	commandFlags = map[string][]candidate{
		"lint":    {{"--strict", "exit 1 on warnings"}, formatFlag},
		"prep":    {cycleFlag},
		"review":  {cycleFlag},
		"explain": {cycleFlag},
		"status":  {cycleFlag, formatFlag},
		"report":  {cycleFlag},
	}
	// valueFlags take a value, as the next word or after "=".
	valueFlags = map[string]bool{"now": true, "out": true, "cycle": true, "format": true}
)

// runComplete implements the hidden "mgmt __complete <words...>": words are
// the command line after "mgmt", the last being the (possibly empty) word
// under the cursor. It prints one "value[<TAB>description]" line per
// candidate, then ":files" if the shell should complete file names. It reads
// only resources/ and reports/, and never fails: completion is best effort.
func runComplete(words []string, root string, stdout io.Writer) int {
	cands, files := complete(words, root)
	for _, c := range cands {
		if c.desc != "" {
			fmt.Fprintf(stdout, "%s\t%s\n", c.value, c.desc)
		} else {
			fmt.Fprintln(stdout, c.value)
		}
	}
	if files {
		fmt.Fprintln(stdout, filesDirective)
	}
	return 0
}

// complete returns the candidates for the last word, filtered by it
// case-insensitively. The values are whole words, including any "--flag="
// prefix the user has already typed.
func complete(words []string, root string) (cands []candidate, files bool) {
	if len(words) == 0 {
		words = []string{""}
	}
	cur, done := words[len(words)-1], words[:len(words)-1]

	var cmd, pending string
	var pos []string
	for _, w := range done {
		switch {
		case pending != "":
			pending = ""
		case strings.HasPrefix(w, "-") && w != "-":
			name, _, hasValue := strings.Cut(strings.TrimLeft(w, "-"), "=")
			if !hasValue && valueFlags[name] {
				pending = name
			}
		case cmd == "":
			cmd = w
		default:
			pos = append(pos, w)
		}
	}

	switch {
	case pending != "":
		cands, files = flagValues(pending, cmd, root)
	case strings.HasPrefix(cur, "-") && strings.Contains(cur, "="):
		flag, _, _ := strings.Cut(cur, "=")
		// file names after "--out=" aren't offered: shells disagree about
		// completing the part after "="
		vals, _ := flagValues(strings.TrimLeft(flag, "-"), cmd, root)
		for _, v := range vals {
			cands = append(cands, candidate{flag + "=" + v.value, v.desc})
		}
	case strings.HasPrefix(cur, "-"):
		cands = append(cands, commandFlags[cmd]...)
		cands = append(cands, globalFlags...)
	case cmd == "":
		cands = commandDescs
	default:
		cands = positional(cmd, pos, root)
	}
	return filterPrefix(cands, cur), files
}

func filterPrefix(cands []candidate, cur string) []candidate {
	var out []candidate
	lower := strings.ToLower(cur)
	for _, c := range cands {
		if strings.HasPrefix(strings.ToLower(c.value), lower) {
			out = append(out, c)
		}
	}
	return out
}

// flagValues returns the values of a value-taking flag; files is true for
// --out, whose value is a file name.
func flagValues(flag, cmd, root string) (cands []candidate, files bool) {
	switch flag {
	case "out":
		return nil, true
	case "format":
		switch cmd {
		case "lint":
			return plain("text", "flat", "json"), false
		case "status":
			return plain("markdown", "yaml"), false
		}
	case "cycle":
		return cycles(root), false
	}
	return nil, false
}

// positional returns the candidates for the next positional argument of cmd,
// given the positional arguments already typed.
func positional(cmd string, pos []string, root string) []candidate {
	n := len(pos)
	switch cmd {
	case "prep", "review":
		if n == 0 {
			return people(root)
		}
	case "explain":
		switch n {
		case 0:
			return people(root)
		case 1:
			return competencies(root, pos[0])
		}
	case "report":
		switch n {
		case 0:
			return reportNames(root)
		case 1:
			return people(root)
		}
	case "completion":
		if n == 0 {
			return plain(completionShells...)
		}
	}
	return nil
}

func plain(values ...string) []candidate {
	out := make([]candidate, len(values))
	for i, v := range values {
		out[i] = candidate{value: v}
	}
	return out
}

// resources loads the workspace's resources, ignoring any that don't parse.
func resources(root string) []load.Resource {
	res, _ := load.LoadResources(root)
	return res
}

func specOf(r load.Resource) map[string]any {
	spec, _ := r.Object["spec"].(map[string]any)
	return spec
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// people lists Person names, then aliases that aren't just another casing of
// a name already listed (every lookup is case-insensitive).
func people(root string) []candidate {
	var out []candidate
	seen := map[string]bool{}
	var aliases []candidate
	for _, r := range resources(root) {
		if r.Kind != "Person" {
			continue
		}
		spec := specOf(r)
		name := str(spec, "displayName")
		out = append(out, candidate{r.Name, name})
		seen[strings.ToLower(r.Name)] = true
		list, _ := spec["aliases"].([]any)
		for _, a := range list {
			if s, ok := a.(string); ok {
				aliases = append(aliases, candidate{s, "alias of " + r.Name})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].value < out[j].value })
	sort.Slice(aliases, func(i, j int) bool { return aliases[i].value < aliases[j].value })
	for _, a := range aliases {
		if k := strings.ToLower(a.value); !seen[k] {
			seen[k] = true
			out = append(out, a)
		}
	}
	return out
}

// competencies lists everything that resolves as a competency for person (a
// name or alias): tokens, ids and unambiguous id suffixes of their framework,
// each described by the competency's title.
func competencies(root, person string) []candidate {
	res := resources(root)
	idx := extract.NewIndex(res)
	name, ok := idx.Person(person)
	if !ok {
		return nil
	}
	var framework string
	for _, r := range res {
		if r.Kind == "Person" && r.Name == name {
			framework = str(specOf(r), "framework")
		}
	}
	titles := map[string]string{} // lower-case token, id or suffix -> title
	for _, r := range res {
		if r.Kind != "CompetencyFramework" || r.Name != framework {
			continue
		}
		comps, _ := specOf(r)["competencies"].([]any)
		for _, c := range comps {
			cm, _ := c.(map[string]any)
			id, title := str(cm, "id"), str(cm, "title")
			keys := []string{id}
			if i := strings.LastIndexByte(id, '.'); i >= 0 {
				keys = append(keys, id[i+1:])
			}
			toks, _ := cm["tokens"].([]any)
			for _, t := range toks {
				if s, ok := t.(string); ok {
					keys = append(keys, s)
				}
			}
			for _, k := range keys {
				titles[strings.ToLower(k)] = title
			}
		}
	}
	var out []candidate
	for i, tok := range idx.TokenNames(name) {
		if i == 0 || tok != out[len(out)-1].value { // sorted, so repeats are adjacent
			out = append(out, candidate{tok, titles[tok]})
		}
	}
	return out
}

// cycles lists ReviewCycle names with their periods.
func cycles(root string) []candidate {
	var out []candidate
	for _, r := range resources(root) {
		if r.Kind != "ReviewCycle" {
			continue
		}
		period, _ := specOf(r)["period"].(map[string]any)
		desc := ""
		if from, to := str(period, "from"), str(period, "to"); from != "" && to != "" {
			desc = from + " to " + to
		}
		out = append(out, candidate{r.Name, desc})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].value < out[j].value })
	return out
}

// reportNames lists the workspace's custom report templates (reports/*.tmpl),
// leaving out the built-in reports, which have their own commands.
func reportNames(root string) []candidate {
	entries, _ := os.ReadDir(filepath.Join(root, "reports"))
	var out []candidate
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".tmpl")
		if _, builtin := builtinReports[name]; ok && !e.IsDir() && !builtin {
			out = append(out, candidate{value: name})
		}
	}
	return out
}

// runCompletion implements "mgmt completion <shell>": it prints the script.
func runCompletion(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		return usageError(stderr, "completion takes <"+strings.Join(completionShells, "|")+">")
	}
	for _, shell := range completionShells {
		if args[0] == shell {
			script, err := completionScripts.ReadFile("completion/mgmt." + shell)
			if err != nil {
				fmt.Fprintf(stderr, "mgmt: %v\n", err)
				return 2
			}
			_, _ = stdout.Write(script)
			return 0
		}
	}
	return usageError(stderr, fmt.Sprintf("completion: unknown shell %q", args[0]))
}
