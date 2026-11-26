package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/hadi-susanto/mergediff/internal/diff"
	"github.com/hadi-susanto/mergediff/internal/dirdiff"
	"github.com/hadi-susanto/mergediff/internal/gitconflict"
	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// windowHandle is the part of a Wails window the registry uses. Tests fake it.
type windowHandle interface {
	Name() string
	Close()
	Focus()
}

// winEntry is one window: its session's App, its handle (nil until the
// window exists) and, for a child window, the window that opened it and
// what it shows (key).
type winEntry struct {
	app    *App
	handle windowHandle
	parent string // "" for the main window
	key    string
}

// errUnknownWindow is returned for a window the registry does not know,
// e.g. a call that arrives after its window closed.
var errUnknownWindow = errors.New("unknown window")

// Windows keeps one App (session, exit code, close guard) per window,
// keyed by window name. The main window, opened from the command line,
// decides the process exit code; child windows close themselves and never
// end the process. Closing a window closes the windows it opened.
//
// A window is registered under its name before it is created, so its
// frontend can never call in before the registry knows it.
type Windows struct {
	mu        sync.Mutex
	wins      map[string]*winEntry
	nextChild int

	// newWindow creates the window name showing sess. Set by runGUI;
	// tests fake it.
	newWindow func(name string, sess *session.Session, app *App) windowHandle
	// quitApp ends the application; called when the main window closes.
	quitApp func()
	// settings are the effective settings, given to every child window.
	settings map[string]string
	// emit sends a custom event to the frontends. Set by runGUI.
	emit func(event string)
}

// conflictsChangedEvent names the event that tells a Conflicts window
// (window parent) to refresh: one of its merge windows opened or closed.
func conflictsChangedEvent(parent string) string {
	return "mergediff:conflicts-changed:" + parent
}

func newWindows() *Windows {
	return &Windows{
		wins:      map[string]*winEntry{},
		newWindow: func(name string, _ *session.Session, _ *App) windowHandle { return nopWindow(name) },
		quitApp:   func() {},
		emit:      func(string) {},
	}
}

// nopWindow is a window handle that does nothing, used until runGUI sets
// Windows.newWindow.
type nopWindow string

func (n nopWindow) Name() string { return string(n) }
func (nopWindow) Close()         {}
func (nopWindow) Focus()         {}

// openMain registers app as the main window and creates the window.
func (w *Windows) openMain(app *App) windowHandle {
	w.mu.Lock()
	w.wins[mainWindowName] = &winEntry{app: app}
	w.mu.Unlock()
	return w.create(mainWindowName, app)
}

// openChild shows sess in a child window of parent. key identifies what the
// window shows: if parent already has a window for key, that window is
// focused instead and sess is dropped. A child's App closes only its own
// window and its exit code is ignored.
func (w *Windows) openChild(parent, key string, sess *session.Session) error {
	w.mu.Lock()
	if _, ok := w.wins[parent]; !ok {
		w.mu.Unlock()
		return fmt.Errorf("open window for %s: parent %s: %w", key, parent, errUnknownWindow)
	}
	if e := w.childLocked(parent, key); e != nil {
		h := e.handle
		w.mu.Unlock()
		if h != nil {
			h.Focus()
		}
		return nil
	}
	w.nextChild++
	name := fmt.Sprintf("child-%d", w.nextChild)
	app := newApp(sess)
	app.settings = w.settings
	app.quit = func() {
		if h := w.handle(name); h != nil {
			h.Close()
		}
	}
	w.wins[name] = &winEntry{app: app, parent: parent, key: key}
	w.mu.Unlock()
	w.create(name, app)
	return nil
}

// childLocked returns parent's child entry for key, or nil. w.mu is held.
func (w *Windows) childLocked(parent, key string) *winEntry {
	for _, e := range w.wins {
		if e.parent == parent && e.key == key {
			return e
		}
	}
	return nil
}

// focusChild focuses parent's child window for key and reports whether
// there is one.
func (w *Windows) focusChild(parent, key string) bool {
	w.mu.Lock()
	e := w.childLocked(parent, key)
	var h windowHandle
	if e != nil {
		h = e.handle
	}
	w.mu.Unlock()
	if h != nil {
		h.Focus()
	}
	return e != nil
}

// openEntry opens the directory comparison entry at path, from window
// parent, in a child window, or focuses the one already showing it.
func (w *Windows) openEntry(parent string, app *App, path string) error {
	key := "entry:" + path
	if w.focusChild(parent, key) {
		return nil
	}
	sess, err := app.sess.EntrySession(path)
	if err != nil {
		return err
	}
	return w.openChild(parent, key, sess)
}

// mergeKey is the child-window key of a conflicted file's merge window.
func mergeKey(path string) string { return "merge:" + path }

const mergeKeyPrefix = "merge:"

// openMerge opens the text conflict at path, from Conflicts window parent,
// in a merge window, or focuses the one already open. The Conflicts window
// is told to refresh, so the row shows as locked.
func (w *Windows) openMerge(parent string, app *App, path string) error {
	if w.focusChild(parent, mergeKey(path)) {
		return nil
	}
	sess, err := app.sess.MergeSession(path)
	if err != nil {
		return err
	}
	if err := w.openChild(parent, mergeKey(path), sess); err != nil {
		return err
	}
	w.emit(conflictsChangedEvent(parent))
	return nil
}

// showChanges opens a modify/delete conflict's changes (base against the
// side that kept the file) in a diff window, or focuses the one open.
func (w *Windows) showChanges(parent string, app *App, path string) error {
	key := "changes:" + path
	if w.focusChild(parent, key) {
		return nil
	}
	sess, err := app.sess.ChangesSession(path)
	if err != nil {
		return err
	}
	return w.openChild(parent, key, sess)
}

// openMerges returns the paths parent has open in merge windows, sorted;
// with dirtyOnly, only those whose result has unsaved edits.
func (w *Windows) openMerges(parent string, dirtyOnly bool) []string {
	w.mu.Lock()
	var apps []*App
	var paths []string
	for _, e := range w.wins {
		if e.parent == parent && strings.HasPrefix(e.key, mergeKeyPrefix) {
			apps = append(apps, e.app)
			paths = append(paths, strings.TrimPrefix(e.key, mergeKeyPrefix))
		}
	}
	w.mu.Unlock()
	out := []string{}
	for i, a := range apps {
		if !dirtyOnly || a.isDirty() {
			out = append(out, paths[i])
		}
	}
	sort.Strings(out)
	return out
}

// errMergeOpen is returned when accepting a file whose merge window is open.
var errMergeOpen = errors.New("its merge window is open; save or cancel it first")

// acceptSide resolves paths from window parent's resolve session, unless
// one of them is open in a merge window.
func (w *Windows) acceptSide(parent string, app *App, paths []string, side gitconflict.Side) (session.ConflictList, error) {
	w.mu.Lock()
	for _, p := range paths {
		if w.childLocked(parent, mergeKey(p)) != nil {
			w.mu.Unlock()
			return session.ConflictList{}, fmt.Errorf("%s: %w", p, errMergeOpen)
		}
	}
	w.mu.Unlock()
	return app.AcceptSide(paths, side)
}

// create makes the window for the registered entry name.
func (w *Windows) create(name string, app *App) windowHandle {
	h := w.newWindow(name, app.sess, app)
	w.mu.Lock()
	defer w.mu.Unlock()
	if e, ok := w.wins[name]; ok {
		e.handle = h
	}
	return h
}

// handle returns window name's handle, or nil.
func (w *Windows) handle(name string) windowHandle {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e, ok := w.wins[name]; ok {
		return e.handle
	}
	return nil
}

// byName returns the App of window name.
func (w *Windows) byName(name string) (*App, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e, ok := w.wins[name]
	if !ok {
		return nil, fmt.Errorf("window %s: %w", name, errUnknownWindow)
	}
	return e.app, nil
}

// children returns the windows parent opened. Each of them closes its own
// children in turn when it closes.
func (w *Windows) children(parent string) []windowHandle {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []windowHandle
	for _, e := range w.wins {
		if e.parent == parent && e.handle != nil {
			out = append(out, e.handle)
		}
	}
	return out
}

// closed is called once window name has closed: it forgets the window and
// closes the windows it opened. When the main window closes, the
// application ends.
func (w *Windows) closed(name string) {
	kids := w.children(name)
	w.mu.Lock()
	e := w.wins[name]
	delete(w.wins, name)
	w.mu.Unlock()
	for _, h := range kids {
		h.Close()
	}
	// A merge window closed (saved or cancelled): its row is unlocked and
	// maybe resolved.
	if e != nil && strings.HasPrefix(e.key, mergeKeyPrefix) {
		w.emit(conflictsChangedEvent(e.parent))
	}
	if name == mainWindowName {
		w.quitApp()
	}
}

// errNoWindow is returned for a bound call that did not come from a window.
var errNoWindow = errors.New("call did not come from a window")

// caller returns the App of the window that made the bound call. Wails
// puts the calling window in the context of every call from a webview.
func (w *Windows) caller(ctx context.Context) (*App, error) {
	_, app, err := w.callerName(ctx)
	return app, err
}

// callerName is caller that also returns the calling window's name.
func (w *Windows) callerName(ctx context.Context) (string, *App, error) {
	win, ok := ctx.Value(application.WindowKey).(application.Window)
	if !ok {
		return "", nil, errNoWindow
	}
	app, err := w.byName(win.Name())
	return win.Name(), app, err
}

// stateless serves the bound methods that use no session.
var stateless App

// The exported methods below are bound to the frontend. Each one forwards
// to the App of the calling window; the leading context is supplied by
// Wails and does not appear in the generated bindings.

// LoadSession returns the calling window's session.
func (w *Windows) LoadSession(ctx context.Context) (session.Session, error) {
	app, err := w.caller(ctx)
	if err != nil {
		return session.Session{}, err
	}
	return app.LoadSession()
}

// DirEntries waits for the calling window's directory comparison and
// returns its entries. The frontend shows progress meanwhile.
func (w *Windows) DirEntries(ctx context.Context) ([]dirdiff.Entry, error) {
	app, err := w.caller(ctx)
	if err != nil {
		return nil, err
	}
	return app.DirEntries()
}

// DirStats returns the line counts of the calling window's directory
// comparison computed since the previous call.
func (w *Windows) DirStats(ctx context.Context) (dirdiff.StatsBatch, error) {
	app, err := w.caller(ctx)
	if err != nil {
		return dirdiff.StatsBatch{}, err
	}
	return app.DirStats()
}

// DirProgress reports how far the calling window's directory comparison got.
func (w *Windows) DirProgress(ctx context.Context) (dirdiff.Snapshot, error) {
	app, err := w.caller(ctx)
	if err != nil {
		return dirdiff.Snapshot{}, err
	}
	return app.DirProgress(), nil
}

// OpenEntry opens a file of the calling window's directory comparison
// (path relative, "/" separators) in a diff window of its own, or focuses
// the window already showing it.
func (w *Windows) OpenEntry(ctx context.Context, path string) error {
	name, app, err := w.callerName(ctx)
	if err != nil {
		return err
	}
	return w.openEntry(name, app, path)
}

// Conflicts re-reads the calling window's conflicts (mergediff resolve).
// Open lists the files it has open in merge windows.
func (w *Windows) Conflicts(ctx context.Context) (session.ConflictList, error) {
	name, app, err := w.callerName(ctx)
	if err != nil {
		return session.ConflictList{}, err
	}
	list, err := app.Conflicts()
	list.Open = w.openMerges(name, false)
	return list, err
}

// OpenMerge opens a text conflict of the calling Conflicts window in a
// merge window of its own, or focuses the one already showing it.
func (w *Windows) OpenMerge(ctx context.Context, path string) error {
	name, app, err := w.callerName(ctx)
	if err != nil {
		return err
	}
	return w.openMerge(name, app, path)
}

// ShowChanges opens a modify/delete conflict's changes in a diff window.
func (w *Windows) ShowChanges(ctx context.Context, path string) error {
	name, app, err := w.callerName(ctx)
	if err != nil {
		return err
	}
	return w.showChanges(name, app, path)
}

// UnsavedMerges returns the files the calling window has open in merge
// windows with unsaved edits, so it can ask before closing them.
func (w *Windows) UnsavedMerges(ctx context.Context) ([]string, error) {
	name, _, err := w.callerName(ctx)
	if err != nil {
		return nil, err
	}
	return w.openMerges(name, true), nil
}

// SetDirty records whether the calling merge window's result has unsaved
// edits.
func (w *Windows) SetDirty(ctx context.Context, dirty bool) error {
	app, err := w.caller(ctx)
	if err != nil {
		return err
	}
	app.SetDirty(dirty)
	return nil
}

// AcceptSide resolves the calling window's conflicts at paths by taking
// side ("yours" or "theirs"), and returns the refreshed list. Files open in
// a merge window are refused.
func (w *Windows) AcceptSide(ctx context.Context, paths []string, side gitconflict.Side) (session.ConflictList, error) {
	name, app, err := w.callerName(ctx)
	if err != nil {
		return session.ConflictList{}, err
	}
	list, err := w.acceptSide(name, app, paths, side)
	list.Open = w.openMerges(name, false)
	return list, err
}

// MergeCounts counts the conflicts and non-conflicting changes of a text
// conflict of the calling window, for its side panel.
func (w *Windows) MergeCounts(ctx context.Context, path string) (session.MergeCounts, error) {
	app, err := w.caller(ctx)
	if err != nil {
		return session.MergeCounts{}, err
	}
	return app.MergeCounts(path)
}

// Edited returns those of paths (conflicts of the calling window) whose
// working-tree file was edited since Git stopped.
func (w *Windows) Edited(ctx context.Context, paths []string) ([]string, error) {
	app, err := w.caller(ctx)
	if err != nil {
		return nil, err
	}
	return app.Edited(paths)
}

// Save writes the calling window's merge result.
func (w *Windows) Save(ctx context.Context, result string) error {
	app, err := w.caller(ctx)
	if err != nil {
		return err
	}
	return app.Save(result)
}

// Quit closes the calling window. For the main window the process then
// exits with code; a child window just closes.
func (w *Windows) Quit(ctx context.Context, code int) error {
	app, err := w.caller(ctx)
	if err != nil {
		return err
	}
	app.Quit(code)
	return nil
}

// GuardClose turns the calling window's close guard on or off.
func (w *Windows) GuardClose(ctx context.Context, on bool) error {
	app, err := w.caller(ctx)
	if err != nil {
		return err
	}
	app.GuardClose(on)
	return nil
}

// CloseRequestSeen acknowledges the calling window's close request.
func (w *Windows) CloseRequestSeen(ctx context.Context) error {
	app, err := w.caller(ctx)
	if err != nil {
		return err
	}
	app.CloseRequestSeen()
	return nil
}

// Settings returns the effective settings of the calling window.
func (w *Windows) Settings(ctx context.Context) (map[string]string, error) {
	app, err := w.caller(ctx)
	if err != nil {
		return nil, err
	}
	return app.Settings(), nil
}

// CharDiff returns character-level differences between two texts.
func (w *Windows) CharDiff(left, right string) []diff.Span {
	return stateless.CharDiff(left, right)
}

// Compare diffs two texts for the scratch view.
func (w *Windows) Compare(left, right string) ([]merge.Chunk, error) {
	return stateless.Compare(left, right)
}

// Version returns the build version for the status bar.
func (w *Windows) Version() string {
	return stateless.Version()
}
