package diff

import (
	"slices"
	"strings"
	"time"
	"unicode"
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
// each list sorted by offset. A span that cuts through a word is widened to
// the whole word ("[sampl]e" becomes "[sample]"). Spans of the same kind
// less than MergeGap characters apart are merged unless the gap contains a
// newline.
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
	return append(widenToWords(left, del.spans), widenToWords(right, ins.spans)...)
}

// isWordRune reports whether r is part of a word: a letter, digit or '_'.
func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// widenToWords widens spans of text whose ends fall inside a word to the
// word's boundaries, then merges spans that now touch, overlap or are less
// than MergeGap characters apart (unless a newline lies between them).
func widenToWords(text string, spans []Span) []Span {
	if len(spans) == 0 {
		return spans
	}
	runes := []rune(text)
	// offs[i] is the UTF-16 offset of rune i; offs[len(runes)] is the length.
	offs := make([]int, len(runes)+1)
	for i, r := range runes {
		offs[i+1] = offs[i] + utf16.RuneLen(r)
	}
	out := make([]Span, 0, len(spans))
	for _, s := range spans {
		a, _ := slices.BinarySearch(offs, s.Start)
		b, _ := slices.BinarySearch(offs, s.End)
		for a > 0 && isWordRune(runes[a-1]) && isWordRune(runes[a]) {
			a--
		}
		for b < len(runes) && isWordRune(runes[b-1]) && isWordRune(runes[b]) {
			b++
		}
		s.Start, s.End = offs[a], offs[b]
		if n := len(out); n > 0 {
			prev := &out[n-1]
			pb, _ := slices.BinarySearch(offs, prev.End)
			gap := runes[pb:max(pb, a)]
			if s.Start <= prev.End || (len(gap) < MergeGap && !slices.Contains(gap, '\n')) {
				prev.End = max(prev.End, s.End)
				continue
			}
		}
		out = append(out, s)
	}
	return out
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
