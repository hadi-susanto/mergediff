package diff

import (
	"reflect"
	"testing"
	"unicode/utf16"
)

func TestChars(t *testing.T) {
	tests := []struct {
		name        string
		left, right string
		want        []Span
	}{
		{"equal", "abc", "abc", nil},
		{"replace word", "return \"Hi, \" + name", "return \"Hey \" + name",
			// "Hi, " -> "Hey ": common "H" and " ", so "i," vs "ey" differ.
			[]Span{{Deleted, 9, 11}, {Inserted, 9, 11}}},
		{"pure insert", "abc", "abXc", []Span{{Inserted, 2, 3}}},
		{"pure delete", "abXc", "abc", []Span{{Deleted, 2, 3}}},
		{"from empty", "", "new", []Span{{Inserted, 0, 3}}},
		{"to empty", "old", "", []Span{{Deleted, 0, 3}}},
		{"gap of 2 is merged", "aXbcYd", "abcd", []Span{{Deleted, 1, 5}}},
		{"gap of 3 is kept", "aXbcdYe", "abcde", []Span{{Deleted, 1, 2}, {Deleted, 5, 6}}},
		{"gap with newline is kept", "aX\nYb", "a\nb", []Span{{Deleted, 1, 2}, {Deleted, 3, 4}}},
		// U+1F600 is two UTF-16 code units.
		{"utf16 offsets", "😀a", "😀b", []Span{{Deleted, 2, 3}, {Inserted, 2, 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Chars(tt.left, tt.right)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			checkSpans(t, tt.left, tt.right, got)
		})
	}
}

// checkSpans verifies that spans are sorted, disjoint and in bounds.
func checkSpans(t *testing.T, left, right string, spans []Span) {
	t.Helper()
	limit := map[SpanKind]int{
		Deleted:  len(utf16.Encode([]rune(left))),
		Inserted: len(utf16.Encode([]rune(right))),
	}
	last := map[SpanKind]int{Deleted: -1, Inserted: -1}
	for _, s := range spans {
		if s.Start >= s.End || s.Start <= last[s.Kind] || s.End > limit[s.Kind] {
			t.Fatalf("bad span %v in %v", s, spans)
		}
		last[s.Kind] = s.End
	}
}

func TestCharsMultiline(t *testing.T) {
	left := "func a() {\n\treturn 1\n}\n"
	right := "func a() {\n\treturn 2\n}\n"
	got := Chars(left, right)
	want := []Span{{Deleted, 19, 20}, {Inserted, 19, 20}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
