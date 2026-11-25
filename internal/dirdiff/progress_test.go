package dirdiff

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProgress(t *testing.T) {
	l, r := trees(t)
	mkTree(t, l, map[string]string{"a": "1", "b": "2", "c/d": "3"})
	mkTree(t, r, map[string]string{"a": "1", "b": "x", "e": "4", ".git/x": "skip"})
	var p Progress
	if s := p.Snapshot(); s != (Snapshot{}) {
		t.Fatalf("new Progress = %+v, want zero", s)
	}
	entries, err := Compare(l, r, &p)
	if err != nil {
		t.Fatal(err)
	}
	want := Snapshot{Found: 6, Compared: int64(len(entries)), Done: true}
	if s := p.Snapshot(); s != want {
		t.Errorf("after Compare: %+v, want %+v", s, want)
	}
}

func TestProgressDoneAfterError(t *testing.T) {
	var p Progress
	if _, err := Compare(filepath.Join(t.TempDir(), "missing"), t.TempDir(), &p); err == nil {
		t.Fatal("Compare with a missing root succeeded")
	}
	if !p.Snapshot().Done {
		t.Error("Progress not done after an error")
	}
}

func TestNilProgress(t *testing.T) {
	var p *Progress
	p.addFound()
	p.addCompared(3)
	p.finish()
	if s := p.Snapshot(); s != (Snapshot{}) {
		t.Errorf("nil Progress = %+v", s)
	}
}

func TestCheckRoot(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	os.WriteFile(file, nil, 0o644)
	if err := CheckRoot(dir); err != nil {
		t.Errorf("CheckRoot(dir) = %v", err)
	}
	if err := CheckRoot(file); !errors.Is(err, errNotDir) {
		t.Errorf("CheckRoot(file) = %v, want errNotDir", err)
	}
	if err := CheckRoot(filepath.Join(dir, "x")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("CheckRoot(missing) = %v, want ErrNotExist", err)
	}
}
