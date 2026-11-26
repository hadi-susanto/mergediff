package session

import (
	"fmt"
	"path/filepath"

	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

// MergeSession is a 3-way merge: git mergetool's files, or a text
// conflict opened from the Conflicts window.
type MergeSession struct {
	view   View
	eol    textio.EOL // the style the result is written in
	output string     // MERGED
	// saveTo, when set, saves the result instead of writing output: a
	// merge opened from the Conflicts window writes the working-tree file
	// and runs git add.
	saveTo func(text string, eol textio.EOL) error
}

// View returns what the frontend shows.
func (s *MergeSession) View() View { return s.view }

// ExitCode: closing a merge without Quit (no Save) counts as cancelled.
func (s *MergeSession) ExitCode(code int, quit bool) int {
	if !quit {
		return exitCancel
	}
	return code
}

// LoadMerge loads the three inputs of a merge whose result goes to output.
// A missing or empty BASE path is treated as an empty file. With autoApply
// the initial result already contains every non-conflicting change;
// otherwise it is the base text.
func LoadMerge(base, local, remote, output string, autoApply bool) (*MergeSession, error) {
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
func newMerge(b, l, r textio.File, paths []string, title, output string, autoApply bool) *MergeSession {
	bl, ll, rl := textio.SplitLines(b.Text), textio.SplitLines(l.Text), textio.SplitLines(r.Text)
	chunks := merge.Merge(bl, ll, rl)
	initial := b.Text
	if autoApply {
		initial = merge.ApplyNonConflicting(bl, ll, rl, chunks, merge.FromBoth)
	}
	eol := textio.DetectEOL(l, b)
	return &MergeSession{
		view: View{
			Mode:  Merge,
			Paths: paths,
			Title: title,
			Text: &Texts{
				Left:     l.Text,
				Right:    r.Text,
				Chunks:   nonNil(chunks),
				Language: Language(output),
				EOL:      eol.Name(),
			},
			Merge: &MergeView{Base: b.Text, InitialResult: initial, AutoApplied: autoApply},
		},
		eol:    eol,
		output: output,
	}
}

// Save writes the merge result (LF text, as the editor produces it) to
// MERGED in the detected line-ending style.
func (s *MergeSession) Save(result string) error {
	if s.saveTo != nil {
		return s.saveTo(result, s.eol)
	}
	if err := textio.WriteFile(s.output, result, s.eol); err != nil {
		return fmt.Errorf("write MERGED: %w", err)
	}
	return nil
}
