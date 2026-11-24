package dirdiff

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestStats(t *testing.T) {
	l, r := trees(t)
	mkTree(t, l, map[string]string{
		"mod.txt":   "a\nb\nc\nd\n",
		"del.txt":   "1\n2\n3\n",
		"crlf.txt":  "one\r\ntwo\r\n",
		"bin.dat":   "x\x00y",
		"latin.txt": "caf\xe9\n",
		"eol.txt":   "no newline",
	})
	mkTree(t, r, map[string]string{
		"mod.txt":   "a\nB\nc\nd\ne\n",
		"add.txt":   "x\ny\n",
		"crlf.txt":  "one\ntwo\n",
		"bin.dat":   "x\x00z",
		"new.bin":   "\x00",
		"latin.txt": "caf\xe8\n",
		"eol.txt":   "no newline\n",
	})
	entries, err := Compare(l, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Stat{
		"mod.txt":   {Added: 2, Deleted: 1},
		"del.txt":   {Deleted: 3},
		"add.txt":   {Added: 2},
		"crlf.txt":  {Label: LabelLineEndings},
		"bin.dat":   {Label: LabelBinaryDiffers, Reason: ReasonBinary},
		"new.bin":   {Label: LabelBinary, Reason: ReasonBinary},
		"latin.txt": {Label: LabelNotUTF8, Reason: ReasonNotUTF8},
		"eol.txt":   {Added: 1, Deleted: 1},
	}
	for _, e := range entries {
		if got := Stats(e); got != want[e.Path] {
			t.Errorf("Stats(%s) = %+v, want %+v", e.Path, got, want[e.Path])
		}
	}
}

func TestStatsLargeFile(t *testing.T) {
	l, r := trees(t)
	small := strings.Repeat("x\n", 10)
	large := strings.Repeat("y\n", MaxOpenSize/2+1) // just over 5 MB
	mkTree(t, l, map[string]string{"big.txt": small, "gone.txt": large})
	mkTree(t, r, map[string]string{"big.txt": large, "huge.txt": large})
	entries, err := Compare(l, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Stat{
		"big.txt":  {Label: "20 B → 5.0 MB", Reason: ReasonTooLarge},
		"gone.txt": {Label: "5.0 MB", Reason: ReasonTooLarge},
		"huge.txt": {Label: "5.0 MB", Reason: ReasonTooLarge},
	}
	for _, e := range entries {
		if got := Stats(e); got != want[e.Path] {
			t.Errorf("Stats(%s) = %+v, want %+v", e.Path, got, want[e.Path])
		}
	}
}

func TestStatsNotCounted(t *testing.T) {
	for _, e := range []Entry{
		{Path: "u", Status: Unchanged},
		{Path: "e", Status: Error, Note: "permission denied"},
		{Path: "s", Status: Modified, Note: NoteSymlinkDir},
	} {
		if Counted(e) {
			t.Errorf("Counted(%s) = true", e.Path)
		}
		if got := Stats(e); got != (Stat{Reason: e.Note}) {
			t.Errorf("Stats(%s) = %+v, want only the note as reason", e.Path, got)
		}
	}
}

func TestStatsAll(t *testing.T) {
	l, r := trees(t)
	files := map[string]string{"same": "1\n"}
	for i := range 50 {
		files[filepath.Join("d", string(rune('a'+i%26))+strings.Repeat("x", i))] = "1\n"
	}
	mkTree(t, l, map[string]string{"same": "1\n"})
	mkTree(t, r, files)
	entries, err := Compare(l, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	got := map[int]Stat{}
	StatsAll(entries, func(i int, s Stat) {
		mu.Lock()
		defer mu.Unlock()
		if _, dup := got[i]; dup {
			t.Errorf("entry %d reported twice", i)
		}
		got[i] = s
	})
	if len(got) != 50 {
		t.Fatalf("%d stats reported, want 50 (the unchanged file is skipped)", len(got))
	}
	for i, s := range got {
		if entries[i].Status != Added || s.Added != 1 {
			t.Errorf("entry %s: %+v", entries[i].Path, s)
		}
	}
}

func TestHumanSize(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 3500: "3.4 KB", 5 << 20: "5.0 MB", 3 << 30: "3.0 GB", 5000 << 30: "5000.0 GB"} {
		if got := humanSize(n); got != want {
			t.Errorf("humanSize(%d) = %q, want %q", n, got, want)
		}
	}
}
