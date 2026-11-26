// Package session loads the files named on the command line and prepares
// everything the UI needs: texts, chunks, the initial result, the language
// and the line-ending style. It also saves the merge result.
package session

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/hadi-susanto/mergediff/internal/dirdiff"
	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

// Mode is the kind of session.
type Mode string

const (
	Merge Mode = "merge"
	Diff  Mode = "diff"
	// Scratch is an editable two-way diff without files: the user pastes
	// text into both panes and compares it with Compare.
	Scratch Mode = "scratch"
	// DirDiff lists the files that differ between two directories.
	DirDiff Mode = "dirdiff"
)

// ErrNotMerge is returned by Save in diff mode.
var ErrNotMerge = errors.New("not a merge session")

// Session is serialised to the frontend.
//
// In diff mode Local holds LEFT and Remote holds RIGHT, Base is empty, and
// Chunks compare LEFT with RIGHT: they are Unchanged or RemoteOnly, and their
// Base ranges equal their Local ranges.
type Session struct {
	Mode  Mode     `json:"mode"`
	Paths []string `json:"paths"` // merge: BASE, LOCAL, REMOTE, MERGED; diff, dirdiff: LEFT, RIGHT; resolve: the repository's top level
	// Title names the file being merged or compared, for the window title.
	Title         string        `json:"title"`
	Base          string        `json:"base"`
	Local         string        `json:"local"`
	Remote        string        `json:"remote"`
	Chunks        []merge.Chunk `json:"chunks"`
	InitialResult string        `json:"initialResult"`
	Language      string        `json:"language"` // from the file extension; "" if unknown
	EOL           string        `json:"eol"`      // "LF" or "CRLF"; the style MERGED is written in
	// AutoApplied reports that InitialResult already contains every
	// non-conflicting chunk (--auto-apply), so the UI marks them applied.
	AutoApplied bool `json:"autoApplied"`
	// LocalLabel and RemoteLabel name the two sides in the pane headers
	// when they are not files: branch names in a merge opened from the
	// Conflicts window, "Base" and a branch in its Show changes diff.
	// Empty means the headers show the file names.
	LocalLabel  string `json:"localLabel"`
	RemoteLabel string `json:"remoteLabel"`

	eol    textio.EOL
	output string
	// saveTo, when set, saves the result instead of writing output: a
	// merge opened from the Conflicts window writes the working-tree file
	// and runs git add.
	saveTo func(text string, eol textio.EOL) error
	dir    *dirCompare   // dirdiff only
	res    *resolveState // resolve only
}

// LoadDiff loads LEFT and RIGHT for a read-only two-way diff.
func LoadDiff(left, right string) (*Session, error) {
	l, err := textio.ReadFile(left)
	if err != nil {
		return nil, fmt.Errorf("read LEFT: %w", err)
	}
	r, err := textio.ReadFile(right)
	if err != nil {
		return nil, fmt.Errorf("read RIGHT: %w", err)
	}
	eol := textio.DetectEOL(r, l)
	return &Session{
		Mode:     Diff,
		Paths:    []string{left, right},
		Title:    diffTitle(left, right),
		Local:    l.Text,
		Remote:   r.Text,
		Chunks:   diffChunks(l.Text, r.Text),
		Language: Language(right),
		EOL:      eol.Name(),
		eol:      eol,
	}, nil
}

// LoadPair loads two files for a read-only diff opened from a directory
// comparison. An empty path is a file missing on that side: it shows as
// empty text. title names the window, e.g. the file's relative path.
func LoadPair(left, right, title string) (*Session, error) {
	l, err := textio.ReadOptional(left)
	if err != nil {
		return nil, fmt.Errorf("read LEFT: %w", err)
	}
	r, err := textio.ReadOptional(right)
	if err != nil {
		return nil, fmt.Errorf("read RIGHT: %w", err)
	}
	eol := textio.DetectEOL(r, l)
	return &Session{
		Mode:     Diff,
		Paths:    []string{left, right},
		Title:    title,
		Local:    l.Text,
		Remote:   r.Text,
		Chunks:   diffChunks(l.Text, r.Text),
		Language: Language(title),
		EOL:      eol.Name(),
		eol:      eol,
	}, nil
}

// LoadMerge loads the three inputs of a merge whose result goes to output.
// A missing or empty BASE path is treated as an empty file. With autoApply
// the initial result already contains every non-conflicting change;
// otherwise it is the base text.
func LoadMerge(base, local, remote, output string, autoApply bool) (*Session, error) {
	b, err := textio.ReadOptional(base)
	if err != nil {
		return nil, fmt.Errorf("read BASE: %w", err)
	}
	l, err := textio.ReadFile(local)
	if err != nil {
		return nil, fmt.Errorf("read LOCAL: %w", err)
	}
	r, err := textio.ReadFile(remote)
	if err != nil {
		return nil, fmt.Errorf("read REMOTE: %w", err)
	}
	return newMerge(b, l, r, []string{base, local, remote, output}, filepath.Base(output), output, autoApply), nil
}

// newMerge builds a merge session from the three decoded texts.
func newMerge(b, l, r textio.File, paths []string, title, output string, autoApply bool) *Session {
	bl, ll, rl := textio.SplitLines(b.Text), textio.SplitLines(l.Text), textio.SplitLines(r.Text)
	chunks := merge.Merge(bl, ll, rl)
	initial := b.Text
	if autoApply {
		initial = merge.ApplyNonConflicting(bl, ll, rl, chunks, merge.FromBoth)
	}
	eol := textio.DetectEOL(l, b)
	return &Session{
		Mode:          Merge,
		Paths:         paths,
		Title:         title,
		Base:          b.Text,
		Local:         l.Text,
		Remote:        r.Text,
		Chunks:        nonNil(chunks),
		InitialResult: initial,
		Language:      Language(output),
		EOL:           eol.Name(),
		AutoApplied:   autoApply,
		eol:           eol,
		output:        output,
	}
}

// Save writes the merge result (LF text, as the editor produces it) to
// MERGED in the detected line-ending style.
func (s *Session) Save(result string) error {
	if s.Mode != Merge {
		return ErrNotMerge
	}
	if s.saveTo != nil {
		return s.saveTo(result, s.eol)
	}
	if err := textio.WriteFile(s.output, result, s.eol); err != nil {
		return fmt.Errorf("write MERGED: %w", err)
	}
	return nil
}

// ErrNotDirDiff is returned by DirEntries outside dirdiff mode.
var ErrNotDirDiff = errors.New("not a directory comparison")

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
func StartDirDiff(left, right string) (*Session, error) {
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
	return &Session{
		Mode:   DirDiff,
		Paths:  []string{left, right},
		Title:  "Compare directories",
		Chunks: []merge.Chunk{},
		EOL:    textio.LF.Name(),
		eol:    textio.LF,
		dir:    c,
	}, nil
}

// DirEntries waits for the directory comparison and returns the files of
// both trees, sorted by path, unchanged files included.
func (s *Session) DirEntries() ([]dirdiff.Entry, error) {
	if s.dir == nil {
		return nil, ErrNotDirDiff
	}
	<-s.dir.done
	return s.dir.entries, s.dir.err
}

// ErrCannotOpen is returned by EntrySession for an entry that cannot be
// shown as a text diff (an error, a symlink to a directory, a large file).
var ErrCannotOpen = errors.New("cannot be opened")

// EntrySession loads the two sides of the directory comparison's entry at
// path (relative, "/" separators) as a read-only diff. Binary and invalid
// UTF-8 files fail with textio's errors.
func (s *Session) EntrySession(path string) (*Session, error) {
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
func (s *Session) DirStats() (dirdiff.StatsBatch, error) {
	if s.dir == nil {
		return dirdiff.StatsBatch{}, ErrNotDirDiff
	}
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
func (s *Session) DirProgress() dirdiff.Snapshot {
	if s.dir == nil {
		return dirdiff.Snapshot{Done: true}
	}
	return s.dir.progress.Snapshot()
}

// NewScratch returns an empty scratch session (mergediff without arguments).
func NewScratch() *Session {
	return &Session{
		Mode:   Scratch,
		Paths:  []string{},
		Title:  "scratch",
		Chunks: []merge.Chunk{},
		EOL:    textio.LF.Name(),
		eol:    textio.LF,
	}
}

// Compare diffs two texts the way a diff session diffs LEFT and RIGHT.
// CRLF line endings are normalised first, so pasted Windows text compares
// equal to the same text with LF endings.
func Compare(left, right string) ([]merge.Chunk, error) {
	l, err := textio.Decode([]byte(left))
	if err != nil {
		return nil, fmt.Errorf("left text: %w", err)
	}
	r, err := textio.Decode([]byte(right))
	if err != nil {
		return nil, fmt.Errorf("right text: %w", err)
	}
	return diffChunks(l.Text, r.Text), nil
}

// diffChunks compares LEFT and RIGHT (LF text). The chunks are Unchanged or
// RemoteOnly, with Base ranges equal to Local ranges.
func diffChunks(left, right string) []merge.Chunk {
	ll, rl := textio.SplitLines(left), textio.SplitLines(right)
	return nonNil(merge.Merge(ll, ll, rl))
}

func diffTitle(left, right string) string {
	l, r := filepath.Base(left), filepath.Base(right)
	if l == r {
		return l
	}
	return l + " ↔ " + r
}

// nonNil makes an empty chunk list serialise as [] rather than null.
func nonNil(chunks []merge.Chunk) []merge.Chunk {
	if chunks == nil {
		return []merge.Chunk{}
	}
	return chunks
}

// languages maps lower-case file extensions (and a few file names) to the
// language identifiers the frontend understands.
var languages = map[string]string{
	".go": "go", ".mod": "go",
	".js": "javascript", ".mjs": "javascript", ".cjs": "javascript", ".jsx": "javascript",
	".ts": "typescript", ".mts": "typescript", ".cts": "typescript", ".tsx": "typescript",
	".py": "python", ".pyi": "python",
	".rs":   "rust",
	".java": "java", ".kt": "kotlin", ".kts": "kotlin",
	".c": "cpp", ".h": "cpp", ".cc": "cpp", ".cpp": "cpp", ".cxx": "cpp", ".hpp": "cpp", ".hh": "cpp",
	".cs":  "csharp",
	".php": "php",
	".rb":  "ruby",
	".sh":  "shell", ".bash": "shell", ".zsh": "shell",
	".html": "html", ".htm": "html", ".vue": "html", ".svelte": "html",
	".css": "css", ".scss": "css", ".less": "css",
	".json": "json", ".jsonc": "json",
	".md": "markdown", ".markdown": "markdown",
	".xml": "xml", ".svg": "xml", ".xsd": "xml", ".plist": "xml", ".csproj": "xml",
	".yml": "yaml", ".yaml": "yaml",
	".toml":      "toml",
	".sql":       "sql",
	".swift":     "swift",
	"dockerfile": "dockerfile",
	"makefile":   "makefile", ".mk": "makefile",
}

// Language returns the language identifier for path, or "" if unknown.
// Git's temporary mergetool files (main_LOCAL_123.go) keep the extension,
// so this works on them too.
func Language(path string) string {
	name := strings.ToLower(filepath.Base(path))
	if lang, ok := languages[name]; ok {
		return lang
	}
	return languages[filepath.Ext(name)]
}
