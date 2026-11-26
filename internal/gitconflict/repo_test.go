package gitconflict

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want Version
	}{
		{"git version 2.43.0", Version{2, 43, 0}},
		{"git version 2.39.3 (Apple Git-145)", Version{2, 39, 3}},
		{"git version 2.45.1.windows.1", Version{2, 45, 1}},
		{"git version 2.11", Version{2, 11, 0}},
		{"git version 2.50.0.rc1", Version{2, 50, 0}},
		{"git version 2.9.5\n", Version{2, 9, 5}},
	}
	for _, c := range cases {
		got, err := parseVersion(c.in)
		if err != nil || got != c.want {
			t.Errorf("parseVersion(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "hub version 2.43", "git version x.y"} {
		if _, err := parseVersion(bad); err == nil {
			t.Errorf("parseVersion(%q): want an error", bad)
		}
	}
}

func TestVersionLess(t *testing.T) {
	if !(Version{2, 10, 9}).Less(MinVersion) {
		t.Error("2.10.9 should be older than 2.11.0")
	}
	if (Version{2, 11, 0}).Less(MinVersion) || (Version{3, 0, 0}).Less(MinVersion) {
		t.Error("2.11.0 and 3.0.0 are not older than 2.11.0")
	}
}

func TestOpenTopAndSubdirectory(t *testing.T) {
	tr := newTestRepo(t)
	tr.write("src/pkg/a.txt", "a\n")
	tr.commit("base")

	r := tr.open("")
	if r.Top != filepath.ToSlash(tr.dir) && r.Top != tr.dir {
		t.Errorf("Top = %q, want %q", r.Top, tr.dir)
	}
	if r.Prefix != "" {
		t.Errorf("Prefix at top = %q, want empty", r.Prefix)
	}
	if r.Name() != filepath.Base(tr.dir) {
		t.Errorf("Name = %q", r.Name())
	}

	sub := tr.open("src/pkg")
	if sub.Prefix != "src/pkg/" {
		t.Errorf("Prefix = %q, want src/pkg/", sub.Prefix)
	}
	if sub.Top != r.Top {
		t.Errorf("Top from subdirectory = %q, want %q", sub.Top, r.Top)
	}
}

func TestOpenOutsideWorkTree(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	if _, err := Open(dir); !errors.Is(err, ErrNotWorkTree) {
		t.Errorf("Open(plain dir) = %v, want ErrNotWorkTree", err)
	}

	tr := newTestRepo(t)
	if _, err := Open(filepath.Join(tr.dir, ".git")); !errors.Is(err, ErrNotWorkTree) {
		t.Errorf("Open(.git) = %v, want ErrNotWorkTree", err)
	}

	bare := filepath.Join(t.TempDir(), "bare.git")
	if out, err := (&testRepo{t: t, dir: t.TempDir()}).run("init", "-q", "--bare", bare); err != nil {
		t.Fatalf("init --bare: %v\n%s", err, out)
	}
	if _, err := Open(bare); !errors.Is(err, ErrNotWorkTree) {
		t.Errorf("Open(bare) = %v, want ErrNotWorkTree", err)
	}
}

// fakeGit puts a "git" script printing version on PATH.
func fakeGit(t *testing.T, version string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-in for git")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'git version " + version + "'\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestOpenGitTooOld(t *testing.T) {
	fakeGit(t, "2.10.0")
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrGitTooOld) {
		t.Errorf("Open with git 2.10.0 = %v, want ErrGitTooOld", err)
	}
}

func TestOpenNoGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNoGit) {
		t.Errorf("Open without git = %v, want ErrNoGit", err)
	}
}

func TestPathspecs(t *testing.T) {
	tr := newTestRepo(t)
	tr.write("src/pkg/a.txt", "a\n")
	tr.commit("base")
	r := tr.open("src")

	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"pkg"}, []string{"src/pkg"}},
		{[]string{"pkg/a.txt", "./b.txt"}, []string{"src/pkg/a.txt", "src/b.txt"}},
		{[]string{"../README.md"}, []string{"README.md"}},
		{[]string{"."}, []string{"src"}},
		{[]string{".."}, nil},
		{[]string{"pkg", ".."}, nil},
		{[]string{tr.path("src/pkg/a.txt")}, []string{"src/pkg/a.txt"}},
		{[]string{tr.path("src/pkg/gone.txt")}, []string{"src/pkg/gone.txt"}},
		{nil, nil},
	}
	for _, c := range cases {
		got, err := r.Pathspecs(c.args)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("Pathspecs(%q) = %q, %v; want %q", c.args, got, err, c.want)
		}
	}
	for _, bad := range [][]string{{"../.."}, {"../../x"}, {filepath.Dir(tr.dir)}} {
		if _, err := r.Pathspecs(bad); !errors.Is(err, ErrOutsideRepo) {
			t.Errorf("Pathspecs(%q) = %v, want ErrOutsideRepo", bad, err)
		}
	}
}

func TestGitPathsInLinkedWorktree(t *testing.T) {
	tr := newTestRepo(t)
	tr.write("a.txt", "a\n")
	tr.commit("base")
	wt := filepath.Join(t.TempDir(), "wt")
	tr.git("worktree", "add", "-q", "-b", "other", wt)
	r, err := Open(wt)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := r.gitPaths("MERGE_HEAD")
	if err != nil {
		t.Fatal(err)
	}
	// A linked worktree keeps its state under .git/worktrees/<name>.
	want := filepath.Join(tr.dir, ".git", "worktrees", "wt", "MERGE_HEAD")
	got := paths["MERGE_HEAD"]
	if g, err := filepath.EvalSymlinks(filepath.Dir(got)); err == nil {
		got = filepath.Join(g, filepath.Base(got))
	}
	if got != want {
		t.Errorf("git-path MERGE_HEAD = %q, want %q", got, want)
	}
}
