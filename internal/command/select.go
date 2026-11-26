package command

import "github.com/hadi-susanto/mergediff/internal/cliargs"

// subcommands are the reserved words. Each constructor gets the positionals
// after the word and checks them and the flags.
var subcommands = map[string]func(cliargs.Option, []string) (Command, error){
	"help":     newHelp,
	"version":  newVersion,
	"settings": newSettings,
	"resolve":  newResolve,
}

// Select picks the command a parsed command line asks for and checks its
// arguments, so a mistake is reported before any window opens.
//
// A reserved word (help, version, settings, resolve) is a subcommand only
// as the first positional and before "--"; otherwise it is a path. A
// subcommand handles --help and --version itself. Without one, --help
// wins, then --version, then the number of paths decides: 3 a merge, 2 a
// diff (files or directories), none the scratch window.
func Select(opt cliargs.Option) (Command, error) {
	pos := opt.Positionals()
	if len(pos) > 0 && opt.DashDash() != 0 {
		if newSub, ok := subcommands[pos[0]]; ok {
			return newSub(opt, pos[1:])
		}
	}
	switch {
	case opt.Help():
		return Help{}, nil
	case opt.Version():
		return Version{}, nil
	}
	switch len(pos) {
	case 3:
		return newMerge(opt, pos)
	case 2:
		return newDiff(opt, pos)
	case 0:
		return newScratch(opt)
	}
	return nil, usagef("expected 2 files or directories (diff) or 3 files and -o (merge), got %d", len(pos))
}
