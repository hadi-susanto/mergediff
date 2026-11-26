package command

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/cliargs"
	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// fake is an Env whose window records what it was asked to open, and
// whose config file is in a fresh temporary directory.
type fake struct {
	env            Env
	stdout, stderr bytes.Buffer
	opened         *session.Session
	cfg            config.Config
	configPath     string
}

func newFake(t *testing.T, code int) *fake {
	t.Helper()
	f := &fake{configPath: filepath.Join(t.TempDir(), "mergediff", "config.toml")}
	f.env = Env{
		Stdout:     &f.stdout,
		Stderr:     &f.stderr,
		Getwd:      os.Getwd,
		ConfigPath: func() (string, error) { return f.configPath, nil },
		Version:    "2026-11-19 (a1b2c3d)",
		OpenWindow: func(s *session.Session, c config.Config) int { f.opened, f.cfg = s, c; return code },
	}
	return f
}

// reset forgets the previous run's output and window.
func (f *fake) reset() {
	f.stdout.Reset()
	f.stderr.Reset()
	f.opened = nil
}

// parse parses args or fails the test.
func parse(t *testing.T, args ...string) cliargs.Option {
	t.Helper()
	opt, err := cliargs.Parse(args)
	if err != nil {
		t.Fatalf("Parse(%q): %v", args, err)
	}
	return opt
}

// wantUsageError checks that err is a *UsageError whose message contains
// msg, and returns it.
func wantUsageError(t *testing.T, err error, msg string) *UsageError {
	t.Helper()
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("err = %v, want a usage error", err)
	}
	if !strings.Contains(ue.Msg, msg) {
		t.Fatalf("err = %q, want it to mention %q", ue.Msg, msg)
	}
	return ue
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
