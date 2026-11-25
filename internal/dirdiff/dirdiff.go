// Package dirdiff compares two directory trees file by file: which files
// were added, deleted or modified, and how many lines changed in each.
package dirdiff

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Status is how a file differs between the two trees.
type Status string

const (
	Added     Status = "added"     // only in the right tree
	Deleted   Status = "deleted"   // only in the left tree
	Modified  Status = "modified"  // in both, with different content
	Unchanged Status = "unchanged" // in both, byte for byte the same
	Error     Status = "error"     // could not be read or compared; see Note
)

// NoteSymlinkDir explains why a symlink to a directory is listed as one
// entry and cannot be opened: following it could loop.
const NoteSymlinkDir = "symlink to directory, not followed"

// Entry is one file of either tree.
type Entry struct {
	// Path is relative to the compared roots, with "/" separators.
	Path   string `json:"path"`
	Status Status `json:"status"`
	// Left and Right are the file's paths in each tree; "" on the side
	// where it does not exist.
	Left      string `json:"left"`
	Right     string `json:"right"`
	LeftSize  int64  `json:"leftSize"`
	RightSize int64  `json:"rightSize"`
	// Note says why the entry cannot be opened as a text diff: the error
	// message for Error entries, or NoteSymlinkDir. "" otherwise.
	Note string `json:"note"`
}

// node is one path found while walking a tree.
type node struct {
	path   string // on disk
	size   int64
	linkTo string // symlink to a directory: its target
	isLink bool   // linkTo is set
	err    error  // could not be read
}

// Compare walks both trees and pairs their files by relative path. The
// entries are sorted by path. Unchanged files are included. Only an
// unusable root is an error; problems with single files become Error
// entries. p (may be nil) counts the work as it goes and is marked done at
// the end, also after an error.
func Compare(left, right string, p *Progress) ([]Entry, error) {
	defer p.finish()
	// Walk both trees at once.
	var r map[string]*node
	var errR error
	walked := make(chan struct{})
	go func() {
		defer close(walked)
		r, errR = walk(right, p)
	}()
	l, err := walk(left, p)
	<-walked
	if err != nil {
		return nil, fmt.Errorf("read LEFT: %w", err)
	}
	if errR != nil {
		return nil, fmt.Errorf("read RIGHT: %w", errR)
	}
	paths := make([]string, 0, len(l)+len(r))
	for p := range l {
		paths = append(paths, p)
	}
	for p := range r {
		if _, ok := l[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)

	entries := make([]Entry, 0, len(paths))
	for _, p := range paths {
		entries = append(entries, pair(p, l[p], r[p]))
	}
	compareContents(entries, p)
	return entries, nil
}

// pair classifies one path from the nodes found on each side (nil if
// absent). Files of equal size on both sides are left Modified with equal
// sizes; compareContents settles them.
func pair(path string, l, r *node) Entry {
	e := Entry{Path: path}
	if l != nil {
		e.Left, e.LeftSize = l.path, l.size
	}
	if r != nil {
		e.Right, e.RightSize = r.path, r.size
	}
	switch {
	case l != nil && l.err != nil:
		e.Status, e.Note = Error, message(l.err)
	case r != nil && r.err != nil:
		e.Status, e.Note = Error, message(r.err)
	case r == nil:
		e.Status = Deleted
	case l == nil:
		e.Status = Added
	case l.isLink && r.isLink && l.linkTo == r.linkTo:
		e.Status = Unchanged
	default:
		e.Status = Modified
	}
	if e.Status != Error && ((l != nil && l.isLink) || (r != nil && r.isLink)) {
		e.Note = NoteSymlinkDir
	}
	return e
}

// walk lists the files under root by relative path ("/" separators).
// Symlinks to files are followed; symlinks to directories are listed as
// one node and not entered. ".git" directories and files are skipped, and
// empty directories leave no trace.
func walk(root string, prog *Progress) (map[string]*node, error) {
	if err := CheckRoot(root); err != nil {
		return nil, err
	}
	// Walk the resolved root: WalkDir does not enter a root that is a
	// symlink. Paths are reported under the root as given.
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	nodes := map[string]*node{}
	err = filepath.WalkDir(real, func(p string, d fs.DirEntry, err error) error {
		rel, relErr := filepath.Rel(real, p)
		if relErr != nil {
			return relErr // cannot happen: p is under real
		}
		if rel == "." {
			return err // an unreadable root
		}
		key := filepath.ToSlash(rel)
		onDisk := filepath.Join(root, rel)
		if err != nil {
			nodes[key] = &node{path: onDisk, err: err}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == ".git" {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		n := &node{path: onDisk}
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := os.Stat(p)
			switch {
			case err != nil:
				n.err = fmt.Errorf("broken symlink: %w", err)
			case target.IsDir():
				n.isLink = true
				n.linkTo, n.err = os.Readlink(p)
			default:
				n.size = target.Size()
			}
			nodes[key] = n
			prog.addFound()
			return nil
		}
		info, err := d.Info()
		if err != nil {
			n.err = err
		} else if info.Mode().IsRegular() {
			n.size = info.Size()
		} else {
			return nil // sockets, pipes, devices: not files to compare
		}
		nodes[key] = n
		prog.addFound()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

var errNotDir = errors.New("not a directory")

// message is err as shown in the list: without the path, which the row
// already shows.
func message(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return strings.Replace(err.Error(), pe.Error(), pe.Err.Error(), 1)
	}
	return err.Error()
}
