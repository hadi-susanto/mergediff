// Package merge implements a classic diff3 three-way merge over lines.
package merge

import (
	"slices"
	"strings"

	"github.com/hadi-susanto/mergediff/internal/diff"
)

// ChunkKind classifies a chunk.
type ChunkKind string

const (
	Unchanged  ChunkKind = "unchanged"
	LocalOnly  ChunkKind = "local"
	RemoteOnly ChunkKind = "remote"
	BothSame   ChunkKind = "both"
	Conflict   ChunkKind = "conflict"
)

// Range is a half-open range of 0-based line indexes.
type Range = diff.Range

// Chunk is a region that lines up across the three files.
type Chunk struct {
	Kind   ChunkKind `json:"kind"`
	Base   Range     `json:"base"`
	Local  Range     `json:"local"`
	Remote Range     `json:"remote"`
}

// Merge splits base, local and remote into chunks that cover all three
// completely and in order, with no gaps or overlaps. Lines keep their
// terminators (see textio.SplitLines).
//
// Changes from the two sides whose base ranges overlap or touch are grouped
// into one chunk, as diff3 and git do; edits on adjacent lines therefore
// conflict.
func Merge(base, local, remote []string) []Chunk {
	hl := diff.Lines(base, local)
	hr := diff.Lines(base, remote)

	var chunks []Chunk
	var i, j int       // next unprocessed hunk in hl, hr
	var offL, offR int // side index minus base index before the current position
	pos := 0           // base index up to which chunks have been emitted
	emitUnchanged := func(end int) {
		if end > pos {
			chunks = append(chunks, Chunk{
				Kind:   Unchanged,
				Base:   Range{Start: pos, End: end},
				Local:  Range{Start: pos + offL, End: end + offL},
				Remote: Range{Start: pos + offR, End: end + offR},
			})
		}
	}

	for i < len(hl) || j < len(hr) {
		// Start a region at the hunk that begins first in base.
		var start, end int
		if j >= len(hr) || (i < len(hl) && hl[i].A.Start <= hr[j].A.Start) {
			start, end = hl[i].A.Start, hl[i].A.End
		} else {
			start, end = hr[j].A.Start, hr[j].A.End
		}
		// Absorb every hunk from either side that overlaps or touches it.
		endL, endR := offL, offR // side offsets after the region
	absorb:
		for {
			switch {
			case i < len(hl) && hl[i].A.Start <= end:
				end = max(end, hl[i].A.End)
				endL = hl[i].B.End - hl[i].A.End
				i++
			case j < len(hr) && hr[j].A.Start <= end:
				end = max(end, hr[j].A.End)
				endR = hr[j].B.End - hr[j].A.End
				j++
			default:
				break absorb
			}
		}
		emitUnchanged(start)
		c := Chunk{
			Base:   Range{Start: start, End: end},
			Local:  Range{Start: start + offL, End: end + endL},
			Remote: Range{Start: start + offR, End: end + endR},
		}
		b := base[start:end]
		l := local[c.Local.Start:c.Local.End]
		r := remote[c.Remote.Start:c.Remote.End]
		localChanged, remoteChanged := !slices.Equal(b, l), !slices.Equal(b, r)
		switch {
		case localChanged && remoteChanged && slices.Equal(l, r):
			c.Kind = BothSame
		case localChanged && remoteChanged:
			c.Kind = Conflict
		case localChanged:
			c.Kind = LocalOnly
		case remoteChanged:
			c.Kind = RemoteOnly
		default:
			c.Kind = Unchanged // unreachable with minimal diffs
		}
		chunks = append(chunks, c)
		pos, offL, offR = end, endL, endR
	}
	emitUnchanged(len(base))
	return chunks
}

// Source selects which non-conflicting changes ApplyNonConflicting applies.
type Source string

const (
	FromBoth   Source = "both"   // LocalOnly, RemoteOnly and BothSame chunks
	FromLocal  Source = "local"  // LocalOnly and BothSame chunks
	FromRemote Source = "remote" // RemoteOnly and BothSame chunks
)

// ApplyNonConflicting returns the base text with the selected
// non-conflicting chunks replaced by their changed text. Conflicts and
// unselected chunks keep the base text.
func ApplyNonConflicting(base, local, remote []string, chunks []Chunk, from Source) string {
	var sb strings.Builder
	write := func(lines []string, r Range) {
		for _, line := range lines[r.Start:r.End] {
			sb.WriteString(line)
		}
	}
	for _, c := range chunks {
		switch {
		case c.Kind == BothSame,
			c.Kind == LocalOnly && from != FromRemote:
			write(local, c.Local)
		case c.Kind == RemoteOnly && from != FromLocal:
			write(remote, c.Remote)
		default:
			write(base, c.Base)
		}
	}
	return sb.String()
}

// Counts summarises chunks by kind.
type Counts struct {
	Changes        int `json:"changes"`        // all non-unchanged chunks
	NonConflicting int `json:"nonConflicting"` // LocalOnly + RemoteOnly + BothSame
	Conflicts      int `json:"conflicts"`
}

// Count returns the number of chunks of each category.
func Count(chunks []Chunk) Counts {
	var n Counts
	for _, c := range chunks {
		switch c.Kind {
		case Unchanged:
			continue
		case Conflict:
			n.Conflicts++
		default:
			n.NonConflicting++
		}
		n.Changes++
	}
	return n
}
