package command

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/config"
)

// Settings is "mergediff settings [list | get KEY | set KEY VALUE |
// unset KEY | path | help]": it shows or changes the config file and opens
// no window.
type Settings struct {
	sub  string
	args []string
}

// settingsArgs is the number of arguments each settings command takes.
var settingsArgs = map[string]int{"list": 0, "path": 0, "get": 1, "unset": 1, "set": 2, "help": 0}

// newSettings checks "mergediff settings ..."; args are the positionals
// after "settings". --help shows the settings usage whatever the args.
func newSettings(opt cliargs.Option, args []string) (Command, error) {
	if opt.Help() {
		return Help{text: settingsUsage}, nil
	}
	switch s := opt.Settings(); {
	case opt.HasOutput():
		return nil, settingsUsagef("-o is not valid with settings")
	case opt.AutoApply():
		return nil, settingsUsagef("--auto-apply is not valid with settings")
	case len(s) > 0:
		// Point at the command that does what the flag would.
		return nil, settingsUsagef("--%s is not valid with settings; to save it, use: mergediff settings set %s %s",
			s[0].Key, s[0].Key, s[0].Value)
	case opt.Version():
		return nil, settingsUsagef("--version is not valid with settings")
	}
	sub := "list"
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	n, known := settingsArgs[sub]
	switch {
	case !known:
		return nil, settingsUsagef("unknown settings command %q", sub)
	case len(args) != n:
		return nil, settingsUsagef("settings %s takes %d argument(s), got %d", sub, n, len(args))
	case sub == "help":
		return Help{text: settingsUsage}, nil
	case sub == "get" || sub == "set" || sub == "unset":
		if _, err := config.Lookup(args[0]); err != nil {
			return nil, settingsUsagef("%v", err)
		}
	}
	return Settings{sub: sub, args: args}, nil
}

func settingsUsagef(format string, args ...any) *UsageError {
	return &UsageError{Msg: fmt.Sprintf(format, args...), Usage: settingsUsage}
}

// Run shows or changes the settings.
func (s Settings) Run(env Env) (int, error) {
	path, err := env.ConfigPath()
	if err != nil {
		return ExitError, err
	}
	switch s.sub {
	case "path":
		fmt.Fprintln(env.Stdout, path)
		return ExitOK, nil
	case "set":
		err = config.Set(path, s.args[0], s.args[1])
	case "unset":
		err = config.Unset(path, s.args[0])
	}
	if err != nil {
		return ExitError, err
	}

	cfg, err := config.Load(path)
	if err != nil {
		return ExitError, err
	}
	switch s.sub {
	case "get":
		fmt.Fprintln(env.Stdout, cfg.Get(s.args[0]))
	case "set", "unset":
		fmt.Fprintf(env.Stdout, "%s = %s (%s)\n", s.args[0], cfg.Get(s.args[0]), cfg.SourceOf(s.args[0]))
	case "list":
		printSettings(env.Stdout, path, cfg)
	}
	return ExitOK, nil
}

func printSettings(w io.Writer, path string, cfg config.Config) {
	fmt.Fprintf(w, "Config file: %s\n\n", path)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tVALUE\tFROM\tALLOWED\tDESCRIPTION")
	for _, s := range config.Registry {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.Key, cfg.Get(s.Key), cfg.SourceOf(s.Key), strings.Join(s.Values, "|"), s.Doc)
	}
	tw.Flush()
}
