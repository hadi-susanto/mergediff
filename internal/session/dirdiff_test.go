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
	"time"

	"github.com/hadi-susanto/mergediff/internal/dirdiff"
)

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
	if s.view.Mode != DirDiff || s.view.Title != "Compare directories" || !reflect.DeepEqual(s.view.Paths, []string{left, right}) {
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
	data, err := json.Marshal(s.View())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"mode":"dirdiff"`) {
		t.Errorf("JSON = %s", data)
	}
}

func TestDirStats(t *testing.T) {
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l"), filepath.Join(dir, "r")
	os.MkdirAll(left, 0o755)
	os.MkdirAll(right, 0o755)
	write(t, left, "m.txt", "a\nb\n")
	write(t, right, "m.txt", "a\nB\nc\n")
	write(t, right, "new.txt", "x\n")
	write(t, left, "same.txt", "s\n")
	write(t, right, "same.txt", "s\n")
	s, err := StartDirDiff(left, right)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]dirdiff.Stat{}
	deadline := time.Now().Add(10 * time.Second)
	for {
		b, err := s.DirStats()
		if err != nil {
			t.Fatal(err)
		}
		for _, ps := range b.Stats {
			if _, dup := got[ps.Path]; dup {
				t.Errorf("%s delivered twice", ps.Path)
			}
			got[ps.Path] = ps.Stat
		}
		if b.Done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stats never done")
		}
		time.Sleep(5 * time.Millisecond)
	}
	want := map[string]dirdiff.Stat{"m.txt": {Added: 2, Deleted: 1}, "new.txt": {Added: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stats = %+v, want %+v", got, want)
	}
	if b, _ := s.DirStats(); !b.Done || len(b.Stats) != 0 {
		t.Errorf("after done: %+v, want done and empty", b)
	}
}

func TestStartDirDiffAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "l"), 0o755)
	os.MkdirAll(filepath.Join(dir, "r"), 0o755)
	t.Chdir(dir)
	s, err := StartDirDiff("l", "r")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(dir, "l"), filepath.Join(dir, "r")}
	if !reflect.DeepEqual(s.view.Paths, want) {
		t.Errorf("Paths = %q, want %q", s.view.Paths, want)
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
