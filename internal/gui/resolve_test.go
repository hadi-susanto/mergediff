package gui

import (
	"errors"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/command"
	"github.com/hadi-susanto/mergediff/internal/gitconflict"
	"github.com/hadi-susanto/mergediff/internal/session"
	"github.com/hadi-susanto/mergediff/internal/testrepo"
)

// TestResolveExitCode checks that a resolve window's exit code comes from
// Git when it closes.
func TestResolveExitCode(t *testing.T) {
	dir := testrepo.Conflicted(t)
	s, err := session.StartResolve(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	win := newWindowState(s)
	if c := win.code(); c != command.ExitCancel {
		t.Errorf("code with conflicts left = %d, want 1", c)
	}
	if _, err := s.Accept([]string{"a.txt", "sub/b.txt"}, gitconflict.Yours); err != nil {
		t.Fatal(err)
	}
	if c := win.code(); c != command.ExitOK {
		t.Errorf("code with none left = %d, want 0", c)
	}
	win.Quit(command.ExitError)
	if c := win.code(); c != command.ExitError {
		t.Errorf("code after Quit(2) = %d, want 2", c)
	}
}

func TestWindowsAcceptSideRefusesOpenMergeWindows(t *testing.T) {
	w, _, _ := fakeWindows()
	win := newWindowState(diffSession())
	w.openMain(win)
	if err := w.openChild(mainWindowName, mergeKey("a.txt"), diffSession()); err != nil {
		t.Fatal(err)
	}
	if _, err := w.acceptSide(mainWindowName, win, []string{"b.txt", "a.txt"}, gitconflict.Yours); !errors.Is(err, errMergeOpen) ||
		!strings.Contains(err.Error(), "a.txt") {
		t.Errorf("accept with a merge window open = %v, want errMergeOpen naming a.txt", err)
	}
	// Not locked: reaches the session check (a diff window cannot accept).
	if _, err := w.acceptSide(mainWindowName, win, []string{"b.txt"}, gitconflict.Yours); !errors.Is(err, errWrongMode) {
		t.Errorf("accept without a lock = %v, want errWrongMode", err)
	}
}

func TestWindowsOpenMerge(t *testing.T) {
	dir := testrepo.Conflicted(t)
	s, err := session.StartResolve(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	w, created, _ := fakeWindows()
	var events []string
	w.emit = func(e string) { events = append(events, e) }
	var opened []session.Session
	inner := w.newWindow
	w.newWindow = func(name string, sess session.Session, win *window) windowHandle {
		opened = append(opened, sess)
		return inner(name, sess, win)
	}
	win := newWindowState(s)
	w.openMain(win)
	changed := conflictsChangedEvent(mainWindowName)

	if err := w.openMerge(mainWindowName, win, "a.txt"); err != nil {
		t.Fatal(err)
	}
	if _, ok := opened[len(opened)-1].(*session.MergeSession); len(opened) != 2 || !ok || opened[1].View().Text.LeftLabel != "main" {
		t.Fatalf("merge window session = %+v", opened[len(opened)-1])
	}
	if len(events) != 1 || events[0] != changed {
		t.Errorf("events after opening = %v, want %s", events, changed)
	}
	if got := w.openMerges(mainWindowName, false); len(got) != 1 || got[0] != "a.txt" {
		t.Errorf("open merges = %v", got)
	}
	// Opening it again focuses the window.
	if err := w.openMerge(mainWindowName, win, "a.txt"); err != nil {
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
	if err := child.sess.(*session.MergeSession).Save("merged\n"); err != nil {
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
	if err := w.openMerge(mainWindowName, win, "a.txt"); !errors.Is(err, session.ErrNotConflicted) {
		t.Errorf("opening a resolved file = %v, want ErrNotConflicted", err)
	}
	// Show changes refuses a text conflict.
	if err := w.showChanges(mainWindowName, win, "sub/b.txt"); !errors.Is(err, session.ErrCannotOpen) {
		t.Errorf("show changes of a text conflict = %v, want ErrCannotOpen", err)
	}
}
