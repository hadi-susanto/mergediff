package main

import (
	"fmt"
	"io"
	"strings"
)

// Process exit codes.
const (
	exitOK     = 0 // merge saved and resolved, or diff closed
	exitCancel = 1 // cancelled, or closed with unresolved conflicts / unsaved changes
	exitError  = 2 // usage or I/O error
)

const usage = `Usage:
  mergediff BASE LOCAL REMOTE -o MERGED [--auto-apply]   3-way merge
  mergediff LEFT RIGHT                                    2-way diff (read-only)
  mergediff --help | --version

Options:
  -o, --output MERGED   file the merge result is written to (required for a merge)
      --auto-apply      apply all non-conflicting changes at startup
  -h, --help            show this help
  -v, --version         print the version
      --                end of options (for paths starting with "-")

Exit codes:
  0  merge saved with all conflicts resolved, or diff window closed
  1  cancelled, or closed with unresolved conflicts or unsaved changes
  2  usage or I/O error
`

// command is what the command line asks for.
type command int

const (
	cmdMerge command = iota
	cmdDiff
	cmdHelp
	cmdVersion
)

// options is the parsed command line.
type options struct {
	cmd       command
	base      string // merge
	local     string // merge
	remote    string // merge
	output    string // merge
	autoApply bool   // merge
	left      string // diff
	right     string // diff
}

// usageError is a command-line mistake; it is reported with the usage text
// and exit code 2.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return &usageError{msg: fmt.Sprintf(format, args...)}
}

// parseArgs parses the arguments after the program name. Options may appear
// before, between or after the paths, because Git's mergetool command puts
// "-o MERGED" last.
func parseArgs(args []string) (options, error) {
	var o options
	var paths []string
	var help, version, haveOutput bool
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			paths = append(paths, arg)
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		switch name {
		case "-o", "--output":
			if !hasValue {
				if i+1 >= len(args) {
					return o, usagef("%s needs a file name", name)
				}
				i++
				value = args[i]
			}
			if value == "" {
				return o, usagef("%s needs a file name", name)
			}
			if haveOutput {
				return o, usagef("%s given more than once", name)
			}
			o.output, haveOutput = value, true
			continue
		}
		if hasValue {
			return o, usagef("%s does not take a value", name)
		}
		switch name {
		case "--auto-apply":
			o.autoApply = true
		case "-h", "--help":
			help = true
		case "-v", "--version":
			version = true
		default:
			return o, usagef("unknown option %s", arg)
		}
	}

	switch {
	case help:
		o.cmd = cmdHelp
		return o, nil
	case version:
		o.cmd = cmdVersion
		return o, nil
	}

	switch len(paths) {
	case 3:
		if !haveOutput {
			return o, usagef("a 3-way merge needs -o MERGED")
		}
		o.cmd = cmdMerge
		o.base, o.local, o.remote = paths[0], paths[1], paths[2]
	case 2:
		if haveOutput {
			return o, usagef("-o is only valid for a 3-way merge")
		}
		if o.autoApply {
			return o, usagef("--auto-apply is only valid for a 3-way merge")
		}
		o.cmd = cmdDiff
		o.left, o.right = paths[0], paths[1]
	case 0:
		return o, usagef("missing file arguments")
	default:
		return o, usagef("expected 2 files (diff) or 3 files and -o (merge), got %d", len(paths))
	}
	return o, nil
}

// printUsageError writes a usage error followed by the usage text.
func printUsageError(w io.Writer, err error) {
	fmt.Fprintf(w, "mergediff: %v\n\n%s", err, usage)
}
