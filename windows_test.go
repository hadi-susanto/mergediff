package main

import (
	"errors"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/session"
)

// fakeWindow is a windowHandle that records Close and Focus calls.
type fakeWindow struct {
	id      uint
	name    string
	closed  int
	focused int
}

func (f *fakeWindow) ID() uint     { return f.id }
func (f *fakeWindow) Name() string { return f.name }
func (f *fakeWindow) Close()       { f.closed++ }
func (f *fakeWindow) Focus()       { f.focused++ }

func TestWindowsRegistry(t *testing.T) {
	w := newWindows()
	mainApp := newApp(&session.Session{Mode: session.Diff})
	main := &fakeWindow{id: 1, name: "main"}
	w.registerMain(main, mainApp)

	if got, err := w.byID(1); err != nil || got != mainApp {
		t.Fatalf("byID(main) = %p, %v; want the main App", got, err)
	}
	if !w.isMain(1) {
		t.Error("isMain(1) = false")
	}

	childApp := newApp(&session.Session{Mode: session.Diff})
	child := &fakeWindow{id: 2, name: "child-2"}
	if err := w.registerChild(child, childApp, 1, "a/b.txt"); err != nil {
		t.Fatal(err)
	}
	grandchild := &fakeWindow{id: 3, name: "child-3"}
	if err := w.registerChild(grandchild, newApp(&session.Session{Mode: session.Diff}), 2, "x"); err != nil {
		t.Fatal(err)
	}
	if w.isMain(2) {
		t.Error("isMain(child) = true")
	}
	if got, err := w.byID(2); err != nil || got != childApp {
		t.Fatalf("byID(child) = %p, %v; want the child App", got, err)
	}
	if h, ok := w.child(1, "a/b.txt"); !ok || h != child {
		t.Errorf("child(1, a/b.txt) = %v, %v; want the child window", h, ok)
	}
	if _, ok := w.child(1, "other"); ok {
		t.Error("child(1, other) found a window")
	}
	if got := w.children(1); len(got) != 2 {
		t.Errorf("children(main) = %d windows, want 2 (child and grandchild)", len(got))
	}

	w.remove(2)
	if _, err := w.byID(2); !errors.Is(err, errUnknownWindow) {
		t.Errorf("byID(removed) error = %v, want errUnknownWindow", err)
	}
	if _, ok := w.child(1, "a/b.txt"); ok {
		t.Error("removed child still found")
	}
}

func TestWindowsRegisterChildUnknownParent(t *testing.T) {
	w := newWindows()
	err := w.registerChild(&fakeWindow{id: 5}, newApp(&session.Session{Mode: session.Diff}), 99, "k")
	if !errors.Is(err, errUnknownWindow) {
		t.Fatalf("registerChild with unknown parent: %v, want errUnknownWindow", err)
	}
}
