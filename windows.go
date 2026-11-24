package main

import (
	"errors"
	"fmt"
	"sync"
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
