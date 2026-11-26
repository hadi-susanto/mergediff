package command

import (
	"fmt"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
)

// Help prints the general usage, or a topic's (only "settings" has its own).
type Help struct{ text string }

// Run prints the help text.
func (h Help) Run(env Env) (int, error) {
	text := h.text
	if text == "" {
		text = Usage
	}
	fmt.Fprint(env.Stdout, text)
	return ExitOK, nil
}

// helpTopics are the topics "mergediff help TOPIC" knows.
var helpTopics = map[string]string{
	"settings": settingsUsage,
	"resolve":  Usage, // resolve is described in the general usage
}

// newHelp checks "mergediff help [TOPIC]"; args are the positionals after
// "help".
func newHelp(opt cliargs.Option, args []string) (Command, error) {
	if err := rejectFlags(opt, "help"); err != nil {
		return nil, err
	}
	switch len(args) {
	case 0:
		return Help{}, nil
	case 1:
		if text, ok := helpTopics[args[0]]; ok {
			return Help{text: text}, nil
		}
		return nil, usagef("unknown help topic %q (known: resolve, settings)", args[0])
	}
	return nil, usagef("help takes at most one topic, got %d", len(args))
}
