package diff

import (
	"math/rand/v2"
	"reflect"
	"strings"
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
		// Spans cutting through a word are widened to the whole word.
		{"replace word", "return \"Hi, \" + name", "return \"Hey \" + name",
			[]Span{{Deleted, 8, 11}, {Inserted, 8, 11}}},
		{"words in a sentence", "this is sample line before edit", "this is edited line after edit",
			[]Span{{Deleted, 8, 14}, {Deleted, 20, 26}, {Inserted, 8, 14}, {Inserted, 20, 25}}},
		{"string argument", `fmt.Println(greet("world"))`, `fmt.Println(greet("Go"))`,
			[]Span{{Deleted, 19, 24}, {Inserted, 19, 21}}},
		{"insert inside a word", "abc", "abXc", []Span{{Inserted, 0, 4}}},
		{"delete inside a word", "abXc", "abc", []Span{{Deleted, 0, 4}}},
		{"non-ASCII word", "naïve café", "naïve cafe", []Span{{Deleted, 6, 10}, {Inserted, 6, 10}}},
		{"from empty", "", "new", []Span{{Inserted, 0, 3}}},
		{"to empty", "old", "", []Span{{Deleted, 0, 3}}},
		// Gap rules, with punctuation so widening does not interfere.
		{"gap of 1 is merged", "1+2+3", "1-2-3", []Span{{Deleted, 1, 4}, {Inserted, 1, 4}}},
		{"gap of 3 is kept", "1+234+5", "1-234-5",
			[]Span{{Deleted, 1, 2}, {Deleted, 5, 6}, {Inserted, 1, 2}, {Inserted, 5, 6}}},
		{"gap with newline is kept", "x+\n+y", "x-\n-y",
			[]Span{{Deleted, 1, 2}, {Deleted, 3, 4}, {Inserted, 1, 2}, {Inserted, 3, 4}}},
		// U+1F600 is two UTF-16 code units and not a word character.
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

func TestCharsRandomWordBoundaries(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 5))
	pieces := []string{"a", "b", "é", "1", "_", " ", ".", "\n", "😀", "+"}
	gen := func() string {
		var sb strings.Builder
		for n := r.IntN(14); n > 0; n-- {
			sb.WriteString(pieces[r.IntN(len(pieces))])
		}
		return sb.String()
	}
	isWordAt := func(u []uint16, i int) bool {
		rs := utf16.Decode(u[i : i+1])
		return len(rs) == 1 && isWordRune(rs[0])
	}
	for iter := 0; iter < 3000; iter++ {
		left, right := gen(), gen()
		spans := Chars(left, right)
		checkSpans(t, left, right, spans)
		for _, s := range spans {
			text := left
			if s.Kind == Inserted {
				text = right
			}
			u := utf16.Encode([]rune(text))
			if s.Start > 0 && isWordAt(u, s.Start-1) && isWordAt(u, s.Start) ||
				s.End < len(u) && isWordAt(u, s.End-1) && isWordAt(u, s.End) {
				t.Fatalf("%q -> %q: span %v cuts through a word", left, right, s)
			}
		}
	}
}
