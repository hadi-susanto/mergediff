package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// settingsRun runs "mergediff settings ARGS" against a config file in a
// temporary directory and returns the exit code, stdout and stderr.
func settingsRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"settings"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestSettingsCommand(t *testing.T) {
	opened := stubGUI(t, 99)
	path, _ := configPath()

	code, out, _ := settingsRun(t, "path")
	if code != 0 || out != path+"\n" {
		t.Fatalf("path: %d %q", code, out)
	}

	code, out, _ = settingsRun(t)
	if code != 0 || !strings.Contains(out, "Config file: "+path) ||
		!regexpMatch(`(?m)^theme\s+system\s+default\s+system\|light\|dark\s+colour theme`, out) ||
		!regexpMatch(`(?m)^window-state\s+normal\s+default\s+normal\|maximized`, out) {
		t.Fatalf("list on no file: %d\n%s", code, out)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("list created the config file")
	}

	if code, out, _ = settingsRun(t, "set", "theme", "dark"); code != 0 || out != "theme = dark (file)\n" {
		t.Fatalf("set: %d %q", code, out)
	}
	if code, out, _ = settingsRun(t, "get", "theme"); code != 0 || out != "dark\n" {
		t.Fatalf("get after set: %d %q", code, out)
	}
	if code, out, _ = settingsRun(t, "list"); !regexpMatch(`(?m)^theme\s+dark\s+file\s`, out) {
		t.Fatalf("list after set: %d\n%s", code, out)
	}
	if code, out, _ = settingsRun(t, "unset", "theme"); code != 0 || out != "theme = system (default)\n" {
		t.Fatalf("unset: %d %q", code, out)
	}
	if *opened != nil {
		t.Fatal("settings opened a window")
	}
}

func TestSettingsCommandKeepsComments(t *testing.T) {
	stubGUI(t, 99)
	path, _ := configPath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	const before = "# my mergediff settings\ntheme = \"light\"  # for daylight\n"
	os.WriteFile(path, []byte(before), 0o644)
	if code, _, stderr := settingsRun(t, "set", "window-state", "maximized"); code != 0 {
		t.Fatalf("set: %d %s", code, stderr)
	}
	if code, _, stderr := settingsRun(t, "set", "theme", "dark"); code != 0 {
		t.Fatalf("set: %d %s", code, stderr)
	}
	got, _ := os.ReadFile(path)
	want := "# my mergediff settings\ntheme = \"dark\"  # for daylight\nwindow-state = \"maximized\"\n"
	if string(got) != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestSettingsCommandErrors(t *testing.T) {
	stubGUI(t, 99)
	path, _ := configPath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	const before = "# keep\ntheme = \"light\"\n"
	os.WriteFile(path, []byte(before), 0o644)
	tests := []struct {
		args   []string
		stderr string
		usage  bool
	}{
		{[]string{"set", "theme", "blue"}, `mergediff: invalid value "blue" for theme (allowed: system, light, dark)`, false},
		{[]string{"set", "colour", "dark"}, `unknown setting "colour" (known: theme, window-state)`, true},
		{[]string{"get", "colour"}, `unknown setting "colour"`, true},
		{[]string{"unset", "colour"}, `unknown setting "colour"`, true},
		{[]string{"set", "theme"}, "settings set takes 2 argument(s), got 1", true},
		{[]string{"get"}, "settings get takes 1 argument(s), got 0", true},
		{[]string{"list", "x"}, "settings list takes 0 argument(s), got 1", true},
		{[]string{"frob"}, `unknown settings command "frob"`, true},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			code, _, stderr := settingsRun(t, tt.args...)
			if code != exitError || !strings.Contains(stderr, tt.stderr) {
				t.Fatalf("code %d, stderr %q, want 2 and %q", code, stderr, tt.stderr)
			}
			if got := strings.Contains(stderr, "mergediff settings set KEY VALUE"); got != tt.usage {
				t.Errorf("usage shown = %v, want %v", got, tt.usage)
			}
			if got, _ := os.ReadFile(path); string(got) != before {
				t.Errorf("file changed to %q", got)
			}
		})
	}

	// A broken file is reported with its path by every command that reads it.
	os.WriteFile(path, []byte("theme = 3\n"), 0o644)
	for _, args := range [][]string{{"list"}, {"get", "theme"}, {"set", "window-state", "normal"}} {
		code, _, stderr := settingsRun(t, args...)
		if code != exitError || !strings.Contains(stderr, "mergediff: config "+path+": ") {
			t.Errorf("%v on a broken file: code %d, stderr %q", args, code, stderr)
		}
	}
}

func TestSettingsHelp(t *testing.T) {
	for _, arg := range []string{"help", "--help", "-h"} {
		code, out, _ := settingsRun(t, arg)
		if code != 0 || !strings.Contains(out, "mergediff settings unset KEY") {
			t.Errorf("settings %s: %d %q", arg, code, out)
		}
	}
}
