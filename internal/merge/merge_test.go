package merge

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/textio"
)

// checkChunks verifies the invariants every Merge result must satisfy.
func checkChunks(t *testing.T, base, local, remote []string, chunks []Chunk) {
	t.Helper()
	var b, l, r int
	for i, c := range chunks {
		if c.Base.Start != b || c.Local.Start != l || c.Remote.Start != r {
			t.Fatalf("chunk %d %+v: gap or overlap (expected starts %d/%d/%d)", i, c, b, l, r)
		}
		if c.Base.Len() < 0 || c.Local.Len() < 0 || c.Remote.Len() < 0 {
			t.Fatalf("chunk %d %+v: negative range", i, c)
		}
		if c.Base.Len() == 0 && c.Local.Len() == 0 && c.Remote.Len() == 0 {
			t.Fatalf("chunk %d %+v: empty", i, c)
		}
		bs := base[c.Base.Start:c.Base.End]
		ls := local[c.Local.Start:c.Local.End]
		rs := remote[c.Remote.Start:c.Remote.End]
		eqL, eqR, eqLR := slices.Equal(bs, ls), slices.Equal(bs, rs), slices.Equal(ls, rs)
		ok := false
		switch c.Kind {
		case Unchanged:
			ok = eqL && eqR
			if i > 0 && chunks[i-1].Kind == Unchanged {
				t.Fatalf("chunk %d: two unchanged chunks in a row", i)
			}
		case LocalOnly:
			ok = !eqL && eqR
		case RemoteOnly:
			ok = eqL && !eqR
		case BothSame:
			ok = !eqL && !eqR && eqLR
		case Conflict:
			ok = !eqL && !eqR && !eqLR
		}
		if !ok {
			t.Fatalf("chunk %d %+v: kind does not match its text", i, c)
		}
		b, l, r = c.Base.End, c.Local.End, c.Remote.End
	}
	if b != len(base) || l != len(local) || r != len(remote) {
		t.Fatalf("chunks end at %d/%d/%d, files have %d/%d/%d lines", b, l, r, len(base), len(local), len(remote))
	}
}

func lines(s string) []string { return textio.SplitLines(s) }

func kinds(chunks []Chunk) string {
	var parts []string
	for _, c := range chunks {
		parts = append(parts, string(c.Kind))
	}
	return strings.Join(parts, " ")
}

func TestMergeKinds(t *testing.T) {
	const base = "1\n2\n3\n4\n5\n6\n7\n"
	tests := []struct {
		name          string
		local, remote string
		kinds         string
		applied       string
	}{
		{"no changes", base, base, "unchanged", base},
		{"local only", "1\nX\n3\n4\n5\n6\n7\n", base,
			"unchanged local unchanged", "1\nX\n3\n4\n5\n6\n7\n"},
		{"remote only", base, "1\n2\n3\n4\n5\nY\n7\n",
			"unchanged remote unchanged", "1\n2\n3\n4\n5\nY\n7\n"},
		{"both same", "1\nZ\n3\n4\n5\n6\n7\n", "1\nZ\n3\n4\n5\n6\n7\n",
			"unchanged both unchanged", "1\nZ\n3\n4\n5\n6\n7\n"},
		{"conflict", "1\nL\n3\n4\n5\n6\n7\n", "1\nR\n3\n4\n5\n6\n7\n",
			"unchanged conflict unchanged", base},
		{"separate edits", "1\nL\n3\n4\n5\n6\n7\n", "1\n2\n3\n4\n5\nR\n7\n",
			"unchanged local unchanged remote unchanged", "1\nL\n3\n4\n5\nR\n7\n"},
		{"adjacent edits conflict", "1\nL\n3\n4\n5\n6\n7\n", "1\n2\nR\n4\n5\n6\n7\n",
			"unchanged conflict unchanged", base},
		{"insert at same point conflicts", "1\n2\nL\n3\n4\n5\n6\n7\n", "1\n2\nR\n3\n4\n5\n6\n7\n",
			"unchanged conflict unchanged", base},
		{"chained overlaps form one conflict", "1\nL\nL\n4\nL\n6\n7\n", "1\n2\nR\nR\n5\n6\n7\n",
			"unchanged conflict unchanged", base},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, l, r := lines(base), lines(tt.local), lines(tt.remote)
			chunks := Merge(b, l, r)
			checkChunks(t, b, l, r, chunks)
			if got := kinds(chunks); got != tt.kinds {
				t.Fatalf("kinds = %q, want %q", got, tt.kinds)
			}
			if got := ApplyNonConflicting(b, l, r, chunks, FromBoth); got != tt.applied {
				t.Fatalf("applied = %q, want %q", got, tt.applied)
			}
		})
	}
}

func TestApplySources(t *testing.T) {
	base := lines("a\nb\nc\nd\ne\nf\ng\n")
	local := lines("a\nL\nc\nd\nS\nf\ng\n")
	remote := lines("a\nb\nc\nd\nS\nf\nR\n")
	chunks := Merge(base, local, remote)
	if got, want := kinds(chunks), "unchanged local unchanged both unchanged remote"; got != want {
		t.Fatalf("kinds = %q, want %q", got, want)
	}
	for _, tt := range []struct {
		from Source
		want string
	}{
		{FromBoth, "a\nL\nc\nd\nS\nf\nR\n"},
		{FromLocal, "a\nL\nc\nd\nS\nf\ng\n"},
		{FromRemote, "a\nb\nc\nd\nS\nf\nR\n"},
	} {
		if got := ApplyNonConflicting(base, local, remote, chunks, tt.from); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.from, got, tt.want)
		}
	}
	want := Counts{Changes: 3, NonConflicting: 3}
	if got := Count(chunks); !reflect.DeepEqual(got, want) {
		t.Errorf("Count = %+v, want %+v", got, want)
	}
}

// Random three-way inputs: invariants hold, and when one side is unchanged
// the merge applies the other side exactly.
func TestMergeRandom(t *testing.T) {
	rnd := rand.New(rand.NewPCG(7, 11))
	mutate := func(s []string) []string {
		out := slices.Clone(s)
		for k := rnd.IntN(4); k > 0; k-- {
			p := rnd.IntN(len(out) + 1)
			switch rnd.IntN(3) {
			case 0: // insert
				out = slices.Insert(out, p, fmt.Sprintf("n%d\n", rnd.IntN(5)))
			case 1: // delete
				if p < len(out) {
					out = slices.Delete(out, p, p+1)
				}
			default: // replace
				if p < len(out) {
					out[p] = fmt.Sprintf("r%d\n", rnd.IntN(5))
				}
			}
		}
		return out
	}
	for iter := 0; iter < 5000; iter++ {
		base := make([]string, rnd.IntN(10))
		for i := range base {
			base[i] = fmt.Sprintf("%d\n", rnd.IntN(6))
		}
		local, remote := mutate(base), mutate(base)
		chunks := Merge(base, local, remote)
		checkChunks(t, base, local, remote, chunks)

		if got := ApplyNonConflicting(base, local, base, Merge(base, local, base), FromBoth); got != strings.Join(local, "") {
			t.Fatalf("local-only merge: got %q, want local %q (base %q)", got, local, base)
		}
		if got := ApplyNonConflicting(base, base, remote, Merge(base, base, remote), FromBoth); got != strings.Join(remote, "") {
			t.Fatalf("remote-only merge: got %q, want remote %q (base %q)", got, remote, base)
		}
		if got := ApplyNonConflicting(base, local, local, Merge(base, local, local), FromBoth); got != strings.Join(local, "") {
			t.Fatalf("identical sides: got %q, want %q", got, local)
		}
	}
}
