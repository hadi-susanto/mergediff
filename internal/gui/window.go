package gui

import (
	"sync"
	"time"

	"github.com/hadi-susanto/mergediff/internal/command"
	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// window is the state of one window: its session, settings, exit code,
// close guard and unsaved-edits flag. The bound methods (api.go) find the
// calling window's state in the registry and use its session directly.
type window struct {
	sess     *session.Session
	settings map[string]string // effective settings, keyed by config key

	mu       sync.Mutex
	exitCode int
	quit     func() // closes the application; set once Wails is created

	// Close guard (see GuardClose): while on, the window's close button asks
	// the frontend instead of closing. closeAsked is when a request was sent
	// that the frontend has not acknowledged yet (zero if none).
	guarded    bool
	closeAsked time.Time
	// dirty: the merge result has unsaved edits (SetDirty).
	dirty bool
}

// closeAckTimeout is how long an unacknowledged close request blocks the
// next one. After it, closing works even if the frontend has stopped
// responding, so the window can never get stuck.
const closeAckTimeout = 2 * time.Second

func newWindowState(sess *session.Session) *window {
	// Closing the window without an explicit Quit: a diff ends normally, a
	// merge counts as cancelled.
	code := command.ExitOK
	if sess.Mode == session.Merge {
		code = command.ExitCancel
	}
	return &window{sess: sess, exitCode: code, quit: func() {}}
}

// settingValues returns the effective settings (config file and
// command-line flags), keyed by setting key, e.g. "theme": "dark".
func (win *window) settingValues() map[string]string {
	if win.settings == nil {
		return config.Defaults().Values()
	}
	return win.settings
}

// Quit closes the window; the process then exits with code. Codes other
// than 0, 1 and 2 are treated as 2.
func (win *window) Quit(code int) {
	if code != command.ExitOK && code != command.ExitCancel {
		code = command.ExitError
	}
	win.mu.Lock()
	win.exitCode = code
	win.guarded = false
	quit := win.quit
	win.mu.Unlock()
	quit()
}

// SetDirty records whether the merge result has unsaved edits.
func (win *window) SetDirty(dirty bool) {
	win.mu.Lock()
	defer win.mu.Unlock()
	win.dirty = dirty
}

func (win *window) isDirty() bool {
	win.mu.Lock()
	defer win.mu.Unlock()
	return win.dirty
}

// GuardClose turns the close guard on or off. The frontend turns it on in
// merge mode once it listens for close requests.
func (win *window) GuardClose(on bool) {
	win.mu.Lock()
	defer win.mu.Unlock()
	win.guarded = on
	win.closeAsked = time.Time{}
}

// CloseRequestSeen acknowledges a close request: the frontend is handling it.
func (win *window) CloseRequestSeen() {
	win.mu.Lock()
	defer win.mu.Unlock()
	win.closeAsked = time.Time{}
}

// interceptClose decides what the window's close button does at now: false
// lets the window close; true means "cancel the close and ask the
// frontend". An unacknowledged request older than closeAckTimeout lets the
// next close through.
func (win *window) interceptClose(now time.Time) bool {
	win.mu.Lock()
	defer win.mu.Unlock()
	if !win.guarded {
		return false
	}
	if !win.closeAsked.IsZero() && now.Sub(win.closeAsked) >= closeAckTimeout {
		return false
	}
	if win.closeAsked.IsZero() {
		win.closeAsked = now
	}
	return true
}

// code is the exit code once the window has closed. A resolve session
// asks Git: 0 if no conflicts are left within its paths, else 1.
func (win *window) code() int {
	win.mu.Lock()
	code := win.exitCode
	win.mu.Unlock()
	if win.sess.Mode != session.Resolve || code == command.ExitError {
		return code
	}
	n, err := win.sess.Remaining()
	switch {
	case err != nil:
		return command.ExitError
	case n > 0:
		return command.ExitCancel
	default:
		return command.ExitOK
	}
}
