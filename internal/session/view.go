package session

import "github.com/hadi-susanto/mergediff/internal/merge"

// View is what the frontend gets for a window (LoadSession). Only the
// parts a mode uses are set; the others are null in JSON: Text for a
// diff, the scratch window and a merge, Merge for a merge only. A
// directory diff and a Conflicts window have neither.
type View struct {
	Mode Mode `json:"mode"`
	// Title names the file being merged or compared, for the window title.
	Title string `json:"title"`
	// Paths: merge: BASE, LOCAL, REMOTE, MERGED; diff, dirdiff: LEFT,
	// RIGHT; resolve: the repository's top level.
	Paths []string   `json:"paths"`
	Text  *Texts     `json:"text"`
	Merge *MergeView `json:"merge"`
}

// Texts are the two side panes: LEFT and RIGHT in a diff, LOCAL (yours)
// and REMOTE (theirs) in a merge.
type Texts struct {
	Left  string `json:"left"`
	Right string `json:"right"`
	// LeftLabel and RightLabel name the panes when they are not files:
	// branch names in a merge opened from the Conflicts window, "Base" and
	// a branch in its Show changes diff. Empty means the file names.
	LeftLabel  string `json:"leftLabel"`
	RightLabel string `json:"rightLabel"`
	// Chunks: a merge's chunks; in a diff they compare LEFT with RIGHT and
	// are Unchanged or RemoteOnly, with Base ranges equal to Local ranges.
	Chunks   []merge.Chunk `json:"chunks"`
	Language string        `json:"language"` // from the file extension; "" if unknown
	EOL      string        `json:"eol"`      // "LF" or "CRLF"; a merge writes MERGED in it
}

// MergeView is what only a merge window has.
type MergeView struct {
	Base          string `json:"base"`
	InitialResult string `json:"initialResult"`
	// AutoApplied reports that InitialResult already contains every
	// non-conflicting chunk (--auto-apply), so the UI marks them applied.
	AutoApplied bool `json:"autoApplied"`
	// EditedOnDisk: the output file was edited since Git stopped (a merge
	// opened from the Conflicts window), so saving asks first.
	EditedOnDisk bool `json:"editedOnDisk"`
}
