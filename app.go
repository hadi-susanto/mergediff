package main

import (
	"sync"
	"time"

	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/diff"
	"github.com/hadi-susanto/mergediff/internal/dirdiff"
	"github.com/hadi-susanto/mergediff/internal/gitconflict"
	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// App is the state of one window: its session, exit code and close guard.
// Its exported methods are thin adapters over the internal packages; the
// frontend reaches them through the bound Windows service (windows.go),
// which forwards each call to the calling window's App.
type App struct {
	sess     *session.Session
	settings map[string]string // effective settings, keyed by config key

	mu       sync.Mutex
	exitCode int
	quit     func() // closes the application; set once Wails is created

	// Close guard (see AllowClose): while on, the window's close button asks
	// the frontend instead of closing. closeAsked is when a request was sent
	// that the frontend has not acknowledged yet (zero if none).
	guarded    bool
	closeAsked time.Time
}

// closeAckTimeout is how long an unacknowledged close request blocks the
// next one. After it, closing works even if the frontend has stopped
// responding, so the window can never get stuck.
const closeAckTimeout = 2 * time.Second

func newApp(sess *session.Session) *App {
	// Closing the window without an explicit Quit: a diff ends normally, a
	// merge counts as cancelled.
	code := exitOK
	if sess.Mode == session.Merge {
		code = exitCancel
	}
	return &App{sess: sess, exitCode: code, quit: func() {}}
}

// Settings returns the effective settings (config file and command-line
// flags), keyed by setting key, e.g. "theme": "dark".
func (a *App) Settings() map[string]string {
	if a.settings == nil {
		return config.Defaults().Values()
	}
	return a.settings
}

// Compare diffs two texts for the scratch view (mergediff without
// arguments). The chunks are like a diff session's.
func (a *App) Compare(left, right string) ([]merge.Chunk, error) {
	return session.Compare(left, right)
}

// LoadSession returns the files, chunks and settings loaded at startup.
func (a *App) LoadSession() (session.Session, error) {
	return *a.sess, nil
}

// CharDiff returns character-level differences between two texts.
func (a *App) CharDiff(left, right string) []diff.Span {
	spans := diff.Chars(left, right)
	if spans == nil {
		return []diff.Span{}
	}
	return spans
}

// DirEntries waits for the directory comparison and returns its entries.
func (a *App) DirEntries() ([]dirdiff.Entry, error) {
	entries, err := a.sess.DirEntries()
	if err == nil && entries == nil {
		entries = []dirdiff.Entry{}
	}
	return entries, err
}

// DirStats returns the line counts computed since the previous call.
func (a *App) DirStats() (dirdiff.StatsBatch, error) {
	return a.sess.DirStats()
}

// DirProgress reports how far the directory comparison got.
func (a *App) DirProgress() dirdiff.Snapshot {
	return a.sess.DirProgress()
}

// Conflicts re-reads the conflicts of a resolve session.
func (a *App) Conflicts() (session.ConflictList, error) {
	return a.sess.Conflicts()
}

// AcceptSide resolves paths by taking side's version.
func (a *App) AcceptSide(paths []string, side gitconflict.Side) (session.ConflictList, error) {
	return a.sess.Accept(paths, side)
}

// MergeCounts counts a text conflict's conflicts and non-conflicting changes.
func (a *App) MergeCounts(path string) (session.MergeCounts, error) {
	return a.sess.MergeCounts(path)
}

// Save writes the merge result to MERGED with the original line endings.
func (a *App) Save(result string) error {
	return a.sess.Save(result)
}

// Quit closes the window; the process then exits with code. Codes other
// than 0, 1 and 2 are treated as 2.
func (a *App) Quit(code int) {
	if code != exitOK && code != exitCancel {
		code = exitError
	}
	a.mu.Lock()
	a.exitCode = code
	a.guarded = false
	quit := a.quit
	a.mu.Unlock()
	quit()
}

// GuardClose turns the close guard on or off. The frontend turns it on in
// merge mode once it listens for close requests.
func (a *App) GuardClose(on bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.guarded = on
	a.closeAsked = time.Time{}
}

// CloseRequestSeen acknowledges a close request: the frontend is handling it.
func (a *App) CloseRequestSeen() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closeAsked = time.Time{}
}

// interceptClose decides what the window's close button does at now: false
// lets the window close; true means "cancel the close and ask the
// frontend". An unacknowledged request older than closeAckTimeout lets the
// next close through.
func (a *App) interceptClose(now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.guarded {
		return false
	}
	if !a.closeAsked.IsZero() && now.Sub(a.closeAsked) >= closeAckTimeout {
		return false
	}
	if a.closeAsked.IsZero() {
		a.closeAsked = now
	}
	return true
}

// Version returns the build version for the status bar.
func (a *App) Version() string {
	return Version()
}

// code is the exit code once the window has closed. A resolve session
// asks Git: 0 if no conflicts are left within its paths, else 1.
func (a *App) code() int {
	a.mu.Lock()
	code := a.exitCode
	a.mu.Unlock()
	if a.sess.Mode != session.Resolve || code == exitError {
		return code
	}
	n, err := a.sess.Remaining()
	switch {
	case err != nil:
		return exitError
	case n > 0:
		return exitCancel
	default:
		return exitOK
	}
}
