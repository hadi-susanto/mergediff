package command

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/config"
)

func TestSettingsFromConfigAndFlags(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "x\n")
	f := newFake(t, 0)
	path := f.configPath

	if code, err := runDiff(t, f, a, a); code != 0 || err != nil || f.cfg.Theme() != config.ThemeSystem || f.cfg.WindowState() != config.WindowNormal {
		t.Fatalf("no config file: code %d, %v, %v", code, err, f.cfg.Values())
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	writeFile(t, filepath.Dir(path), "config.toml", "theme = \"dark\"\nwindow-state = \"maximized\"\n")
	if code, err := runDiff(t, f, a, a); code != 0 || err != nil || f.cfg.Theme() != config.ThemeDark || f.cfg.WindowState() != config.WindowMaximized {
		t.Fatalf("config file: code %d, %v, %v", code, err, f.cfg.Values())
	}
	if code, err := runDiff(t, f, "--theme", "light", a, a, "--window-state=normal"); code != 0 || err != nil ||
		f.cfg.Theme() != config.ThemeLight || f.cfg.SourceOf(config.KeyTheme) != config.FromFlag ||
		f.cfg.WindowState() != config.WindowNormal {
		t.Fatalf("flags override the file: code %d, %v, %v", code, err, f.cfg.Values())
	}

	writeFile(t, filepath.Dir(path), "config.toml", "theme = \"blue\"\n")
	code, err := runDiff(t, f, a, a)
	var ue *UsageError
	if code != ExitError || f.opened != nil || err == nil || errors.As(err, &ue) ||
		!strings.HasPrefix(err.Error(), "config "+path+": ") {
		t.Fatalf("bad config: code %d, opened %v, err %v", code, f.opened != nil, err)
	}

	// Without a config directory the defaults apply.
	f.env.ConfigPath = func() (string, error) { return "", errors.New("no home") }
	if code, err := runDiff(t, f, a, a); code != 0 || err != nil || f.cfg.Theme() != config.ThemeSystem {
		t.Fatalf("no config directory: code %d, %v, %v", code, err, f.cfg.Values())
	}
}
