package dirdiff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// largeTrees writes two trees of files files each, changed of them
// differing: a third modified (half keep their size, so only a byte
// comparison tells), a third added, a third deleted.
func largeTrees(t testing.TB, files, changed int) (string, string) {
	t.Helper()
	l, r := filepath.Join(t.TempDir(), "left"), filepath.Join(t.TempDir(), "right")
	step := files / changed
	for i := range files {
		dir := filepath.Join(fmt.Sprintf("pkg%02d", i%50), fmt.Sprintf("mod%02d", (i/50)%40))
		name := filepath.Join(dir, fmt.Sprintf("file%05d.txt", i))
		var b strings.Builder
		for j := range 16 + i%112 {
			fmt.Fprintf(&b, "line %d of file %d: %s\n", j, i, strings.Repeat("x", j%40))
		}
		left, right := b.String(), b.String()
		write := func(root, text string) {
			p := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		switch k := i / step; {
		case i%step != 0 || k >= changed:
			write(l, left)
			write(r, right)
		case k%3 == 0:
			if k%2 == 0 {
				right = strings.Replace(right, "line 3 ", "LINE 3 ", 1)
			} else {
				right += "one more line\n"
			}
			write(l, left)
			write(r, right)
		case k%3 == 1:
			write(r, right)
		default:
			write(l, left)
		}
	}
	return l, r
}

// The list must show within 1 s for 20,000 files with 500 changed (warm
// cache). Line counts come later and are not part of the budget.
func TestCompareLargeTreeIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 40,000 files")
	}
	if raceEnabled {
		t.Skip("timing is meaningless under the race detector")
	}
	l, r := largeTrees(t, 20000, 500)
	Compare(l, r, nil) // warm the cache, as Git just wrote the files

	best := time.Hour
	var entries []Entry
	for range 3 {
		start := time.Now()
		var err error
		if entries, err = Compare(l, r, nil); err != nil {
			t.Fatal(err)
		}
		best = min(best, time.Since(start))
	}
	changed := 0
	for _, e := range entries {
		if e.Status != Unchanged {
			changed++
		}
	}
	if changed != 500 {
		t.Errorf("%d changed entries, want 500", changed)
	}
	t.Logf("20,000 files, 500 changed: compared in %v (best of 3)", best)
	if best > time.Second {
		t.Errorf("comparison took %v, want under 1 s", best)
	}
}

func BenchmarkCompare20k(b *testing.B) {
	l, r := largeTrees(b, 20000, 500)
	for b.Loop() {
		if _, err := Compare(l, r, nil); err != nil {
			b.Fatal(err)
		}
	}
}
