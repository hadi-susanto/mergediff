package diff

import (
	"strings"
	"time"
	"unicode/utf16"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// SpanKind tells which text a Span refers to.
type SpanKind string

const (
	// Deleted spans are offsets into the left (old) text.
	Deleted SpanKind = "delete"
	// Inserted spans are offsets into the right (new) text.
	Inserted SpanKind = "insert"
)

// Span marks characters that differ between two texts. Offsets are in
// UTF-16 code units, the unit JavaScript and CodeMirror use.
type Span struct {
	Kind  SpanKind `json:"kind"`
	Start int      `json:"start"`
	End   int      `json:"end"` // exclusive
}

// MergeGap is the gap below which neighbouring spans of the same kind are
// merged, to avoid noisy highlighting.
const MergeGap = 3

// charDiffTimeout bounds the character diff. Highlighting does not need to be
// minimal, so a slightly worse result on huge chunks is fine.
const charDiffTimeout = time.Second

// Chars returns the deleted spans of left and the inserted spans of right,
// each list sorted by offset. Spans of the same kind less than MergeGap
// characters apart are merged unless the gap contains a newline.
func Chars(left, right string) []Span {
	if left == right {
		return nil
	}
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = charDiffTimeout
	diffs := dmp.DiffMain(left, right, false)

	var del, ins spanBuilder
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			del.skip(d.Text)
			ins.skip(d.Text)
		case diffmatchpatch.DiffDelete:
			del.add(Deleted, d.Text)
		case diffmatchpatch.DiffInsert:
			ins.add(Inserted, d.Text)
		}
	}
	return append(del.spans, ins.spans...)
}

// spanBuilder collects spans for one side, merging those separated by a
// short gap.
type spanBuilder struct {
	pos   int    // current UTF-16 offset
	gap   string // unchanged text since the last span
	spans []Span
}

func (b *spanBuilder) skip(text string) {
	b.pos += utf16Len(text)
	b.gap += text
}

func (b *spanBuilder) add(kind SpanKind, text string) {
	n := utf16Len(text)
	last := len(b.spans) - 1
	if last >= 0 && len([]rune(b.gap)) < MergeGap && !strings.Contains(b.gap, "\n") {
		b.spans[last].End = b.pos + n
	} else {
		b.spans = append(b.spans, Span{Kind: kind, Start: b.pos, End: b.pos + n})
	}
	b.pos += n
	b.gap = ""
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}
