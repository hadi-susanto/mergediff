package main

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/wailsapp/wails/v3/pkg/application"

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
	}

	var sess *session.Session
	if opts.cmd == cmdMerge {
		sess, err = session.LoadMerge(opts.base, opts.local, opts.remote, opts.output, opts.autoApply)
	} else {
		sess, err = session.LoadDiff(opts.left, opts.right)
	}
	if err != nil {
		fmt.Fprintf(stderr, "mergediff: %v\n", err)
		return exitError
	}
	return startGUI(sess)
}

func runGUI(sess *session.Session) int {
	app := newApp(sess)
	wails := application.New(application.Options{
		Name:        "mergediff",
		Description: "3-way merge and 2-way diff tool",
		Services:    []application.Service{application.NewService(app)},
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

	wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "mergediff — " + sess.Title,
		Width:  1400,
		Height: 900,
		URL:    "/",
	})

	if err := wails.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "mergediff: %v\n", err)
		return exitError
	}
	return app.code()
}
