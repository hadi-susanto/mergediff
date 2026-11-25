package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

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
	w.newWindow = func(name string, _ *session.Session, _ *App) windowHandle {
		f := &fakeWindow{name: name, wins: w}
		created[name] = f
		return f
	}
	w.quitApp = func() { quits++ }
	return w, created, &quits
}

func diffSession() *session.Session { return &session.Session{Mode: session.Diff} }

func TestWindowsOpenChild(t *testing.T) {
	w, created, _ := fakeWindows()
	mainApp := newApp(diffSession())
	w.openMain(mainApp)
	if got, err := w.byName(mainWindowName); err != nil || got != mainApp {
		t.Fatalf("byName(main) = %p, %v; want the main App", got, err)
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
	childApp, err := w.byName("child-1")
	if err != nil || childApp == mainApp {
		t.Fatalf("byName(child-1) = %p, %v; want its own App", childApp, err)
	}

	// Opening the same key again focuses the existing window.
	if err := w.openChild(mainWindowName, "a/b.txt", diffSession()); err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || child.focused != 1 {
		t.Errorf("reopening a/b.txt: %d windows, focused %d times; want 2 windows, focused once", len(created), child.focused)
	}

	// A child's Quit closes only that window.
	childApp.Quit(exitOK)
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
	w.openMain(newApp(diffSession()))
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
	load := func(out string) *session.Session {
		s, err := session.LoadMerge(write("base", "a\n"), write("local", "b\n"), write("remote", "c\n"), filepath.Join(dir, out), false)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	w, created, quits := fakeWindows()
	mainApp := newApp(load("main.out"))
	mainQuits := 0
	mainApp.quit = func() { mainQuits++ }
	w.openMain(mainApp)
	if c := mainApp.code(); c != exitCancel {
		t.Fatalf("main exit code before any action = %d, want %d", c, exitCancel)
	}

	if err := w.openChild(mainWindowName, "first", load("child1.out")); err != nil {
		t.Fatal(err)
	}
	child, err := w.byName("child-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Save("merged\n"); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "child1.out")); err != nil || string(got) != "merged\n" {
		t.Fatalf("child saved %q, %v; want %q", got, err, "merged\n")
	}
	child.Quit(exitOK)

	// A child closed by its close button (no Quit).
	if err := w.openChild(mainWindowName, "second", load("child2.out")); err != nil {
		t.Fatal(err)
	}
	created["child-2"].Close()

	if c := mainApp.code(); c != exitCancel {
		t.Errorf("main exit code after child save/quit/close = %d, want %d", c, exitCancel)
	}
	if mainQuits != 0 || *quits != 0 {
		t.Errorf("children ended the main window (%d) or the application (%d)", mainQuits, *quits)
	}
	if _, err := os.Stat(filepath.Join(dir, "main.out")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the main MERGED file was written by a child: %v", err)
	}

	mainApp.Quit(exitOK)
	if c := mainApp.code(); c != exitOK || mainQuits != 1 {
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
	mainApp := newApp(sess)
	w.openMain(mainApp)

	if err := w.openEntry(mainWindowName, mainApp, "m.txt"); err != nil {
		t.Fatal(err)
	}
	child, err := w.byName("child-1")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := child.LoadSession()
	if s.Mode != session.Diff || s.Local != "a\n" || s.Remote != "b\n" || s.Title != "m.txt" {
		t.Errorf("child session = %+v", s)
	}

	// Opening it again focuses the open window.
	if err := w.openEntry(mainWindowName, mainApp, "m.txt"); err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || created["child-1"].focused != 1 {
		t.Errorf("reopening: %d windows, focused %d; want 2 windows, focused once", len(created), created["child-1"].focused)
	}

	if err := w.openEntry(mainWindowName, mainApp, "bin"); err == nil {
		t.Error("opening a binary file succeeded")
	}
	if len(created) != 2 {
		t.Errorf("a refused file opened a window (%d windows)", len(created))
	}
}
