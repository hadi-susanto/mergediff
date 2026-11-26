package gitconflict

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// ErrOutsideRepo is returned for a PATH argument outside the working tree.
var ErrOutsideRepo = errors.New("outside the repository")

// Repo is a Git working tree.
type Repo struct {
	// Top is the working tree's top-level directory, as Git reports it.
	Top string
	// Prefix is the directory Open was called in, relative to Top, with "/"
	// separators and a trailing "/"; "" at the top level.
	Prefix string
	// Version is the version of the git executable.
	Version Version

	exe string
}

// Open finds the Git working tree that contains dir. It fails with
// ErrNoGit, ErrGitTooOld or ErrNotWorkTree (all checked with errors.Is).
func Open(dir string) (*Repo, error) {
	exe, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoGit, err)
	}
	out, err := runGit(exe, "", nil, "version")
	if err != nil {
		return nil, err
	}
	v, err := parseVersion(firstLine(out))
	if err != nil {
		return nil, err
	}
	if v.Less(MinVersion) {
		return nil, fmt.Errorf("%w: found %s, need %s or later", ErrGitTooOld, v, MinVersion)
	}
	// --show-toplevel fails inside .git and prints nothing useful in a
	// bare repository, so ask first whether dir is in a working tree.
	out, err = runGit(exe, dir, nil, "rev-parse", "--is-inside-work-tree")
	if err != nil || firstLine(out) != "true" {
		return nil, fmt.Errorf("%s: %w", dir, ErrNotWorkTree)
	}
	out, err = runGit(exe, dir, nil, "rev-parse", "--show-toplevel", "--show-prefix")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\r\n"), "\n")
	if len(lines) < 1 || lines[0] == "" {
		return nil, fmt.Errorf("%s: %w", dir, ErrNotWorkTree)
	}
	r := &Repo{Top: strings.TrimSuffix(lines[0], "\r"), Version: v, exe: exe}
	if len(lines) > 1 {
		r.Prefix = strings.TrimSuffix(lines[1], "\r")
	}
	return r, nil
}

// git runs git in the top-level directory and returns its output.
func (r *Repo) git(args ...string) ([]byte, error) {
	return runGit(r.exe, r.Top, nil, args...)
}

// gitIn is git with standard input.
func (r *Repo) gitIn(stdin io.Reader, args ...string) ([]byte, error) {
	return runGit(r.exe, r.Top, stdin, args...)
}

// Name is the name of the top-level directory, e.g. for a window title.
func (r *Repo) Name() string {
	return filepath.Base(filepath.FromSlash(r.Top))
}

// Abs returns the absolute file name of path (relative to Top, "/"
// separators).
func (r *Repo) Abs(p string) string {
	return filepath.Join(filepath.FromSlash(r.Top), filepath.FromSlash(p))
}

// Pathspecs turns PATH arguments, relative to the directory Open was called
// in (or absolute), into paths relative to Top. A path naming the whole
// working tree yields nil, which means "no limit". Paths outside the
// working tree fail with ErrOutsideRepo.
func (r *Repo) Pathspecs(args []string) ([]string, error) {
	var specs []string
	for _, a := range args {
		var p string
		if filepath.IsAbs(a) {
			rel, err := r.relFromAbs(a)
			if err != nil {
				return nil, err
			}
			p = rel
		} else {
			p = path.Clean(r.Prefix + filepath.ToSlash(a))
		}
		if p == ".." || strings.HasPrefix(p, "../") {
			return nil, fmt.Errorf("%s: %w", a, ErrOutsideRepo)
		}
		if p == "." {
			return nil, nil // the whole working tree
		}
		specs = append(specs, p)
	}
	return specs, nil
}

// relFromAbs returns abs relative to Top, comparing real paths so a
// symlinked top level or temporary directory still matches.
func (r *Repo) relFromAbs(abs string) (string, error) {
	top := filepath.FromSlash(r.Top)
	if t, err := filepath.EvalSymlinks(top); err == nil {
		top = t
	}
	target := filepath.Clean(abs)
	// The path itself may not exist (a deleted file); resolve its
	// directory instead.
	if t, err := filepath.EvalSymlinks(target); err == nil {
		target = t
	} else if d, err := filepath.EvalSymlinks(filepath.Dir(target)); err == nil {
		target = filepath.Join(d, filepath.Base(target))
	}
	rel, err := filepath.Rel(top, target)
	if err != nil {
		return "", fmt.Errorf("%s: %w", abs, ErrOutsideRepo)
	}
	return path.Clean(filepath.ToSlash(rel)), nil
}

// gitPaths returns the file names of Git state files (e.g. "MERGE_HEAD",
// "rebase-merge"), with "rev-parse --git-path" so linked worktrees work.
func (r *Repo) gitPaths(names ...string) (map[string]string, error) {
	args := []string{"rev-parse"}
	for _, n := range names {
		args = append(args, "--git-path", n)
	}
	out, err := r.git(args...)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\r\n"), "\n")
	if len(lines) != len(names) {
		return nil, fmt.Errorf("git rev-parse --git-path: expected %d lines, got %d", len(names), len(lines))
	}
	paths := make(map[string]string, len(names))
	for i, n := range names {
		p := filepath.FromSlash(strings.TrimSuffix(lines[i], "\r"))
		if !filepath.IsAbs(p) {
			p = filepath.Join(filepath.FromSlash(r.Top), p)
		}
		paths[n] = p
	}
	return paths, nil
}
