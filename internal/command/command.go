// Package command holds mergediff's commands: Select picks one from a
// parsed command line (internal/cliargs) and checks its arguments, and
// Run executes it. It never imports Wails: the window is opened through
// Env.OpenWindow, which main supplies.
package command

import (
	"fmt"
	"io"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// Process exit codes.
const (
	ExitOK     = 0 // merge saved and resolved, or diff closed
	ExitCancel = 1 // cancelled, or closed with unresolved conflicts / unsaved changes
	ExitError  = 2 // usage or I/O error
)

// Command is one thing mergediff can do. Run returns the exit code, or an
// error that main reports with exit code 2 (with the usage text for a
// *UsageError). A command writes its normal output to env.Stdout itself.
type Command interface {
	Run(env Env) (int, error)
}

// Env is what commands need from the process. main supplies the real
// ones; tests supply fakes.
type Env struct {
	Stdout, Stderr io.Writer
	Getwd          func() (string, error)
	// ConfigPath locates the config file; an error means there is none
	// (settings then use their defaults when a window opens).
	ConfigPath func() (string, error)
	// Version is the build version, e.g. "2026-11-19 (a1b2c3d)".
	Version string
	// OpenWindow opens the main window and blocks until it closes,
	// returning the exit code.
	OpenWindow func(*session.Session, config.Config) int
}

// UsageError is a command-line mistake. main prints "mergediff: Msg", a
// blank line and Usage (the general usage text when empty), and exits 2.
type UsageError struct {
	Msg   string
	Usage string
}

func (e *UsageError) Error() string { return e.Msg }

func usagef(format string, args ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, args...)}
}

// rejectFlags returns a usage error naming the first of -o, --auto-apply
// and the settings flags that opt has; cmd is the command that does not
// take them.
func rejectFlags(opt cliargs.Option, cmd string) error {
	switch s := opt.Settings(); {
	case opt.HasOutput():
		return usagef("-o is not valid with %s", cmd)
	case opt.AutoApply():
		return usagef("--auto-apply is not valid with %s", cmd)
	case len(s) > 0:
		return usagef("--%s is not valid with %s", s[0].Key, cmd)
	}
	return nil
}
