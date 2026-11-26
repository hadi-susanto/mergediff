package command

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/gitconflict"
	"github.com/hadi-susanto/mergediff/internal/session"
	"github.com/hadi-susanto/mergediff/internal/testrepo"
)

// runResolve checks and runs "mergediff resolve ARGS" in f's env.
func runResolve(t *testing.T, f *fake, args ...string) (int, error) {
	t.Helper()
	f.reset()
	opt := parse(t, append([]string{"resolve"}, args...)...)
	cmd, err := newResolve(opt, opt.Positionals()[1:])
	if err != nil {
		return ExitError, err
	}
	return cmd.Run(f.env)
}

func TestResolve(t *testing.T) {
	dir := testrepo.Conflicted(t)
	f := newFake(t, 7)

	t.Chdir(filepath.Join(dir, "sub"))
	if code, err := runResolve(t, f, "--auto-apply"); code != 7 || err != nil {
		t.Fatalf("exit code %d, %v", code, err)
	}
	s, ok := f.opened.(*session.ResolveSession)
	if !ok || !strings.HasPrefix(s.View().Title, "Resolve conflicts · ") {
		t.Fatalf("session = %#v", f.opened)
	}
	list, err := s.Conflicts()
	if err != nil || len(list.Rows) != 2 {
		t.Fatalf("Conflicts = %+v, %v", list, err)
	}

	// PATH limits are relative to the working directory.
	if code, err := runResolve(t, f, "--", "b.txt"); code != 7 || err != nil {
		t.Fatalf("resolve b.txt: %d, %v", code, err)
	}
	if list, err := f.opened.(*session.ResolveSession).Conflicts(); err != nil || len(list.Rows) != 1 || list.Rows[0].Path != "sub/b.txt" {
		t.Fatalf("resolve b.txt: %+v, %v", list, err)
	}

	// Nothing left: a message, exit 0, no window.
	if _, err := s.Accept([]string{"a.txt", "sub/b.txt"}, gitconflict.Yours); err != nil {
		t.Fatal(err)
	}
	if code, err := runResolve(t, f); code != ExitOK || err != nil || f.opened != nil ||
		f.stdout.String() != "mergediff: no conflicts to resolve\n" {
		t.Errorf("no conflicts: code %d, %v, opened %v, stdout %q", code, err, f.opened != nil, f.stdout.String())
	}
}

func TestResolveErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	f := newFake(t, 7)
	t.Chdir(t.TempDir())
	var ue *UsageError
	if code, err := runResolve(t, f); code != ExitError || f.opened != nil || err == nil || errors.As(err, &ue) ||
		!strings.Contains(err.Error(), "not inside a Git working tree") {
		t.Errorf("outside a repository: code %d, %v", code, err)
	}

	dir := testrepo.Conflicted(t)
	t.Chdir(dir)
	if code, err := runResolve(t, f, "../x"); code != ExitError || err == nil || !strings.Contains(err.Error(), "outside the repository") {
		t.Errorf("path outside: code %d, %v", code, err)
	}

	f.env.Getwd = func() (string, error) { return "", errors.New("no cwd") }
	if _, err := runResolve(t, f); err == nil || err.Error() != "no cwd" {
		t.Errorf("Getwd error: %v", err)
	}

	_, err := runResolve(t, f, "-o", "m")
	wantUsageError(t, err, "-o is not valid with resolve: each file is written where it is")
}

func TestResolveHelpAndVersion(t *testing.T) {
	for args, want := range map[string]Command{"--help": Help{}, "-v": Version{}, "--version --help": Help{}} {
		opt := parse(t, append([]string{"resolve"}, strings.Fields(args)...)...)
		if cmd, err := newResolve(opt, nil); err != nil || cmd != want {
			t.Errorf("resolve %s = %#v, %v; want %#v", args, cmd, err, want)
		}
	}
}
