package merge

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

// largeInputs builds a 10,000-line base and two sides with a few hundred
// scattered edits each (changed lines, inserted and deleted blocks).
func largeInputs() (base, local, remote []string) {
	base = make([]string, 10000)
	for i := range base {
		base[i] = fmt.Sprintf("\tcount%d := len(items) + %d // line %d\n", i%40, (i*7919)%1000, i)
	}
	mutate := func(seed uint64, tag string) []string {
		r := rand.New(rand.NewPCG(seed, seed))
		out := slices.Clone(base)
		for pos := len(out) - 1; pos > 10; {
			pos -= 10 + r.IntN(40)
			size := []int{1, 1, 1, 2, 3, 5, 8, 20, 60}[r.IntN(9)]
			switch k := r.IntN(10); {
			case k < 5:
				for j := pos; j < min(pos+size, len(out)); j++ {
					out[j] = tag + out[j]
				}
			case k < 8:
				block := make([]string, size)
				for j := range block {
					block[j] = fmt.Sprintf("\t// %s added %d\n", tag, j)
				}
				out = slices.Insert(out, max(pos, 0), block...)
			default:
				out = slices.Delete(out, max(pos, 0), min(max(pos, 0)+size, len(out)))
			}
		}
		return out
	}
	return base, mutate(2, "L"), mutate(3, "R")
}

// TestLargeMergeIsFast guards against performance regressions (e.g. from a
// dependency upgrade) on the 10,000-line size the UI must handle. The
// budget is generous; on a developer machine this takes well under 0.5 s.
func TestLargeMergeIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("large input")
	}
	base, local, remote := largeInputs()
	start := time.Now()
	chunks := Merge(base, local, remote)
	elapsed := time.Since(start)
	checkChunks(t, base, local, remote, chunks)
	n := Count(chunks)
	t.Logf("10k lines: %d changes (%d conflicts) in %v", n.Changes, n.Conflicts, elapsed)
	if n.Changes < 300 || n.Conflicts == 0 {
		t.Fatalf("generator too tame: %+v", n)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("merging 10k lines took %v, budget 3s", elapsed)
	}
}
