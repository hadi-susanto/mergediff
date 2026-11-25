package gitconflict

import "testing"

// divergent builds main and feature from a common base commit: base runs
// first and is committed on main, then onFeature on branch feature, then
// onMain on main. It returns with main checked out and a clean tree.
func divergent(t *testing.T, base, onMain, onFeature func(tr *testRepo)) *testRepo {
	t.Helper()
	tr := newTestRepo(t)
	base(tr)
	tr.commit("base")
	tr.git("checkout", "-q", "-b", "feature")
	onFeature(tr)
	tr.commit("feature change")
	tr.git("checkout", "-q", "main")
	onMain(tr)
	tr.commit("main change")
	return tr
}

// editLine2 writes name with three lines, line replacing the middle one.
func editLine2(name, line string) func(tr *testRepo) {
	return func(tr *testRepo) { tr.write(name, "1\n"+line+"\n3\n") }
}

// simpleConflict is a.txt changed on both branches: main has "M", feature
// has "F" on line 2.
func simpleConflict(t *testing.T) *testRepo {
	return divergent(t, editLine2("a.txt", "2"), editLine2("a.txt", "M"), editLine2("a.txt", "F"))
}

// mergeFeature merges feature into main and expects conflicts.
func mergeFeature(tr *testRepo) {
	tr.t.Helper()
	tr.gitConflict("merge", "--no-edit", "feature")
}

// rebaseFeature rebases feature onto main and expects conflicts.
func rebaseFeature(tr *testRepo, extra ...string) {
	tr.t.Helper()
	tr.git("checkout", "-q", "feature")
	tr.gitConflict(append([]string{"rebase"}, append(extra, "main")...)...)
}
