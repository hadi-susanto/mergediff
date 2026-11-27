package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hadi-susanto/mergediff/internal/command"
	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// fakeEnv is an Env whose window records the session it gets and returns
// code, and whose config file is in a fresh temporary directory.
func fakeEnv(t *testing.T, code int) (command.Env, *bytes.Buffer, *bytes.Buffer, *session.Session) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var opened session.Session
	path := filepath.Join(t.TempDir(), "mergediff", "config.toml")
	env := command.Env{
		Stdout:     &stdout,
		Stderr:     &stderr,
		Getwd:      os.Getwd,
		ConfigPath: func() (string, error) { return path, nil },
		Version:    "2026-11-19 (a1b2c3d)",
		OpenWindow: func(s session.Session, _ config.Config) int { opened = s; return code },
	}
	return env, &stdout, &stderr, &opened
}

// TestRun checks the wiring: parse, select, run, and how errors become
// output and exit codes. The commands themselves are tested in
// internal/command.
func TestRun(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "x\n")
	tests := []struct {
		name     string
		args     []string
		code     int
		stdout   string // exact, unless empty
		stderr   string // prefix
		usage    string // substring of stderr that shows which usage text ("" = none)
		isWindow bool
	}{
		{name: "help", args: []string{"--help"}, code: 0, stdout: command.Usage},
		{name: "version", args: []string{"version"}, code: 0, stdout: "mergediff 2026-11-19 (a1b2c3d)\n"},
		{name: "parse error", args: []string{"a", "b", "--nope"}, code: 2,
			stderr: "mergediff: unknown option --nope\n\nUsage:", usage: "mergediff resolve [--auto-apply]"},
		{name: "select error", args: []string{"a"}, code: 2,
			stderr: "mergediff: expected 2 files or directories", usage: "mergediff resolve [--auto-apply]"},
		{name: "settings usage error", args: []string{"settings", "frob"}, code: 2,
			stderr: "mergediff: unknown settings command \"frob\"\n\nUsage:\n  mergediff settings [list]", usage: "mergediff settings unset KEY"},
		{name: "run error", args: []string{a, filepath.Join(dir, "missing")}, code: 2, stderr: "mergediff: read RIGHT: "},
		{name: "window", args: []string{a, a}, code: 7, isWindow: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, stdout, stderr, opened := fakeEnv(t, 7)
			if code := run(tt.args, env); code != tt.code {
				t.Fatalf("exit code %d, want %d (stderr %q)", code, tt.code, stderr.String())
			}
			if (*opened != nil) != tt.isWindow {
				t.Fatalf("window opened = %v", *opened != nil)
			}
			if tt.stdout != "" && stdout.String() != tt.stdout {
				t.Errorf("stdout %q, want %q", stdout.String(), tt.stdout)
			}
			if !strings.HasPrefix(stderr.String(), tt.stderr) {
				t.Errorf("stderr %q, want it to start with %q", stderr.String(), tt.stderr)
			}
			if got := strings.Contains(stderr.String(), "Usage:"); got != (tt.usage != "") ||
				!strings.Contains(stderr.String(), tt.usage) {
				t.Errorf("stderr %q, want usage %q", stderr.String(), tt.usage)
			}
		})
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
