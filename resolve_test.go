package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/gitconflict"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// conflictRepo creates a repository where merging feature into main
// stops with a.txt and sub/b.txt conflicted, and returns its directory.
func conflictRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	os.WriteFile(global, nil, 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	git := func(fail bool, args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t",
			"-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if (err != nil) != fail {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(text string) {
		os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
		writeFile(t, dir, "a.txt", text+"\n")
		writeFile(t, filepath.Join(dir, "sub"), "b.txt", text+"\n")
	}
	git(false, "init", "-q")
	git(false, "symbolic-ref", "HEAD", "refs/heads/main")
	write("base")
	git(false, "add", "-A")
	git(false, "commit", "-q", "-m", "base")
	git(false, "checkout", "-q", "-b", "feature")
	write("feature")
	git(false, "commit", "-q", "-am", "feature")
	git(false, "checkout", "-q", "main")
	write("main")
	git(false, "commit", "-q", "-am", "main")
	git(true, "merge", "--no-edit", "feature")
	return dir
}

func TestRunResolve(t *testing.T) {
	dir := conflictRepo(t)
	opened := stubGUI(t, 7)
	var stdout, stderr bytes.Buffer

	t.Chdir(filepath.Join(dir, "sub"))
	if code := run([]string{"resolve", "--auto-apply"}, &stdout, &stderr); code != 7 {
		t.Fatalf("exit code %d (stderr %q)", code, stderr.String())
	}
	s := *opened
	if s == nil || s.Mode != session.Resolve || !strings.HasPrefix(s.Title, "Resolve conflicts · ") {
		t.Fatalf("session = %+v", s)
	}
	list, err := s.Conflicts()
	if err != nil || len(list.Rows) != 2 {
		t.Fatalf("Conflicts = %+v, %v", list, err)
	}

	// The exit code comes from Git when the window closes.
	app := newApp(s)
	if c := app.code(); c != exitCancel {
		t.Errorf("code with conflicts left = %d, want 1", c)
	}
	if _, err := s.Accept([]string{"a.txt", "sub/b.txt"}, gitconflict.Yours); err != nil {
		t.Fatal(err)
	}
	if c := app.code(); c != exitOK {
		t.Errorf("code with none left = %d, want 0", c)
	}
	app.Quit(exitError)
	if c := app.code(); c != exitError {
		t.Errorf("code after Quit(2) = %d, want 2", c)
	}

	// Nothing left: a message, exit 0, no window.
	*opened = nil
	stdout.Reset()
	if code := run([]string{"resolve"}, &stdout, &stderr); code != exitOK || *opened != nil ||
		stdout.String() != "mergediff: no conflicts to resolve\n" {
		t.Errorf("no conflicts: code %d, opened %v, stdout %q", code, *opened != nil, stdout.String())
	}
}

func TestRunResolveErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	opened := stubGUI(t, 7)
	var stdout, stderr bytes.Buffer
	t.Chdir(t.TempDir())
	if code := run([]string{"resolve"}, &stdout, &stderr); code != exitError || *opened != nil ||
		!strings.Contains(stderr.String(), "not inside a Git working tree") || strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("outside a repository: code %d, stderr %q", code, stderr.String())
	}

	dir := conflictRepo(t)
	t.Chdir(dir)
	stderr.Reset()
	if code := run([]string{"resolve", "../x"}, &stdout, &stderr); code != exitError ||
		!strings.Contains(stderr.String(), "outside the repository") {
		t.Errorf("path outside: code %d, stderr %q", code, stderr.String())
	}
}

func TestWindowsAcceptSideRefusesOpenMergeWindows(t *testing.T) {
	w, _, _ := fakeWindows()
	app := newApp(diffSession())
	w.openMain(app)
	if err := w.openChild(mainWindowName, mergeKey("a.txt"), diffSession()); err != nil {
		t.Fatal(err)
	}
	if _, err := w.acceptSide(mainWindowName, app, []string{"b.txt", "a.txt"}, gitconflict.Yours); !errors.Is(err, errMergeOpen) ||
		!strings.Contains(err.Error(), "a.txt") {
		t.Errorf("accept with a merge window open = %v, want errMergeOpen naming a.txt", err)
	}
	// Not locked: reaches the session (a diff session refuses it).
	if _, err := w.acceptSide(mainWindowName, app, []string{"b.txt"}, gitconflict.Yours); !errors.Is(err, session.ErrNotResolve) {
		t.Errorf("accept without a lock = %v, want the session's ErrNotResolve", err)
	}
}

func TestWindowsOpenMerge(t *testing.T) {
	dir := conflictRepo(t)
	s, err := session.StartResolve(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	w, created, _ := fakeWindows()
	var events []string
	w.emit = func(e string) { events = append(events, e) }
	var opened []*session.Session
	inner := w.newWindow
	w.newWindow = func(name string, sess *session.Session, app *App) windowHandle {
		opened = append(opened, sess)
		return inner(name, sess, app)
	}
	app := newApp(s)
	w.openMain(app)
	changed := conflictsChangedEvent(mainWindowName)

	if err := w.openMerge(mainWindowName, app, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 2 || opened[1].Mode != session.Merge || opened[1].LocalLabel != "main" {
		t.Fatalf("merge window session = %+v", opened[len(opened)-1])
	}
	if len(events) != 1 || events[0] != changed {
		t.Errorf("events after opening = %v, want %s", events, changed)
	}
	if got := w.openMerges(mainWindowName, false); len(got) != 1 || got[0] != "a.txt" {
		t.Errorf("open merges = %v", got)
	}
	// Opening it again focuses the window.
	if err := w.openMerge(mainWindowName, app, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 2 || created["child-1"].focused != 1 {
		t.Errorf("second open: %d windows, focused %d", len(opened), created["child-1"].focused)
	}

	// Unsaved edits are reported; saving and closing unlocks and refreshes.
	child, _ := w.byName("child-1")
	if got := w.openMerges(mainWindowName, true); len(got) != 0 {
		t.Errorf("unsaved before an edit = %v", got)
	}
	child.SetDirty(true)
	if got := w.openMerges(mainWindowName, true); len(got) != 1 {
		t.Errorf("unsaved after an edit = %v", got)
	}
	if err := child.Save("merged\n"); err != nil {
		t.Fatal(err)
	}
	created["child-1"].Close()
	if len(events) != 2 || events[1] != changed {
		t.Errorf("events after closing = %v", events)
	}
	if got := w.openMerges(mainWindowName, false); len(got) != 0 {
		t.Errorf("open merges after closing = %v", got)
	}
	list, err := s.Conflicts()
	if err != nil || list.Unresolved != 1 {
		t.Errorf("after saving: %+v, %v", list, err)
	}

	// A resolved file cannot be opened.
	if err := w.openMerge(mainWindowName, app, "a.txt"); !errors.Is(err, session.ErrNotConflicted) {
		t.Errorf("opening a resolved file = %v, want ErrNotConflicted", err)
	}
	// Show changes refuses a text conflict.
	if err := w.showChanges(mainWindowName, app, "sub/b.txt"); !errors.Is(err, session.ErrCannotOpen) {
		t.Errorf("show changes of a text conflict = %v, want ErrCannotOpen", err)
	}
}
