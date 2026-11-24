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
	ID() uint
	Name() string
	Close()
	Focus()
}

// winEntry is one open window: its session's App, its handle and, for a
// child window, the window that opened it and what it shows (key).
type winEntry struct {
	app       *App
	handle    windowHandle
	parent    uint
	hasParent bool
	key       string
}

// errUnknownWindow is returned for a window the registry does not know,
// e.g. a call that arrives after its window closed.
var errUnknownWindow = errors.New("unknown window")

// Windows keeps one App (session, exit code, close guard) per window. The
// main window, opened from the command line, decides the process exit
// code; child windows close themselves and never end the process.
type Windows struct {
	mu      sync.Mutex
	wins    map[uint]*winEntry
	mainID  uint
	hasMain bool
}

func newWindows() *Windows {
	return &Windows{wins: map[uint]*winEntry{}}
}

// registerMain records the main window and its App.
func (w *Windows) registerMain(h windowHandle, app *App) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wins[h.ID()] = &winEntry{app: app, handle: h}
	w.mainID, w.hasMain = h.ID(), true
}

// registerChild records a window opened by parent to show key.
func (w *Windows) registerChild(h windowHandle, app *App, parent uint, key string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.wins[parent]; !ok {
		return fmt.Errorf("open window for %s: parent %d: %w", key, parent, errUnknownWindow)
	}
	w.wins[h.ID()] = &winEntry{app: app, handle: h, parent: parent, hasParent: true, key: key}
	return nil
}

// byID returns the App of window id.
func (w *Windows) byID(id uint) (*App, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e, ok := w.wins[id]
	if !ok {
		return nil, fmt.Errorf("window %d: %w", id, errUnknownWindow)
	}
	return e.app, nil
}

// isMain reports whether id is the main window.
func (w *Windows) isMain(id uint) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.hasMain && w.mainID == id
}

// child returns parent's open child window showing key, if any.
func (w *Windows) child(parent uint, key string) (windowHandle, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, e := range w.wins {
		if e.hasParent && e.parent == parent && e.key == key {
			return e.handle, true
		}
	}
	return nil, false
}

// children returns the windows parent opened, directly or through them.
func (w *Windows) children(parent uint) []windowHandle {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []windowHandle
	queue := []uint{parent}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for id, e := range w.wins {
			if e.hasParent && e.parent == p {
				out = append(out, e.handle)
				queue = append(queue, id)
			}
		}
	}
	return out
}

// remove forgets window id once it has closed.
func (w *Windows) remove(id uint) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.wins, id)
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
	return w.byID(win.ID())
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
