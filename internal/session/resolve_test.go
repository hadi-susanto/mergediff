package session

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/gitconflict"
)

// conflictRepo builds a repository whose main and feature branches change
// files differently, then merges feature into main (stopping with
// conflicts). files maps a path to its base, main and feature content;
// gone instead of content means the file does not exist there.
func conflictRepo(t *testing.T, files map[string][3]string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	global := filepath.Join(t.TempDir(), "gitconfig")
	os.WriteFile(global, nil, 0o644)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	git := func(fail bool, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid",
			"-c", "commit.gpgsign=false"}, args...)...)
		out, err := cmd.CombinedOutput()
		if (err != nil) != fail {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	write := func(i int) {
		for p, c := range files {
			full := filepath.Join(dir, filepath.FromSlash(p))
			if c[i] == gone {
				os.Remove(full)
				continue
			}
			os.MkdirAll(filepath.Dir(full), 0o755)
			if err := os.WriteFile(full, []byte(c[i]), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	git(false, "init", "-q")
	git(false, "symbolic-ref", "HEAD", "refs/heads/main")
	write(0)
	git(false, "add", "-A")
	git(false, "commit", "-q", "-m", "base")
	git(false, "checkout", "-q", "-b", "feature")
	write(2)
	git(false, "add", "-A")
	git(false, "commit", "-q", "-m", "feature")
	git(false, "checkout", "-q", "main")
	write(1)
	git(false, "add", "-A")
	git(false, "commit", "-q", "-m", "main")
	git(true, "merge", "--no-edit", "feature")
	return dir
}

// gone marks a file missing from one of conflictRepo's commits.
const gone = "\x00gone"

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func rowsByPath(list ConflictList) map[string]ConflictRow {
	m := map[string]ConflictRow{}
	for _, r := range list.Rows {
		m[r.Path] = r
	}
	return m
}

func TestStartResolveListsAndRefreshes(t *testing.T) {
	dir := conflictRepo(t, map[string][3]string{
		"a.txt":     {"1\n2\n3\n", "1\nM\n3\n", "1\nF\n3\n"},
		"sub/b.txt": {"b\n", "b main\n", "b feature\n"},
		"img.bin":   {"\x00base", "\x00main", "\x00feature"},
		"same.txt":  {"s\n", "s\n", "s feature\n"},
	})
	s, err := StartResolve(filepath.Join(dir, "sub"), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if s.view.Mode != Resolve || s.view.Title != "Resolve conflicts · "+filepath.Base(dir) || len(s.view.Paths) != 1 {
		t.Fatalf("session = %+v", s)
	}
	list, err := s.Conflicts()
	if err != nil {
		t.Fatal(err)
	}
	if list.Operation.Kind != gitconflict.OpMerge || list.Operation.YoursLabel != "main" || list.Operation.TheirsLabel != "feature" {
		t.Errorf("operation = %+v", list.Operation)
	}
	if len(list.Rows) != 3 || list.Unresolved != 3 {
		t.Fatalf("rows = %+v", list.Rows)
	}
	if list.Rows[0].Path != "a.txt" || list.Rows[1].Path != "img.bin" || list.Rows[2].Path != "sub/b.txt" {
		t.Errorf("rows not sorted by path: %+v", list.Rows)
	}
	if r := rowsByPath(list)["img.bin"]; r.State != gitconflict.StateBinary || r.Yours != gitconflict.Modified {
		t.Errorf("img.bin = %+v", r)
	}

	// Accept theirs: the row stays, resolved, with its old statuses.
	list, err = s.Accept([]string{"sub/b.txt"}, gitconflict.Theirs)
	if err != nil {
		t.Fatal(err)
	}
	b := rowsByPath(list)["sub/b.txt"]
	if !b.Resolved || b.Yours != gitconflict.Modified || b.State != gitconflict.StateConflict || list.Unresolved != 2 {
		t.Errorf("after accept: %+v, unresolved %d", b, list.Unresolved)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "sub", "b.txt")); string(data) != "b feature\n" {
		t.Errorf("sub/b.txt = %q", data)
	}

	// Resolved in a terminal meanwhile: a refresh shows it.
	gitIn(t, dir, "checkout", "--ours", "--", "a.txt")
	gitIn(t, dir, "add", "--", "a.txt")
	list, err = s.Conflicts()
	if err != nil {
		t.Fatal(err)
	}
	if !rowsByPath(list)["a.txt"].Resolved || list.Unresolved != 1 || len(list.Rows) != 3 {
		t.Errorf("after terminal resolve: %+v", list)
	}
	if n, err := s.Remaining(); n != 1 || err != nil {
		t.Errorf("Remaining = %d, %v; want 1", n, err)
	}
	if _, err := s.Accept([]string{"img.bin"}, gitconflict.Yours); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Remaining(); n != 0 || err != nil {
		t.Errorf("Remaining = %d, %v; want 0", n, err)
	}
}

func TestStartResolvePathLimits(t *testing.T) {
	dir := conflictRepo(t, map[string][3]string{
		"a.txt":     {"a\n", "a main\n", "a feature\n"},
		"sub/b.txt": {"b\n", "b main\n", "b feature\n"},
	})
	s, err := StartResolve(filepath.Join(dir, "sub"), []string{"."}, false)
	if err != nil {
		t.Fatal(err)
	}
	list, _ := s.Conflicts()
	if len(list.Rows) != 1 || list.Rows[0].Path != "sub/b.txt" {
		t.Errorf("rows = %+v, want only sub/b.txt", list.Rows)
	}
	if _, err := s.Accept([]string{"sub/b.txt"}, gitconflict.Yours); err != nil {
		t.Fatal(err)
	}
	// a.txt is still conflicted, but outside the limits.
	if n, _ := s.Remaining(); n != 0 {
		t.Errorf("Remaining = %d, want 0 within the limits", n)
	}

	if _, err := StartResolve(dir, []string{"sub"}, false); !errors.Is(err, ErrNoConflicts) {
		t.Errorf("no conflicts left under sub: %v, want ErrNoConflicts", err)
	}
	if _, err := StartResolve(dir, []string{"../elsewhere"}, false); !errors.Is(err, gitconflict.ErrOutsideRepo) {
		t.Errorf("path outside: %v, want ErrOutsideRepo", err)
	}
}

func TestStartResolveErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if _, err := StartResolve(t.TempDir(), nil, false); !errors.Is(err, gitconflict.ErrNotWorkTree) {
		t.Errorf("outside a repository: %v, want ErrNotWorkTree", err)
	}
	dir := conflictRepo(t, map[string][3]string{"a.txt": {"a\n", "a main\n", "a feature\n"}})
	gitIn(t, dir, "add", "a.txt")
	if _, err := StartResolve(dir, nil, false); !errors.Is(err, ErrNoConflicts) {
		t.Errorf("all resolved: %v, want ErrNoConflicts", err)
	}
}

func TestMergeCounts(t *testing.T) {
	dir := conflictRepo(t, map[string][3]string{
		// Line 2: both changed (conflict). Line 5: only main. Line 8: only feature.
		"a.txt":   {"1\n2\n3\n4\n5\n6\n7\n8\n", "1\nM\n3\n4\nM5\n6\n7\n8\n", "1\nF\n3\n4\n5\n6\n7\nF8\n"},
		"img.bin": {"\x00base", "\x00main", "\x00feature"},
	})
	s, err := StartResolve(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.MergeCounts("a.txt")
	if err != nil || n != (MergeCounts{Conflicts: 1, NonConflicting: 2}) {
		t.Errorf("MergeCounts(a.txt) = %+v, %v", n, err)
	}
	if n, err := s.MergeCounts("img.bin"); err != nil || n != (MergeCounts{}) {
		t.Errorf("MergeCounts(img.bin) = %+v, %v; want zero", n, err)
	}
	if _, err := s.MergeCounts("nope.txt"); !errors.Is(err, ErrNotConflicted) {
		t.Errorf("MergeCounts(nope.txt) = %v, want ErrNotConflicted", err)
	}
}

func TestAcceptUnknownSide(t *testing.T) {
	dir := conflictRepo(t, map[string][3]string{"a.txt": {"a\n", "a main\n", "a feature\n"}})
	s, err := StartResolve(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Accept([]string{"a.txt"}, "mine"); err == nil {
		t.Error("unknown side: want an error")
	}
}

func TestMergeSession(t *testing.T) {
	dir := conflictRepo(t, map[string][3]string{
		"a.txt":     {"one\ntwo\nthree\n", "one\nTWO main\nthree\nfour\n", "one\nTWO feature\nthree\n"},
		"b.txt":     {"x\n", gone, "x changed\n"},
		"crlf.txt":  {"a\r\nb\r\n", "a main\r\nb\r\n", "a feature\r\nb\r\n"},
		"plain.txt": {"same\n", "same\n", "same\n"},
	})
	s, err := StartResolve(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.Merge("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if m.view.Mode != Merge || m.view.Merge.Base != "one\ntwo\nthree\n" || m.view.Text.Left != "one\nTWO main\nthree\nfour\n" || m.view.Text.Right != "one\nTWO feature\nthree\n" {
		t.Fatalf("texts: %+v", m)
	}
	if m.view.Merge.InitialResult != m.view.Merge.Base || m.view.Merge.AutoApplied {
		t.Errorf("initial result should be base without --auto-apply")
	}
	if m.view.Text.LeftLabel != "main" || m.view.Text.RightLabel != "feature" {
		t.Errorf("labels = %q, %q; want main, feature", m.view.Text.LeftLabel, m.view.Text.RightLabel)
	}
	if m.view.Title != "a.txt" || m.view.Text.Language != "" {
		t.Errorf("title %q language %q", m.view.Title, m.view.Text.Language)
	}
	if err := m.Save("one\nmerged\nthree\n"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(got) != "one\nmerged\nthree\n" {
		t.Errorf("working tree = %q", got)
	}
	if out := gitIn(t, dir, "ls-files", "-u", "--", "a.txt"); out != "" {
		t.Errorf("still unmerged after save:\n%s", out)
	}
	if _, err := s.Merge("a.txt"); !errors.Is(err, ErrNotConflicted) {
		t.Errorf("merge of a resolved file: %v, want ErrNotConflicted", err)
	}
	if _, err := s.Merge("b.txt"); !errors.Is(err, ErrCannotOpen) {
		t.Errorf("merge of modify/delete: %v, want ErrCannotOpen", err)
	}

	// CRLF in the checkout: the merge keeps it.
	c, err := s.Merge("crlf.txt")
	if err != nil {
		t.Fatal(err)
	}
	if c.view.Text.EOL != "CRLF" || strings.Contains(c.view.Text.Left, "\r") {
		t.Errorf("EOL %s, local %q", c.view.Text.EOL, c.view.Text.Left)
	}
	if err := c.Save("a merged\nb\n"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "crlf.txt")); string(got) != "a merged\r\nb\r\n" {
		t.Errorf("crlf working tree = %q", got)
	}

	// --auto-apply starts with the non-conflicting changes applied.
	dir2 := conflictRepo(t, map[string][3]string{
		"a.txt": {"one\ntwo\nthree\n", "one\nTWO main\nthree\nfour\n", "one\nTWO feature\nthree\n"},
	})
	s2, err := StartResolve(dir2, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := s2.Merge("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !m2.view.Merge.AutoApplied || m2.view.Merge.InitialResult != "one\ntwo\nthree\nfour\n" {
		t.Errorf("auto-applied initial result = %q", m2.view.Merge.InitialResult)
	}
}

func TestChangesSession(t *testing.T) {
	dir := conflictRepo(t, map[string][3]string{
		"theirs-deleted.txt": {"base\n", "base mine\n", gone},
		"yours-deleted.txt":  {"base\n", gone, "base theirs\n"},
		"both.txt":           {"1\n", "2\n", "3\n"},
	})
	s, err := StartResolve(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, remote, label string }{
		{"theirs-deleted.txt", "base mine\n", "main"},
		{"yours-deleted.txt", "base theirs\n", "feature"},
	} {
		d, err := s.Changes(tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if d.view.Mode != Diff || d.view.Text.Left != "base\n" || d.view.Text.Right != tc.remote {
			t.Errorf("%s: local %q remote %q", tc.path, d.view.Text.Left, d.view.Text.Right)
		}
		if d.view.Text.LeftLabel != "Base" || d.view.Text.RightLabel != tc.label {
			t.Errorf("%s: labels %q, %q", tc.path, d.view.Text.LeftLabel, d.view.Text.RightLabel)
		}
		if len(d.view.Text.Chunks) == 0 {
			t.Errorf("%s: no chunks", tc.path)
		}
	}
	if _, err := s.Changes("both.txt"); !errors.Is(err, ErrCannotOpen) {
		t.Errorf("changes of a text conflict: %v, want ErrCannotOpen", err)
	}
	if out := gitIn(t, dir, "status", "--porcelain"); !strings.Contains(out, "UD theirs-deleted.txt") {
		t.Errorf("opening changes must not touch the index:\n%s", out)
	}
}

func TestEdited(t *testing.T) {
	dir := conflictRepo(t, map[string][3]string{
		"a.txt": {"1\n2\n3\n", "1\nM\n3\n", "1\nF\n3\n"},
		"b.txt": {"1\n2\n3\n", "1\nM\n3\n", "1\nF\n3\n"},
	})
	s, err := StartResolve(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Edited([]string{"a.txt", "b.txt"}); err != nil || len(got) != 0 {
		t.Fatalf("untouched: %v, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("1\nhand\n3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Edited([]string{"a.txt", "b.txt"}); err != nil || len(got) != 1 || got[0] != "b.txt" {
		t.Errorf("after editing b.txt: %v, %v", got, err)
	}
	if m, err := s.Merge("b.txt"); err != nil || !m.view.Merge.EditedOnDisk {
		t.Errorf("merge of an edited file: EditedOnDisk false (%v)", err)
	}
	if m, err := s.Merge("a.txt"); err != nil || m.view.Merge.EditedOnDisk {
		t.Errorf("merge of an untouched file: EditedOnDisk true (%v)", err)
	}
}

// TestResolveExitCode: a Conflicts window's exit code comes from Git when
// it closes, whatever Quit said, except an error exit.
func TestResolveExitCode(t *testing.T) {
	dir := conflictRepo(t, map[string][3]string{"a.txt": {"1\n", "M\n", "F\n"}})
	s, err := StartResolve(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		code int
		quit bool
		want int
	}{{0, false, 1}, {0, true, 1}, {2, true, 2}} {
		if got := s.ExitCode(c.code, c.quit); got != c.want {
			t.Errorf("conflicts left: ExitCode(%d, %v) = %d, want %d", c.code, c.quit, got, c.want)
		}
	}
	if _, err := s.Accept([]string{"a.txt"}, gitconflict.Yours); err != nil {
		t.Fatal(err)
	}
	if got := s.ExitCode(1, true); got != 0 {
		t.Errorf("none left: ExitCode(1, true) = %d, want 0", got)
	}
}
