package command

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/session"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

// runDiff checks and runs "mergediff ARGS" as a diff in f's env.
func runDiff(t *testing.T, f *fake, args ...string) (int, error) {
	t.Helper()
	f.reset()
	opt := parse(t, args...)
	cmd, err := newDiff(opt, opt.Positionals())
	if err != nil {
		return ExitError, err
	}
	return cmd.Run(f.env)
}

func TestDiffOpensWindow(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.go", "x\n")
	b := writeFile(t, dir, "b.go", "y\n")
	f := newFake(t, 7)
	if code, err := runDiff(t, f, a, b); code != 7 || err != nil {
		t.Fatalf("exit code %d, %v; want the window's code 7", code, err)
	}
	if s := f.opened; s == nil || s.Mode != session.Diff || s.Text.Language != "go" {
		t.Fatalf("diff session = %+v", s)
	}
}

func TestDiffErrors(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "x\n")
	bin := writeFile(t, dir, "bin", "\x00")
	f := newFake(t, 99)
	for _, tt := range []struct {
		args []string
		msg  string
	}{
		{[]string{"a", "b", "-o", "m"}, "-o is only valid for a 3-way merge"},
		{[]string{"a", "b", "--auto-apply"}, "--auto-apply is only valid for a 3-way merge"},
	} {
		_, err := runDiff(t, f, tt.args...)
		wantUsageError(t, err, tt.msg)
	}

	// I/O errors are not usage errors.
	_, err := runDiff(t, f, a, filepath.Join(dir, "missing"))
	var ue *UsageError
	if err == nil || errors.As(err, &ue) || !strings.HasPrefix(err.Error(), "read RIGHT: ") {
		t.Errorf("missing file: %v", err)
	}
	if _, err := runDiff(t, f, bin, a); !errors.Is(err, textio.ErrBinary) {
		t.Errorf("binary file: %v", err)
	}
	if f.opened != nil {
		t.Error("a window opened")
	}
}

func TestDiffDirectories(t *testing.T) {
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left"), filepath.Join(dir, "right")
	os.Mkdir(left, 0o755)
	os.Mkdir(right, 0o755)
	writeFile(t, left, "a.txt", "1\n")
	writeFile(t, right, "a.txt", "2\n")
	file := writeFile(t, dir, "f.txt", "x\n")
	f := newFake(t, ExitOK)

	if code, err := runDiff(t, f, left, right); code != ExitOK || err != nil {
		t.Fatalf("two directories: exit %d, %v", code, err)
	}
	s := f.opened
	if s == nil || s.Mode != session.DirDiff {
		t.Fatalf("two directories opened %+v, want a dirdiff session", s)
	}
	if entries, err := s.DirEntries(); err != nil || len(entries) != 1 || entries[0].Path != "a.txt" {
		t.Fatalf("dirdiff entries %+v, %v; want a.txt", entries, err)
	}

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{left, file}, "LEFT " + left + " is a directory but RIGHT " + file + " is a file"},
		{[]string{file, right}, "LEFT " + file + " is a file but RIGHT " + right + " is a directory"},
	} {
		_, err := runDiff(t, f, tc.args...)
		wantUsageError(t, err, tc.want)
		if f.opened != nil {
			t.Errorf("%q opened a window", tc.args)
		}
	}

	// A missing path is a file that cannot be read: an I/O error, not a usage error.
	_, err := runDiff(t, f, filepath.Join(dir, "missing"), file)
	var ue *UsageError
	if err == nil || errors.As(err, &ue) {
		t.Errorf("missing LEFT: %v; want an I/O error", err)
	}
}
