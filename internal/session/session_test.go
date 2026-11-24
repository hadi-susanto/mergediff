package session

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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

func TestLoadDiff(t *testing.T) {
	dir := t.TempDir()
	left := write(t, dir, "a.txt", "one\ntwo\nthree\n")
	right := write(t, dir, "b.txt", "one\n2\nthree\n")
	s, err := LoadDiff(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if s.Mode != Diff || s.Local != "one\ntwo\nthree\n" || s.Remote != "one\n2\nthree\n" || s.Base != "" {
		t.Fatalf("unexpected session: %+v", s)
	}
	if got := kinds(s.Chunks); got != "unchanged remote unchanged" {
		t.Fatalf("kinds = %q", got)
	}
	if c := s.Chunks[1]; c.Local != (merge.Range{Start: 1, End: 2}) || c.Remote != (merge.Range{Start: 1, End: 2}) {
		t.Fatalf("changed chunk = %+v", c)
	}
	if s.Title != "a.txt ↔ b.txt" || s.EOL != "LF" {
		t.Fatalf("title %q eol %q", s.Title, s.EOL)
	}
	if err := s.Save("x"); !errors.Is(err, ErrNotMerge) {
		t.Fatalf("Save in diff mode: err = %v, want ErrNotMerge", err)
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
	data, err := json.Marshal(s)
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
	if s.Mode != Merge || s.Title != "main.go" || s.Language != "go" || s.EOL != "CRLF" {
		t.Fatalf("unexpected session header: %+v", s)
	}
	if s.InitialResult != "a\nb\nc\nd\ne\n" {
		t.Fatalf("initial result = %q, want the base text", s.InitialResult)
	}
	if got := kinds(s.Chunks); got != "local unchanged remote unchanged remote" {
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
	if auto.InitialResult != "A\nb\nC\nd\nE\n" || !auto.AutoApplied {
		t.Fatalf("auto-applied initial result = %q, AutoApplied = %v", auto.InitialResult, auto.AutoApplied)
	}
	if s.AutoApplied {
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
		if s.Base != "" || kinds(s.Chunks) != "conflict" || s.InitialResult != "" {
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

func TestScratchAndCompare(t *testing.T) {
	s := NewScratch()
	if s.Mode != Scratch || s.Chunks == nil || s.Paths == nil || s.Local != "" || s.Remote != "" {
		t.Fatalf("NewScratch = %+v", s)
	}
	if err := s.Save("x"); !errors.Is(err, ErrNotMerge) {
		t.Fatalf("Save in scratch mode: %v", err)
	}

	chunks, err := Compare("a\nb\nc\n", "a\nB\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []merge.Chunk{
		{Kind: merge.Unchanged, Base: merge.Range{Start: 0, End: 1}, Local: merge.Range{Start: 0, End: 1}, Remote: merge.Range{Start: 0, End: 1}},
		{Kind: merge.RemoteOnly, Base: merge.Range{Start: 1, End: 2}, Local: merge.Range{Start: 1, End: 2}, Remote: merge.Range{Start: 1, End: 2}},
		{Kind: merge.Unchanged, Base: merge.Range{Start: 2, End: 3}, Local: merge.Range{Start: 2, End: 3}, Remote: merge.Range{Start: 2, End: 3}},
	}
	if !reflect.DeepEqual(chunks, want) {
		t.Fatalf("Compare = %+v", chunks)
	}

	// Pasted CRLF text equals the LF text; empty texts give [] not nil.
	for _, pair := range [][2]string{{"a\r\nb\r\n", "a\nb\n"}, {"", ""}} {
		chunks, err := Compare(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range chunks {
			if c.Kind != merge.Unchanged {
				t.Fatalf("Compare(%q, %q) has a change: %+v", pair[0], pair[1], chunks)
			}
		}
		if chunks == nil {
			t.Fatal("chunks are nil")
		}
	}

	if _, err := Compare("a\x00b", "x"); !errors.Is(err, textio.ErrBinary) {
		t.Fatalf("NUL text: err = %v", err)
	}
}

func TestStartDirDiff(t *testing.T) {
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l"), filepath.Join(dir, "r")
	for _, d := range []string{left, right} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, left, "same.txt", "x\n")
	write(t, right, "same.txt", "x\n")
	write(t, right, "new.txt", "y\n")

	s, err := StartDirDiff(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if s.Mode != DirDiff || s.Title != "Compare directories" || !reflect.DeepEqual(s.Paths, []string{left, right}) {
		t.Errorf("session = %+v", s)
	}
	entries, err := s.DirEntries()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Path+" "+string(e.Status))
	}
	if want := []string{"new.txt added", "same.txt unchanged"}; !reflect.DeepEqual(got, want) {
		t.Errorf("entries = %q, want %q", got, want)
	}
	if p := s.DirProgress(); !p.Done || p.Found != 3 || p.Compared != 2 {
		t.Errorf("progress after the comparison = %+v, want done, 3 found, 2 compared", p)
	}
	// A copy (as LoadSession returns) shares the comparison.
	c := *s
	if again, err := c.DirEntries(); err != nil || len(again) != 2 {
		t.Errorf("copy: %d entries, %v", len(again), err)
	}
	if err := s.Save("x"); !errors.Is(err, ErrNotMerge) {
		t.Errorf("Save in dirdiff mode: %v, want ErrNotMerge", err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"mode":"dirdiff"`) {
		t.Errorf("JSON = %s", data)
	}
}

func TestStartDirDiffErrors(t *testing.T) {
	dir := t.TempDir()
	file := write(t, dir, "f.txt", "x")
	if _, err := StartDirDiff(dir, file); err == nil || !strings.Contains(err.Error(), "RIGHT") {
		t.Errorf("RIGHT is a file: %v, want an error naming RIGHT", err)
	}
	if _, err := StartDirDiff(filepath.Join(dir, "missing"), dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("LEFT missing: %v, want ErrNotExist", err)
	}
}

func TestDirEntriesOtherModes(t *testing.T) {
	s := NewScratch()
	if _, err := s.DirEntries(); !errors.Is(err, ErrNotDirDiff) {
		t.Errorf("DirEntries in scratch mode: %v, want ErrNotDirDiff", err)
	}
	if p := s.DirProgress(); !p.Done {
		t.Errorf("DirProgress in scratch mode = %+v, want done", p)
	}
}
