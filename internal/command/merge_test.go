package command

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/config"
	"github.com/hadi-susanto/mergediff/internal/session"
)

func TestMergeOpensWindow(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.go", "x\n")
	b := writeFile(t, dir, "b.go", "y\n")
	out := filepath.Join(dir, "out.go")
	f := newFake(t, 7)

	// An empty BASE is how Git passes an add/add conflict.
	opt := parse(t, "", a, b, "-o", out, "--auto-apply", "--theme", "dark")
	cmd, err := newMerge(opt, opt.Positionals())
	if err != nil {
		t.Fatal(err)
	}
	if code, err := cmd.Run(f.env); code != 7 || err != nil {
		t.Fatalf("exit code %d, %v", code, err)
	}
	if s := f.opened; s == nil || s.Mode != session.Merge || s.Title != "out.go" || !s.Merge.AutoApplied {
		t.Fatalf("merge session = %+v", s)
	}
	if f.cfg.Theme() != config.ThemeDark {
		t.Errorf("theme %q, want the flag's dark", f.cfg.Theme())
	}

	opt = parse(t, "b", "l", "r")
	_, err = newMerge(opt, opt.Positionals())
	wantUsageError(t, err, "a 3-way merge needs -o MERGED")

	// A missing LOCAL is an I/O error, reported before any window.
	f.reset()
	opt = parse(t, a, filepath.Join(dir, "nope"), a, "-o", out)
	cmd, _ = newMerge(opt, opt.Positionals())
	var ue *UsageError
	if _, err := cmd.Run(f.env); err == nil || errors.As(err, &ue) || !strings.HasPrefix(err.Error(), "read LOCAL: ") || f.opened != nil {
		t.Fatalf("missing LOCAL: %v, opened %v", err, f.opened != nil)
	}
}
