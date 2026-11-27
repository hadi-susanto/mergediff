package gui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/command"
	"github.com/hadi-susanto/mergediff/internal/gitconflict"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// fakeWindow is a windowHandle that records Close and Focus calls. Closing
// it reports the close to the registry, as the real window's
// WindowClosing listener does.
type fakeWindow struct {
	name    string
	wins    *Windows
	closed  int
	focused int
}

func (f *fakeWindow) Name() string { return f.name }
func (f *fakeWindow) Focus()       { f.focused++ }
func (f *fakeWindow) Close() {
	f.closed++
	f.wins.closed(f.name)
}

// fakeWindows returns a registry whose windows are fakes, a map of the
// windows it created by name, and a counter of application quits.
func fakeWindows() (*Windows, map[string]*fakeWindow, *int) {
	w := newWindows()
	created := map[string]*fakeWindow{}
	quits := 0
	w.newWindow = func(name string, _ session.Session, _ *window) windowHandle {
		f := &fakeWindow{name: name, wins: w}
		created[name] = f
		return f
	}
	w.quitApp = func() { quits++ }
	return w, created, &quits
}

// fakeSession is a session.Session with a fixed View. Its ExitCode
// records its arguments and returns the code.
type fakeSession struct {
	view          session.View
	gotCode       int
	gotQuit, used bool
}

func (f *fakeSession) View() session.View { return f.view }
func (f *fakeSession) ExitCode(code int, quit bool) int {
	f.gotCode, f.gotQuit, f.used = code, quit, true
	return code
}

func diffSession() *fakeSession { return &fakeSession{view: session.View{Mode: session.Diff}} }

func TestWindowsOpenChild(t *testing.T) {
	w, created, _ := fakeWindows()
	mainWin := newWindowState(diffSession())
	w.openMain(mainWin)
	if got, err := w.byName(mainWindowName); err != nil || got != mainWin {
		t.Fatalf("byName(main) = %p, %v; want the main window state", got, err)
	}

	if err := w.openChild(mainWindowName, "a/b.txt", diffSession()); err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("%d windows created, want 2", len(created))
	}
	child := created["child-1"]
	if child == nil {
		t.Fatalf("no window child-1; created %v", created)
	}
	childWin, err := w.byName("child-1")
	if err != nil || childWin == mainWin {
		t.Fatalf("byName(child-1) = %p, %v; want its own state", childWin, err)
	}

	// Opening the same key again focuses the existing window.
	if err := w.openChild(mainWindowName, "a/b.txt", diffSession()); err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || child.focused != 1 {
		t.Errorf("reopening a/b.txt: %d windows, focused %d times; want 2 windows, focused once", len(created), child.focused)
	}

	// A child's Quit closes only that window.
	childWin.Quit(command.ExitOK)
	if child.closed != 1 {
		t.Errorf("child closed %d times after Quit, want 1", child.closed)
	}
	if _, err := w.byName("child-1"); !errors.Is(err, errUnknownWindow) {
		t.Errorf("byName(closed child) error = %v, want errUnknownWindow", err)
	}
	if created[mainWindowName].closed != 0 {
		t.Error("closing the child closed the main window")
	}
}

func TestWindowsOpenChildUnknownParent(t *testing.T) {
	w, _, _ := fakeWindows()
	if err := w.openChild("nope", "k", diffSession()); !errors.Is(err, errUnknownWindow) {
		t.Fatalf("openChild with unknown parent: %v, want errUnknownWindow", err)
	}
}

func TestWindowsMainClosesChildren(t *testing.T) {
	w, created, quits := fakeWindows()
	w.openMain(newWindowState(diffSession()))
	for _, key := range []string{"a", "b"} {
		if err := w.openChild(mainWindowName, key, diffSession()); err != nil {
			t.Fatal(err)
		}
	}
	// A grandchild, opened by a child.
	if err := w.openChild("child-1", "c", diffSession()); err != nil {
		t.Fatal(err)
	}

	created[mainWindowName].Close()
	for _, name := range []string{"child-1", "child-2", "child-3"} {
		if created[name].closed != 1 {
			t.Errorf("%s closed %d times, want 1", name, created[name].closed)
		}
	}
	if *quits != 1 {
		t.Errorf("application quit %d times, want 1", *quits)
	}
}

// A second session saving, quitting or closing does not change the main
// session's exit code (Phase 1 acceptance).
func TestWindowsChildDoesNotAffectMainExitCode(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	load := func(out string) *session.MergeSession {
		s, err := session.LoadMerge(write("base", "a\n"), write("local", "b\n"), write("remote", "c\n"), filepath.Join(dir, out), false)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	w, created, quits := fakeWindows()
	mainWin := newWindowState(load("main.out"))
	mainQuits := 0
	mainWin.quit = func() { mainQuits++ }
	w.openMain(mainWin)
	if c := mainWin.code(); c != command.ExitCancel {
		t.Fatalf("main exit code before any action = %d, want %d", c, command.ExitCancel)
	}

	if err := w.openChild(mainWindowName, "first", load("child1.out")); err != nil {
		t.Fatal(err)
	}
	child, err := w.byName("child-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := child.sess.(*session.MergeSession).Save("merged\n"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "child1.out")); err != nil || string(got) != "merged\n" {
		t.Fatalf("child saved %q, %v; want %q", got, err, "merged\n")
	}
	child.Quit(command.ExitOK)

	// A child closed by its close button (no Quit).
	if err := w.openChild(mainWindowName, "second", load("child2.out")); err != nil {
		t.Fatal(err)
	}
	created["child-2"].Close()

	if c := mainWin.code(); c != command.ExitCancel {
		t.Errorf("main exit code after child save/quit/close = %d, want %d", c, command.ExitCancel)
	}
	if mainQuits != 0 || *quits != 0 {
		t.Errorf("children ended the main window (%d) or the application (%d)", mainQuits, *quits)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.out")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the main MERGED file was written by a child: %v", err)
	}

	mainWin.Quit(command.ExitOK)
	if c := mainWin.code(); c != command.ExitOK || mainQuits != 1 {
		t.Errorf("main Quit(0): code %d, quits %d; want 0, 1", c, mainQuits)
	}
}

func TestWindowsOpenEntry(t *testing.T) {
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l"), filepath.Join(dir, "r")
	os.Mkdir(left, 0o755)
	os.Mkdir(right, 0o755)
	writeFile(t, left, "m.txt", "a\n")
	writeFile(t, right, "m.txt", "b\n")
	writeFile(t, right, "bin", "x\x00")
	sess, err := session.StartDirDiff(left, right)
	if err != nil {
		t.Fatal(err)
	}
	w, created, _ := fakeWindows()
	mainWin := newWindowState(sess)
	w.openMain(mainWin)

	if err := w.openEntry(mainWindowName, mainWin, "m.txt"); err != nil {
		t.Fatal(err)
	}
	child, err := w.byName("child-1")
	if err != nil {
		t.Fatal(err)
	}
	s := child.sess.View()
	if _, ok := child.sess.(*session.DiffSession); !ok || s.Text.Left != "a\n" || s.Text.Right != "b\n" || s.Title != "m.txt" {
		t.Errorf("child session = %+v", s)
	}

	// Opening it again focuses the open window.
	if err := w.openEntry(mainWindowName, mainWin, "m.txt"); err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || created["child-1"].focused != 1 {
		t.Errorf("reopening: %d windows, focused %d; want 2 windows, focused once", len(created), created["child-1"].focused)
	}

	if err := w.openEntry(mainWindowName, mainWin, "bin"); err == nil {
		t.Error("opening a binary file succeeded")
	}
	if len(created) != 2 {
		t.Errorf("a refused file opened a window (%d windows)", len(created))
	}
}

// TestWrongModeCalls: a window asking for what its session does not serve
// gets errWrongMode naming the call, and no window opens.
func TestWrongModeCalls(t *testing.T) {
	w, created, _ := fakeWindows()
	win := newWindowState(diffSession())
	w.openMain(win)
	for name, call := range map[string]func() error{
		"OpenEntry":   func() error { return w.openEntry(mainWindowName, win, "a.txt") },
		"OpenMerge":   func() error { return w.openMerge(mainWindowName, win, "a.txt") },
		"ShowChanges": func() error { return w.showChanges(mainWindowName, win, "a.txt") },
		"AcceptSide": func() error {
			_, err := w.acceptSide(mainWindowName, win, []string{"a.txt"}, gitconflict.Yours)
			return err
		},
	} {
		if err := call(); !errors.Is(err, errWrongMode) || !strings.HasPrefix(err.Error(), name+": ") {
			t.Errorf("%s from a diff window = %v, want errWrongMode naming it", name, err)
		}
	}
	if len(created) != 1 {
		t.Errorf("windows created: %d, want only the main one", len(created))
	}
	if _, err := as[*session.MergeSession](win, "Save"); !errors.Is(err, errWrongMode) {
		t.Errorf("Save from a diff window = %v, want errWrongMode", err)
	}
	if _, err := as[*session.DirDiffSession](win, "DirEntries"); !errors.Is(err, errWrongMode) {
		t.Errorf("DirEntries from a diff window = %v, want errWrongMode", err)
	}
}
