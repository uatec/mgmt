package cli

import (
	"fmt"
	"io"

	mgmt "github.com/uatec/mgmt"
)

// helpTopics maps each `mgmt help <topic>` to the text it prints.
var helpTopics = map[string]string{
	"notes": mgmt.NotesCheatSheet,
}

// runHelp prints the usage, or the named topic's text.
func runHelp(args []string, stdout, stderr io.Writer) int {
	switch len(args) {
	case 0:
		_, _ = io.WriteString(stdout, usage)
		return 0
	case 1:
		text, ok := helpTopics[args[0]]
		if !ok {
			return usageError(stderr, fmt.Sprintf("unknown help topic %q", args[0]))
		}
		_, _ = io.WriteString(stdout, text)
		return 0
	default:
		return usageError(stderr, "help takes [topic]")
	}
}
