package gitconflict

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/textio"
)

// accept runs Accept with the operation's sides.
func accept(t *testing.T, tr *testRepo, side Side, paths ...string) {
	t.Helper()
	r := tr.open("")
	if err := r.Accept(paths, side, r.Operation().Sides()); err != nil {
		t.Fatalf("Accept(%q, %s): %v", paths, side, err)
	}
}

func checkResolved(t *testing.T, tr *testRepo) {
	t.Helper()
	if u := tr.unmerged(); len(u) != 0 {
		t.Errorf("still unmerged: %q", u)
	}
}

func TestAcceptBothModified(t *testing.T) {
	t.Run("yours", func(t *testing.T) {
		tr := simpleConflict(t)
		mergeFeature(tr)
		accept(t, tr, Yours, "a.txt")
		checkResolved(t, tr)
		if got := tr.read("a.txt"); got != "1\nM\n3\n" {
			t.Errorf("a.txt = %q, want main's", got)
		}
		if st := tr.porcelain("a.txt"); st != "" {
			t.Errorf("status = %q, want clean (same as HEAD)", st)
		}
	})
	t.Run("theirs", func(t *testing.T) {
		tr := simpleConflict(t)
		mergeFeature(tr)
		accept(t, tr, Theirs, "a.txt")
		checkResolved(t, tr)
		if got := tr.read("a.txt"); got != "1\nF\n3\n" {
			t.Errorf("a.txt = %q, want feature's", got)
		}
		if st := tr.porcelain("a.txt"); st != "M " {
			t.Errorf("status = %q, want staged modification", st)
		}
	})
}

func TestAcceptModifyDelete(t *testing.T) {
	theirsDeleted := func(t *testing.T) *testRepo {
		tr := divergent(t, editLine2("a.txt", "2"), editLine2("a.txt", "M"), func(tr *testRepo) { tr.remove("a.txt") })
		mergeFeature(tr)
		return tr
	}
	t.Run("keep modified", func(t *testing.T) {
		tr := theirsDeleted(t)
		accept(t, tr, Yours, "a.txt")
		checkResolved(t, tr)
		if got := tr.read("a.txt"); got != "1\nM\n3\n" {
			t.Errorf("a.txt = %q", got)
		}
	})
	t.Run("take the deletion", func(t *testing.T) {
		tr := theirsDeleted(t)
		accept(t, tr, Theirs, "a.txt")
		checkResolved(t, tr)
		if tr.exists("a.txt") {
			t.Error("a.txt should be deleted")
		}
		if st := tr.porcelain("a.txt"); st != "D " {
			t.Errorf("status = %q, want staged deletion", st)
		}
	})
	t.Run("yours deleted", func(t *testing.T) {
		tr := divergent(t, editLine2("a.txt", "2"), func(tr *testRepo) { tr.remove("a.txt") }, editLine2("a.txt", "F"))
		mergeFeature(tr)
		accept(t, tr, Yours, "a.txt")
		checkResolved(t, tr)
		if tr.exists("a.txt") {
			t.Error("a.txt should stay deleted")
		}
	})
}

func TestAcceptWhileRebasing(t *testing.T) {
	t.Run("yours is your commit", func(t *testing.T) {
		tr := simpleConflict(t)
		rebaseFeature(tr)
		accept(t, tr, Yours, "a.txt")
		checkResolved(t, tr)
		if got := tr.read("a.txt"); got != "1\nF\n3\n" {
			t.Errorf("a.txt = %q, want feature's (your commit)", got)
		}
	})
	t.Run("theirs is the upstream", func(t *testing.T) {
		tr := simpleConflict(t)
		rebaseFeature(tr)
		accept(t, tr, Theirs, "a.txt")
		checkResolved(t, tr)
		if got := tr.read("a.txt"); got != "1\nM\n3\n" {
			t.Errorf("a.txt = %q, want main's (upstream)", got)
		}
	})
	t.Run("your commit deleted it", func(t *testing.T) {
		tr := divergent(t, editLine2("a.txt", "2"), editLine2("a.txt", "M"), func(tr *testRepo) { tr.remove("a.txt") })
		rebaseFeature(tr)
		accept(t, tr, Yours, "a.txt")
		checkResolved(t, tr)
		if tr.exists("a.txt") {
			t.Error("a.txt should be deleted (your commit deleted it)")
		}
	})
}

func TestAcceptSeveralAndBinary(t *testing.T) {
	names := []string{"dir with space/a.txt", "b.txt", "ü.txt"}
	both := func(line string) func(*testRepo) {
		return func(tr *testRepo) {
			for _, n := range names {
				editLine2(n, line)(tr)
			}
			tr.write("img.bin", "\x00"+line)
		}
	}
	tr := divergent(t, both("2"), both("M"), both("F"))
	mergeFeature(tr)
	old := pathsPerCommand
	pathsPerCommand = 2 // exercise the chunking
	t.Cleanup(func() { pathsPerCommand = old })

	accept(t, tr, Theirs, append(names, "img.bin", "not-conflicted.txt")...)
	checkResolved(t, tr)
	for _, n := range names {
		if got := tr.read(n); got != "1\nF\n3\n" {
			t.Errorf("%s = %q", n, got)
		}
	}
	if got := tr.read("img.bin"); got != "\x00F" {
		t.Errorf("img.bin = %q, want theirs byte for byte", got)
	}
}

func TestAcceptOnlyTheGivenPaths(t *testing.T) {
	tr := divergent(t,
		func(tr *testRepo) { editLine2("src/a.txt", "2")(tr); editLine2("srcfile.txt", "2")(tr) },
		func(tr *testRepo) { editLine2("src/a.txt", "M")(tr); editLine2("srcfile.txt", "M")(tr) },
		func(tr *testRepo) { editLine2("src/a.txt", "F")(tr); editLine2("srcfile.txt", "F")(tr) })
	mergeFeature(tr)
	// "src" is a directory, not a conflicted file: nothing under it changes.
	accept(t, tr, Theirs, "src")
	if got := tr.unmerged(); !reflect.DeepEqual(got, []string{"src/a.txt", "srcfile.txt"}) {
		t.Errorf("unmerged = %q, want both still", got)
	}
}

func TestAcceptBothDeleted(t *testing.T) {
	tr := divergent(t, editLine2("a.txt", "2"),
		func(tr *testRepo) { tr.git("mv", "a.txt", "b.txt") },
		func(tr *testRepo) { tr.git("mv", "a.txt", "c.txt") })
	mergeFeature(tr)
	accept(t, tr, Yours, tr.unmerged()...)
	checkResolved(t, tr)
}

func TestAcceptSubmoduleRefused(t *testing.T) {
	tr := simpleConflict(t)
	mergeFeature(tr)
	r := tr.open("")
	// Add an unmerged gitlink "mod" next to a.txt, with three commits of
	// another repository as its stages.
	sub := newTestRepo(t)
	sub.write("s.txt", "0\n")
	sub.commit("s0")
	c0 := strings.TrimSpace(sub.git("rev-parse", "HEAD"))
	sub.write("s.txt", "1\n")
	sub.commit("s1")
	c1 := strings.TrimSpace(sub.git("rev-parse", "HEAD"))
	sub.git("checkout", "-q", "-b", "other", c0)
	sub.write("s.txt", "2\n")
	sub.commit("s2")
	c2 := strings.TrimSpace(sub.git("rev-parse", "HEAD"))
	info := "160000 " + c0 + " 1\tmod\n160000 " + c1 + " 2\tmod\n160000 " + c2 + " 3\tmod\n"
	if _, err := r.gitIn(strings.NewReader(info), "update-index", "--index-info"); err != nil {
		t.Fatal(err)
	}
	err := r.Accept([]string{"a.txt", "mod"}, Yours, r.Operation().Sides())
	if !errors.Is(err, ErrSubmodule) {
		t.Fatalf("Accept(submodule) = %v, want ErrSubmodule", err)
	}
	if got := tr.unmerged(); !reflect.DeepEqual(got, []string{"a.txt", "mod"}) {
		t.Errorf("unmerged = %q, want nothing resolved", got)
	}
}

func TestSaveMerged(t *testing.T) {
	tr := simpleConflict(t)
	mergeFeature(tr)
	r := tr.open("")
	if err := r.SaveMerged("a.txt", "1\nmerged\n3\n", textio.CRLF); err != nil {
		t.Fatal(err)
	}
	checkResolved(t, tr)
	if got := tr.read("a.txt"); got != "1\r\nmerged\r\n3\r\n" {
		t.Errorf("a.txt = %q, want CRLF result", got)
	}
	if st := tr.porcelain("a.txt"); st != "M " {
		t.Errorf("status = %q, want staged", st)
	}
}
