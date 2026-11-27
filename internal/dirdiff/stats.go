package dirdiff

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/hadi-susanto/mergediff/internal/diff"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

// MaxOpenSize is the largest file the list counts lines for and opens in
// a diff window.
const MaxOpenSize = 5 * 1024 * 1024

// Labels shown instead of line counts.
const (
	LabelBinary        = "binary"
	LabelBinaryDiffers = "binary, differs"
	LabelNotUTF8       = "not UTF-8"
	LabelLineEndings   = "line endings"
)

// Reasons a changed file cannot be opened as a text diff.
const (
	ReasonBinary   = "binary file"
	ReasonNotUTF8  = "not valid UTF-8"
	ReasonTooLarge = "larger than 5 MB"
)

// Stat is the size of one entry's change.
type Stat struct {
	Added   int `json:"added"`   // lines
	Deleted int `json:"deleted"` // lines
	// Label replaces the line counts when set, e.g. "binary, differs".
	Label string `json:"label"`
	// Reason says why the file cannot be opened; "" if it can.
	Reason string `json:"reason"`
}

// PathStat is the Stat of the entry at Path.
type PathStat struct {
	Path string `json:"path"`
	Stat Stat   `json:"stat"`
}

// StatsBatch is the stats computed since the previous batch was taken.
type StatsBatch struct {
	Stats []PathStat `json:"stats"`
	// Done reports that every stat has been delivered (this batch included).
	Done bool `json:"done"`
}

// Counted reports whether s is worth computing for e: a changed file
// that could be read.
func Counted(e Entry) bool {
	switch e.Status {
	case Added, Deleted, Modified:
		return e.Note == ""
	}
	return false
}

// Stats measures one entry's change. For entries that are not Counted it
// returns a zero Stat with the entry's Note as the reason.
func Stats(e Entry) Stat {
	if !Counted(e) {
		return Stat{Reason: e.Note}
	}
	if e.LeftSize > MaxOpenSize || e.RightSize > MaxOpenSize {
		return Stat{Label: sizeChange(e), Reason: ReasonTooLarge}
	}
	l, errL := readSide(e.Left)
	r, errR := readSide(e.Right)
	for _, err := range []error{errL, errR} {
		switch {
		case errors.Is(err, textio.ErrBinary):
			if e.Status == Modified {
				return Stat{Label: LabelBinaryDiffers, Reason: ReasonBinary}
			}
			return Stat{Label: LabelBinary, Reason: ReasonBinary}
		case errors.Is(err, textio.ErrNotUTF8):
			return Stat{Label: LabelNotUTF8, Reason: ReasonNotUTF8}
		case err != nil:
			return Stat{Label: message(err), Reason: message(err)}
		}
	}
	if l.Text == r.Text {
		// Bytes differ but the LF-normalised texts are equal.
		return Stat{Label: LabelLineEndings}
	}
	var s Stat
	for _, h := range diff.Lines(textio.SplitLines(l.Text), textio.SplitLines(r.Text)) {
		s.Deleted += h.A.Len()
		s.Added += h.B.Len()
	}
	return s
}

// readSide reads one side of an entry; a missing side is empty.
func readSide(path string) (textio.File, error) {
	if path == "" {
		return textio.File{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return textio.File{}, err
	}
	return textio.Decode(data)
}

// StatsAll computes Stats for every Counted entry with a pool of workers
// and reports each result to done as soon as it is ready, from any
// goroutine. It returns when all are done.
func StatsAll(entries []Entry, done func(i int, s Stat)) {
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers() {
		wg.Go(func() {
			for i := range jobs {
				done(i, Stats(entries[i]))
			}
		})
	}
	for i, e := range entries {
		if Counted(e) {
			jobs <- i
		}
	}
	close(jobs)
	wg.Wait()
}

// sizeChange describes a large file's size: "4.1 MB → 5.3 MB", or one
// size for a file on one side only.
func sizeChange(e Entry) string {
	switch e.Status {
	case Added:
		return humanSize(e.RightSize)
	case Deleted:
		return humanSize(e.LeftSize)
	}
	return humanSize(e.LeftSize) + " → " + humanSize(e.RightSize)
}

// humanSize formats n bytes with one decimal: "512 B", "3.4 KB", "5.3 MB".
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	f := float64(n) / unit
	for _, u := range []string{"KB", "MB", "GB"} {
		if f < unit || u == "GB" {
			return fmt.Sprintf("%.1f %s", f, u)
		}
		f /= unit
	}
	panic("unreachable")
}
