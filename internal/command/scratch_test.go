package command

import (
	"testing"

	"github.com/hadi-susanto/mergediff/internal/session"
)

func TestScratchOpensWindow(t *testing.T) {
	f := newFake(t, 7)
	cmd, err := newScratch(parse(t))
	if err != nil {
		t.Fatal(err)
	}
	if code, err := cmd.Run(f.env); code != 7 || err != nil || !isScratch(f.opened) {
		t.Fatalf("scratch: code %d, %v, session %+v", code, err, f.opened)
	}
	for _, args := range [][]string{{"-o", "m"}, {"--auto-apply"}} {
		_, err := newScratch(parse(t, args...))
		wantUsageError(t, err, "-o and --auto-apply need BASE LOCAL REMOTE")
	}
}

func isScratch(s session.Session) bool {
	_, ok := s.(*session.ScratchSession)
	return ok
}
