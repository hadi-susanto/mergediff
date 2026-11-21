package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// Set writes key = value into the config file at path, creating the file
// and its directory if needed. The file is edited line by line, so comments
// and the other settings are kept: an existing `key = ...` line is replaced
// (keeping its trailing comment), otherwise a line is appended. The result
// is validated before it is written, and written atomically.
func Set(path, key, value string) error {
	if err := Check(key, value); err != nil {
		return err
	}
	line, err := toml.Marshal(map[string]string{key: value})
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	assignment := strings.TrimSpace(string(line))
	return edit(path, func(lines []string) []string {
		found := false
		for i, l := range lines {
			if m := keyLine(key).FindStringSubmatch(l); m != nil {
				lines[i] = m[1] + assignment + m[2]
				found = true
			}
		}
		if !found {
			lines = append(lines, assignment)
		}
		return lines
	})
}

// Unset removes key from the config file at path, so it returns to its
// default. A missing file or key is not an error.
func Unset(path, key string) error {
	if _, err := Lookup(key); err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return edit(path, func(lines []string) []string {
		kept := lines[:0]
		for _, l := range lines {
			if !keyLine(key).MatchString(l) {
				kept = append(kept, l)
			}
		}
		return kept
	})
}

// keyLine matches a top-level `key = value  # comment` line; group 1 is the
// indentation, group 2 the trailing comment (with its leading space).
func keyLine(key string) *regexp.Regexp {
	k := regexp.QuoteMeta(key)
	return regexp.MustCompile(`^(\s*)(?:` + k + `|"` + k + `"|'` + k + `')\s*=\s*(?:"[^"]*"|'[^']*'|[^\s#]*)(\s*#.*)?\s*$`)
}

// edit applies change to the file's lines, validates the result with
// Parse and replaces the file atomically. Line endings (LF or CRLF) are kept.
func edit(path string, change func([]string) []string) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	eol := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		eol = "\r\n"
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	lines = change(lines)
	out := ""
	if len(lines) > 0 {
		out = strings.Join(lines, eol) + eol
	}
	if _, err := Parse([]byte(out)); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	return writeAtomic(path, []byte(out))
}

// writeAtomic replaces path with data via a temporary file in the same
// directory, keeping the existing file's permissions.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
