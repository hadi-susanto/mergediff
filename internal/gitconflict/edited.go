package gitconflict

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Edited reports whether the working-tree file of the conflict c was
// changed since Git stopped: Accept and saving a merge would then replace
// those edits. Best effort; a false positive only costs a confirmation.
//
//   - A text conflict is compared with what Git wrote: the stages merged
//     again with "git merge-file" (temporary files, never the working
//     tree), in the configured merge.conflictStyle. Marker lines are
//     compared without their labels, line endings are ignored.
//   - A file equal to one of its stages is not edited (e.g. after
//     "git checkout --theirs" without "git add").
//   - Binary files and symlinks: Git leaves one side's version; anything
//     else is edited. Modify/delete: the side that kept the file.
//   - Submodules are never edited (mergediff does not resolve them).
//   - A file that is missing although a side has it is edited.
func (r *Repo) Edited(c *Conflict) (bool, error) {
	if c.Kind == KindSubmodule {
		return false, nil
	}
	work, exists, err := r.workContent(c.Path)
	if err != nil {
		return false, err
	}
	if !exists {
		return c.Has(StageOurs) || c.Has(StageTheirs), nil
	}
	for s := StageBase; s <= StageTheirs; s++ {
		if !c.Has(s) {
			continue
		}
		data, err := r.stageContent(c, s)
		if err != nil {
			return false, err
		}
		if bytes.Equal(work, data) {
			return false, nil
		}
	}
	if c.State != StateConflict || c.Kind != KindText {
		return true, nil
	}
	merged, err := r.remerge(c)
	if err != nil {
		return false, err
	}
	return !bytes.Equal(normalizeMarkers(work), normalizeMarkers(merged)), nil
}

// workContent reads the working-tree file at path (a symlink's target for
// a symlink), and whether it exists.
func (r *Repo) workContent(path string) ([]byte, bool, error) {
	abs := r.Abs(path)
	fi, err := os.Lstat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(abs)
		if err != nil {
			return nil, false, err
		}
		return []byte(filepath.ToSlash(target)), true, nil
	}
	if fi.IsDir() {
		return nil, true, nil // a directory where the file was: edited
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// stageContent is a stage as the working tree would have it: checkout
// conversions applied, a symlink's raw target.
func (r *Repo) stageContent(c *Conflict, stage int) ([]byte, error) {
	if c.Stages[stage].Mode == modeSymlink {
		blobs, err := r.readBlobs([]string{c.Stages[stage].Object})
		if err != nil {
			return nil, err
		}
		return blobs[c.Stages[stage].Object], nil
	}
	return r.Content(c, stage)
}

// remerge merges the text conflict's stages again the way Git did when it
// stopped, with "git merge-file -p" on temporary copies (an empty base for
// add/add). Stage 2 is the "ours" Git wrote first, rebase or not.
func (r *Repo) remerge(c *Conflict) ([]byte, error) {
	dir, err := os.MkdirTemp("", "mergediff-remerge-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	var files [3]string
	for i, s := range []int{StageOurs, StageBase, StageTheirs} {
		data, err := r.Content(c, s) // nil for a missing stage
		if err != nil {
			return nil, err
		}
		files[i] = filepath.Join(dir, fmt.Sprintf("stage%d", s))
		if err := os.WriteFile(files[i], data, 0o600); err != nil {
			return nil, err
		}
	}
	args := []string{"merge-file", "-p"}
	switch r.conflictStyle() {
	case "diff3":
		args = append(args, "--diff3")
	case "zdiff3":
		if !r.Version.Less(Version{2, 35, 0}) {
			args = append(args, "--zdiff3")
		}
	}
	args = append(args, files[0], files[1], files[2])
	out, err := r.git(args...)
	var exit *exec.ExitError
	// merge-file exits with the number of conflicts (capped at 127);
	// higher codes are errors.
	if errors.As(err, &exit) && exit.ExitCode() > 0 && exit.ExitCode() < 128 {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// conflictStyle is merge.conflictStyle ("merge", "diff3", "zdiff3"), or
// "merge" when unset.
func (r *Repo) conflictStyle() string {
	out, err := r.git("config", "--get", "merge.conflictStyle")
	if err != nil {
		return "merge"
	}
	return strings.ToLower(firstLine(out))
}

// markers are the conflict marker prefixes. Git writes 7 characters unless
// a conflict-marker-size attribute says otherwise; a line of 7 or more is
// reduced to the bare marker.
var markers = []byte("<|=>")

// normalizeMarkers drops CRs and reduces conflict marker lines to their
// marker character, so labels (branch names, file names) do not matter.
func normalizeMarkers(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	lines := bytes.SplitAfter(data, []byte("\n"))
	var out bytes.Buffer
	for _, l := range lines {
		if m := markerOf(l); m != 0 {
			out.WriteByte(m)
			if bytes.HasSuffix(l, []byte("\n")) {
				out.WriteByte('\n')
			}
			continue
		}
		out.Write(l)
	}
	return out.Bytes()
}

// markerOf returns the marker character of a conflict marker line, or 0.
func markerOf(line []byte) byte {
	if len(line) < 7 || bytes.IndexByte(markers, line[0]) < 0 {
		return 0
	}
	m := line[0]
	n := 0
	for n < len(line) && line[n] == m {
		n++
	}
	if n < 7 {
		return 0
	}
	rest := line[n:]
	if len(rest) == 0 || rest[0] == '\n' || rest[0] == ' ' || rest[0] == '\r' {
		return m
	}
	return 0
}
