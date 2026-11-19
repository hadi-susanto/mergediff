// Package textio reads and writes the text files mergediff works on.
//
// Internally all text uses LF line endings. Reading normalises CRLF to LF and
// records which style the file used; writing converts back. Lines keep their
// terminator ("a\n"), so a file without a trailing newline simply has a last
// line without one, and joining the lines reproduces the text exactly.
package textio

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"
)

// SniffLen is how many leading bytes are checked for a NUL byte.
const SniffLen = 8 * 1024

var (
	// ErrBinary is returned for files with a NUL byte in the first SniffLen bytes.
	ErrBinary = errors.New("file looks binary")
	// ErrNotUTF8 is returned for files that are not valid UTF-8. Such text
	// cannot round-trip through the UI unchanged, so it is rejected rather
	// than silently corrupted.
	ErrNotUTF8 = errors.New("file is not valid UTF-8")
)

// EOL is a line-ending style.
type EOL string

const (
	LF   EOL = "\n"
	CRLF EOL = "\r\n"
)

// Name returns "LF" or "CRLF".
func (e EOL) Name() string {
	if e == CRLF {
		return "CRLF"
	}
	return "LF"
}

// File is a decoded text file.
type File struct {
	// Text is the content with CRLF normalised to LF.
	Text string
	// EOL is the dominant line ending of the original bytes. It is LF when
	// the file had no line endings at all; check HasEOL to tell the cases apart.
	EOL EOL
	// HasEOL reports whether the original bytes contained any line ending.
	HasEOL bool
	// Exists is false when the file was optional and missing.
	Exists bool
}

// Decode checks and normalises raw file bytes.
func Decode(data []byte) (File, error) {
	if bytes.IndexByte(data[:min(len(data), SniffLen)], 0) >= 0 {
		return File{}, ErrBinary
	}
	if !utf8.Valid(data) {
		return File{}, ErrNotUTF8
	}
	crlf := bytes.Count(data, []byte("\r\n"))
	lf := bytes.Count(data, []byte("\n")) - crlf
	f := File{Text: string(data), EOL: LF, HasEOL: crlf+lf > 0, Exists: true}
	if crlf > 0 {
		f.Text = strings.ReplaceAll(f.Text, "\r\n", "\n")
	}
	if crlf > lf {
		f.EOL = CRLF
	}
	return f, nil
}

// ReadFile reads and decodes path.
func ReadFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	f, err := Decode(data)
	if err != nil {
		return File{}, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// ReadOptional is like ReadFile, but an empty path or a missing file yields
// an empty File with Exists == false. Git passes such a BASE for add/add
// conflicts.
func ReadOptional(path string) (File, error) {
	if path == "" {
		return File{EOL: LF}, nil
	}
	f, err := ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return File{EOL: LF}, nil
	}
	return f, err
}

// DetectEOL returns the line ending of the first file that has any line
// ending, or LF. Pass LOCAL first and BASE second.
func DetectEOL(files ...File) EOL {
	for _, f := range files {
		if f.HasEOL {
			return f.EOL
		}
	}
	return LF
}

// SplitLines splits LF text into lines that keep their "\n". The last line
// has no terminator if the text does not end with one. Empty text has no lines.
func SplitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// Encode converts LF text to bytes with the given line ending.
func Encode(text string, eol EOL) []byte {
	if eol == CRLF {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	return []byte(text)
}

// WriteFile writes LF text to path using the given line ending. An existing
// file keeps its permissions.
func WriteFile(path, text string, eol EOL) error {
	return os.WriteFile(path, Encode(text, eol), 0o644)
}
