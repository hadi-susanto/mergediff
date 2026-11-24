package main

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/hadi-susanto/mergediff/internal/diff"
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
}

func newWindows() *Windows {
	return &Windows{
		wins:      map[string]*winEntry{},
		newWindow: func(name string, _ *session.Session, _ *App) windowHandle { return nopWindow(name) },
		quitApp:   func() {},
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
	for _, e := range w.wins {
		if e.parent == parent && e.key == key {
			h := e.handle
			w.mu.Unlock()
			if h != nil {
				h.Focus()
			}
			return nil
		}
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
	delete(w.wins, name)
	w.mu.Unlock()
	for _, h := range kids {
		h.Close()
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
	win, ok := ctx.Value(application.WindowKey).(application.Window)
	if !ok {
		return nil, errNoWindow
	}
	return w.byName(win.Name())
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
