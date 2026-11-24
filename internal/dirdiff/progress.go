package dirdiff

import (
	"fmt"
	"os"
	"sync/atomic"
)

// Progress counts a running comparison's work so a window can show that it
// is still busy. A nil *Progress counts nothing. Safe for concurrent use.
type Progress struct {
	found    atomic.Int64
	compared atomic.Int64
	done     atomic.Bool
}

// Snapshot is a Progress at one moment.
type Snapshot struct {
	Found    int64 `json:"found"`    // files found in both trees so far
	Compared int64 `json:"compared"` // entries whose status is settled
	Done     bool  `json:"done"`     // the comparison has finished
}

// Snapshot returns the counts so far.
func (p *Progress) Snapshot() Snapshot {
	if p == nil {
		return Snapshot{}
	}
	return Snapshot{Found: p.found.Load(), Compared: p.compared.Load(), Done: p.done.Load()}
}

func (p *Progress) addFound() {
	if p != nil {
		p.found.Add(1)
	}
}

func (p *Progress) addCompared(n int64) {
	if p != nil {
		p.compared.Add(n)
	}
}

func (p *Progress) finish() {
	if p != nil {
		p.done.Store(true)
	}
}

// CheckRoot reports an error if root is not a directory that exists. It is
// the cheap part of Compare's checks, for reporting bad arguments before
// starting the comparison.
func CheckRoot(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: %w", root, errNotDir)
	}
	return nil
}
