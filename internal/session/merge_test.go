package session

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/textio"
)

func TestLoadMergeAndSave(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base", "a\r\nb\r\nc\r\nd\r\ne\r\n")
	local := write(t, dir, "local", "A\r\nb\r\nc\r\nd\r\ne\r\n")
	remote := write(t, dir, "remote", "a\r\nb\r\nC\r\nd\r\nE\r\n")
	out := filepath.Join(dir, "main.go")

	s, err := LoadMerge(base, local, remote, out, false)
	if err != nil {
		t.Fatal(err)
	}
	if s.view.Mode != Merge || s.view.Title != "main.go" || s.view.Text.Language != "go" || s.view.Text.EOL != "CRLF" {
		t.Fatalf("unexpected session header: %+v", s)
	}
	if s.view.Merge.InitialResult != "a\nb\nc\nd\ne\n" {
		t.Fatalf("initial result = %q, want the base text", s.view.Merge.InitialResult)
	}
	if got := kinds(s.view.Text.Chunks); got != "local unchanged remote unchanged remote" {
		t.Fatalf("kinds = %q", got)
	}
	if err := s.Save("A\nb\nC\nd\nE\n"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "A\r\nb\r\nC\r\nd\r\nE\r\n" {
		t.Fatalf("saved %q, want CRLF result", got)
	}

	auto, err := LoadMerge(base, local, remote, out, true)
	if err != nil {
		t.Fatal(err)
	}
	if auto.view.Merge.InitialResult != "A\nb\nC\nd\nE\n" || !auto.view.Merge.AutoApplied {
		t.Fatalf("auto-applied initial result = %q, AutoApplied = %v", auto.view.Merge.InitialResult, auto.view.Merge.AutoApplied)
	}
	if s.view.Merge.AutoApplied {
		t.Fatal("AutoApplied set without --auto-apply")
	}
}

func TestLoadMergeMissingBase(t *testing.T) {
	dir := t.TempDir()
	local := write(t, dir, "local", "x\n")
	remote := write(t, dir, "remote", "y\n")
	for _, base := range []string{"", filepath.Join(dir, "nope")} {
		s, err := LoadMerge(base, local, remote, filepath.Join(dir, "out"), true)
		if err != nil {
			t.Fatalf("base %q: %v", base, err)
		}
		if s.view.Merge.Base != "" || kinds(s.view.Text.Chunks) != "conflict" || s.view.Merge.InitialResult != "" {
			t.Fatalf("base %q: unexpected session %+v", base, s)
		}
	}
}

func TestLoadMergeErrors(t *testing.T) {
	dir := t.TempDir()
	ok := write(t, dir, "ok", "x\n")
	bad := write(t, dir, "bad", "\xff\n")
	missing := filepath.Join(dir, "missing")
	out := filepath.Join(dir, "out")
	tests := []struct {
		name                string
		base, local, remote string
		prefix              string
		target              error
	}{
		{"missing local", ok, missing, ok, "read LOCAL: ", fs.ErrNotExist},
		{"missing remote", ok, ok, missing, "read REMOTE: ", fs.ErrNotExist},
		{"invalid utf8 base", bad, ok, ok, "read BASE: ", textio.ErrNotUTF8},
	}
	for _, tt := range tests {
		_, err := LoadMerge(tt.base, tt.local, tt.remote, out, false)
		if !errors.Is(err, tt.target) || !strings.HasPrefix(err.Error(), tt.prefix) {
			t.Errorf("%s: err = %v", tt.name, err)
		}
	}
	// Saving to an unwritable location reports the MERGED role.
	s, err := LoadMerge(ok, ok, ok, filepath.Join(dir, "no-such-dir", "out"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save("x\n"); err == nil || !strings.HasPrefix(err.Error(), "write MERGED: ") {
		t.Fatalf("Save to missing dir: err = %v", err)
	}
}
