// Package gui is mergediff's window layer: it runs Wails, keeps one
// session per window (the main window and the child windows it opens) and
// serves the bound methods the frontend calls. It is the only internal
// package that imports Wails. It never exits the process: Run returns the
// exit code, and crashes go to the hooks in Options.
package gui

import (
	"errors"
	"io/fs"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/hadi-susanto/mergediff/internal/command"
	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// Options is what Run needs from main.
type Options struct {
	// Assets is the built frontend (main embeds frontend/dist).
	Assets fs.FS
	// Version is the build version, for the status bar.
	Version string
	// OnPanic is called for a panic in a bound method, and OnFatal for a
	// fatal Wails error. Wails would otherwise recover the panic and keep
	// running, or exit with 1 ("cancelled" to Git); main reports the crash
	// and exits 2.
	OnPanic func(value any, stack []byte)
	OnFatal func(err error)
	// OnError is called for the other errors Wails reports, and when the
	// application cannot run (Run then returns exit code 2). main prints
	// them; gui never prints.
	OnError func(err error)
}

// Run opens the main window showing sess and blocks until the application
// ends, returning the process exit code.
func Run(sess session.Session, cfg config.Config, opts Options) int {
	win := newWindowState(sess)
	win.settings = cfg.Values()
	wins := newWindows()
	wails := application.New(application.Options{
		Name:        "mergediff",
		Description: "3-way merge and 2-way diff tool",
		Services:    []application.Service{application.NewService(wins)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(opts.Assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		PanicHandler: func(d *application.PanicDetails) {
			opts.OnPanic(d.Error, []byte(d.FullStackTrace))
		},
		ErrorHandler: func(err error) {
			var fatal *application.FatalError
			if errors.As(err, &fatal) {
				opts.OnFatal(err)
				return
			}
			opts.OnError(err)
		},
	})
	win.quit = wails.Quit
	wins.quitApp = wails.Quit
	wins.settings = win.settings
	wins.version = opts.Version
	wins.emit = func(event string) { wails.Event.Emit(event) }
	wins.newWindow = func(name string, sess session.Session, win *window) windowHandle {
		wopts := windowOptions(sess, cfg)
		wopts.Name = name
		wv := wails.Window.NewWithOptions(wopts)
		// In merge mode the frontend guards the close button, so unsaved
		// edits are confirmed the same way as Cancel (see window.GuardClose).
		wv.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			if win.interceptClose(time.Now()) {
				e.Cancel()
				wails.Event.Emit(closeRequestedEvent(name))
			}
		})
		// Runs only when no hook cancelled the close.
		wv.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
			wins.closed(name)
		})
		return wv
	}
	wins.openMain(win)

	if err := wails.Run(); err != nil {
		opts.OnError(err)
		return command.ExitError
	}
	return win.code()
}
