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

func TestWindowExitCodes(t *testing.T) {
	diffSess := &session.Session{View: session.View{Mode: session.Diff}}
	mergeSess := &session.Session{View: session.View{Mode: session.Merge}}
	if c := newWindowState(diffSess).code(); c != command.ExitOK {
		t.Errorf("diff default code %d, want 0", c)
	}
	if c := newWindowState(mergeSess).code(); c != command.ExitCancel {
		t.Errorf("merge default code %d, want 1", c)
	}
	for in, want := range map[int]int{0: 0, 1: 1, 2: 2, 5: 2, -1: 2} {
		win := newWindowState(mergeSess)
		quits := 0
		win.quit = func() { quits++ }
		win.Quit(in)
		if win.code() != want || quits != 1 {
			t.Errorf("Quit(%d): code %d quits %d, want code %d and one quit", in, win.code(), quits, want)
		}
	}
}

func TestCharDiffNeverNil(t *testing.T) {
	w := newWindows()
	if spans := w.CharDiff("same", "same"); spans == nil || len(spans) != 0 {
		t.Fatalf("CharDiff of equal texts = %#v, want empty non-nil", spans)
	}
	if spans := w.CharDiff("a", "b"); len(spans) != 2 {
		t.Fatalf("CharDiff(a, b) = %v", spans)
	}
}

func TestCloseGuard(t *testing.T) {
	win := newWindowState(&session.Session{View: session.View{Mode: session.Merge}})
	t0 := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)

	if win.interceptClose(t0) {
		t.Fatal("without the guard the window must close")
	}

	win.GuardClose(true)
	if !win.interceptClose(t0) {
		t.Fatal("guarded: the first close must be intercepted")
	}
	// The frontend has not acknowledged yet: a quick second click is still
	// intercepted (the confirmation may be on its way)...
	if !win.interceptClose(t0.Add(closeAckTimeout - time.Millisecond)) {
		t.Fatal("unacknowledged, within the timeout: still intercepted")
	}
	// ...but after the timeout the window closes, so it can never get stuck.
	if win.interceptClose(t0.Add(closeAckTimeout)) {
		t.Fatal("unacknowledged after the timeout: must close")
	}

	// Acknowledged requests keep the guard working indefinitely.
	win.GuardClose(true)
	for i := 0; i < 3; i++ {
		now := t0.Add(time.Duration(i) * time.Minute)
		if !win.interceptClose(now) {
			t.Fatalf("request %d: acknowledged guard must intercept", i)
		}
		win.CloseRequestSeen()
	}

	// Quit turns the guard off.
	win.quit = func() {}
	win.Quit(command.ExitCancel)
	if win.interceptClose(t0) {
		t.Fatal("after Quit the window must close")
	}
}

func TestWindowSettings(t *testing.T) {
	win := newWindowState(&session.Session{View: session.View{Mode: session.Diff}})
	if got := win.settingValues(); got[config.KeyTheme] != config.ThemeSystem || got[config.KeyWindowState] != config.WindowNormal {
		t.Errorf("defaults: %v", got)
	}
	win.settings = map[string]string{config.KeyTheme: config.ThemeDark}
	if got := win.settingValues(); got[config.KeyTheme] != config.ThemeDark {
		t.Errorf("set: %v", got)
	}
}

func TestCompare(t *testing.T) {
	win := newWindowState(session.NewScratch())
	chunks, err := newWindows().Compare("a\n", "b\n")
	if err != nil || len(chunks) != 1 || chunks[0].Kind != "remote" {
		t.Fatalf("Compare = %+v, %v", chunks, err)
	}
	if c := win.code(); c != command.ExitOK {
		t.Errorf("scratch default exit code %d, want 0", c)
	}
}
