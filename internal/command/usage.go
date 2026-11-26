package command

import (
	"fmt"
	"strings"

	"github.com/hadi-susanto/mergediff/internal/config"
)

// Usage is the general help text; the settings flags come from
// config.Registry.
var Usage = `Usage:
  mergediff BASE LOCAL REMOTE -o MERGED [--auto-apply]    3-way merge
  mergediff LEFT RIGHT                                    2-way diff (read-only)
  mergediff LEFTDIR RIGHTDIR                              compare two directories (git difftool -d)
  mergediff                                               scratch diff: paste text into both panes
  mergediff resolve [--auto-apply] [--] [PATH...]         list Git's merge conflicts and resolve them
  mergediff settings [list | get KEY | set KEY VALUE | unset KEY | path]
                                                          view or change saved settings
  mergediff help [settings]                               this help, or the settings commands
  mergediff version | --version                           print the version
  mergediff --help

help, version, settings and resolve are reserved words: as the first path
they name a command. For a file with one of these names, put -- before the
paths (preferred) or write ./NAME; options go before --:
  mergediff -- help other.txt
  mergediff -o MERGED -- help local remote
resolve lists the conflicts under PATH... (relative to the current directory),
or all of the repository's.

Options:
  -o, --output MERGED   file the merge result is written to (required for a merge)
      --auto-apply      apply all non-conflicting changes when a merge opens
  -h, --help            show this help
  -v, --version         print the version
      --                end of options (for paths starting with "-")

` + settingsFlagsUsage() + `
Exit codes:
  0  merge saved with all conflicts resolved, diff or directory window closed,
     or no conflicts left when resolve closes
  1  cancelled, or closed with unresolved conflicts or unsaved changes
  2  usage or I/O error
`

// settingsFlagsUsage describes the --KEY VALUE flags of the registered settings.
func settingsFlagsUsage() string {
	var b strings.Builder
	b.WriteString("Settings (override the config file for this run):\n")
	for _, s := range config.Registry {
		if !s.Flag {
			continue
		}
		fmt.Fprintf(&b, "      --%s %s\n", s.Key, strings.Join(s.Values, "|"))
		fmt.Fprintf(&b, "                        %s (default %s)\n", s.Doc, s.Default)
	}
	return b.String()
}

const settingsUsage = `Usage:
  mergediff settings [list]           show every setting, its value and where it comes from
  mergediff settings get KEY          print one setting's value
  mergediff settings set KEY VALUE    save a setting in the config file
  mergediff settings unset KEY        remove a setting from the config file (back to its default)
  mergediff settings path             print the config file location

Comments and other lines in the config file are kept when it is changed.
`
