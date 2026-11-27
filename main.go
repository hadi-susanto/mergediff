package main

import (
	"cmp"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/command"
	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/gui"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// The built frontend. frontend/dist/.gitkeep keeps this compiling before the
// first frontend build.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	defer func() {
		if r := recover(); r != nil {
			reportCrash(os.Stderr, "panic", r, debug.Stack())
			os.Exit(command.ExitError)
		}
	}()
	os.Exit(run(devArgs(os.Args[1:]), command.Env{
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Getwd:      os.Getwd,
		ConfigPath: config.Path,
		Version:    Version(),
		OpenWindow: func(s session.Session, c config.Config) int { return gui.Run(s, c, guiOptions) },
	}))
}

// guiOptions are what the window layer needs from main: the embedded
// frontend, the version, and the crash hooks. Only main exits the process.
var guiOptions = gui.Options{
	Assets:  assets,
	Version: Version(),
	OnPanic: func(value any, stack []byte) {
		reportCrash(os.Stderr, "panic in bound method", value, stack)
		os.Exit(command.ExitError)
	},
	// Wails exits with 1 after a fatal error, which would read as
	// "cancelled" to Git; exit with 2 first.
	OnFatal: func(err error) {
		reportCrash(os.Stderr, "fatal error", err, debug.Stack())
		os.Exit(command.ExitError)
	},
	OnError: func(err error) { fmt.Fprintf(os.Stderr, "mergediff: %v\n", err) },
}

// run parses the command line, selects the command and runs it, returning
// the process exit code. Bad arguments and unreadable files are reported
// before any window opens. Tests call it with a fake Env.
func run(args []string, env command.Env) int {
	opt, err := cliargs.Parse(args)
	if err != nil {
		return report(env.Stderr, err)
	}
	cmd, err := command.Select(opt)
	if err != nil {
		return report(env.Stderr, err)
	}
	code, err := cmd.Run(env)
	if err != nil {
		return report(env.Stderr, err)
	}
	return code
}

// report is the only place errors become output and exit code 2: a usage
// error with the usage text, any other error alone.
func report(w io.Writer, err error) int {
	var cu *command.UsageError
	var pu *cliargs.UsageError
	switch {
	case errors.As(err, &cu):
		fmt.Fprintf(w, "mergediff: %s\n\n%s", cu.Msg, cmp.Or(cu.Usage, command.Usage))
	case errors.As(err, &pu):
		fmt.Fprintf(w, "mergediff: %v\n\n%s", err, command.Usage)
	default:
		fmt.Fprintf(w, "mergediff: %v\n", err)
	}
	return command.ExitError
}
