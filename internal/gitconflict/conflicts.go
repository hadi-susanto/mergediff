package gitconflict

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hadi-susanto/mergediff/internal/textio"
)

// Status is what one side did to a conflicted file, relative to the base.
type Status string

const (
	Modified Status = "modified" // the side has the file and so does the base
	Added    Status = "added"    // the side has the file, the base does not
	Deleted  Status = "deleted"  // the side does not have the file
)

// Kind is what a conflicted file is, from its modes and content.
type Kind string

const (
	KindText      Kind = "text"
	KindBinary    Kind = "binary"  // a NUL byte in the first textio.SniffLen bytes
	KindNotUTF8   Kind = "notUTF8" // text that is not valid UTF-8; handled like binary
	KindSymlink   Kind = "symlink"
	KindSubmodule Kind = "submodule"
)

// State is the conflict's state as the Conflicts window shows it.
type State string

const (
	StateConflict     State = "conflict"     // both sides changed the content
	StateModifyDelete State = "modifyDelete" // one side (or both) deleted it
	StateBinary       State = "binary"       // binary or not UTF-8: no 3-way merge
	StateSymlink      State = "symlink"
	StateSubmodule    State = "submodule"
)

// Git file modes of special entries.
const (
	modeSymlink   = "120000"
	modeSubmodule = "160000"
)

// Stage is one index stage of an unmerged path.
type Stage struct {
	Mode   string `json:"mode"`
	Object string `json:"object"`
}

// Conflict is one unmerged path.
type Conflict struct {
	// Path is relative to the top level, with "/" separators.
	Path string `json:"path"`
	// Stages are Git's stages 1 (base), 2 (ours) and 3 (theirs); index 0 is
	// unused. A nil stage does not exist.
	Stages [4]*Stage `json:"-"`
	// Yours and Theirs are the user's sides (see Sides).
	Yours  Status `json:"yours"`
	Theirs Status `json:"theirs"`
	Kind   Kind   `json:"kind"`
	State  State  `json:"state"`
}

// Has reports whether Git's stage exists.
func (c *Conflict) Has(stage int) bool {
	return stage >= 1 && stage <= 3 && c.Stages[stage] != nil
}

// status returns what the side in Git's stage did.
func (c *Conflict) status(stage int) Status {
	switch {
	case !c.Has(stage):
		return Deleted
	case c.Has(StageBase):
		return Modified
	default:
		return Added
	}
}

// Side returns what side did to the file.
func (c *Conflict) Side(side Side) Status {
	if side == Yours {
		return c.Yours
	}
	return c.Theirs
}

// List returns the unmerged paths under pathspecs (all when empty), sorted
// by path, with each side's status mapped through sides. It reads the
// stages' content once to tell text from binary files; nothing is written.
func (r *Repo) List(pathspecs []string, sides Sides) ([]Conflict, error) {
	args := append([]string{"ls-files", "--unmerged", "-z", "--"}, pathspecs...)
	out, err := r.git(args...)
	if err != nil {
		return nil, err
	}
	byPath := map[string]*Conflict{}
	var order []string
	for _, rec := range splitNUL(out) {
		meta, path, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 {
			return nil, fmt.Errorf("git ls-files: unexpected record %q", rec)
		}
		stage, err := strconv.Atoi(f[2])
		if err != nil || stage < 1 || stage > 3 {
			return nil, fmt.Errorf("git ls-files: unexpected stage in %q", rec)
		}
		c := byPath[path]
		if c == nil {
			c = &Conflict{Path: path}
			byPath[path] = c
			order = append(order, path)
		}
		c.Stages[stage] = &Stage{Mode: f[0], Object: f[1]}
	}
	sort.Strings(order)

	conflicts := make([]Conflict, 0, len(order))
	for _, p := range order {
		c := byPath[p]
		c.Yours = c.status(sides.Stage(Yours))
		c.Theirs = c.status(sides.Stage(Theirs))
		conflicts = append(conflicts, *c)
	}
	if err := r.classify(conflicts); err != nil {
		return nil, err
	}
	return conflicts, nil
}

// classify sets Kind and State. Modes decide submodules and symlinks; for
// regular files the blobs are read in one "cat-file --batch" process and
// sniffed like textio does: a NUL byte early means binary, otherwise the
// content must be valid UTF-8.
func (r *Repo) classify(conflicts []Conflict) error {
	var want []string
	for i := range conflicts {
		c := &conflicts[i]
		c.Kind = modeKind(c)
		if c.Kind != KindText {
			continue
		}
		for s := 1; s <= 3; s++ {
			if c.Has(s) {
				want = append(want, c.Stages[s].Object)
			}
		}
	}
	blobs, err := r.readBlobs(want)
	if err != nil {
		return err
	}
	for i := range conflicts {
		c := &conflicts[i]
		if c.Kind == KindText {
			for s := 1; s <= 3; s++ {
				if !c.Has(s) {
					continue
				}
				if k := contentKind(blobs[c.Stages[s].Object]); k != KindText {
					// Binary wins over not-UTF-8: it says more.
					if c.Kind == KindText || k == KindBinary {
						c.Kind = k
					}
				}
			}
		}
		c.State = state(c)
	}
	return nil
}

// modeKind is Submodule if any stage is a gitlink, else Symlink if any is
// a symlink, else Text (pending a look at the content).
func modeKind(c *Conflict) Kind {
	kind := KindText
	for s := 1; s <= 3; s++ {
		if !c.Has(s) {
			continue
		}
		switch c.Stages[s].Mode {
		case modeSubmodule:
			return KindSubmodule
		case modeSymlink:
			kind = KindSymlink
		}
	}
	return kind
}

func contentKind(data []byte) Kind {
	if bytes.IndexByte(data[:min(len(data), textio.SniffLen)], 0) >= 0 {
		return KindBinary
	}
	if !utf8.Valid(data) {
		return KindNotUTF8
	}
	return KindText
}

func state(c *Conflict) State {
	switch {
	case c.Kind == KindSubmodule:
		return StateSubmodule
	case c.Yours == Deleted || c.Theirs == Deleted:
		return StateModifyDelete
	case c.Kind == KindSymlink:
		return StateSymlink
	case c.Kind == KindBinary || c.Kind == KindNotUTF8:
		return StateBinary
	default:
		return StateConflict
	}
}

// readBlobs returns the raw content of objects, read with one
// "git cat-file --batch".
func (r *Repo) readBlobs(objects []string) (map[string][]byte, error) {
	blobs := make(map[string][]byte, len(objects))
	if len(objects) == 0 {
		return blobs, nil
	}
	var in strings.Builder
	for _, o := range objects {
		in.WriteString(o)
		in.WriteByte('\n')
	}
	out, err := r.gitIn(strings.NewReader(in.String()), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	br := bufio.NewReader(bytes.NewReader(out))
	for range objects {
		header, err := br.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("git cat-file --batch: short output: %w", err)
		}
		f := strings.Fields(header)
		if len(f) == 2 && f[1] == "missing" {
			continue // e.g. a partial clone; the content read will report it
		}
		if len(f) != 3 {
			return nil, fmt.Errorf("git cat-file --batch: unexpected header %q", header)
		}
		size, err := strconv.Atoi(f[2])
		if err != nil {
			return nil, fmt.Errorf("git cat-file --batch: unexpected header %q", header)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(br, data); err != nil {
			return nil, fmt.Errorf("git cat-file --batch: short output: %w", err)
		}
		if _, err := br.Discard(1); err != nil { // the "\n" after the content
			return nil, fmt.Errorf("git cat-file --batch: short output: %w", err)
		}
		blobs[f[0]] = data
	}
	return blobs, nil
}

// Content returns the content of Git's stage of c as a checkout would
// write it: "cat-file --filters" applies the same smudge and line-ending
// conversion, without touching the working tree. A stage that does not
// exist is empty (nil).
func (r *Repo) Content(c *Conflict, stage int) ([]byte, error) {
	if !c.Has(stage) {
		return nil, nil
	}
	out, err := r.git("cat-file", "--filters", ":"+strconv.Itoa(stage)+":"+c.Path)
	if err != nil {
		return nil, fmt.Errorf("read stage %d of %s: %w", stage, c.Path, err)
	}
	return out, nil
}
