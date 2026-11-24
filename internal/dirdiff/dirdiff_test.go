package dirdiff

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// mkTree creates the files in root: path ("/" separators) → content. A
// path ending in "/" creates an empty directory instead.
func mkTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// symlink creates link (relative to root) pointing at target.
func symlink(t *testing.T, root, link, target string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(link))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, full); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
}

// trees returns two fresh roots, left and right.
func trees(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	l, r := filepath.Join(dir, "left"), filepath.Join(dir, "right")
	for _, d := range []string{l, r} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return l, r
}

// summary renders entries as "path status[ (note)]" lines.
func summary(entries []Entry) []string {
	out := []string{}
	for _, e := range entries {
		s := e.Path + " " + string(e.Status)
		if e.Note != "" {
			s += " (" + e.Note + ")"
		}
		out = append(out, s)
	}
	return out
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name        string
		left, right map[string]string
		want        []string
	}{
		{
			name:  "added deleted modified unchanged",
			left:  map[string]string{"same.txt": "x\n", "gone.txt": "a\n", "edit.txt": "one\n"},
			right: map[string]string{"same.txt": "x\n", "new.txt": "b\n", "edit.txt": "one\ntwo\n"},
			want:  []string{"edit.txt modified", "gone.txt deleted", "new.txt added", "same.txt unchanged"},
		},
		{
			name:  "nested paths use slashes and sort by path",
			left:  map[string]string{"a/b/c.txt": "1\n", "a/z.txt": "1\n"},
			right: map[string]string{"a/b/c.txt": "1\n2\n", "a/b/d.txt": "1\n", "b.txt": "1\n"},
			want:  []string{"a/b/c.txt modified", "a/b/d.txt added", "a/z.txt deleted", "b.txt added"},
		},
		{
			name:  "unicode and spaces in names",
			left:  map[string]string{"dir with space/ünïcødé 文件.txt": "a\n"},
			right: map[string]string{"dir with space/ünïcødé 文件.txt": "ab\n"},
			want:  []string{"dir with space/ünïcødé 文件.txt modified"},
		},
		{
			name:  "empty directories are ignored",
			left:  map[string]string{"empty/": "", "deep/er/": ""},
			right: map[string]string{"other/": ""},
			want:  []string{},
		},
		{
			name:  ".git directories and files are skipped",
			left:  map[string]string{".git/HEAD": "ref\n", "sub/.git": "gitdir: x\n", "a.txt": "1\n"},
			right: map[string]string{".git/config": "x\n", "a.txt": "1\n"},
			want:  []string{"a.txt unchanged"},
		},
		{
			name:  "file on one side, directory on the other",
			left:  map[string]string{"x": "file\n"},
			right: map[string]string{"x/inner.txt": "1\n"},
			want:  []string{"x deleted", "x/inner.txt added"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, r := trees(t)
			mkTree(t, l, tt.left)
			mkTree(t, r, tt.right)
			entries, err := Compare(l, r, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := summary(entries); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Compare:\n got %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestComparePaths(t *testing.T) {
	l, r := trees(t)
	mkTree(t, l, map[string]string{"a/both.txt": "1\n", "a/left.txt": "12\n"})
	mkTree(t, r, map[string]string{"a/both.txt": "123\n"})
	entries, err := Compare(l, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Path: "a/both.txt", Status: Modified, Left: filepath.Join(l, "a", "both.txt"), Right: filepath.Join(r, "a", "both.txt"), LeftSize: 2, RightSize: 4},
		{Path: "a/left.txt", Status: Deleted, Left: filepath.Join(l, "a", "left.txt"), LeftSize: 3},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("Compare:\n got %+v\nwant %+v", entries, want)
	}
}

func TestCompareSymlinks(t *testing.T) {
	l, r := trees(t)
	outside := t.TempDir()
	mkTree(t, outside, map[string]string{"target.txt": "linked\n", "dir1/f": "1", "dir2/f": "2"})
	mkTree(t, l, map[string]string{"file.txt": "linked\n"})

	// Git's --dir-diff symlinks right-side files to the working tree.
	symlink(t, r, "file.txt", filepath.Join(outside, "target.txt"))
	symlink(t, l, "broken.txt", filepath.Join(outside, "missing.txt"))
	symlink(t, l, "samedir", filepath.Join(outside, "dir1"))
	symlink(t, r, "samedir", filepath.Join(outside, "dir1"))
	symlink(t, l, "otherdir", filepath.Join(outside, "dir1"))
	symlink(t, r, "otherdir", filepath.Join(outside, "dir2"))
	symlink(t, r, "newdir", filepath.Join(outside, "dir1"))

	entries, err := Compare(l, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"broken.txt error (broken symlink: no such file or directory)",
		"file.txt unchanged",
		"newdir added (" + NoteSymlinkDir + ")",
		"otherdir modified (" + NoteSymlinkDir + ")",
		"samedir unchanged (" + NoteSymlinkDir + ")",
	}
	if got := summary(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("Compare:\n got %q\nwant %q", got, want)
	}
}

func TestCompareRootIsSymlink(t *testing.T) {
	l, r := trees(t)
	mkTree(t, l, map[string]string{"a.txt": "1\n"})
	mkTree(t, r, map[string]string{"a.txt": "1\n"})
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(r, link); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	entries, err := Compare(l, link, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := summary(entries); !reflect.DeepEqual(got, []string{"a.txt unchanged"}) {
		t.Errorf("Compare = %q, want a.txt unchanged", got)
	}
	if entries[0].Right != filepath.Join(link, "a.txt") {
		t.Errorf("Right = %s, want it under the root as given (%s)", entries[0].Right, link)
	}
}

func TestCompareBadRoot(t *testing.T) {
	l, _ := trees(t)
	file := filepath.Join(l, "f")
	mkTree(t, l, map[string]string{"f": "x"})
	if _, err := Compare(l, file, nil); !errors.Is(err, errNotDir) {
		t.Errorf("RIGHT is a file: %v, want errNotDir", err)
	}
	if _, err := Compare(filepath.Join(l, "missing"), l, nil); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LEFT missing: %v, want ErrNotExist", err)
	}
}

func TestCompareUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions and a non-root user")
	}
	l, r := trees(t)
	mkTree(t, l, map[string]string{"locked/a.txt": "1\n", "ok.txt": "1\n"})
	mkTree(t, r, map[string]string{"ok.txt": "1\n"})
	locked := filepath.Join(l, "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	entries, err := Compare(l, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"locked error (permission denied)", "ok.txt unchanged"}
	if got := summary(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("Compare:\n got %q\nwant %q", got, want)
	}
}
