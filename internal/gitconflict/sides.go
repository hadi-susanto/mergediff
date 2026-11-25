package gitconflict

// Git's index stages of an unmerged path.
const (
	StageBase   = 1 // the common ancestor
	StageOurs   = 2 // "ours": HEAD, or the upstream while rebasing
	StageTheirs = 3 // "theirs": the branch being merged, or your commit while rebasing
)

// Side is a side as the user sees it.
type Side string

const (
	Yours  Side = "yours"
	Theirs Side = "theirs"
)

// Sides maps the user's Yours and Theirs to Git's stages and checkout flags.
//
// While rebasing, Git replays your commits onto the upstream, so its
// stage 2 ("ours") is the upstream and stage 3 ("theirs") is your own
// commit. Like the JetBrains IDEs, mergediff shows your commit as Yours
// and the upstream as Theirs, so the mapping is swapped. This is the only
// place that knows about the swap.
type Sides struct {
	// Swapped is true while rebasing.
	Swapped bool
}

// SidesFor returns the mapping for an operation in progress.
func SidesFor(op OpKind) Sides {
	return Sides{Swapped: op == OpRebase}
}

// Stage returns Git's index stage holding side.
func (s Sides) Stage(side Side) int {
	if (side == Yours) != s.Swapped {
		return StageOurs
	}
	return StageTheirs
}

// CheckoutFlag returns the "git checkout" flag that takes side.
func (s Sides) CheckoutFlag(side Side) string {
	if s.Stage(side) == StageOurs {
		return "--ours"
	}
	return "--theirs"
}
