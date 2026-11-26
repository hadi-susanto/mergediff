package session

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"github.com/hadi-susanto/mergediff/internal/dirdiff"
)

// DirDiffSession is a directory comparison (git difftool -d), running in
// the background.
type DirDiffSession struct {
	view View
	dir  *dirCompare
}

// View returns what the frontend shows.
func (s *DirDiffSession) View() View { return s.view }

// ExitCode: closing the list ends normally.
func (s *DirDiffSession) ExitCode(code int, quit bool) int { return code }

// dirCompare is a directory comparison running in the background: first
// the entries, then the line counts of the changed files.
type dirCompare struct {
	progress dirdiff.Progress
	done     chan struct{} // closed when entries and err are set
	entries  []dirdiff.Entry
	err      error

	mu        sync.Mutex
	stats     []dirdiff.PathStat // computed, not yet taken by DirStats
	statsDone bool
}

// StartDirDiff checks that LEFT and RIGHT are directories and starts
// comparing them in the background, so the window can open at once.
// DirEntries waits for the result; DirProgress reports how far it got.
func StartDirDiff(left, right string) (*DirDiffSession, error) {
	// Absolute roots, so the window shows full paths and the entries'
	// paths do not depend on the working directory.
	var err error
	if left, err = filepath.Abs(left); err != nil {
		return nil, fmt.Errorf("read LEFT: %w", err)
	}
	if right, err = filepath.Abs(right); err != nil {
		return nil, fmt.Errorf("read RIGHT: %w", err)
	}
	if err := dirdiff.CheckRoot(left); err != nil {
		return nil, fmt.Errorf("read LEFT: %w", err)
	}
	if err := dirdiff.CheckRoot(right); err != nil {
		return nil, fmt.Errorf("read RIGHT: %w", err)
	}
	c := &dirCompare{done: make(chan struct{})}
	go func() {
		c.entries, c.err = dirdiff.Compare(left, right, &c.progress)
		close(c.done)
		if c.err == nil {
			dirdiff.StatsAll(c.entries, func(i int, st dirdiff.Stat) {
				c.mu.Lock()
				defer c.mu.Unlock()
				c.stats = append(c.stats, dirdiff.PathStat{Path: c.entries[i].Path, Stat: st})
			})
		}
		c.mu.Lock()
		c.statsDone = true
		c.mu.Unlock()
	}()
	return &DirDiffSession{
		view: View{Mode: DirDiff, Paths: []string{left, right}, Title: "Compare directories"},
		dir:  c,
	}, nil
}

// DirEntries waits for the directory comparison and returns the files of
// both trees, sorted by path, unchanged files included.
func (s *DirDiffSession) DirEntries() ([]dirdiff.Entry, error) {
	<-s.dir.done
	return s.dir.entries, s.dir.err
}

// Entry loads the two sides of the directory comparison's entry at path
// (relative, "/" separators) as a read-only diff. An error, a symlink to a
// directory or a large file fails with ErrCannotOpen; binary and invalid
// UTF-8 files with textio's errors.
func (s *DirDiffSession) Entry(path string) (*DiffSession, error) {
	entries, err := s.DirEntries()
	if err != nil {
		return nil, err
	}
	i := sort.Search(len(entries), func(i int) bool { return entries[i].Path >= path })
	if i == len(entries) || entries[i].Path != path {
		return nil, fmt.Errorf("%s: not in the comparison", path)
	}
	e := entries[i]
	switch {
	case e.Note != "":
		return nil, fmt.Errorf("%s %w: %s", path, ErrCannotOpen, e.Note)
	case e.LeftSize > dirdiff.MaxOpenSize || e.RightSize > dirdiff.MaxOpenSize:
		return nil, fmt.Errorf("%s %w: %s", path, ErrCannotOpen, dirdiff.ReasonTooLarge)
	}
	return LoadPair(e.Left, e.Right, path)
}

// DirStats returns the line counts computed since the previous call, and
// whether all have now been delivered. They start once the entries are
// ready; files that are not dirdiff.Counted get none.
func (s *DirDiffSession) DirStats() (dirdiff.StatsBatch, error) {
	c := s.dir
	c.mu.Lock()
	defer c.mu.Unlock()
	b := dirdiff.StatsBatch{Stats: c.stats, Done: c.statsDone}
	if b.Stats == nil {
		b.Stats = []dirdiff.PathStat{}
	}
	c.stats = nil
	return b, nil
}

// DirProgress reports how far the directory comparison got.
func (s *DirDiffSession) DirProgress() dirdiff.Snapshot {
	return s.dir.progress.Snapshot()
}
