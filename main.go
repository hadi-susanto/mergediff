package main

import (
	"cmp"
	"embed"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime/debug"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/command"
	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// The built frontend. frontend/dist/.gitkeep keeps this compiling before the
// first frontend build.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	defer func() {
		if r := recover(); r != nil {
			reportCrash(os.Stderr, "panic", r, debug.Stack())
			os.Exit(command.ExitError)
		}
	}()
	os.Exit(run(devArgs(os.Args[1:]), command.Env{
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Getwd:      os.Getwd,
		ConfigPath: config.Path,
		Version:    Version(),
		OpenWindow: runGUI,
	}))
}

// run parses the command line, selects the command and runs it, returning
// the process exit code. Bad arguments and unreadable files are reported
// before any window opens. Tests call it with a fake Env.
func run(args []string, env command.Env) int {
	opt, err := cliargs.Parse(args)
	if err != nil {
		return report(env.Stderr, err)
	}
	cmd, err := command.Select(opt)
	if err != nil {
		return report(env.Stderr, err)
	}
	code, err := cmd.Run(env)
	if err != nil {
		return report(env.Stderr, err)
	}
	return code
}

// report is the only place errors become output and exit code 2: a usage
// error with the usage text, any other error alone.
func report(w io.Writer, err error) int {
	var cu *command.UsageError
	var pu *cliargs.UsageError
	switch {
	case errors.As(err, &cu):
		fmt.Fprintf(w, "mergediff: %s\n\n%s", cu.Msg, cmp.Or(cu.Usage, command.Usage))
	case errors.As(err, &pu):
		fmt.Fprintf(w, "mergediff: %v\n\n%s", err, command.Usage)
	default:
		fmt.Fprintf(w, "mergediff: %v\n", err)
	}
	return command.ExitError
}

// closeRequestedEvent names the event that tells a window's frontend its
// close button was used. Events reach every window, so the name carries
// the window's name and each window listens only for its own.
func closeRequestedEvent(window string) string {
	return "mergediff:close-requested:" + window
}

// mainWindowName names the window opened from the command line.
const mainWindowName = "main"

func runGUI(sess *session.Session, cfg config.Config) int {
	app := newApp(sess)
	app.settings = cfg.Values()
	wins := newWindows()
	wails := application.New(application.Options{
		Name:        "mergediff",
		Description: "3-way merge and 2-way diff tool",
		Services:    []application.Service{application.NewService(wins)},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// Wails recovers panics in bound methods and would keep running;
		// report them and end the program instead (see CLAUDE.md).
		PanicHandler: func(d *application.PanicDetails) {
			reportCrash(os.Stderr, "panic in bound method", d.Error, []byte(d.FullStackTrace))
			os.Exit(command.ExitError)
		},
		// Wails exits with 1 after a fatal error, which would read as
		// "cancelled" to Git; exit with 2 first.
		ErrorHandler: func(err error) {
			var fatal *application.FatalError
			if errors.As(err, &fatal) {
				reportCrash(os.Stderr, "fatal error", err, debug.Stack())
				os.Exit(command.ExitError)
			}
			fmt.Fprintf(os.Stderr, "mergediff: %v\n", err)
		},
	})
	app.quit = wails.Quit
	wins.quitApp = wails.Quit
	wins.settings = app.settings
	wins.emit = func(event string) { wails.Event.Emit(event) }
	wins.newWindow = func(name string, sess *session.Session, app *App) windowHandle {
		opts := windowOptions(sess, cfg)
		opts.Name = name
		window := wails.Window.NewWithOptions(opts)
		// In merge mode the frontend guards the close button, so unsaved
		// edits are confirmed the same way as Cancel (see App.GuardClose).
		window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			if app.interceptClose(time.Now()) {
				e.Cancel()
				wails.Event.Emit(closeRequestedEvent(name))
			}
		})
		// Runs only when no hook cancelled the close.
		window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
			wins.closed(name)
		})
		return window
	}
	wins.openMain(app)

	if err := wails.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mergediff: %v\n", err)
		return command.ExitError
	}
	return app.code()
}

// windowOptions configures a window from its session and the settings.
func windowOptions(sess *session.Session, cfg config.Config) application.WebviewWindowOptions {
	opts := application.WebviewWindowOptions{
		Title:  "mergediff — " + sess.Title,
		Width:  1400,
		Height: 900,
		// The theme is in the URL so index.html can apply it before the
		// first paint; asking the backend would arrive a frame too late.
		URL: "/?theme=" + url.QueryEscape(cfg.Theme()),
	}
	if cfg.WindowState() == config.WindowMaximized {
		opts.StartState = application.WindowStateMaximised
	}
	// The page applies the theme itself (data-theme); these make the native
	// title bar and the first paint match. Linux has no such option: the
	// GTK title bar follows the system theme.
	switch cfg.Theme() {
	case config.ThemeDark:
		opts.Windows.Theme = application.Dark
		opts.Mac.Appearance = application.NSAppearanceNameDarkAqua
		opts.BackgroundColour = application.NewRGB(0x26, 0x26, 0x24) // --chrome-bg, dark
	case config.ThemeLight:
		opts.Windows.Theme = application.Light
		opts.Mac.Appearance = application.NSAppearanceNameAqua
		opts.BackgroundColour = application.NewRGB(0xf4, 0xf3, 0xef) // --chrome-bg, light
	}
	return opts
}
