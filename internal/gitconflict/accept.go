package gitconflict

import (
	"errors"
	"fmt"

	"github.com/hadi-susanto/mergediff/internal/textio"
)

// ErrSubmodule is returned when asked to resolve a submodule conflict:
// those are left to Git.
var ErrSubmodule = errors.New("submodule conflicts must be resolved with git")

// pathsPerCommand caps how many paths one git command gets, to stay under
// command-line length limits (32 KB on Windows).
var pathsPerCommand = 100

// Accept resolves paths (relative to Top) by taking side's version, the
// way the JetBrains IDEs do: when that side has the file, "git checkout
// --ours|--theirs" (through sides, so rebases are swapped) and "git add";
// when that side deleted it, "git rm". Paths that are no longer unmerged
// are skipped. A submodule among them fails with ErrSubmodule before
// anything changes.
func (r *Repo) Accept(paths []string, side Side, sides Sides) error {
	if len(paths) == 0 {
		return nil
	}
	cs, err := r.List(paths, sides)
	if err != nil {
		return err
	}
	want := make(map[string]bool, len(paths))
	for _, p := range paths {
		want[p] = true
	}
	var keep, remove []string
	for i := range cs {
		c := &cs[i]
		if !want[c.Path] {
			continue // matched as a directory prefix; not asked for
		}
		if c.Kind == KindSubmodule {
			return fmt.Errorf("%s: %w", c.Path, ErrSubmodule)
		}
		if c.Side(side) == Deleted {
			remove = append(remove, c.Path)
		} else {
			keep = append(keep, c.Path)
		}
	}
	flag := sides.CheckoutFlag(side)
	for _, chunk := range chunks(keep) {
		if _, err := r.git(append([]string{"checkout", flag, "--"}, chunk...)...); err != nil {
			return err
		}
		if _, err := r.git(append([]string{"add", "--"}, chunk...)...); err != nil {
			return err
		}
	}
	for _, chunk := range chunks(remove) {
		if _, err := r.git(append([]string{"rm", "--quiet", "--"}, chunk...)...); err != nil {
			return err
		}
	}
	return nil
}

// SaveMerged writes a merge result (LF text, as the editor produces it) to
// the working-tree file path in the given line-ending style and marks it
// resolved with "git add".
func (r *Repo) SaveMerged(path, text string, eol textio.EOL) error {
	if err := textio.WriteFile(r.Abs(path), text, eol); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if _, err := r.git("add", "--", path); err != nil {
		return err
	}
	return nil
}

func chunks(paths []string) [][]string {
	var out [][]string
	for len(paths) > 0 {
		n := min(len(paths), pathsPerCommand)
		out = append(out, paths[:n])
		paths = paths[n:]
	}
	return out
}
