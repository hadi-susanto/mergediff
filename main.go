package main

import (
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
			os.Exit(exitError)
		}
	}()
	os.Exit(run(devArgs(os.Args[1:]), os.Stdout, os.Stderr))
}

// startGUI opens the window and blocks until it closes, returning the exit
// code. It is a variable so tests can run everything up to the window.
var startGUI = runGUI

// run handles the command line and returns the process exit code. Bad
// arguments and unreadable files are reported before any window opens.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseArgs(args)
	if err != nil {
		printUsageError(stderr, err)
		return exitError
	}
	switch opts.cmd {
	case cmdHelp:
		fmt.Fprint(stdout, usage)
		return exitOK
	case cmdVersion:
		fmt.Fprintf(stdout, "mergediff %s\n", Version())
		return exitOK
	case cmdSettings:
		return runSettings(opts.settingsArgs, stdout, stderr)
	}

	var sess *session.Session
	switch opts.cmd {
	case cmdMerge:
		sess, err = session.LoadMerge(opts.base, opts.local, opts.remote, opts.output, opts.autoApply)
	case cmdDiff:
		var dirs bool
		if dirs, err = bothDirs(opts.left, opts.right); err != nil {
			printUsageError(stderr, err)
			return exitError
		}
		if dirs {
			sess, err = session.LoadDirDiff(opts.left, opts.right)
		} else {
			sess, err = session.LoadDiff(opts.left, opts.right)
		}
	case cmdScratch:
		sess = session.NewScratch()
	}
	if err != nil {
		fmt.Fprintf(stderr, "mergediff: %v\n", err)
		return exitError
	}
	cfg, err := loadSettings(opts.settings)
	if err != nil {
		fmt.Fprintf(stderr, "mergediff: %v\n", err)
		return exitError
	}
	return startGUI(sess, cfg)
}

// bothDirs reports whether LEFT and RIGHT are both directories (a
// directory diff). One directory and one file is a usage error. A path
// that cannot be read counts as a file, so loading it reports the error.
func bothDirs(left, right string) (bool, error) {
	l, r := isDir(left), isDir(right)
	switch {
	case l && !r:
		return false, usagef("LEFT %s is a directory but RIGHT %s is a file; compare two files or two directories", left, right)
	case r && !l:
		return false, usagef("LEFT %s is a file but RIGHT %s is a directory; compare two files or two directories", left, right)
	}
	return l && r, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// configPath locates the config file; a variable so tests can redirect it.
var configPath = config.Path

// loadSettings returns the config file's settings overridden by the
// command-line flags. Without a config directory (e.g. no home directory)
// the defaults apply: settings are optional and must not block a merge.
func loadSettings(flags []setting) (config.Config, error) {
	cfg := config.Defaults()
	if path, err := configPath(); err == nil {
		if cfg, err = config.Load(path); err != nil {
			return config.Config{}, err
		}
	}
	for _, f := range flags {
		if err := cfg.Override(f.key, f.value, config.FromFlag); err != nil {
			return config.Config{}, err // parseArgs already validated; a registry bug
		}
	}
	return cfg, nil
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
			os.Exit(exitError)
		},
		// Wails exits with 1 after a fatal error, which would read as
		// "cancelled" to Git; exit with 2 first.
		ErrorHandler: func(err error) {
			var fatal *application.FatalError
			if errors.As(err, &fatal) {
				reportCrash(os.Stderr, "fatal error", err, debug.Stack())
				os.Exit(exitError)
			}
			fmt.Fprintf(os.Stderr, "mergediff: %v\n", err)
		},
	})
	app.quit = wails.Quit
	wins.quitApp = wails.Quit
	wins.settings = app.settings
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
		return exitError
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
