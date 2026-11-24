package dirdiff

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestCompareContents(t *testing.T) {
	big := bytes.Repeat([]byte("0123456789abcdef"), 3*chunkSize/16+100) // over 3 chunks
	bigEnd := bytes.Clone(big)
	bigEnd[len(bigEnd)-1] = 'X'
	bigMid := bytes.Clone(big)
	bigMid[chunkSize+5] = 'X'

	l, r := trees(t)
	mkTree(t, l, map[string]string{
		"same.txt":     "abc\n",
		"samesize.txt": "abc\n",
		"empty":        "",
		"big-same":     string(big),
		"big-end":      string(big),
		"big-mid":      string(big),
	})
	mkTree(t, r, map[string]string{
		"same.txt":     "abc\n",
		"samesize.txt": "abd\n",
		"empty":        "",
		"big-same":     string(big),
		"big-end":      string(bigEnd),
		"big-mid":      string(bigMid),
	})
	entries, err := Compare(l, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"big-end modified", "big-mid modified", "big-same unchanged",
		"empty unchanged", "same.txt unchanged", "samesize.txt modified",
	}
	if got := summary(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("Compare:\n got %q\nwant %q", got, want)
	}
}

func TestCompareUnreadableFiles(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	l, r := trees(t)
	mkTree(t, l, map[string]string{"samesize": "abc", "othersize": "a", "ok.txt": "1\n"})
	mkTree(t, r, map[string]string{"samesize": "abd", "othersize": "abc", "added": "x", "ok.txt": "1\n"})
	for _, p := range []string{filepath.Join(l, "samesize"), filepath.Join(r, "othersize"), filepath.Join(r, "added")} {
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := Compare(l, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"added error (permission denied)",
		"ok.txt unchanged",
		"othersize error (permission denied)",
		"samesize error (permission denied)",
	}
	if got := summary(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("Compare:\n got %q\nwant %q", got, want)
	}
}

func TestSameFilesSizesDiffer(t *testing.T) {
	// sameFiles is only called for equal sizes, but must still be right
	// when a file changed size meanwhile.
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	os.WriteFile(a, []byte("abc"), 0o644)
	os.WriteFile(b, []byte("abcd"), 0o644)
	if same, err := sameFiles(a, b); err != nil || same {
		t.Errorf("sameFiles(abc, abcd) = %v, %v; want false, nil", same, err)
	}
}
