package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRegistryIsConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range Registry {
		if s.Key == "" || s.Doc == "" {
			t.Errorf("setting %+v needs a key and a doc", s)
		}
		if seen[s.Key] {
			t.Errorf("duplicate key %q", s.Key)
		}
		seen[s.Key] = true
		if !slices.Contains(s.Values, s.Default) {
			t.Errorf("%s: default %q is not an allowed value %v", s.Key, s.Default, s.Values)
		}
		if strings.ContainsAny(s.Key, " =.\"'") {
			t.Errorf("%s: key must be a bare TOML key without dots", s.Key)
		}
	}
}

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string // "-" = no file
		want    map[string]string
		source  Source // of theme
		err     error
		errText string // substring of the message, where the message is the contract
	}{
		{name: "missing file", content: "-", want: map[string]string{KeyTheme: ThemeSystem, KeyWindowState: WindowNormal}, source: FromDefault},
		{name: "empty file", content: "", want: map[string]string{KeyTheme: ThemeSystem, KeyWindowState: WindowNormal}, source: FromDefault},
		{
			name:    "all set, with comments",
			content: "# mine\ntheme = \"dark\"   # comment\nwindow-state = 'maximized'\n",
			want:    map[string]string{KeyTheme: ThemeDark, KeyWindowState: WindowMaximized},
			source:  FromFile,
		},
		{name: "one set", content: "window-state = \"maximized\"\n", want: map[string]string{KeyTheme: ThemeSystem, KeyWindowState: WindowMaximized}, source: FromDefault},
		{name: "unknown key", content: "colour = \"dark\"\n", err: ErrUnknownKey, errText: `"colour"`},
		{name: "unknown table", content: "[window]\nstate = \"normal\"\n", err: ErrUnknownKey, errText: `"window"`},
		{name: "bad value", content: "theme = \"blue\"\n", err: ErrBadValue, errText: "allowed: system, light, dark"},
		{name: "not a string", content: "theme = 1\n", err: ErrBadValue},
		{name: "syntax error has a line", content: "theme = \"dark\"\nwindow-state = \n", errText: "line 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if tt.content != "-" {
				path = write(t, tt.content)
			}
			c, err := Load(path)
			if tt.err != nil || tt.errText != "" {
				if err == nil {
					t.Fatalf("Load = %v, want an error", c.Values())
				}
				if tt.err != nil && !errors.Is(err, tt.err) {
					t.Errorf("err = %v, want %v", err, tt.err)
				}
				if !strings.Contains(err.Error(), tt.errText) || !strings.Contains(err.Error(), path) {
					t.Errorf("err = %q, want it to name %q and contain %q", err, path, tt.errText)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for k, v := range tt.want {
				if got := c.Get(k); got != v {
					t.Errorf("%s = %q, want %q", k, got, v)
				}
			}
			if got := c.SourceOf(KeyTheme); got != tt.source {
				t.Errorf("source of theme = %q, want %q", got, tt.source)
			}
		})
	}
}

func TestLoadUnreadable(t *testing.T) {
	dir := t.TempDir() // a directory cannot be read as a file
	if _, err := Load(dir); err == nil {
		t.Fatal("Load(directory) succeeded")
	}
}

func TestOverride(t *testing.T) {
	c, err := Load(write(t, "theme = \"dark\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Override(KeyTheme, ThemeLight, FromFlag); err != nil {
		t.Fatal(err)
	}
	if c.Theme() != ThemeLight || c.SourceOf(KeyTheme) != FromFlag {
		t.Errorf("theme = %q from %q, want light from flag", c.Theme(), c.SourceOf(KeyTheme))
	}
	if err := c.Override(KeyTheme, "blue", FromFlag); !errors.Is(err, ErrBadValue) {
		t.Errorf("bad value: err = %v", err)
	}
	if err := c.Override("nope", "x", FromFlag); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("unknown key: err = %v", err)
	}
	if c.Theme() != ThemeLight {
		t.Errorf("failed overrides changed theme to %q", c.Theme())
	}
	v := c.Values()
	v[KeyTheme] = "changed"
	if c.Theme() != ThemeLight {
		t.Error("Values() is not a copy")
	}
}

func TestPathUsesUserConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	t.Setenv("HOME", "/tmp/home-test")
	t.Setenv("AppData", "/tmp/appdata-test")
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "config.toml" || filepath.Base(filepath.Dir(p)) != "mergediff" {
		t.Errorf("Path = %q", p)
	}
}
