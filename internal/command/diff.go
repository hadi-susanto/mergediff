package command

import (
	"os"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// Diff compares two files, or two directories (git difftool -d).
type Diff struct {
	left, right string
	settings    []cliargs.Setting
}

// newDiff checks "mergediff LEFT RIGHT"; pos are the two paths.
func newDiff(opt cliargs.Option, pos []string) (Command, error) {
	switch {
	case opt.HasOutput():
		return nil, usagef("-o is only valid for a 3-way merge")
	case opt.AutoApply():
		return nil, usagef("--auto-apply is only valid for a 3-way merge")
	}
	return Diff{left: pos[0], right: pos[1], settings: opt.Settings()}, nil
}

// Run opens the diff window: a directory diff when both paths are
// directories, a file diff otherwise.
func (d Diff) Run(env Env) (int, error) {
	dirs, err := bothDirs(d.left, d.right)
	if err != nil {
		return ExitError, err
	}
	var sess *session.Session
	if dirs {
		sess, err = session.StartDirDiff(d.left, d.right)
	} else {
		sess, err = session.LoadDiff(d.left, d.right)
	}
	if err != nil {
		return ExitError, err
	}
	return openWindow(env, sess, d.settings)
}

// bothDirs reports whether LEFT and RIGHT are both directories (a
// directory diff). One directory and one file is a usage error. A path
// that cannot be read counts as a file, so loading it reports the error.
func bothDirs(left, right string) (bool, error) {
	l, r := isDir(left), isDir(right)
	switch {
	case l && !r:
		return false, usagef("LEFT %s is a directory but RIGHT %s is a file; compare two files or two directories", left, right)
	case r && !l:
		return false, usagef("LEFT %s is a file but RIGHT %s is a directory; compare two files or two directories", left, right)
	}
	return l && r, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
