// Package gitconflict reads and resolves the merge conflicts of a Git
// working tree, the way the JetBrains IDEs' Conflicts dialog does: it lists
// the unmerged paths from the index, reads each side's content from the
// index stages and marks files resolved with checkout / add / rm.
//
// Git always runs through exec (never a shell) as
//
//	git -C <toplevel> -c core.quotepath=off --literal-pathspecs ...
//
// with LC_ALL=C, paths after "--" and -z output where Git offers it. The
// only commands that change anything are "checkout --ours|--theirs",
// "add" and "rm", plus writing a merged file; nothing here changes history.
package gitconflict

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

var (
	// ErrNoGit is returned when the git executable cannot be found.
	ErrNoGit = errors.New("git not found")
	// ErrNotWorkTree is returned for a directory outside a Git working tree
	// (including a bare repository or the .git directory itself).
	ErrNotWorkTree = errors.New("not inside a Git working tree")
	// ErrGitTooOld is returned for Git older than MinVersion.
	ErrGitTooOld = errors.New("git is too old")
)

// MinVersion is the oldest Git this package works with: "cat-file
// --filters" appeared in Git 2.11.
var MinVersion = Version{2, 11, 0}

// Error is a git command that failed. Stderr holds what Git printed.
type Error struct {
	Args   []string // arguments after the fixed options, e.g. ["ls-files", ...]
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	name := "git"
	if len(e.Args) > 0 {
		name += " " + e.Args[0]
	}
	if msg := strings.TrimSpace(e.Stderr); msg != "" {
		return name + ": " + msg
	}
	return name + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// gitEnv is the environment git runs with: the caller's, with messages in
// the C locale so nothing depends on the user's language.
func gitEnv() []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "LC_ALL=") {
			env = append(env, kv)
		}
	}
	return append(env, "LC_ALL=C")
}

// runGit runs the git executable at exe in dir (as -C dir) with args and
// returns its standard output. stdin may be nil.
func runGit(exe, dir string, stdin io.Reader, args ...string) ([]byte, error) {
	full := make([]string, 0, len(args)+6)
	if dir != "" {
		full = append(full, "-C", dir)
	}
	full = append(full, "-c", "core.quotepath=off", "--literal-pathspecs")
	full = append(full, args...)
	cmd := exec.Command(exe, full...)
	cmd.Env = gitEnv()
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), &Error{Args: args, Stderr: stderr.String(), Err: err}
	}
	return stdout.Bytes(), nil
}

// splitNUL splits -z output into its records, dropping the empty record
// after the final NUL.
func splitNUL(out []byte) []string {
	if len(out) == 0 {
		return nil
	}
	s := strings.TrimSuffix(string(out), "\x00")
	return strings.Split(s, "\x00")
}

// firstLine returns the first line of out without its line ending.
func firstLine(out []byte) string {
	s := string(out)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSuffix(s, "\r")
}

// Version is a Git version: major, minor, patch.
type Version [3]int

func (v Version) String() string { return fmt.Sprintf("%d.%d.%d", v[0], v[1], v[2]) }

// Less reports whether v is older than w.
func (v Version) Less(w Version) bool {
	for i := range v {
		if v[i] != w[i] {
			return v[i] < w[i]
		}
	}
	return false
}

// parseVersion reads "git version 2.43.0", "git version 2.39.3 (Apple
// Git-145)" or "git version 2.45.1.windows.1". A missing patch is 0.
func parseVersion(out string) (Version, error) {
	var v Version
	rest, ok := strings.CutPrefix(strings.TrimSpace(out), "git version ")
	if !ok {
		return v, fmt.Errorf("unrecognised git version output %q", out)
	}
	fields := strings.SplitN(strings.Fields(rest + " ")[0], ".", 4)
	for i := 0; i < 3 && i < len(fields); i++ {
		n := 0
		digits := 0
		for _, c := range fields[i] {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
			digits++
		}
		if digits == 0 {
			if i < 2 {
				return v, fmt.Errorf("unrecognised git version output %q", out)
			}
			break
		}
		v[i] = n
	}
	if len(fields) < 2 {
		return v, fmt.Errorf("unrecognised git version output %q", out)
	}
	return v, nil
}
