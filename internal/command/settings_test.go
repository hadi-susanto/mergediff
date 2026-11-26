package command

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// settingsRun checks and runs "mergediff settings ARGS" in f's env and
// returns the exit code and error (a check error has code ExitError).
func settingsRun(t *testing.T, f *fake, args ...string) (int, error) {
	t.Helper()
	f.reset()
	opt := parse(t, append([]string{"settings"}, args...)...)
	cmd, err := newSettings(opt, opt.Positionals()[1:])
	if err != nil {
		return ExitError, err
	}
	return cmd.Run(f.env)
}

func TestSettingsCommand(t *testing.T) {
	f := newFake(t, 99)
	path := f.configPath

	if code, err := settingsRun(t, f, "path"); code != 0 || err != nil || f.stdout.String() != path+"\n" {
		t.Fatalf("path: %d %v %q", code, err, f.stdout.String())
	}

	code, err := settingsRun(t, f)
	out := f.stdout.String()
	if code != 0 || err != nil || !strings.Contains(out, "Config file: "+path) ||
		!regexp.MustCompile(`(?m)^theme\s+system\s+default\s+system\|light\|dark\s+colour theme`).MatchString(out) ||
		!regexp.MustCompile(`(?m)^window-state\s+normal\s+default\s+normal\|maximized`).MatchString(out) {
		t.Fatalf("list on no file: %d %v\n%s", code, err, out)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("list created the config file")
	}

	if code, err = settingsRun(t, f, "set", "theme", "dark"); code != 0 || err != nil || f.stdout.String() != "theme = dark (file)\n" {
		t.Fatalf("set: %d %v %q", code, err, f.stdout.String())
	}
	if code, err = settingsRun(t, f, "get", "theme"); code != 0 || err != nil || f.stdout.String() != "dark\n" {
		t.Fatalf("get after set: %d %v %q", code, err, f.stdout.String())
	}
	if settingsRun(t, f, "list"); !regexp.MustCompile(`(?m)^theme\s+dark\s+file\s`).MatchString(f.stdout.String()) {
		t.Fatalf("list after set:\n%s", f.stdout.String())
	}
	if code, err = settingsRun(t, f, "unset", "theme"); code != 0 || err != nil || f.stdout.String() != "theme = system (default)\n" {
		t.Fatalf("unset: %d %v %q", code, err, f.stdout.String())
	}
	if f.opened != nil {
		t.Fatal("settings opened a window")
	}
}

func TestSettingsCommandKeepsComments(t *testing.T) {
	f := newFake(t, 99)
	path := f.configPath
	os.MkdirAll(filepath.Dir(path), 0o755)
	const before = "# my mergediff settings\ntheme = \"light\"  # for daylight\n"
	os.WriteFile(path, []byte(before), 0o644)
	if code, err := settingsRun(t, f, "set", "window-state", "maximized"); code != 0 || err != nil {
		t.Fatalf("set: %d %v", code, err)
	}
	if code, err := settingsRun(t, f, "set", "theme", "dark"); code != 0 || err != nil {
		t.Fatalf("set: %d %v", code, err)
	}
	got, _ := os.ReadFile(path)
	want := "# my mergediff settings\ntheme = \"dark\"  # for daylight\nwindow-state = \"maximized\"\n"
	if string(got) != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
}

func TestSettingsCommandErrors(t *testing.T) {
	f := newFake(t, 99)
	path := f.configPath
	os.MkdirAll(filepath.Dir(path), 0o755)
	const before = "# keep\ntheme = \"light\"\n"
	os.WriteFile(path, []byte(before), 0o644)
	tests := []struct {
		args  []string
		msg   string
		usage bool // a usage error with the settings usage
	}{
		{[]string{"set", "theme", "blue"}, `invalid value "blue" for theme (allowed: system, light, dark)`, false},
		{[]string{"set", "colour", "dark"}, `unknown setting "colour" (known: theme, window-state)`, true},
		{[]string{"get", "colour"}, `unknown setting "colour"`, true},
		{[]string{"unset", "colour"}, `unknown setting "colour"`, true},
		{[]string{"set", "theme"}, "settings set takes 2 argument(s), got 1", true},
		{[]string{"get"}, "settings get takes 1 argument(s), got 0", true},
		{[]string{"list", "x"}, "settings list takes 0 argument(s), got 1", true},
		{[]string{"help", "x"}, "settings help takes 0 argument(s), got 1", true},
		{[]string{"frob"}, `unknown settings command "frob"`, true},
		{[]string{"list", "--theme", "dark"}, "--theme is not valid with settings; to save it, use: mergediff settings set theme dark", true},
		{[]string{"--window-state=normal"}, "--window-state is not valid with settings", true},
		{[]string{"path", "-o", "m"}, "-o is not valid with settings", true},
		{[]string{"--auto-apply"}, "--auto-apply is not valid with settings", true},
		{[]string{"-v"}, "--version is not valid with settings", true},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			code, err := settingsRun(t, f, tt.args...)
			if code != ExitError || err == nil || !strings.Contains(err.Error(), tt.msg) {
				t.Fatalf("code %d, err %v, want 2 and %q", code, err, tt.msg)
			}
			if tt.usage {
				if ue := wantUsageError(t, err, tt.msg); ue.Usage != settingsUsage {
					t.Errorf("Usage = %q, want the settings usage", ue.Usage)
				}
			} else if _, ok := err.(*UsageError); ok {
				t.Errorf("%v is a usage error", err)
			}
			if got, _ := os.ReadFile(path); string(got) != before {
				t.Errorf("file changed to %q", got)
			}
		})
	}

	// A broken file is reported with its path by every command that reads it.
	os.WriteFile(path, []byte("theme = 3\n"), 0o644)
	for _, args := range [][]string{{"list"}, {"get", "theme"}, {"set", "window-state", "normal"}} {
		code, err := settingsRun(t, f, args...)
		if code != ExitError || err == nil || !strings.HasPrefix(err.Error(), "config "+path+": ") {
			t.Errorf("%v on a broken file: code %d, err %v", args, code, err)
		}
	}
}

func TestSettingsHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"set", "--help"}, {"--theme", "dark", "-h"}} {
		f := newFake(t, 99)
		code, err := settingsRun(t, f, args...)
		if code != 0 || err != nil || !strings.Contains(f.stdout.String(), "mergediff settings unset KEY") {
			t.Errorf("settings %v: %d %v %q", args, code, err, f.stdout.String())
		}
	}
}
