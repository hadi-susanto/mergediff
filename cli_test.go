package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want options
	}{
		{"git mergetool order", []string{"B", "L", "R", "-o", "M"},
			options{cmd: cmdMerge, base: "B", local: "L", remote: "R", output: "M"}},
		{"output first", []string{"-o", "M", "B", "L", "R"},
			options{cmd: cmdMerge, base: "B", local: "L", remote: "R", output: "M"}},
		{"long output with =", []string{"B", "L", "R", "--output=M"},
			options{cmd: cmdMerge, base: "B", local: "L", remote: "R", output: "M"}},
		{"auto apply", []string{"B", "L", "R", "-o", "M", "--auto-apply"},
			options{cmd: cmdMerge, base: "B", local: "L", remote: "R", output: "M", autoApply: true}},
		{"diff", []string{"a.txt", "b.txt"}, options{cmd: cmdDiff, left: "a.txt", right: "b.txt"}},
		{"double dash allows dash paths", []string{"--", "-a", "-b"}, options{cmd: cmdDiff, left: "-a", right: "-b"}},
		{"single dash is a path", []string{"-", "b"}, options{cmd: cmdDiff, left: "-", right: "b"}},
		{"help", []string{"--help"}, options{cmd: cmdHelp}},
		{"short help wins over paths", []string{"a", "-h"}, options{cmd: cmdHelp}},
		{"version", []string{"--version"}, options{cmd: cmdVersion}},
		{"short version", []string{"-v"}, options{cmd: cmdVersion}},
		{"no args is scratch", nil, options{cmd: cmdScratch}},
		{"scratch with a theme", []string{"--theme", "dark"}, options{cmd: cmdScratch, settings: []setting{{"theme", "dark"}}}},
		{"settings subcommand", []string{"settings", "set", "theme", "dark"},
			options{cmd: cmdSettings, settingsArgs: []string{"set", "theme", "dark"}}},
		{"settings with --help is the subcommand's", []string{"settings", "--help"},
			options{cmd: cmdSettings, settingsArgs: []string{"--help"}}},
		{"./settings is a path", []string{"./settings", "b"}, options{cmd: cmdDiff, left: "./settings", right: "b"}},
		{"settings after -- is a path", []string{"--", "settings", "b"}, options{cmd: cmdDiff, left: "settings", right: "b"}},
		{"settings flags anywhere", []string{"--theme", "dark", "B", "L", "R", "-o", "M", "--window-state=maximized"},
			options{cmd: cmdMerge, base: "B", local: "L", remote: "R", output: "M",
				settings: []setting{{"theme", "dark"}, {"window-state", "maximized"}}}},
		{"resolve", []string{"resolve"}, options{cmd: cmdResolve}},
		{"resolve with paths and options", []string{"resolve", "src", "--auto-apply", "--theme", "dark", "--", "-x"},
			options{cmd: cmdResolve, autoApply: true, paths: []string{"src", "-x"}, settings: []setting{{"theme", "dark"}}}},
		{"resolve --help", []string{"resolve", "--help"}, options{cmd: cmdHelp}},
		{"./resolve is a path", []string{"./resolve", "b"}, options{cmd: cmdDiff, left: "./resolve", right: "b"}},
		{"resolve after -- is a path", []string{"--", "resolve", "b"}, options{cmd: cmdDiff, left: "resolve", right: "b"}},
		{"resolve not first is a path", []string{"a", "resolve"}, options{cmd: cmdDiff, left: "a", right: "resolve"}},
		{"theme with =", []string{"a", "b", "--theme=system"},
			options{cmd: cmdDiff, left: "a", right: "b", settings: []setting{{"theme", "system"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseArgsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		msg  string // substring of the message
	}{
		{"output without paths", []string{"-o", "m"}, "-o and --auto-apply need BASE LOCAL REMOTE"},
		{"auto apply without paths", []string{"--auto-apply"}, "-o and --auto-apply need BASE LOCAL REMOTE"},
		{"one path", []string{"a"}, "got 1"},
		{"four paths", []string{"a", "b", "c", "d", "-o", "m"}, "got 4"},
		{"merge without output", []string{"b", "l", "r"}, "needs -o"},
		{"output in diff", []string{"a", "b", "-o", "m"}, "only valid for a 3-way merge"},
		{"auto apply in diff", []string{"a", "b", "--auto-apply"}, "only valid for a 3-way merge"},
		{"output missing value", []string{"b", "l", "r", "-o"}, "needs a file name"},
		{"output empty value", []string{"b", "l", "r", "--output="}, "needs a file name"},
		{"output twice", []string{"b", "l", "r", "-o", "m", "-o", "n"}, "more than once"},
		{"unknown option", []string{"a", "b", "--frobnicate"}, "unknown option --frobnicate"},
		{"value on a switch", []string{"b", "l", "r", "-o", "m", "--auto-apply=yes"}, "does not take a value"},
		{"bad theme", []string{"a", "b", "--theme", "blue"}, "--theme: invalid value \"blue\" for theme (allowed: system, light, dark)"},
		{"theme without value", []string{"a", "b", "--theme"}, "--theme needs a value (system, light, dark)"},
		{"theme twice", []string{"a", "b", "--theme", "dark", "--theme=light"}, "--theme given more than once"},
		{"bad window state", []string{"a", "b", "--window-state=full"}, "allowed: normal, maximized"},
		{"resolve with output", []string{"resolve", "-o", "m"}, "-o is not valid with resolve"},
		{"resolve unknown option", []string{"resolve", "--ours"}, "unknown option --ours"},
		{"settings key needs two dashes", []string{"a", "b", "-theme", "dark"}, "unknown option -theme"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseArgs(tt.args)
			var ue *usageError
			if !errors.As(err, &ue) {
				t.Fatalf("err = %v, want a usage error", err)
			}
			if !strings.Contains(err.Error(), tt.msg) {
				t.Fatalf("err = %q, want it to mention %q", err, tt.msg)
			}
		})
	}
}

func TestUsageListsSettingsFlags(t *testing.T) {
	for _, want := range []string{"--theme system|light|dark\n", "light/dark setting (default system)", "--window-state normal|maximized\n", "(default normal)"} {
		if !strings.Contains(usage, want) {
			t.Errorf("usage is missing %q:\n%s", want, usage)
		}
	}
}

func TestVersion(t *testing.T) {
	if got := Version(); got != "dev (unknown)" {
		t.Fatalf("Version() = %q without ldflags, want %q", got, "dev (unknown)")
	}
}
