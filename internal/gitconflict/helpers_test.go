package gitconflict

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestMain isolates every git command (the tests' and the package's) from
// the user's configuration: no global or system config, so signing,
// hooks, aliases or autocrlf settings cannot change the results.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gitconflict-config")
	if err != nil {
		panic(err)
	}
	global := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(global, nil, 0o644); err != nil {
		panic(err)
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL":   global,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "test",
		"GIT_AUTHOR_EMAIL":    "test@example.invalid",
		"GIT_COMMITTER_NAME":  "test",
		"GIT_COMMITTER_EMAIL": "test@example.invalid",
		"GIT_EDITOR":          "true",
		"GIT_SEQUENCE_EDITOR": "true",
		"GIT_MERGE_AUTOEDIT":  "no",
	} {
		os.Setenv(k, v)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// testRepo is a temporary repository built with the real git.
type testRepo struct {
	t   *testing.T
	dir string
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

// newTestRepo creates an empty repository on branch main.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	// Resolve symlinks (macOS /var → /private/var) so paths compare equal
	// with what git reports.
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	tr := &testRepo{t: t, dir: dir}
	tr.git("init", "-q")
	tr.git("symbolic-ref", "HEAD", "refs/heads/main")
	tr.git("config", "commit.gpgsign", "false")
	tr.git("config", "core.autocrlf", "false")
	return tr
}

// run runs git in the repository and returns its output and error.
func (tr *testRepo) run(args ...string) (string, error) {
	tr.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", tr.dir}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// git runs git and fails the test if it fails.
func (tr *testRepo) git(args ...string) string {
	tr.t.Helper()
	out, err := tr.run(args...)
	if err != nil {
		tr.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

// gitConflict runs a git command that must stop with conflicts (fail).
func (tr *testRepo) gitConflict(args ...string) {
	tr.t.Helper()
	out, err := tr.run(args...)
	if err == nil {
		tr.t.Fatalf("git %s: expected conflicts, succeeded\n%s", strings.Join(args, " "), out)
	}
}

func (tr *testRepo) path(p string) string {
	return filepath.Join(tr.dir, filepath.FromSlash(p))
}

// write writes content to path (relative, "/" separators), creating
// directories.
func (tr *testRepo) write(p, content string) {
	tr.t.Helper()
	full := tr.path(p)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr *testRepo) read(p string) string {
	tr.t.Helper()
	data, err := os.ReadFile(tr.path(p))
	if err != nil {
		tr.t.Fatal(err)
	}
	return string(data)
}

func (tr *testRepo) exists(p string) bool {
	_, err := os.Lstat(tr.path(p))
	return err == nil
}

func (tr *testRepo) remove(p string) {
	tr.t.Helper()
	tr.git("rm", "-q", "--", p)
}

// commit stages everything and commits.
func (tr *testRepo) commit(msg string) {
	tr.t.Helper()
	tr.git("add", "-A")
	tr.git("commit", "-q", "--allow-empty", "-m", msg)
}

// open opens the repository with this package, from subdir ("" for the top).
func (tr *testRepo) open(subdir string) *Repo {
	tr.t.Helper()
	r, err := Open(tr.path(subdir))
	if err != nil {
		tr.t.Fatalf("Open: %v", err)
	}
	return r
}

// unmerged lists the paths git status reports as unmerged.
func (tr *testRepo) unmerged() []string {
	tr.t.Helper()
	out := tr.git("diff", "--name-only", "--diff-filter=U", "-z")
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

// porcelain returns "git status --porcelain" for path: e.g. "M ", "D ", "UU".
func (tr *testRepo) porcelain(p string) string {
	tr.t.Helper()
	out := tr.git("status", "--porcelain=v1", "-z", "--", p)
	if len(out) < 2 {
		return ""
	}
	return out[:2]
}

// snapshot lists every file and directory under the working tree, .git
// excluded, to prove that reading conflicts creates nothing.
func (tr *testRepo) snapshot() []string {
	tr.t.Helper()
	var all []string
	err := filepath.WalkDir(tr.dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, _ := filepath.Rel(tr.dir, p)
		all = append(all, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		tr.t.Fatal(err)
	}
	return all
}
