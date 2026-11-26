package gui

import (
	"context"

	"github.com/hadi-susanto/mergediff/internal/diff"
	"github.com/hadi-susanto/mergediff/internal/dirdiff"
	"github.com/hadi-susanto/mergediff/internal/gitconflict"
	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/session"
)

// The exported methods below are bound to the frontend. Each one finds the
// calling window's state and uses its session; the leading context is
// supplied by Wails and does not appear in the generated bindings.

// LoadSession returns what the calling window shows: its session's View.
func (w *Windows) LoadSession(ctx context.Context) (session.View, error) {
	win, err := w.caller(ctx)
	if err != nil {
		return session.View{}, err
	}
	return win.sess.View, nil
}

// DirEntries waits for the calling window's directory comparison and
// returns its entries. The frontend shows progress meanwhile.
func (w *Windows) DirEntries(ctx context.Context) ([]dirdiff.Entry, error) {
	win, err := w.caller(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := win.sess.DirEntries()
	if err == nil && entries == nil {
		entries = []dirdiff.Entry{}
	}
	return entries, err
}

// DirStats returns the line counts of the calling window's directory
// comparison computed since the previous call.
func (w *Windows) DirStats(ctx context.Context) (dirdiff.StatsBatch, error) {
	win, err := w.caller(ctx)
	if err != nil {
		return dirdiff.StatsBatch{}, err
	}
	return win.sess.DirStats()
}

// DirProgress reports how far the calling window's directory comparison got.
func (w *Windows) DirProgress(ctx context.Context) (dirdiff.Snapshot, error) {
	win, err := w.caller(ctx)
	if err != nil {
		return dirdiff.Snapshot{}, err
	}
	return win.sess.DirProgress(), nil
}

// OpenEntry opens a file of the calling window's directory comparison
// (path relative, "/" separators) in a diff window of its own, or focuses
// the window already showing it.
func (w *Windows) OpenEntry(ctx context.Context, path string) error {
	name, win, err := w.callerName(ctx)
	if err != nil {
		return err
	}
	return w.openEntry(name, win, path)
}

// Conflicts re-reads the calling window's conflicts (mergediff resolve).
// Open lists the files it has open in merge windows.
func (w *Windows) Conflicts(ctx context.Context) (session.ConflictList, error) {
	name, win, err := w.callerName(ctx)
	if err != nil {
		return session.ConflictList{}, err
	}
	list, err := win.sess.Conflicts()
	list.Open = w.openMerges(name, false)
	return list, err
}

// OpenMerge opens a text conflict of the calling Conflicts window in a
// merge window of its own, or focuses the one already showing it.
func (w *Windows) OpenMerge(ctx context.Context, path string) error {
	name, win, err := w.callerName(ctx)
	if err != nil {
		return err
	}
	return w.openMerge(name, win, path)
}

// ShowChanges opens a modify/delete conflict's changes in a diff window.
func (w *Windows) ShowChanges(ctx context.Context, path string) error {
	name, win, err := w.callerName(ctx)
	if err != nil {
		return err
	}
	return w.showChanges(name, win, path)
}

// UnsavedMerges returns the files the calling window has open in merge
// windows with unsaved edits, so it can ask before closing them.
func (w *Windows) UnsavedMerges(ctx context.Context) ([]string, error) {
	name, _, err := w.callerName(ctx)
	if err != nil {
		return nil, err
	}
	return w.openMerges(name, true), nil
}

// SetDirty records whether the calling merge window's result has unsaved
// edits.
func (w *Windows) SetDirty(ctx context.Context, dirty bool) error {
	win, err := w.caller(ctx)
	if err != nil {
		return err
	}
	win.SetDirty(dirty)
	return nil
}

// AcceptSide resolves the calling window's conflicts at paths by taking
// side ("yours" or "theirs"), and returns the refreshed list. Files open in
// a merge window are refused.
func (w *Windows) AcceptSide(ctx context.Context, paths []string, side gitconflict.Side) (session.ConflictList, error) {
	name, win, err := w.callerName(ctx)
	if err != nil {
		return session.ConflictList{}, err
	}
	list, err := w.acceptSide(name, win, paths, side)
	list.Open = w.openMerges(name, false)
	return list, err
}

// MergeCounts counts the conflicts and non-conflicting changes of a text
// conflict of the calling window, for its side panel.
func (w *Windows) MergeCounts(ctx context.Context, path string) (session.MergeCounts, error) {
	win, err := w.caller(ctx)
	if err != nil {
		return session.MergeCounts{}, err
	}
	return win.sess.MergeCounts(path)
}

// Edited returns those of paths (conflicts of the calling window) whose
// working-tree file was edited since Git stopped.
func (w *Windows) Edited(ctx context.Context, paths []string) ([]string, error) {
	win, err := w.caller(ctx)
	if err != nil {
		return nil, err
	}
	return win.sess.Edited(paths)
}

// Save writes the calling window's merge result.
func (w *Windows) Save(ctx context.Context, result string) error {
	win, err := w.caller(ctx)
	if err != nil {
		return err
	}
	return win.sess.Save(result)
}

// Quit closes the calling window. For the main window the process then
// exits with code; a child window just closes.
func (w *Windows) Quit(ctx context.Context, code int) error {
	win, err := w.caller(ctx)
	if err != nil {
		return err
	}
	win.Quit(code)
	return nil
}

// GuardClose turns the calling window's close guard on or off.
func (w *Windows) GuardClose(ctx context.Context, on bool) error {
	win, err := w.caller(ctx)
	if err != nil {
		return err
	}
	win.GuardClose(on)
	return nil
}

// CloseRequestSeen acknowledges the calling window's close request.
func (w *Windows) CloseRequestSeen(ctx context.Context) error {
	win, err := w.caller(ctx)
	if err != nil {
		return err
	}
	win.CloseRequestSeen()
	return nil
}

// Settings returns the effective settings of the calling window.
func (w *Windows) Settings(ctx context.Context) (map[string]string, error) {
	win, err := w.caller(ctx)
	if err != nil {
		return nil, err
	}
	return win.settingValues(), nil
}

// CharDiff returns character-level differences between two texts.
func (w *Windows) CharDiff(left, right string) []diff.Span {
	spans := diff.Chars(left, right)
	if spans == nil {
		return []diff.Span{}
	}
	return spans
}

// Compare diffs two texts for the scratch view.
func (w *Windows) Compare(left, right string) ([]merge.Chunk, error) {
	return session.Compare(left, right)
}

// Version returns the build version for the status bar.
func (w *Windows) Version() string {
	return w.version
}
