package diff

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func split(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func TestLines(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want []Hunk
	}{
		{"equal", "a\nb\n", "a\nb\n", nil},
		{"both empty", "", "", nil},
		{"insert into empty", "", "a\n", []Hunk{{Range{0, 0}, Range{0, 1}}}},
		{"delete all", "a\nb\n", "", []Hunk{{Range{0, 2}, Range{0, 0}}}},
		{"change middle", "a\nb\nc\n", "a\nX\nc\n", []Hunk{{Range{1, 2}, Range{1, 2}}}},
		{"insert at start", "a\nb\n", "X\na\nb\n", []Hunk{{Range{0, 0}, Range{0, 1}}}},
		{"insert at end", "a\nb\n", "a\nb\nX\n", []Hunk{{Range{2, 2}, Range{2, 3}}}},
		{"delete middle", "a\nb\nc\n", "a\nc\n", []Hunk{{Range{1, 2}, Range{1, 1}}}},
		{"two hunks", "a\nb\nc\nd\ne\n", "a\nB\nc\nD\ne\n",
			[]Hunk{{Range{1, 2}, Range{1, 2}}, {Range{3, 4}, Range{3, 4}}}},
		{"trailing newline added", "a\nb", "a\nb\n", []Hunk{{Range{1, 2}, Range{1, 2}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Lines(split(tt.a), split(tt.b))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// lcsLen is a reference O(n*m) longest-common-subsequence length.
func lcsLen(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else {
				cur[j] = max(prev[j], cur[j-1])
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// checkHunks verifies structural invariants and that the hunks turn a into b.
func checkHunks(t *testing.T, a, b []string, hunks []Hunk) {
	t.Helper()
	var out []string
	posA, posB := 0, 0
	for i, h := range hunks {
		if h.A.Len() == 0 && h.B.Len() == 0 {
			t.Fatalf("hunk %d is empty: %v", i, h)
		}
		if i > 0 && (h.A.Start <= posA || h.B.Start <= posB) {
			t.Fatalf("hunk %d not separated from previous by an equal line: %v", i, hunks)
		}
		if h.A.Start-posA != h.B.Start-posB {
			t.Fatalf("hunk %d: equal run lengths differ: %v", i, hunks)
		}
		if !slices.Equal(a[posA:h.A.Start], b[posB:h.B.Start]) {
			t.Fatalf("hunk %d: gap before it is not equal", i)
		}
		out = append(out, a[posA:h.A.Start]...)
		out = append(out, b[h.B.Start:h.B.End]...)
		posA, posB = h.A.End, h.B.End
	}
	if !slices.Equal(a[posA:], b[posB:]) {
		t.Fatalf("tail after last hunk is not equal")
	}
	out = append(out, a[posA:]...)
	if !slices.Equal(out, b) {
		t.Fatalf("applying hunks does not reproduce b")
	}
}

func TestLinesRandomMinimal(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	alphabet := []string{"a\n", "b\n", "c\n", "d\n", "\n"}
	gen := func() []string {
		s := make([]string, r.IntN(12))
		for i := range s {
			s[i] = alphabet[r.IntN(len(alphabet))]
		}
		// Only the last line may lack a terminator.
		if len(s) > 0 && r.IntN(3) == 0 {
			s[len(s)-1] = strings.TrimSuffix(s[len(s)-1], "\n") + "z"
		}
		return s
	}
	for iter := 0; iter < 3000; iter++ {
		a, b := gen(), gen()
		hunks := Lines(a, b)
		checkHunks(t, a, b, hunks)
		edits := 0
		for _, h := range hunks {
			edits += h.A.Len() + h.B.Len()
		}
		if want := len(a) + len(b) - 2*lcsLen(a, b); edits != want {
			t.Fatalf("a=%q b=%q: %d edits, minimal is %d", a, b, edits, want)
		}
	}
}

// Many scattered edits must stay separate hunks (go-udiff collapsed them).
func TestLinesManyScatteredEdits(t *testing.T) {
	const n, every = 10000, 10
	a := make([]string, n)
	b := make([]string, n)
	for i := range a {
		a[i] = "line " + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + "\n"
		a[i] = string(rune(0x4e00+i)) + a[i] // make every line unique
		b[i] = a[i]
		if i%every == every/2 {
			b[i] = "changed\n"
		}
	}
	hunks := Lines(a, b)
	checkHunks(t, a, b, hunks)
	if len(hunks) != n/every {
		t.Fatalf("got %d hunks, want %d", len(hunks), n/every)
	}
	for _, h := range hunks {
		if h.A.Len() != 1 || h.B.Len() != 1 {
			t.Fatalf("hunk %v is not a single-line change", h)
		}
	}
}
