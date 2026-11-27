package session

import (
	"fmt"

	"github.com/hadi-susanto/mergediff/internal/textio"
)

// DiffSession is a read-only two-way diff: two files, a file of a
// directory comparison, or a modify/delete conflict's changes.
type DiffSession struct{ view View }

// View returns what the frontend shows.
func (s *DiffSession) View() View { return s.view }

// ExitCode: closing a diff ends normally, unless Quit said otherwise.
func (s *DiffSession) ExitCode(code int, quit bool) int { return code }

// LoadDiff loads LEFT and RIGHT for a read-only two-way diff.
func LoadDiff(left, right string) (*DiffSession, error) {
	l, err := textio.ReadFile(left)
	if err != nil {
		return nil, fmt.Errorf("read LEFT: %w", err)
	}
	r, err := textio.ReadFile(right)
	if err != nil {
		return nil, fmt.Errorf("read RIGHT: %w", err)
	}
	eol := textio.DetectEOL(r, l)
	return &DiffSession{
		view: View{
			Mode:  Diff,
			Paths: []string{left, right},
			Title: diffTitle(left, right),
			Text:  diffTexts(l.Text, r.Text, Language(right), eol),
		},
	}, nil
}

// LoadPair loads two files for a read-only diff opened from a directory
// comparison. An empty path is a file missing on that side: it shows as
// empty text. title names the window, e.g. the file's relative path.
func LoadPair(left, right, title string) (*DiffSession, error) {
	l, err := textio.ReadOptional(left)
	if err != nil {
		return nil, fmt.Errorf("read LEFT: %w", err)
	}
	r, err := textio.ReadOptional(right)
	if err != nil {
		return nil, fmt.Errorf("read RIGHT: %w", err)
	}
	eol := textio.DetectEOL(r, l)
	return &DiffSession{
		view: View{
			Mode:  Diff,
			Paths: []string{left, right},
			Title: title,
			Text:  diffTexts(l.Text, r.Text, Language(title), eol),
		},
	}, nil
}
