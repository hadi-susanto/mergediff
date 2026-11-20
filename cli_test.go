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
		{"no args", nil, "missing file arguments"},
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

func TestVersion(t *testing.T) {
	if got := Version(); got != "dev (unknown)" {
		t.Fatalf("Version() = %q without ldflags, want %q", got, "dev (unknown)")
	}
}
