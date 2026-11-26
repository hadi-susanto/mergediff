// Package testrepo builds Git repositories for tests. Only tests import it.
package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Conflicted creates a repository where merging feature into main stops
// with a.txt and sub/b.txt conflicted, and returns its directory. Git's
// global and system config are ignored. It skips the test when git is not
// installed.
func Conflicted(t testing.TB) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	git := func(fail bool, args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t",
			"-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if (err != nil) != fail {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"a.txt", filepath.Join("sub", "b.txt")} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(text+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	git(false, "init", "-q")
	git(false, "symbolic-ref", "HEAD", "refs/heads/main")
	write("base")
	git(false, "add", "-A")
	git(false, "commit", "-q", "-m", "base")
	git(false, "checkout", "-q", "-b", "feature")
	write("feature")
	git(false, "commit", "-q", "-am", "feature")
	git(false, "checkout", "-q", "main")
	write("main")
	git(false, "commit", "-q", "-am", "main")
	git(true, "merge", "--no-edit", "feature")
	return dir
}
