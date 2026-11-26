package command

import (
	"errors"
	"fmt"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// Resolve is "mergediff resolve": the Conflicts window for the Git working
// tree containing the working directory.
type Resolve struct {
	paths     []string
	autoApply bool
	settings  []cliargs.Setting
}

// newResolve checks "mergediff resolve ..."; args are the positionals
// after "resolve" (PATH limits). --help shows the general usage and
// --version the version, as before resolve was a command of its own.
func newResolve(opt cliargs.Option, args []string) (Command, error) {
	switch {
	case opt.Help():
		return Help{}, nil
	case opt.Version():
		return Version{}, nil
	case opt.HasOutput():
		return nil, usagef("-o is not valid with resolve: each file is written where it is")
	}
	return Resolve{paths: args, autoApply: opt.AutoApply(), settings: opt.Settings()}, nil
}

// Run lists the conflicts and opens the window; with none, it says so and
// exits 0 without a window.
func (r Resolve) Run(env Env) (int, error) {
	cwd, err := env.Getwd()
	if err != nil {
		return ExitError, err
	}
	sess, err := session.StartResolve(cwd, r.paths, r.autoApply)
	if errors.Is(err, session.ErrNoConflicts) {
		fmt.Fprintf(env.Stdout, "mergediff: %v\n", err)
		return ExitOK, nil
	}
	if err != nil {
		return ExitError, err
	}
	return openWindow(env, sess, r.settings)
}
