package main

import (
	"sync"

	"github.com/hadi-susanto/mergediff/internal/diff"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// App is the Wails service bound to the frontend. Its exported methods are
// thin adapters over the internal packages.
type App struct {
	sess *session.Session

	mu       sync.Mutex
	exitCode int
	quit     func() // closes the application; set once Wails is created
}

func newApp(sess *session.Session) *App {
	// Closing the window without an explicit Quit: a diff ends normally, a
	// merge counts as cancelled.
	code := exitOK
	if sess.Mode == session.Merge {
		code = exitCancel
	}
	return &App{sess: sess, exitCode: code, quit: func() {}}
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
	quit := a.quit
	a.mu.Unlock()
	quit()
}

// Version returns the build version for the status bar.
func (a *App) Version() string {
	return Version()
}

func (a *App) code() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.exitCode
}
