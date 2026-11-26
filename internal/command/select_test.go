package command

import (
	"reflect"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
)

func TestSelect(t *testing.T) {
	theme := func(v string) []cliargs.Setting { return []cliargs.Setting{{Key: "theme", Value: v}} }
	tests := []struct {
		name string
		args []string
		want Command
	}{
		// Ported from the old parseArgs tests.
		{"git mergetool order", []string{"B", "L", "R", "-o", "M"}, Merge{base: "B", local: "L", remote: "R", output: "M"}},
		{"output first", []string{"-o", "M", "B", "L", "R"}, Merge{base: "B", local: "L", remote: "R", output: "M"}},
		{"auto apply", []string{"B", "L", "R", "-o", "M", "--auto-apply"},
			Merge{base: "B", local: "L", remote: "R", output: "M", autoApply: true}},
		{"settings flags anywhere", []string{"--theme", "dark", "B", "L", "R", "-o", "M", "--window-state=maximized"},
			Merge{base: "B", local: "L", remote: "R", output: "M",
				settings: []cliargs.Setting{{Key: "theme", Value: "dark"}, {Key: "window-state", Value: "maximized"}}}},
		{"diff", []string{"a.txt", "b.txt"}, Diff{left: "a.txt", right: "b.txt"}},
		{"diff with a theme", []string{"a", "b", "--theme=system"}, Diff{left: "a", right: "b", settings: theme("system")}},
		{"double dash allows dash paths", []string{"--", "-a", "-b"}, Diff{left: "-a", right: "-b"}},
		{"single dash is a path", []string{"-", "b"}, Diff{left: "-", right: "b"}},
		{"help", []string{"--help"}, Help{}},
		{"short help wins over paths", []string{"a", "-h"}, Help{}},
		{"help wins over version", []string{"-v", "-h"}, Help{}},
		{"version", []string{"--version"}, Version{}},
		{"short version", []string{"-v", "a"}, Version{}},
		{"no args is scratch", nil, Scratch{}},
		{"scratch with a theme", []string{"--theme", "dark"}, Scratch{settings: theme("dark")}},
		{"settings subcommand", []string{"settings", "set", "theme", "dark"}, Settings{sub: "set", args: []string{"theme", "dark"}}},
		{"settings defaults to list", []string{"settings"}, Settings{sub: "list", args: []string{}}},
		{"settings with --help is the subcommand's", []string{"settings", "--help"}, Help{text: settingsUsage}},
		{"./settings is a path", []string{"./settings", "b"}, Diff{left: "./settings", right: "b"}},
		{"settings after -- is a path", []string{"--", "settings", "b"}, Diff{left: "settings", right: "b"}},
		{"resolve", []string{"resolve"}, Resolve{paths: []string{}}},
		{"resolve with paths and options", []string{"resolve", "src", "--auto-apply", "--theme", "dark", "--", "-x"},
			Resolve{autoApply: true, paths: []string{"src", "-x"}, settings: theme("dark")}},
		{"resolve with a path named settings", []string{"resolve", "settings"}, Resolve{paths: []string{"settings"}}},
		{"resolve --help", []string{"resolve", "--help"}, Help{}},
		{"./resolve is a path", []string{"./resolve", "b"}, Diff{left: "./resolve", right: "b"}},
		{"resolve after -- is a path", []string{"--", "resolve", "b"}, Diff{left: "resolve", right: "b"}},
		{"resolve not first is a path", []string{"a", "resolve"}, Diff{left: "a", right: "resolve"}},

		// New: help and version subcommands, reserved words after options.
		{"help subcommand", []string{"help"}, Help{}},
		{"help settings", []string{"help", "settings"}, Help{text: settingsUsage}},
		{"help resolve", []string{"help", "resolve"}, Help{text: Usage}},
		{"version subcommand", []string{"version"}, Version{}},
		{"version --help", []string{"version", "--help"}, Help{}},
		{"subcommand after an option", []string{"--auto-apply", "resolve"}, Resolve{autoApply: true, paths: []string{}}},
		{"reserved word before --", []string{"resolve", "--", "help"}, Resolve{paths: []string{"help"}}},
		{"-- help b", []string{"--", "help", "b"}, Diff{left: "help", right: "b"}},
		{"-- help version", []string{"--", "help", "version"}, Diff{left: "help", right: "version"}},
		{"options before --", []string{"-o", "M", "--", "help", "L", "R"}, Merge{base: "help", local: "L", remote: "R", output: "M"}},
		{"help not first is a path", []string{"a", "help"}, Diff{left: "a", right: "help"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Select(parse(t, tt.args...))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSelectErrors(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		msg      string // substring of the message
		settings bool   // the settings usage, not the general one
	}{
		// Ported from the old parseArgs tests.
		{"output without paths", []string{"-o", "m"}, "-o and --auto-apply need BASE LOCAL REMOTE", false},
		{"auto apply without paths", []string{"--auto-apply"}, "-o and --auto-apply need BASE LOCAL REMOTE", false},
		{"one path", []string{"a"}, "got 1", false},
		{"four paths", []string{"a", "b", "c", "d", "-o", "m"}, "got 4", false},
		{"merge without output", []string{"b", "l", "r"}, "needs -o", false},
		{"output in diff", []string{"a", "b", "-o", "m"}, "only valid for a 3-way merge", false},
		{"auto apply in diff", []string{"a", "b", "--auto-apply"}, "only valid for a 3-way merge", false},
		{"resolve with output", []string{"resolve", "-o", "m"}, "-o is not valid with resolve", false},

		// New.
		{"help with an unknown topic", []string{"help", "nope"}, `unknown help topic "nope"`, false},
		{"version with an argument", []string{"version", "x"}, "version takes no arguments", false},
		{"settings list --theme dark", []string{"settings", "list", "--theme", "dark"}, "--theme is not valid with settings", true},
		{"--theme dark settings list", []string{"--theme", "dark", "settings", "list"}, "--theme is not valid with settings", true},
		{"settings -o", []string{"settings", "-o", "m"}, "-o is not valid with settings", true},
		{"one path after --", []string{"--", "help"}, "got 1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Select(parse(t, tt.args...))
			ue := wantUsageError(t, err, tt.msg)
			if got := ue.Usage == settingsUsage; got != tt.settings {
				t.Errorf("settings usage = %v, want %v", got, tt.settings)
			}
		})
	}
}

// TestSelectReservedWords checks every reserved word is a subcommand only
// as the first positional before "--".
func TestSelectReservedWords(t *testing.T) {
	for word := range subcommands {
		if cmd, err := Select(parse(t, "--", word, "b")); err != nil || !reflect.DeepEqual(cmd, Diff{left: word, right: "b"}) {
			t.Errorf("-- %s b = %#v, %v; want a diff", word, cmd, err)
		}
		if cmd, err := Select(parse(t, "./"+word, "b")); err != nil || !reflect.DeepEqual(cmd, Diff{left: "./" + word, right: "b"}) {
			t.Errorf("./%s b = %#v, %v; want a diff", word, cmd, err)
		}
		if cmd, err := Select(parse(t, "a", word)); err != nil || !reflect.DeepEqual(cmd, Diff{left: "a", right: word}) {
			t.Errorf("a %s = %#v, %v; want a diff", word, cmd, err)
		}
		if cmd, err := Select(parse(t, word)); err != nil {
			t.Errorf("%s alone: %v", word, err)
		} else if _, isDiff := cmd.(Diff); isDiff {
			t.Errorf("%s alone = %#v; want the subcommand", word, cmd)
		}
	}
}
