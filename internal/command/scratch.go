package command

import (
	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// Scratch opens an editable 2-way diff of pasted text (no arguments).
type Scratch struct{ settings []cliargs.Setting }

// newScratch checks "mergediff" without paths.
func newScratch(opt cliargs.Option) (Command, error) {
	if opt.HasOutput() || opt.AutoApply() {
		return nil, usagef("-o and --auto-apply need BASE LOCAL REMOTE")
	}
	return Scratch{settings: opt.Settings()}, nil
}

// Run opens the scratch window.
func (s Scratch) Run(env Env) (int, error) {
	return openWindow(env, session.NewScratch(), s.settings)
}
