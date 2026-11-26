package gui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hadi-susanto/mergediff/internal/command"
	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAppExitCodes(t *testing.T) {
	diffSess := &session.Session{Mode: session.Diff}
	mergeSess := &session.Session{Mode: session.Merge}
	if c := newApp(diffSess).code(); c != command.ExitOK {
		t.Errorf("diff default code %d, want 0", c)
	}
	if c := newApp(mergeSess).code(); c != command.ExitCancel {
		t.Errorf("merge default code %d, want 1", c)
	}
	for in, want := range map[int]int{0: 0, 1: 1, 2: 2, 5: 2, -1: 2} {
		app := newApp(mergeSess)
		quits := 0
		app.quit = func() { quits++ }
		app.Quit(in)
		if app.code() != want || quits != 1 {
			t.Errorf("Quit(%d): code %d quits %d, want code %d and one quit", in, app.code(), quits, want)
		}
	}
}

func TestAppCharDiffNeverNil(t *testing.T) {
	app := newApp(&session.Session{Mode: session.Diff})
	if spans := app.CharDiff("same", "same"); spans == nil || len(spans) != 0 {
		t.Fatalf("CharDiff of equal texts = %#v, want empty non-nil", spans)
	}
	if spans := app.CharDiff("a", "b"); len(spans) != 2 {
		t.Fatalf("CharDiff(a, b) = %v", spans)
	}
}

func TestCloseGuard(t *testing.T) {
	app := newApp(&session.Session{Mode: session.Merge})
	t0 := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)

	if app.interceptClose(t0) {
		t.Fatal("without the guard the window must close")
	}

	app.GuardClose(true)
	if !app.interceptClose(t0) {
		t.Fatal("guarded: the first close must be intercepted")
	}
	// The frontend has not acknowledged yet: a quick second click is still
	// intercepted (the confirmation may be on its way)...
	if !app.interceptClose(t0.Add(closeAckTimeout - time.Millisecond)) {
		t.Fatal("unacknowledged, within the timeout: still intercepted")
	}
	// ...but after the timeout the window closes, so it can never get stuck.
	if app.interceptClose(t0.Add(closeAckTimeout)) {
		t.Fatal("unacknowledged after the timeout: must close")
	}

	// Acknowledged requests keep the guard working indefinitely.
	app.GuardClose(true)
	for i := 0; i < 3; i++ {
		now := t0.Add(time.Duration(i) * time.Minute)
		if !app.interceptClose(now) {
			t.Fatalf("request %d: acknowledged guard must intercept", i)
		}
		app.CloseRequestSeen()
	}

	// Quit turns the guard off.
	app.quit = func() {}
	app.Quit(command.ExitCancel)
	if app.interceptClose(t0) {
		t.Fatal("after Quit the window must close")
	}
}

func TestAppSettings(t *testing.T) {
	app := newApp(&session.Session{Mode: session.Diff})
	if got := app.Settings(); got[config.KeyTheme] != config.ThemeSystem || got[config.KeyWindowState] != config.WindowNormal {
		t.Errorf("defaults: %v", got)
	}
	app.settings = map[string]string{config.KeyTheme: config.ThemeDark}
	if got := app.Settings(); got[config.KeyTheme] != config.ThemeDark {
		t.Errorf("set: %v", got)
	}
}

func TestAppCompare(t *testing.T) {
	app := newApp(session.NewScratch())
	chunks, err := app.Compare("a\n", "b\n")
	if err != nil || len(chunks) != 1 || chunks[0].Kind != "remote" {
		t.Fatalf("Compare = %+v, %v", chunks, err)
	}
	if c := app.code(); c != command.ExitOK {
		t.Errorf("scratch default exit code %d, want 0", c)
	}
}
