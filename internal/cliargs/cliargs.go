// Package cliargs splits mergediff's command line into flags and positional
// arguments. It knows the flags (one table, including the settings flags of
// config.Registry) but nothing about commands: which command runs and which
// arguments it accepts is decided by internal/command.
package cliargs

import (
	"fmt"
	"slices"
	"strings"

	"github.com/hadi-susanto/mergediff/internal/config"
)

// Option is a parsed command line. It is immutable: its fields are
// unexported and the slice getters return copies.
type Option struct {
	positionals []string
	// dashDash is the index in positionals of the first argument after
	// "--" (len(positionals) when nothing followed it), or -1 without "--".
	dashDash int
	// output is the -o / --output value; Parse rejects an empty one, so
	// "" means the flag was not given.
	output    string
	autoApply bool
	help      bool
	version   bool
	settings  []Setting
}

// Setting is one --KEY VALUE settings flag.
type Setting struct{ Key, Value string }

// Positionals returns the arguments that are not flags, in order,
// including those after "--".
func (o Option) Positionals() []string { return slices.Clone(o.positionals) }

// DashDash returns the index in Positionals of the first argument given
// after "--", or -1 when there was no "--". A positional before that index
// may be a subcommand; one at or after it is always a path.
func (o Option) DashDash() int { return o.dashDash }

// Output returns the -o / --output value ("" when not given).
func (o Option) Output() string { return o.output }

// HasOutput reports whether -o / --output was given.
func (o Option) HasOutput() bool { return o.output != "" }

// AutoApply reports whether --auto-apply was given.
func (o Option) AutoApply() bool { return o.autoApply }

// Help reports whether -h / --help was given.
func (o Option) Help() bool { return o.help }

// Version reports whether -v / --version was given.
func (o Option) Version() bool { return o.version }

// Settings returns the settings flags in the order given.
func (o Option) Settings() []Setting { return slices.Clone(o.settings) }

// UsageError is a command-line mistake: main reports it with the usage text
// and exit code 2.
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

func usagef(format string, args ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, args...)}
}

// flag is one entry of the flag table.
type flag struct {
	takesValue bool
	// missing is the message when a value-taking flag has none.
	missing string
	// set records the flag; name is as typed (e.g. "-o" or "--output").
	set func(o *Option, name, value string) error
}

var switches = map[string]flag{
	"--auto-apply": {set: func(o *Option, _, _ string) error { o.autoApply = true; return nil }},
	"-h":           {set: setHelp},
	"--help":       {set: setHelp},
	"-v":           {set: setVersion},
	"--version":    {set: setVersion},
}

func setHelp(o *Option, _, _ string) error    { o.help = true; return nil }
func setVersion(o *Option, _, _ string) error { o.version = true; return nil }

var output = flag{takesValue: true, missing: "%s needs a file name", set: func(o *Option, name, value string) error {
	if value == "" {
		return usagef("%s needs a file name", name)
	}
	if o.output != "" {
		return usagef("%s given more than once", name)
	}
	o.output = value
	return nil
}}

// lookup returns the flag a name (without "=value") stands for.
func lookup(name string) (flag, bool) {
	if name == "-o" || name == "--output" {
		return output, true
	}
	if f, ok := switches[name]; ok {
		return f, true
	}
	key, ok := strings.CutPrefix(name, "--")
	if !ok {
		return flag{}, false
	}
	s, err := config.Lookup(key)
	if err != nil || !s.Flag {
		return flag{}, false
	}
	return flag{
		takesValue: true,
		missing:    "%s needs a value (" + strings.Join(s.Values, ", ") + ")",
		set: func(o *Option, name, value string) error {
			if err := s.Validate(value); err != nil {
				return usagef("%s: %v", name, err)
			}
			if slices.ContainsFunc(o.settings, func(x Setting) bool { return x.Key == s.Key }) {
				return usagef("%s given more than once", name)
			}
			o.settings = append(o.settings, Setting{s.Key, value})
			return nil
		},
	}, true
}

// Parse splits args (without the program name) into flags and positional
// arguments. Flags may appear before, between or after the positionals,
// because Git's mergetool command puts "-o MERGED" last; "--" ends the
// flags and "-" alone is positional.
func Parse(args []string) (Option, error) {
	o := Option{dashDash: -1}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			o.dashDash = len(o.positionals)
			o.positionals = append(o.positionals, args[i+1:]...)
			return o, nil
		case arg == "-" || !strings.HasPrefix(arg, "-"):
			o.positionals = append(o.positionals, arg)
			continue
		}
		name, value, inline := strings.Cut(arg, "=")
		f, ok := lookup(name)
		switch {
		case !ok:
			return Option{}, usagef("unknown option %s", arg)
		case inline && !f.takesValue:
			return Option{}, usagef("%s does not take a value", name)
		case !inline && f.takesValue:
			if i+1 >= len(args) {
				return Option{}, usagef(f.missing, name)
			}
			i++
			value = args[i]
		}
		if err := f.set(&o, name, value); err != nil {
			return Option{}, err
		}
	}
	return o, nil
}
