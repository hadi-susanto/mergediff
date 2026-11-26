package command

import (
	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// Merge is the 3-way merge (git mergetool).
type Merge struct {
	base, local, remote, output string
	autoApply                   bool
	settings                    []cliargs.Setting
}

// newMerge checks "mergediff BASE LOCAL REMOTE -o MERGED"; pos are the
// three paths.
func newMerge(opt cliargs.Option, pos []string) (Command, error) {
	if !opt.HasOutput() {
		return nil, usagef("a 3-way merge needs -o MERGED")
	}
	return Merge{
		base: pos[0], local: pos[1], remote: pos[2], output: opt.Output(),
		autoApply: opt.AutoApply(), settings: opt.Settings(),
	}, nil
}

// Run loads the three files and opens the merge window.
func (m Merge) Run(env Env) (int, error) {
	sess, err := session.LoadMerge(m.base, m.local, m.remote, m.output, m.autoApply)
	if err != nil {
		return ExitError, err
	}
	return openWindow(env, sess, m.settings)
}
