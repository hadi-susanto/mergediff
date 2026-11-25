package gitconflict

import (
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// list opens tr from the top and lists every conflict with the
// operation's sides.
func list(t *testing.T, tr *testRepo) (*Repo, []Conflict) {
	t.Helper()
	r := tr.open("")
	cs, err := r.List(nil, r.Operation().Sides())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return r, cs
}

// only returns the single conflict in cs, failing otherwise.
func only(t *testing.T, cs []Conflict, path string) *Conflict {
	t.Helper()
	if len(cs) != 1 || cs[0].Path != path {
		t.Fatalf("conflicts = %+v, want only %s", cs, path)
	}
	return &cs[0]
}

func checkConflict(t *testing.T, c *Conflict, yours, theirs Status, kind Kind, st State) {
	t.Helper()
	if c.Yours != yours || c.Theirs != theirs || c.Kind != kind || c.State != st {
		t.Errorf("%s: yours %s theirs %s kind %s state %s; want %s %s %s %s",
			c.Path, c.Yours, c.Theirs, c.Kind, c.State, yours, theirs, kind, st)
	}
}

func content(t *testing.T, r *Repo, c *Conflict, stage int) string {
	t.Helper()
	data, err := r.Content(c, stage)
	if err != nil {
		t.Fatalf("Content(%s, %d): %v", c.Path, stage, err)
	}
	return string(data)
}

func TestListBothModified(t *testing.T) {
	tr := simpleConflict(t)
	mergeFeature(tr)
	r, cs := list(t, tr)
	c := only(t, cs, "a.txt")
	checkConflict(t, c, Modified, Modified, KindText, StateConflict)
	for stage, want := range map[int]string{StageBase: "1\n2\n3\n", StageOurs: "1\nM\n3\n", StageTheirs: "1\nF\n3\n"} {
		if got := content(t, r, c, stage); got != want {
			t.Errorf("stage %d = %q, want %q", stage, got, want)
		}
	}
}

func TestListBothAdded(t *testing.T) {
	tr := divergent(t, func(tr *testRepo) { tr.write("keep.txt", "k\n") },
		func(tr *testRepo) { tr.write("new.txt", "main\n") },
		func(tr *testRepo) { tr.write("new.txt", "feature\n") })
	mergeFeature(tr)
	r, cs := list(t, tr)
	c := only(t, cs, "new.txt")
	checkConflict(t, c, Added, Added, KindText, StateConflict)
	if c.Has(StageBase) {
		t.Error("add/add has no base stage")
	}
	if data, err := r.Content(c, StageBase); data != nil || err != nil {
		t.Errorf("missing base = %q, %v; want empty, nil", data, err)
	}
	if got := content(t, r, c, StageTheirs); got != "feature\n" {
		t.Errorf("theirs = %q", got)
	}
}

func TestListModifyDelete(t *testing.T) {
	t.Run("theirs deleted", func(t *testing.T) {
		tr := divergent(t, editLine2("a.txt", "2"), editLine2("a.txt", "M"), func(tr *testRepo) { tr.remove("a.txt") })
		mergeFeature(tr)
		_, cs := list(t, tr)
		checkConflict(t, only(t, cs, "a.txt"), Modified, Deleted, KindText, StateModifyDelete)
	})
	t.Run("yours deleted", func(t *testing.T) {
		tr := divergent(t, editLine2("a.txt", "2"), func(tr *testRepo) { tr.remove("a.txt") }, editLine2("a.txt", "F"))
		mergeFeature(tr)
		r, cs := list(t, tr)
		c := only(t, cs, "a.txt")
		checkConflict(t, c, Deleted, Modified, KindText, StateModifyDelete)
		if data, err := r.Content(c, StageOurs); data != nil || err != nil {
			t.Errorf("deleted side = %q, %v; want empty, nil", data, err)
		}
	})
}

func TestListBinaryAndNotUTF8(t *testing.T) {
	tr := divergent(t,
		func(tr *testRepo) {
			tr.write("img.bin", "\x89PNG\x00base")
			tr.write("latin1.txt", "caf\xe9 base\n")
			tr.write("mixed.txt", "text\n")
		},
		func(tr *testRepo) {
			tr.write("img.bin", "\x89PNG\x00main")
			tr.write("latin1.txt", "caf\xe9 main\n")
			tr.write("mixed.txt", "main\x00binary\n")
		},
		func(tr *testRepo) {
			tr.write("img.bin", "\x89PNG\x00feature")
			tr.write("latin1.txt", "caf\xe9 feature\n")
			tr.write("mixed.txt", "feature text\n")
		})
	mergeFeature(tr)
	_, cs := list(t, tr)
	if len(cs) != 3 {
		t.Fatalf("conflicts = %+v", cs)
	}
	checkConflict(t, &cs[0], Modified, Modified, KindBinary, StateBinary)  // img.bin
	checkConflict(t, &cs[1], Modified, Modified, KindNotUTF8, StateBinary) // latin1.txt
	checkConflict(t, &cs[2], Modified, Modified, KindBinary, StateBinary)  // mixed.txt: one side binary
}

func TestListSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	tr := divergent(t,
		func(tr *testRepo) { mustSymlink(t, "base-target", tr.path("link")) },
		func(tr *testRepo) { os.Remove(tr.path("link")); mustSymlink(t, "main-target", tr.path("link")) },
		func(tr *testRepo) { os.Remove(tr.path("link")); mustSymlink(t, "feature-target", tr.path("link")) })
	mergeFeature(tr)
	r, cs := list(t, tr)
	c := only(t, cs, "link")
	checkConflict(t, c, Modified, Modified, KindSymlink, StateSymlink)
	if got := content(t, r, c, StageOurs); got != "main-target" {
		t.Errorf("symlink stage = %q, want its target", got)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestListSubmodule(t *testing.T) {
	requireGit(t)
	sub := newTestRepo(t)
	sub.write("s.txt", "0\n")
	sub.commit("s0")
	c0 := strings.TrimSpace(sub.git("rev-parse", "HEAD"))
	commitSub := func(text string) string {
		sub.write("s.txt", text)
		sub.commit(text)
		return strings.TrimSpace(sub.git("rev-parse", "HEAD"))
	}
	// Two commits that both descend from c0 but not from each other, so
	// Git cannot fast-forward the submodule.
	c1 := commitSub("1\n")
	sub.git("checkout", "-q", "-b", "other", c0)
	c2 := commitSub("2\n")

	tr := newTestRepo(t)
	if out, err := tr.run("-c", "protocol.file.allow=always", "submodule", "add", "-q", sub.dir, "mod"); err != nil {
		t.Skipf("cannot add a local submodule: %v\n%s", err, out)
	}
	// Commit the index as is: "add -A" would stage the submodule's
	// checked-out commit instead.
	tr.git("update-index", "--cacheinfo", "160000,"+c0+",mod")
	tr.git("commit", "-q", "-m", "base")
	setSub := func(commit string) func(*testRepo) {
		return func(tr *testRepo) {
			tr.git("update-index", "--cacheinfo", "160000,"+commit+",mod")
			tr.git("commit", "-q", "-m", "sub "+commit[:7])
		}
	}
	tr.git("checkout", "-q", "-b", "feature")
	setSub(c1)(tr)
	tr.git("checkout", "-q", "main")
	setSub(c2)(tr)
	tr.gitConflict("merge", "--no-edit", "feature")
	_, cs := list(t, tr)
	checkConflict(t, only(t, cs, "mod"), Modified, Modified, KindSubmodule, StateSubmodule)
}

func TestListCRLFMatchesCheckout(t *testing.T) {
	tr := newTestRepo(t)
	tr.write(".gitattributes", "*.txt text eol=crlf\n")
	tr.write("a.txt", "1\r\n2\r\n3\r\n")
	tr.commit("base")
	tr.git("checkout", "-q", "-b", "feature")
	tr.write("a.txt", "1\r\nF\r\n3\r\n")
	tr.commit("feature")
	tr.git("checkout", "-q", "main")
	tr.write("a.txt", "1\r\nM\r\n3\r\n")
	tr.commit("main")
	// What a checkout of main's version produces.
	os.Remove(tr.path("a.txt"))
	tr.git("checkout", "--", "a.txt")
	want := tr.read("a.txt")
	if !strings.Contains(want, "\r\n") {
		t.Fatalf("checkout did not produce CRLF: %q", want)
	}
	mergeFeature(tr)
	r, cs := list(t, tr)
	c := only(t, cs, "a.txt")
	if got := content(t, r, c, StageOurs); got != want {
		t.Errorf("stage 2 = %q, want the checkout bytes %q", got, want)
	}
	if got := content(t, r, c, StageBase); got != "1\r\n2\r\n3\r\n" {
		t.Errorf("base = %q, want CRLF", got)
	}
}

func TestListSpecialPathsFromSubdirectory(t *testing.T) {
	names := []string{"dir with space/ünïcode file.txt", "日本/文字.txt", "top.txt"}
	both := func(line string) func(*testRepo) {
		return func(tr *testRepo) {
			for _, n := range names {
				editLine2(n, line)(tr)
			}
		}
	}
	tr := divergent(t, both("2"), both("M"), both("F"))
	mergeFeature(tr)

	r := tr.open("日本")
	cs, err := r.List(nil, r.Operation().Sides())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range cs {
		got = append(got, c.Path)
	}
	if !reflect.DeepEqual(got, tr.unmerged()) || len(got) != 3 {
		t.Errorf("paths = %q, want %q (top-relative, unquoted)", got, tr.unmerged())
	}
	c := cs[0]
	if text := content(t, r, &c, StageTheirs); text != "1\nF\n3\n" {
		t.Errorf("content of %s = %q", c.Path, text)
	}
}

func TestListPathLimits(t *testing.T) {
	names := []string{"src/a.txt", "src/b.txt", "docs/c.txt", "lit.txt"}
	if runtime.GOOS != "windows" {
		names = append(names, "lit*.txt")
	}
	both := func(line string) func(*testRepo) {
		return func(tr *testRepo) {
			for _, n := range names {
				editLine2(n, line)(tr)
			}
		}
	}
	tr := divergent(t, both("2"), both("M"), both("F"))
	mergeFeature(tr)

	r := tr.open("src")
	paths := func(args ...string) []string {
		t.Helper()
		specs, err := r.Pathspecs(args)
		if err != nil {
			t.Fatal(err)
		}
		cs, err := r.List(specs, r.Operation().Sides())
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, c := range cs {
			out = append(out, c.Path)
		}
		return out
	}
	if got := paths("."); !reflect.DeepEqual(got, []string{"src/a.txt", "src/b.txt"}) {
		t.Errorf(". from src = %q", got)
	}
	if got := paths("b.txt", "../docs"); !reflect.DeepEqual(got, []string{"docs/c.txt", "src/b.txt"}) {
		t.Errorf("b.txt ../docs = %q", got)
	}
	if got := paths(".."); len(got) != len(names) {
		t.Errorf(".. = %q, want all %d", got, len(names))
	}
	if runtime.GOOS != "windows" {
		// Literal pathspecs: "*" is part of the name, not a wildcard.
		if got := paths("../lit*.txt"); !reflect.DeepEqual(got, []string{"lit*.txt"}) {
			t.Errorf("../lit*.txt = %q, want only the literal name", got)
		}
	}
}

func TestListRebaseSwapsSides(t *testing.T) {
	// feature deletes the file, main modifies it. Rebasing feature onto
	// main replays the deletion: Git's "ours" is main (modified), "theirs"
	// is feature (deleted). Yours must be feature's side.
	tr := divergent(t, editLine2("a.txt", "2"), editLine2("a.txt", "M"), func(tr *testRepo) { tr.remove("a.txt") })
	rebaseFeature(tr)
	_, cs := list(t, tr)
	checkConflict(t, only(t, cs, "a.txt"), Deleted, Modified, KindText, StateModifyDelete)

	tr2 := simpleConflict(t)
	rebaseFeature(tr2)
	r, cs := list(t, tr2)
	c := only(t, cs, "a.txt")
	sides := r.Operation().Sides()
	if got := content(t, r, c, sides.Stage(Yours)); got != "1\nF\n3\n" {
		t.Errorf("yours while rebasing = %q, want feature's commit", got)
	}
	if got := content(t, r, c, sides.Stage(Theirs)); got != "1\nM\n3\n" {
		t.Errorf("theirs while rebasing = %q, want the upstream", got)
	}
}

func TestListOtherOperations(t *testing.T) {
	t.Run("cherry-pick", func(t *testing.T) {
		tr := simpleConflict(t)
		tr.gitConflict("cherry-pick", "feature")
		_, cs := list(t, tr)
		checkConflict(t, only(t, cs, "a.txt"), Modified, Modified, KindText, StateConflict)
	})
	t.Run("revert", func(t *testing.T) {
		tr := newTestRepo(t)
		tr.write("a.txt", "1\n2\n3\n")
		tr.commit("base")
		tr.write("a.txt", "1\nX\n3\n")
		tr.commit("x")
		tr.write("a.txt", "1\nY\n3\n")
		tr.commit("y")
		tr.gitConflict("revert", "--no-edit", "HEAD~1")
		_, cs := list(t, tr)
		checkConflict(t, only(t, cs, "a.txt"), Modified, Modified, KindText, StateConflict)
	})
	t.Run("stash pop", func(t *testing.T) {
		tr := newTestRepo(t)
		tr.write("a.txt", "1\n2\n3\n")
		tr.commit("base")
		tr.write("a.txt", "1\nstashed\n3\n")
		tr.git("stash", "-q")
		tr.write("a.txt", "1\ncommitted\n3\n")
		tr.commit("committed")
		tr.gitConflict("stash", "pop")
		r, cs := list(t, tr)
		c := only(t, cs, "a.txt")
		checkConflict(t, c, Modified, Modified, KindText, StateConflict)
		if got := content(t, r, c, StageTheirs); got != "1\nstashed\n3\n" {
			t.Errorf("theirs = %q, want the stashed text", got)
		}
	})
}

func TestListRenameConflict(t *testing.T) {
	// rename/rename: main renames a.txt to b.txt, feature to c.txt. Git
	// reports whatever paths it considers unmerged; they are listed as is.
	tr := divergent(t, editLine2("a.txt", "2"),
		func(tr *testRepo) { tr.git("mv", "a.txt", "b.txt") },
		func(tr *testRepo) { tr.git("mv", "a.txt", "c.txt") })
	mergeFeature(tr)
	_, cs := list(t, tr)
	var got []string
	for _, c := range cs {
		got = append(got, c.Path)
		if c.State != StateModifyDelete {
			t.Errorf("%s: state %s, want modifyDelete (one side lacks it)", c.Path, c.State)
		}
	}
	if len(got) == 0 || !reflect.DeepEqual(got, tr.unmerged()) {
		t.Errorf("paths = %q, want git's %q", got, tr.unmerged())
	}
}

func TestListNoConflicts(t *testing.T) {
	tr := newTestRepo(t)
	tr.write("a.txt", "a\n")
	tr.commit("base")
	_, cs := list(t, tr)
	if len(cs) != 0 {
		t.Errorf("conflicts = %+v, want none", cs)
	}
}

func TestReadingCreatesNoFiles(t *testing.T) {
	tr := divergent(t,
		func(tr *testRepo) {
			tr.write("a.txt", "1\n2\n3\n")
			tr.write("bin.dat", "\x00base")
			tr.write("gone.txt", "1\n2\n3\n")
		},
		func(tr *testRepo) {
			editLine2("a.txt", "M")(tr)
			tr.write("bin.dat", "\x00main")
			editLine2("gone.txt", "M")(tr)
			tr.write("added.txt", "main\n")
		},
		func(tr *testRepo) {
			editLine2("a.txt", "F")(tr)
			tr.write("bin.dat", "\x00feature")
			tr.remove("gone.txt")
			tr.write("added.txt", "feature\n")
		})
	mergeFeature(tr)
	before := tr.snapshot()
	r := tr.open("")
	op := r.Operation()
	cs, err := r.List(nil, op.Sides())
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 4 {
		t.Fatalf("conflicts = %+v", cs)
	}
	for i := range cs {
		for s := 1; s <= 3; s++ {
			if _, err := r.Content(&cs[i], s); err != nil {
				t.Fatal(err)
			}
		}
	}
	if after := tr.snapshot(); !reflect.DeepEqual(before, after) {
		t.Errorf("working tree changed while reading:\nbefore %q\nafter  %q", before, after)
	}
}
