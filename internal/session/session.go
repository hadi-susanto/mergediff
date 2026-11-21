// Package session loads the files named on the command line and prepares
// everything the UI needs: texts, chunks, the initial result, the language
// and the line-ending style. It also saves the merge result.
package session

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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
	Paths []string `json:"paths"` // merge: BASE, LOCAL, REMOTE, MERGED; diff: LEFT, RIGHT
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

	eol    textio.EOL
	output string
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
	bl, ll, rl := textio.SplitLines(b.Text), textio.SplitLines(l.Text), textio.SplitLines(r.Text)
	chunks := merge.Merge(bl, ll, rl)
	initial := b.Text
	if autoApply {
		initial = merge.ApplyNonConflicting(bl, ll, rl, chunks, merge.FromBoth)
	}
	eol := textio.DetectEOL(l, b)
	return &Session{
		Mode:          Merge,
		Paths:         []string{base, local, remote, output},
		Title:         filepath.Base(output),
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
	}, nil
}

// Save writes the merge result (LF text, as the editor produces it) to
// MERGED in the detected line-ending style.
func (s *Session) Save(result string) error {
	if s.Mode != Merge {
		return ErrNotMerge
	}
	if err := textio.WriteFile(s.output, result, s.eol); err != nil {
		return fmt.Errorf("write MERGED: %w", err)
	}
	return nil
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
