package session

import (
	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

// ScratchSession is the editable two-way diff of pasted text (mergediff
// without arguments); the frontend compares with Compare.
type ScratchSession struct{ view View }

// View returns what the frontend shows.
func (s *ScratchSession) View() View { return s.view }

// ExitCode: closing the scratch window ends normally.
func (s *ScratchSession) ExitCode(code int, quit bool) int { return code }

// NewScratch returns an empty scratch session (mergediff without arguments).
func NewScratch() *ScratchSession {
	return &ScratchSession{
		view: View{
			Mode:  Scratch,
			Paths: []string{},
			Title: "scratch",
			Text:  &Texts{Chunks: []merge.Chunk{}, EOL: textio.LF.Name()},
		},
	}
}
