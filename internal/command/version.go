package command

import (
	"fmt"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
)

// Version prints "mergediff <version>".
type Version struct{}

// Run prints the version.
func (Version) Run(env Env) (int, error) {
	fmt.Fprintf(env.Stdout, "mergediff %s\n", env.Version)
	return ExitOK, nil
}

// newVersion checks "mergediff version"; --help shows the general usage.
func newVersion(opt cliargs.Option, args []string) (Command, error) {
	if opt.Help() {
		return Help{}, nil
	}
	if err := rejectFlags(opt, "version"); err != nil {
		return nil, err
	}
	if len(args) > 0 {
		return nil, usagef("version takes no arguments, got %d", len(args))
	}
	return Version{}, nil
}
