package session

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/hadi-susanto/mergediff/internal/gitconflict"
	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

// Resolve lists a Git working tree's merge conflicts (mergediff resolve).
const Resolve Mode = "resolve"

var (
	// ErrNoConflicts is returned by StartResolve when nothing is unmerged.
	ErrNoConflicts = errors.New("no conflicts to resolve")
	// ErrNotResolve is returned by the conflict methods in other modes.
	ErrNotResolve = errors.New("not a conflict resolution session")
	// ErrNotConflicted is returned for a path that is not (or no longer)
	// unmerged.
	ErrNotConflicted = errors.New("not conflicted")
)

// ConflictRow is one file of the Conflicts window.
type ConflictRow struct {
	// Path is relative to the repository's top level, "/" separators.
	Path   string             `json:"path"`
	Yours  gitconflict.Status `json:"yours"`
	Theirs gitconflict.Status `json:"theirs"`
	Kind   gitconflict.Kind   `json:"kind"`
	State  gitconflict.State  `json:"state"`
	// Resolved: no longer unmerged. Yours, Theirs, Kind and State then keep
	// what they were while the file was conflicted.
	Resolved bool `json:"resolved"`
}

// ConflictList is the Conflicts window's content.
type ConflictList struct {
	Operation gitconflict.Operation `json:"operation"`
	Rows      []ConflictRow         `json:"rows"`
	// Unresolved counts the rows not yet resolved.
	Unresolved int `json:"unresolved"`
}

// MergeCounts describes a text conflict's merge, for the side panel.
type MergeCounts struct {
	Conflicts      int `json:"conflicts"`
	NonConflicting int `json:"nonConflicting"`
}

// resolveState is a resolve session's repository and rows.
type resolveState struct {
	repo      *gitconflict.Repo
	specs     []string // PATH limits, relative to the top level; nil for all
	autoApply bool

	mu   sync.Mutex
	rows map[string]*ConflictRow
}

// StartResolve opens the Git working tree containing dir and lists its
// conflicts under paths (PATH arguments, relative to dir). It fails with
// ErrNoConflicts when there are none, and with gitconflict's errors when
// dir is not in a working tree or Git is missing or too old.
func StartResolve(dir string, paths []string, autoApply bool) (*Session, error) {
	repo, err := gitconflict.Open(dir)
	if err != nil {
		return nil, err
	}
	specs, err := repo.Pathspecs(paths)
	if err != nil {
		return nil, err
	}
	rs := &resolveState{repo: repo, specs: specs, autoApply: autoApply, rows: map[string]*ConflictRow{}}
	list, err := rs.refresh()
	if err != nil {
		return nil, err
	}
	if list.Unresolved == 0 {
		return nil, ErrNoConflicts
	}
	return &Session{
		Mode:   Resolve,
		Paths:  []string{repo.Top},
		Title:  "Resolve conflicts · " + repo.Name(),
		Chunks: []merge.Chunk{},
		EOL:    textio.LF.Name(),
		eol:    textio.LF,
		res:    rs,
	}, nil
}

// refresh re-reads the unmerged paths. Rows listed before stay; those no
// longer unmerged become resolved. New conflicts are added.
func (rs *resolveState) refresh() (ConflictList, error) {
	op := rs.repo.Operation()
	cs, err := rs.repo.List(rs.specs, op.Sides())
	if err != nil {
		return ConflictList{}, err
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	current := make(map[string]bool, len(cs))
	for _, c := range cs {
		current[c.Path] = true
		rs.rows[c.Path] = &ConflictRow{Path: c.Path, Yours: c.Yours, Theirs: c.Theirs, Kind: c.Kind, State: c.State}
	}
	list := ConflictList{Operation: op, Rows: make([]ConflictRow, 0, len(rs.rows))}
	for p, row := range rs.rows {
		row.Resolved = !current[p]
		if !row.Resolved {
			list.Unresolved++
		}
		list.Rows = append(list.Rows, *row)
	}
	sort.Slice(list.Rows, func(i, j int) bool { return list.Rows[i].Path < list.Rows[j].Path })
	return list, nil
}

func (s *Session) resolve() (*resolveState, error) {
	if s.res == nil {
		return nil, ErrNotResolve
	}
	return s.res, nil
}

// Conflicts re-reads the conflicts and returns the window's rows.
func (s *Session) Conflicts() (ConflictList, error) {
	rs, err := s.resolve()
	if err != nil {
		return ConflictList{}, err
	}
	return rs.refresh()
}

// Accept resolves paths by taking side's version (see gitconflict.Accept)
// and returns the refreshed rows.
func (s *Session) Accept(paths []string, side gitconflict.Side) (ConflictList, error) {
	rs, err := s.resolve()
	if err != nil {
		return ConflictList{}, err
	}
	if side != gitconflict.Yours && side != gitconflict.Theirs {
		return ConflictList{}, fmt.Errorf("accept: unknown side %q", side)
	}
	op := rs.repo.Operation()
	if err := rs.repo.Accept(paths, side, op.Sides()); err != nil {
		return ConflictList{}, err
	}
	return rs.refresh()
}

// Remaining counts the conflicts left within the session's PATH limits,
// read fresh from Git; it decides the exit code.
func (s *Session) Remaining() (int, error) {
	rs, err := s.resolve()
	if err != nil {
		return 0, err
	}
	cs, err := rs.repo.List(rs.specs, gitconflict.Sides{})
	if err != nil {
		return 0, err
	}
	return len(cs), nil
}

// conflict returns the current state of the unmerged path and the
// operation in progress.
func (rs *resolveState) conflict(path string) (*gitconflict.Conflict, gitconflict.Operation, error) {
	op := rs.repo.Operation()
	cs, err := rs.repo.List([]string{path}, op.Sides())
	if err != nil {
		return nil, op, err
	}
	for i := range cs {
		if cs[i].Path == path {
			return &cs[i], op, nil
		}
	}
	return nil, op, fmt.Errorf("%s: %w", path, ErrNotConflicted)
}

// stages reads the base, yours and theirs text of a text conflict. A
// missing stage is empty.
func (rs *resolveState) stages(c *gitconflict.Conflict, sides gitconflict.Sides) (base, yours, theirs textio.File, err error) {
	read := func(stage int, what string) (textio.File, error) {
		data, err := rs.repo.Content(c, stage)
		if err != nil {
			return textio.File{}, err
		}
		f, err := textio.Decode(data)
		if err != nil {
			return textio.File{}, fmt.Errorf("%s (%s): %w", c.Path, what, err)
		}
		f.Exists = c.Has(stage)
		return f, nil
	}
	if base, err = read(gitconflict.StageBase, "base"); err != nil {
		return
	}
	if yours, err = read(sides.Stage(gitconflict.Yours), "yours"); err != nil {
		return
	}
	theirs, err = read(sides.Stage(gitconflict.Theirs), "theirs")
	return
}

// MergeCounts merges the text conflict at path and counts its conflicts
// and non-conflicting changes, for the side panel.
func (s *Session) MergeCounts(path string) (MergeCounts, error) {
	rs, err := s.resolve()
	if err != nil {
		return MergeCounts{}, err
	}
	c, op, err := rs.conflict(path)
	if err != nil {
		return MergeCounts{}, err
	}
	if c.Kind != gitconflict.KindText {
		return MergeCounts{}, nil
	}
	base, yours, theirs, err := rs.stages(c, op.Sides())
	if err != nil {
		return MergeCounts{}, err
	}
	var n MergeCounts
	for _, ch := range merge.Merge(textio.SplitLines(base.Text), textio.SplitLines(yours.Text), textio.SplitLines(theirs.Text)) {
		switch ch.Kind {
		case merge.Conflict:
			n.Conflicts++
		case merge.LocalOnly, merge.RemoteOnly, merge.BothSame:
			n.NonConflicting++
		}
	}
	return n, nil
}

// MergeSession opens the text conflict at path as a 3-way merge: base,
// yours and theirs from Git's stages (with the checkout's line endings),
// the working-tree file as the output. Saving writes it and runs git add.
// The panes are labelled with the operation's branch names. Other kinds of
// conflict fail with ErrCannotOpen.
func (s *Session) MergeSession(path string) (*Session, error) {
	rs, err := s.resolve()
	if err != nil {
		return nil, err
	}
	c, op, err := rs.conflict(path)
	if err != nil {
		return nil, err
	}
	if c.State != gitconflict.StateConflict {
		return nil, fmt.Errorf("%s %w: not a text conflict (%s)", path, ErrCannotOpen, c.State)
	}
	base, yours, theirs, err := rs.stages(c, op.Sides())
	if err != nil {
		return nil, err
	}
	abs := rs.repo.Abs(path)
	m := newMerge(base, yours, theirs, []string{abs, abs, abs, abs}, path, abs, rs.autoApply)
	m.LocalLabel = label(op.YoursLabel, "yours")
	m.RemoteLabel = label(op.TheirsLabel, "theirs")
	repo := rs.repo
	m.saveTo = func(text string, eol textio.EOL) error { return repo.SaveMerged(path, text, eol) }
	return m, nil
}

// ChangesSession opens a modify/delete conflict at path as a read-only
// diff of base (empty when there is none) against the side that kept the
// file. Other conflicts fail with ErrCannotOpen; binary content with
// textio's errors.
func (s *Session) ChangesSession(path string) (*Session, error) {
	rs, err := s.resolve()
	if err != nil {
		return nil, err
	}
	c, op, err := rs.conflict(path)
	if err != nil {
		return nil, err
	}
	if c.State != gitconflict.StateModifyDelete {
		return nil, fmt.Errorf("%s %w: not a modify/delete conflict", path, ErrCannotOpen)
	}
	sides := op.Sides()
	side, name := gitconflict.Yours, label(op.YoursLabel, "yours")
	if c.Yours == gitconflict.Deleted {
		side, name = gitconflict.Theirs, label(op.TheirsLabel, "theirs")
	}
	if c.Side(side) == gitconflict.Deleted {
		return nil, fmt.Errorf("%s %w: both sides deleted it", path, ErrCannotOpen)
	}
	base, yours, theirs, err := rs.stages(c, sides)
	if err != nil {
		return nil, err
	}
	kept := yours
	if side == gitconflict.Theirs {
		kept = theirs
	}
	abs := rs.repo.Abs(path)
	eol := textio.DetectEOL(kept, base)
	return &Session{
		Mode:        Diff,
		Paths:       []string{abs, abs},
		Title:       path + " · changes",
		Local:       base.Text,
		Remote:      kept.Text,
		Chunks:      diffChunks(base.Text, kept.Text),
		Language:    Language(path),
		EOL:         eol.Name(),
		LocalLabel:  "Base",
		RemoteLabel: name,
		eol:         eol,
	}, nil
}

// label is a side's branch label, or its role when Git gave none.
func label(name, role string) string {
	if name == "" {
		return role
	}
	return name
}
