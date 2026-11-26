package gitconflict

import (
	"strings"
	"testing"
)

func TestSides(t *testing.T) {
	merge := SidesFor(OpMerge)
	if merge.Stage(Yours) != StageOurs || merge.Stage(Theirs) != StageTheirs {
		t.Errorf("merge stages: yours %d theirs %d", merge.Stage(Yours), merge.Stage(Theirs))
	}
	if merge.CheckoutFlag(Yours) != "--ours" || merge.CheckoutFlag(Theirs) != "--theirs" {
		t.Errorf("merge flags: %s %s", merge.CheckoutFlag(Yours), merge.CheckoutFlag(Theirs))
	}
	rebase := SidesFor(OpRebase)
	if rebase.Stage(Yours) != StageTheirs || rebase.Stage(Theirs) != StageOurs {
		t.Errorf("rebase stages: yours %d theirs %d", rebase.Stage(Yours), rebase.Stage(Theirs))
	}
	if rebase.CheckoutFlag(Yours) != "--theirs" || rebase.CheckoutFlag(Theirs) != "--ours" {
		t.Errorf("rebase flags: %s %s", rebase.CheckoutFlag(Yours), rebase.CheckoutFlag(Theirs))
	}
	for _, op := range []OpKind{OpNone, OpCherryPick, OpRevert} {
		if SidesFor(op).Swapped {
			t.Errorf("%s must not swap", op)
		}
	}
}

func checkOp(t *testing.T, got Operation, kind OpKind, text, yours, theirs, next string) {
	t.Helper()
	if got.Kind != kind || got.Text() != text || got.YoursLabel != yours || got.TheirsLabel != theirs || got.Next != next {
		t.Errorf("Operation = %+v (%q)\nwant kind %s, %q, yours %q, theirs %q, next %q",
			got, got.Text(), kind, text, yours, theirs, next)
	}
}

func TestOperationMerge(t *testing.T) {
	tr := simpleConflict(t)
	mergeFeature(tr)
	op := tr.open("").Operation()
	checkOp(t, op, OpMerge, "Merging feature into main", "main", "feature", "git merge --continue")
	if len(op.Sentence) != 4 || !op.Sentence[1].Strong || !op.Sentence[3].Strong || op.Sentence[0].Strong {
		t.Errorf("names must be strong: %+v", op.Sentence)
	}
}

func TestOperationMergeRemoteBranchAndDetachedHead(t *testing.T) {
	tr := simpleConflict(t)
	tr.git("update-ref", "refs/remotes/origin/feature", "feature")
	tr.git("branch", "-q", "-D", "feature")
	head := strings.TrimSpace(tr.git("rev-parse", "--short", "HEAD"))
	tr.git("checkout", "-q", "--detach")
	tr.gitConflict("merge", "--no-edit", "origin/feature")
	op := tr.open("").Operation()
	checkOp(t, op, OpMerge, "Merging origin/feature into "+head, head, "origin/feature", "git merge --continue")
}

func TestOperationRebase(t *testing.T) {
	for _, backend := range []string{"--merge", "--apply"} {
		t.Run(backend, func(t *testing.T) {
			tr := simpleConflict(t)
			rebaseFeature(tr, backend)
			op := tr.open("").Operation()
			checkOp(t, op, OpRebase, "Rebasing feature onto main", "feature", "main", "git rebase --continue")
			if !op.Sides().Swapped {
				t.Error("rebase must swap the sides")
			}
		})
	}
}

func TestOperationRebaseDetached(t *testing.T) {
	tr := simpleConflict(t)
	tr.git("checkout", "-q", "--detach", "feature")
	short := strings.TrimSpace(tr.git("rev-parse", "--short", "HEAD"))
	tr.gitConflict("rebase", "main")
	op := tr.open("").Operation()
	// The detached commit is named by a branch pointing at it.
	if op.Kind != OpRebase || op.TheirsLabel != "main" || (op.YoursLabel != "feature" && op.YoursLabel != short) {
		t.Errorf("Operation = %+v", op)
	}
}

func TestOperationCherryPick(t *testing.T) {
	tr := simpleConflict(t)
	short := strings.TrimSpace(tr.git("rev-parse", "--short", "feature"))
	tr.gitConflict("cherry-pick", "feature")
	op := tr.open("").Operation()
	checkOp(t, op, OpCherryPick, "Cherry-picking "+short+" feature change", "main", short, "git cherry-pick --continue")
}

func TestOperationRevert(t *testing.T) {
	tr := newTestRepo(t)
	tr.write("a.txt", "1\n2\n3\n")
	tr.commit("base")
	tr.write("a.txt", "1\nX\n3\n")
	tr.commit("make it X")
	short := strings.TrimSpace(tr.git("rev-parse", "--short", "HEAD"))
	tr.write("a.txt", "1\nY\n3\n")
	tr.commit("make it Y")
	tr.gitConflict("revert", "--no-edit", short)
	op := tr.open("").Operation()
	checkOp(t, op, OpRevert, "Reverting "+short+" make it X", "main", short, "git revert --continue")
}

func TestOperationStashPop(t *testing.T) {
	tr := newTestRepo(t)
	tr.write("a.txt", "1\n2\n3\n")
	tr.commit("base")
	tr.write("a.txt", "1\nstashed\n3\n")
	tr.git("stash", "-q")
	tr.write("a.txt", "1\ncommitted\n3\n")
	tr.commit("committed")
	tr.gitConflict("stash", "pop")
	op := tr.open("").Operation()
	checkOp(t, op, OpNone, "Resolving conflicts", "main", "", "git commit")
}

func TestOperationWithoutConflicts(t *testing.T) {
	tr := newTestRepo(t)
	// No commit yet: HEAD names an unborn branch.
	op := tr.open("").Operation()
	checkOp(t, op, OpNone, "Resolving conflicts", "main", "", "git commit")
}
