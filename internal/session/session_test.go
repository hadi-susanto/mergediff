package session

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func kinds(chunks []merge.Chunk) string {
	var s []string
	for _, c := range chunks {
		s = append(s, string(c.Kind))
	}
	return strings.Join(s, " ")
}

func TestLanguage(t *testing.T) {
	tests := map[string]string{
		"main.go":               "go",
		"/tmp/main_LOCAL_12.go": "go",
		"App.TSX":               "typescript",
		"Dockerfile":            "dockerfile",
		"Makefile":              "makefile",
		"notes.txt":             "",
		"noext":                 "",
	}
	for path, want := range tests {
		if got := Language(path); got != want {
			t.Errorf("Language(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestEntry(t *testing.T) {
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l"), filepath.Join(dir, "r")
	os.MkdirAll(filepath.Join(left, "sub"), 0o755)
	os.MkdirAll(right, 0o755)
	write(t, left, "sub/m.txt", "a\n")
	write(t, right, "sub", "now a file\n") // "sub" is a file on the right
	write(t, right, "bin", "x\x00")
	big := strings.Repeat("x", 5*1024*1024+1)
	write(t, right, "big.txt", big)
	if err := os.Symlink(dir, filepath.Join(right, "loop")); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	s, err := StartDirDiff(left, right)
	if err != nil {
		t.Fatal(err)
	}

	child, err := s.Entry("sub/m.txt")
	if err != nil {
		t.Fatal(err)
	}
	if child.view.Mode != Diff || child.view.Text.Left != "a\n" || child.view.Text.Right != "" || child.view.Title != "sub/m.txt" {
		t.Errorf("deleted file: %+v", child)
	}
	if child, err := s.Entry("sub"); err != nil || child.view.Text.Right != "now a file\n" || child.view.Text.Left != "" {
		t.Errorf("added file: %+v, %v", child, err)
	}
	for path, want := range map[string]error{
		"bin":     textio.ErrBinary,
		"big.txt": ErrCannotOpen,
		"loop":    ErrCannotOpen,
	} {
		if _, err := s.Entry(path); !errors.Is(err, want) {
			t.Errorf("Entry(%s) = %v, want %v", path, err, want)
		}
	}
	if _, err := s.Entry("nope"); err == nil {
		t.Error("Entry(unknown path) succeeded")
	}
}

func TestExitCode(t *testing.T) {
	type call struct {
		code int
		quit bool
	}
	normal := map[call]int{{0, false}: 0, {0, true}: 0, {1, true}: 1, {2, true}: 2}
	tests := []struct {
		name string
		sess Session
		want map[call]int
	}{
		{"diff", &DiffSession{}, normal},
		{"scratch", &ScratchSession{}, normal},
		{"dirdiff", &DirDiffSession{}, normal},
		// Closing a merge without Save (no Quit) is a cancel.
		{"merge", &MergeSession{}, map[call]int{{0, false}: 1, {0, true}: 0, {1, true}: 1, {2, true}: 2}},
	}
	for _, tt := range tests {
		for c, want := range tt.want {
			if got := tt.sess.ExitCode(c.code, c.quit); got != want {
				t.Errorf("%s: ExitCode(%d, %v) = %d, want %d", tt.name, c.code, c.quit, got, want)
			}
		}
	}
}
