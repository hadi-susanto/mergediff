package session

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

func TestLoadDiff(t *testing.T) {
	dir := t.TempDir()
	left := write(t, dir, "a.txt", "one\ntwo\nthree\n")
	right := write(t, dir, "b.txt", "one\n2\nthree\n")
	s, err := LoadDiff(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if s.view.Mode != Diff || s.view.Text.Left != "one\ntwo\nthree\n" || s.view.Text.Right != "one\n2\nthree\n" || s.view.Merge != nil {
		t.Fatalf("unexpected session: %+v", s)
	}
	if got := kinds(s.view.Text.Chunks); got != "unchanged remote unchanged" {
		t.Fatalf("kinds = %q", got)
	}
	if c := s.view.Text.Chunks[1]; c.Local != (merge.Range{Start: 1, End: 2}) || c.Remote != (merge.Range{Start: 1, End: 2}) {
		t.Fatalf("changed chunk = %+v", c)
	}
	if s.view.Title != "a.txt ↔ b.txt" || s.view.Text.EOL != "LF" {
		t.Fatalf("title %q eol %q", s.view.Title, s.view.Text.EOL)
	}
}

func TestLoadDiffIdenticalSerialisesEmptyChunks(t *testing.T) {
	dir := t.TempDir()
	a := write(t, dir, "a", "")
	b := write(t, dir, "b", "")
	s, err := LoadDiff(a, b)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s.View())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"chunks":[]`) {
		t.Fatalf("chunks not serialised as []: %s", data)
	}
	for _, private := range []string{"output", `"eol":"\n"`} {
		if strings.Contains(string(data), private) {
			t.Fatalf("JSON leaks %q: %s", private, data)
		}
	}
}

func TestLoadDiffErrors(t *testing.T) {
	dir := t.TempDir()
	ok := write(t, dir, "ok.txt", "x\n")
	bin := write(t, dir, "bin", "a\x00b")
	missing := filepath.Join(dir, "missing")

	_, err := LoadDiff(missing, ok)
	if !errors.Is(err, fs.ErrNotExist) || !strings.HasPrefix(err.Error(), "read LEFT: ") || !strings.Contains(err.Error(), missing) {
		t.Fatalf("missing LEFT: err = %v", err)
	}
	_, err = LoadDiff(ok, bin)
	if !errors.Is(err, textio.ErrBinary) || !strings.HasPrefix(err.Error(), "read RIGHT: ") {
		t.Fatalf("binary RIGHT: err = %v", err)
	}
}

func TestLoadPairMissingSides(t *testing.T) {
	dir := t.TempDir()
	f := write(t, dir, "a.go", "one\r\ntwo\r\n")
	for _, tc := range []struct{ left, right, local, remote string }{
		{"", f, "", "one\ntwo\n"},
		{f, "", "one\ntwo\n", ""},
	} {
		s, err := LoadPair(tc.left, tc.right, "pkg/a.go")
		if err != nil {
			t.Fatal(err)
		}
		if s.view.Mode != Diff || s.view.Text.Left != tc.local || s.view.Text.Right != tc.remote || s.view.Title != "pkg/a.go" ||
			s.view.Text.Language != "go" || s.view.Text.EOL != "CRLF" || !reflect.DeepEqual(s.view.Paths, []string{tc.left, tc.right}) {
			t.Errorf("LoadPair(%q, %q) = %+v", tc.left, tc.right, s)
		}
		if len(s.view.Text.Chunks) == 0 {
			t.Errorf("LoadPair(%q, %q): no chunks", tc.left, tc.right)
		}
	}
}
