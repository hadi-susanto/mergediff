//go:build race

package dirdiff

// raceEnabled: the race detector slows the code several times, so timing
// budgets do not apply.
const raceEnabled = true
