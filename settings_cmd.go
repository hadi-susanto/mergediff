package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/hadi-susanto/mergediff/internal/config"
)

const settingsUsage = `Usage:
  mergediff settings [list]           show every setting, its value and where it comes from
  mergediff settings get KEY          print one setting's value
  mergediff settings set KEY VALUE    save a setting in the config file
  mergediff settings unset KEY        remove a setting from the config file (back to its default)
  mergediff settings path             print the config file location

Comments and other lines in the config file are kept when it is changed.
`

// runSettings implements "mergediff settings ..." and returns the exit code.
func runSettings(args []string, stdout, stderr io.Writer) int {
	sub := "list"
	if len(args) > 0 {
		sub, args = args[0], args[1:]
	}
	want := map[string]int{"list": 0, "path": 0, "get": 1, "unset": 1, "set": 2, "help": 0, "-h": 0, "--help": 0}
	n, known := want[sub]
	switch {
	case !known:
		return settingsUsageError(stderr, "unknown settings command %q", sub)
	case len(args) != n:
		return settingsUsageError(stderr, "settings %s takes %d argument(s), got %d", sub, n, len(args))
	}
	if sub == "help" || sub == "-h" || sub == "--help" {
		fmt.Fprint(stdout, settingsUsage)
		return exitOK
	}

	path, err := configPath()
	if err != nil {
		fmt.Fprintf(stderr, "mergediff: %v\n", err)
		return exitError
	}
	if sub == "path" {
		fmt.Fprintln(stdout, path)
		return exitOK
	}
	if sub == "set" || sub == "unset" {
		if _, err := config.Lookup(args[0]); err != nil {
			return settingsUsageError(stderr, "%v", err)
		}
		if sub == "set" {
			err = config.Set(path, args[0], args[1])
		} else {
			err = config.Unset(path, args[0])
		}
		if err != nil {
			fmt.Fprintf(stderr, "mergediff: %v\n", err)
			return exitError
		}
	}

	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "mergediff: %v\n", err)
		return exitError
	}
	switch sub {
	case "get":
		if _, err := config.Lookup(args[0]); err != nil {
			return settingsUsageError(stderr, "%v", err)
		}
		fmt.Fprintln(stdout, cfg.Get(args[0]))
	case "set", "unset":
		fmt.Fprintf(stdout, "%s = %s (%s)\n", args[0], cfg.Get(args[0]), cfg.SourceOf(args[0]))
	case "list":
		printSettings(stdout, path, cfg)
	}
	return exitOK
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

func settingsUsageError(w io.Writer, format string, args ...any) int {
	fmt.Fprintf(w, "mergediff: %s\n\n%s", fmt.Sprintf(format, args...), settingsUsage)
	return exitError
}
