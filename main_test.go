package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// stubGUI replaces the window with a function that records the session.
// It also points the config file at a fresh temporary directory.
func stubGUI(t *testing.T, code int) **session.Session {
	t.Helper()
	got, _ := stubGUIConfig(t, code)
	return got
}

// stubGUIConfig is stubGUI that also records the settings the window got,
// and returns the config file path.
func stubGUIConfig(t *testing.T, code int) (**session.Session, *config.Config) {
	t.Helper()
	var got *session.Session
	var cfg config.Config
	old, oldPath := startGUI, configPath
	path := filepath.Join(t.TempDir(), "mergediff", "config.toml")
	configPath = func() (string, error) { return path, nil }
	startGUI = func(s *session.Session, c config.Config) int { got, cfg = s, c; return code }
	t.Cleanup(func() { startGUI, configPath = old, oldPath })
	return &got, &cfg
}

func TestSettingsFromConfigAndFlags(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "x\n")
	opened, cfg := stubGUIConfig(t, 0)
	path, _ := configPath()
	run := func(args ...string) (int, string) {
		var stdout, stderr bytes.Buffer
		*opened = nil
		code := run(args, &stdout, &stderr)
		return code, stderr.String()
	}

	if code, _ := run(a, a); code != 0 || cfg.Theme() != config.ThemeSystem || cfg.WindowState() != config.WindowNormal {
		t.Fatalf("no config file: code %d, %v", code, cfg.Values())
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	writeFile(t, filepath.Dir(path), "config.toml", "theme = \"dark\"\nwindow-state = \"maximized\"\n")
	if code, _ := run(a, a); code != 0 || cfg.Theme() != config.ThemeDark || cfg.WindowState() != config.WindowMaximized {
		t.Fatalf("config file: code %d, %v", code, cfg.Values())
	}
	if code, _ := run("--theme", "light", a, a, "--window-state=normal"); code != 0 ||
		cfg.Theme() != config.ThemeLight || cfg.SourceOf(config.KeyTheme) != config.FromFlag ||
		cfg.WindowState() != config.WindowNormal {
		t.Fatalf("flags override the file: code %d, %v", code, cfg.Values())
	}

	writeFile(t, filepath.Dir(path), "config.toml", "theme = \"blue\"\n")
	code, stderr := run(a, a)
	if code != exitError || *opened != nil || !strings.Contains(stderr, "mergediff: config "+path+": ") ||
		strings.Contains(stderr, "Usage:") {
		t.Fatalf("bad config: code %d, opened %v, stderr %q", code, *opened != nil, stderr)
	}
	if code, _ := run("--version"); code != 0 {
		t.Fatalf("--version must not read the config, code %d", code)
	}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunWithoutWindow(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "x\n")
	bin := writeFile(t, dir, "bin", "\x00")
	tests := []struct {
		name       string
		args       []string
		code       int
		stdout     string // substring
		stderr     string // substring
		usageShown bool
	}{
		{name: "help", args: []string{"--help"}, code: 0, stdout: "Usage:"},
		{name: "version", args: []string{"--version"}, code: 0, stdout: "mergediff dev (unknown)\n"},
		{name: "no args", args: nil, code: 2, stderr: "mergediff: missing file arguments", usageShown: true},
		{name: "bad option", args: []string{"a", "b", "--nope"}, code: 2, stderr: "unknown option --nope", usageShown: true},
		{name: "missing file", args: []string{a, filepath.Join(dir, "missing")}, code: 2, stderr: "mergediff: read RIGHT: "},
		{name: "binary file", args: []string{bin, a}, code: 2, stderr: "file looks binary"},
		{name: "merge missing local", args: []string{a, filepath.Join(dir, "nope"), a, "-o", "m"}, code: 2, stderr: "read LOCAL: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opened := stubGUI(t, 99)
			var stdout, stderr bytes.Buffer
			if code := run(tt.args, &stdout, &stderr); code != tt.code {
				t.Fatalf("exit code %d, want %d (stderr %q)", code, tt.code, stderr.String())
			}
			if *opened != nil {
				t.Fatal("window opened")
			}
			if !strings.Contains(stdout.String(), tt.stdout) || !strings.Contains(stderr.String(), tt.stderr) {
				t.Fatalf("stdout %q, stderr %q", stdout.String(), stderr.String())
			}
			if got := strings.Contains(stderr.String(), "Usage:"); got != tt.usageShown {
				t.Fatalf("usage shown = %v, want %v", got, tt.usageShown)
			}
		})
	}
}

func TestRunOpensWindow(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.go", "x\n")
	b := writeFile(t, dir, "b.go", "y\n")

	opened := stubGUI(t, 7)
	var stdout, stderr bytes.Buffer
	if code := run([]string{a, b}, &stdout, &stderr); code != 7 {
		t.Fatalf("exit code %d, want the GUI's code 7", code)
	}
	if s := *opened; s == nil || s.Mode != session.Diff || s.Language != "go" {
		t.Fatalf("diff session = %+v", s)
	}

	out := filepath.Join(dir, "out.go")
	if code := run([]string{"", a, b, "-o", out, "--auto-apply"}, &stdout, &stderr); code != 7 {
		t.Fatalf("merge exit code %d (stderr %q)", code, stderr.String())
	}
	if s := *opened; s.Mode != session.Merge || s.Title != "out.go" {
		t.Fatalf("merge session = %+v", s)
	}
}

func TestAppExitCodes(t *testing.T) {
	diffSess := &session.Session{Mode: session.Diff}
	mergeSess := &session.Session{Mode: session.Merge}
	if c := newApp(diffSess).code(); c != exitOK {
		t.Errorf("diff default code %d, want 0", c)
	}
	if c := newApp(mergeSess).code(); c != exitCancel {
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

func TestReportCrash(t *testing.T) {
	dir := t.TempDir()
	old := crashDir
	crashDir = func() (string, error) { return filepath.Join(dir, "mergediff"), nil }
	t.Cleanup(func() { crashDir = old })

	var stderr bytes.Buffer
	reportCrash(&stderr, "panic", errors.New("boom"), []byte("goroutine 1 [running]:"))
	logs, err := filepath.Glob(filepath.Join(dir, "mergediff", "crash-*.log"))
	if err != nil || len(logs) != 1 {
		t.Fatalf("crash logs = %v, %v", logs, err)
	}
	content, err := os.ReadFile(logs[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"mergediff dev (unknown): panic: boom", "goroutine 1 [running]:"} {
		if !strings.Contains(string(content), want) || !strings.Contains(stderr.String(), want) {
			t.Fatalf("report missing %q\nlog: %s\nstderr: %s", want, content, stderr.String())
		}
	}
	if !strings.Contains(stderr.String(), logs[0]) {
		t.Fatalf("stderr does not name the crash log: %s", stderr.String())
	}

	// An unwritable crash directory is reported, not fatal.
	crashDir = func() (string, error) { return "", errors.New("no cache dir") }
	stderr.Reset()
	reportCrash(&stderr, "panic", "x", nil)
	if !strings.Contains(stderr.String(), "write crash log: no cache dir") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCrashLogName(t *testing.T) {
	dir := t.TempDir()
	old := crashDir
	crashDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { crashDir = old })
	path, err := writeCrashLog("r", time.Date(2026, 11, 20, 9, 8, 7, 123e6, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "crash-20261120-090807.123.log" {
		t.Fatalf("crash log name %q", filepath.Base(path))
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
	app.Quit(exitCancel)
	if app.interceptClose(t0) {
		t.Fatal("after Quit the window must close")
	}
}

func regexpMatch(pattern, s string) bool { return regexp.MustCompile(pattern).MatchString(s) }
