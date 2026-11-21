package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestSet(t *testing.T) {
	tests := []struct {
		name, before, key, value, after string
	}{
		{"new file", "-", KeyTheme, ThemeDark, "theme = \"dark\"\n"},
		{"empty file", "", KeyTheme, ThemeDark, "theme = \"dark\"\n"},
		{
			"replace keeps comments and other lines",
			"# my settings\ntheme = \"light\"   # was light\n\nwindow-state = \"maximized\"\n",
			KeyTheme, ThemeDark,
			"# my settings\ntheme = \"dark\"   # was light\n\nwindow-state = \"maximized\"\n",
		},
		{"replace single-quoted and indented", "  theme='light'\n", KeyTheme, ThemeSystem, "  theme = \"system\"\n"},
		{"append after a comment without newline", "# only a comment", KeyWindowState, WindowMaximized, "# only a comment\nwindow-state = \"maximized\"\n"},
		{"keeps CRLF", "# c\r\ntheme = \"light\"\r\n", KeyWindowState, WindowMaximized, "# c\r\ntheme = \"light\"\r\nwindow-state = \"maximized\"\r\n"},
		{"commented-out line is not replaced", "# theme = \"light\"\n", KeyTheme, ThemeDark, "# theme = \"light\"\ntheme = \"dark\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sub", "config.toml")
			if tt.before != "-" {
				os.MkdirAll(filepath.Dir(path), 0o755)
				os.WriteFile(path, []byte(tt.before), 0o600)
			}
			if err := Set(path, tt.key, tt.value); err != nil {
				t.Fatal(err)
			}
			if got := read(t, path); got != tt.after {
				t.Errorf("file =\n%q\nwant\n%q", got, tt.after)
			}
			c, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if c.Get(tt.key) != tt.value {
				t.Errorf("%s = %q after Set", tt.key, c.Get(tt.key))
			}
			if tt.before != "-" {
				if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
					t.Errorf("mode = %v, want 0600 kept", info.Mode().Perm())
				}
			}
		})
	}
}

func TestSetRejectsAndLeavesFileUntouched(t *testing.T) {
	const before = "# keep me\ntheme = \"light\"\n"
	for name, kv := range map[string][2]string{
		"bad value":   {KeyTheme, "blue"},
		"unknown key": {"colour", "dark"},
	} {
		t.Run(name, func(t *testing.T) {
			path := write(t, before)
			err := Set(path, kv[0], kv[1])
			if !errors.Is(err, ErrBadValue) && !errors.Is(err, ErrUnknownKey) {
				t.Errorf("err = %v", err)
			}
			if got := read(t, path); got != before {
				t.Errorf("file changed to %q", got)
			}
		})
	}
	t.Run("broken file", func(t *testing.T) {
		const broken = "theme = \"blue\"\n"
		path := write(t, broken)
		if err := Set(path, KeyWindowState, WindowMaximized); !errors.Is(err, ErrBadValue) {
			t.Errorf("err = %v, want the file's own error", err)
		}
		if got := read(t, path); got != broken {
			t.Errorf("file changed to %q", got)
		}
	})
}

func TestUnset(t *testing.T) {
	path := write(t, "# c\ntheme = \"dark\" # x\nwindow-state = \"maximized\"\n")
	if err := Unset(path, KeyTheme); err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, path), "# c\nwindow-state = \"maximized\"\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	if err := Unset(path, KeyTheme); err != nil {
		t.Errorf("unset again: %v", err)
	}
	missing := filepath.Join(t.TempDir(), "none.toml")
	if err := Unset(missing, KeyTheme); err != nil {
		t.Errorf("missing file: %v", err)
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Error("Unset created a file")
	}
	if err := Unset(path, "colour"); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("unknown key: err = %v", err)
	}
}

func TestSetLeavesNoTempFiles(t *testing.T) {
	path := write(t, "")
	for i := 0; i < 3; i++ {
		if err := Set(path, KeyTheme, ThemeDark); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only config.toml", len(entries))
	}
}
