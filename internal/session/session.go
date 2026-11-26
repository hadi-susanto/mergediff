// Package session loads what a window shows and serves its calls: the
// texts, chunks, initial result, language and line-ending style of a diff
// or merge, a directory comparison, or Git's conflicts. Each kind of
// window has its own type (DiffSession, ScratchSession, MergeSession,
// DirDiffSession, ResolveSession) behind the Session interface; the
// frontend gets their View.
package session

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

// Mode is the kind of session, for the frontend to pick its view.
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

// Session is what a window shows.
type Session interface {
	// View is what the frontend gets.
	View() View
	// ExitCode is the process exit code when the main window showing the
	// session closes. code is what Quit set (0 when it was not called);
	// quit reports whether it was.
	ExitCode(code int, quit bool) int
}

// The process exit codes ExitCode returns (as in internal/command, which
// imports this package).
const (
	exitOK     = 0
	exitCancel = 1
	exitError  = 2
)

// ErrCannotOpen is returned for a file that cannot be shown as a text diff
// or merge: a directory comparison's error, symlink to a directory or
// large file, or a conflict of the wrong kind.
var ErrCannotOpen = errors.New("cannot be opened")

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

// diffTexts are the panes of a diff of LEFT and RIGHT (LF text).
func diffTexts(left, right, language string, eol textio.EOL) *Texts {
	return &Texts{Left: left, Right: right, Chunks: diffChunks(left, right), Language: language, EOL: eol.Name()}
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
