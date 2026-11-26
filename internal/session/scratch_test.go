package session

import (
	"errors"
	"reflect"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/merge"
	"github.com/hadi-susanto/mergediff/internal/textio"
)

func TestScratchAndCompare(t *testing.T) {
	s := NewScratch()
	if s.view.Mode != Scratch || s.view.Text.Chunks == nil || s.view.Paths == nil || s.view.Text.Left != "" || s.view.Text.Right != "" {
		t.Fatalf("NewScratch = %+v", s)
	}

	chunks, err := Compare("a\nb\nc\n", "a\nB\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []merge.Chunk{
		{Kind: merge.Unchanged, Base: merge.Range{Start: 0, End: 1}, Local: merge.Range{Start: 0, End: 1}, Remote: merge.Range{Start: 0, End: 1}},
		{Kind: merge.RemoteOnly, Base: merge.Range{Start: 1, End: 2}, Local: merge.Range{Start: 1, End: 2}, Remote: merge.Range{Start: 1, End: 2}},
		{Kind: merge.Unchanged, Base: merge.Range{Start: 2, End: 3}, Local: merge.Range{Start: 2, End: 3}, Remote: merge.Range{Start: 2, End: 3}},
	}
	if !reflect.DeepEqual(chunks, want) {
		t.Fatalf("Compare = %+v", chunks)
	}

	// Pasted CRLF text equals the LF text; empty texts give [] not nil.
	for _, pair := range [][2]string{{"a\r\nb\r\n", "a\nb\n"}, {"", ""}} {
		chunks, err := Compare(pair[0], pair[1])
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range chunks {
			if c.Kind != merge.Unchanged {
				t.Fatalf("Compare(%q, %q) has a change: %+v", pair[0], pair[1], chunks)
			}
		}
		if chunks == nil {
			t.Fatal("chunks are nil")
		}
	}

	if _, err := Compare("a\x00b", "x"); !errors.Is(err, textio.ErrBinary) {
		t.Fatalf("NUL text: err = %v", err)
	}
}
