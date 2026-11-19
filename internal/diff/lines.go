// Package diff computes line-level and character-level differences.
package diff

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Range is a half-open range of 0-based line indexes [Start, End).
type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Len returns the number of lines in the range.
func (r Range) Len() int { return r.End - r.Start }

// Hunk replaces lines A of the old sequence with lines B of the new one.
// At least one of A and B is non-empty.
type Hunk struct {
	A Range
	B Range
}

// maxLines bounds the input of Lines: go-diff's line mode encodes every
// distinct line as one Unicode code point, so it cannot handle more than
// about 1.1 million distinct lines. Larger inputs get a single hunk.
const maxLines = 1_000_000

// Lines returns a minimal line diff between a and b, as sorted hunks.
// Consecutive hunks are separated by at least one unchanged line.
// Every line except the last must end with "\n", as produced by
// textio.SplitLines; the slices are joined and re-split internally.
func Lines(a, b []string) []Hunk {
	if slices.Equal(a, b) {
		return nil
	}
	if len(a)+len(b) > maxLines {
		return []Hunk{{A: Range{0, len(a)}, B: Range{0, len(b)}}}
	}
	dmp := diffmatchpatch.New()
	// No timeout: a timeout also enables a "half-match" shortcut that can
	// return non-minimal diffs.
	dmp.DiffTimeout = 0
	ra, rb, _ := dmp.DiffLinesToRunes(strings.Join(a, ""), strings.Join(b, ""))
	diffs := dmp.DiffMainRunes(ra, rb, false)

	var hunks []Hunk
	var cur *Hunk
	posA, posB := 0, 0
	for _, d := range diffs {
		n := utf8.RuneCountInString(d.Text) // one rune per line
		if d.Type == diffmatchpatch.DiffEqual {
			cur = nil
			posA += n
			posB += n
			continue
		}
		if cur == nil {
			hunks = append(hunks, Hunk{A: Range{posA, posA}, B: Range{posB, posB}})
			cur = &hunks[len(hunks)-1]
		}
		if d.Type == diffmatchpatch.DiffDelete {
			posA += n
			cur.A.End = posA
		} else {
			posB += n
			cur.B.End = posB
		}
	}
	return hunks
}
