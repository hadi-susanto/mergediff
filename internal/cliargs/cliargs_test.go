package cliargs

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// opt builds an expected Option; dashDash defaults to -1.
func opt(pos ...string) Option { return Option{positionals: pos, dashDash: -1} }

func TestParse(t *testing.T) {
	with := func(o Option, f func(*Option)) Option { f(&o); return o }
	out := func(m string) func(*Option) { return func(o *Option) { o.output = m } }
	tests := []struct {
		name string
		args []string
		want Option
	}{
		{"no args", nil, opt()},
		{"git mergetool order", []string{"B", "L", "R", "-o", "M"}, with(opt("B", "L", "R"), out("M"))},
		{"output first", []string{"-o", "M", "B", "L", "R"}, with(opt("B", "L", "R"), out("M"))},
		{"long output with =", []string{"B", "L", "R", "--output=M"}, with(opt("B", "L", "R"), out("M"))},
		{"long output with a space", []string{"--output", "M"}, with(opt(), out("M"))},
		{"auto apply", []string{"B", "--auto-apply"}, with(opt("B"), func(o *Option) { o.autoApply = true })},
		{"auto apply twice", []string{"--auto-apply", "--auto-apply"}, with(opt(), func(o *Option) { o.autoApply = true })},
		{"help", []string{"--help"}, with(opt(), func(o *Option) { o.help = true })},
		{"short help after paths", []string{"a", "-h"}, with(opt("a"), func(o *Option) { o.help = true })},
		{"help twice", []string{"-h", "--help"}, with(opt(), func(o *Option) { o.help = true })},
		{"version", []string{"--version"}, with(opt(), func(o *Option) { o.version = true })},
		{"short version", []string{"-v", "-v"}, with(opt(), func(o *Option) { o.version = true })},
		{"single dash is positional", []string{"-", "b"}, opt("-", "b")},
		{"order is kept", []string{"settings", "set", "theme", "dark"}, opt("settings", "set", "theme", "dark")},
		{"double dash allows dash paths", []string{"--", "-a", "-b"},
			with(opt("-a", "-b"), func(o *Option) { o.dashDash = 0 })},
		{"double dash after a positional", []string{"a", "--", "--help"},
			with(opt("a", "--help"), func(o *Option) { o.dashDash = 1 })},
		{"double dash at the end", []string{"a", "--"}, with(opt("a"), func(o *Option) { o.dashDash = 1 })},
		{"second double dash is positional", []string{"--", "--"}, with(opt("--"), func(o *Option) { o.dashDash = 0 })},
		{"options before double dash", []string{"-o", "M", "--", "help", "L", "R"},
			with(opt("help", "L", "R"), func(o *Option) { out("M")(o); o.dashDash = 0 })},
		{"settings flags anywhere", []string{"--theme", "dark", "B", "--window-state=maximized"},
			with(opt("B"), func(o *Option) {
				o.settings = []Setting{{"theme", "dark"}, {"window-state", "maximized"}}
			})},
		{"settings flags around a subcommand", []string{"settings", "list", "--theme", "dark"},
			with(opt("settings", "list"), func(o *Option) { o.settings = []Setting{{"theme", "dark"}} })},
		{"settings with --help", []string{"settings", "--help"},
			with(opt("settings"), func(o *Option) { o.help = true })},
		{"output value may start with a dash", []string{"-o", "-m"}, with(opt(), out("-m"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		msg  string // the whole message
	}{
		{"output missing value", []string{"b", "l", "r", "-o"}, "-o needs a file name"},
		{"long output missing value", []string{"--output"}, "--output needs a file name"},
		{"output empty value", []string{"b", "l", "r", "--output="}, "--output needs a file name"},
		{"output twice", []string{"-o", "m", "--output", "n"}, "--output given more than once"},
		{"unknown option", []string{"a", "b", "--frobnicate"}, "unknown option --frobnicate"},
		{"unknown option with a value", []string{"--frobnicate=1"}, "unknown option --frobnicate=1"},
		{"value on a switch", []string{"--auto-apply=yes"}, "--auto-apply does not take a value"},
		{"value on help", []string{"--help=x"}, "--help does not take a value"},
		{"bad theme", []string{"--theme", "blue"}, `--theme: invalid value "blue" for theme (allowed: system, light, dark)`},
		{"theme without value", []string{"a", "b", "--theme"}, "--theme needs a value (system, light, dark)"},
		{"theme twice", []string{"--theme", "dark", "--theme=light"}, "--theme given more than once"},
		{"bad window state", []string{"--window-state=full"}, `--window-state: invalid value "full" for window-state (allowed: normal, maximized)`},
		{"settings key needs two dashes", []string{"-theme", "dark"}, "unknown option -theme"},
		{"not a setting", []string{"--nope", "x"}, "unknown option --nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.args)
			var ue *UsageError
			if !errors.As(err, &ue) {
				t.Fatalf("err = %v, want a usage error", err)
			}
			if err.Error() != tt.msg {
				t.Fatalf("err = %q, want %q", err, tt.msg)
			}
		})
	}
}

func TestOptionGettersReturnCopies(t *testing.T) {
	o, err := Parse([]string{"a", "b", "--theme", "dark"})
	if err != nil {
		t.Fatal(err)
	}
	pos := o.Positionals()
	pos[0] = "changed"
	set := o.Settings()
	set[0].Value = "light"
	if got := o.Positionals(); got[0] != "a" {
		t.Errorf("Positionals changed through a returned slice: %q", got)
	}
	if got := o.Settings(); got[0].Value != "dark" {
		t.Errorf("Settings changed through a returned slice: %+v", got)
	}
}

func TestParseDoesNotKeepTheArgsSlice(t *testing.T) {
	args := []string{"--", "a", "b"}
	o, err := Parse(args)
	if err != nil {
		t.Fatal(err)
	}
	args[1] = "changed"
	if got := o.Positionals(); got[0] != "a" {
		t.Fatalf("Positionals share the args slice: %q", got)
	}
}

func TestGetters(t *testing.T) {
	o, err := Parse([]string{"-o", "M", "--auto-apply", "-h", "-v", "x", "--", "y"})
	if err != nil {
		t.Fatal(err)
	}
	if o.Output() != "M" || !o.HasOutput() || !o.AutoApply() || !o.Help() || !o.Version() ||
		o.DashDash() != 1 || strings.Join(o.Positionals(), " ") != "x y" || len(o.Settings()) != 0 {
		t.Fatalf("getters: %+v", o)
	}
	var zero Option
	if z, _ := Parse(nil); z.HasOutput() || z.DashDash() != -1 || zero.Positionals() != nil {
		t.Fatalf("empty: %+v", z)
	}
}
