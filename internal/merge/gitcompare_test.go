package merge

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/textio"
)

// TestAgainstGitMergeFile runs `git merge-file -p --diff3` on every testdata
// case and checks that git produces exactly the output our chunks imply:
// non-conflicting changes applied, and one diff3-style conflict block per
// Conflict chunk with the same local, base and remote text. Git's exit code
// (the number of conflicts) must match too.
func TestAgainstGitMergeFile(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	for _, c := range loadCases(t) {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, text string) string {
				p := filepath.Join(dir, name)
				if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
				return p
			}
			// Compare on LF-normalised text, which is what the engine sees.
			lp := write("local", c.local.Text)
			bp := write("base", c.base.Text)
			rp := write("remote", c.remote.Text)

			cmd := exec.Command(git, "merge-file", "-p", "--diff3",
				"-L", "local", "-L", "base", "-L", "remote", lp, bp, rp)
			// Keep the user's and the system's git config out of the test.
			cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
			out, err := cmd.Output()
			gitConflicts := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
				gitConflicts = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("git merge-file: %v", err)
			}

			base := textio.SplitLines(c.base.Text)
			local := textio.SplitLines(c.local.Text)
			remote := textio.SplitLines(c.remote.Text)
			chunks := Merge(base, local, remote)
			if n := Count(chunks).Conflicts; n != gitConflicts {
				t.Errorf("conflicts: ours %d, git %d", n, gitConflicts)
			}
			if want := renderDiff3(base, local, remote, chunks); string(out) != want {
				t.Errorf("output differs from git\n--- git ---\n%s\n--- ours ---\n%s", out, want)
			}
		})
	}
}

// renderDiff3 renders chunks the way `git merge-file -p --diff3` with labels
// local/base/remote does.
func renderDiff3(base, local, remote []string, chunks []Chunk) string {
	var sb strings.Builder
	section := func(marker string, lines []string, r Range) {
		sb.WriteString(marker + "\n")
		text := strings.Join(lines[r.Start:r.End], "")
		sb.WriteString(text)
		// Git ends a section that lacks a final newline before the next marker.
		if text != "" && !strings.HasSuffix(text, "\n") {
			sb.WriteString("\n")
		}
	}
	for _, c := range chunks {
		if c.Kind != Conflict {
			sb.WriteString(ApplyNonConflicting(base, local, remote, []Chunk{c}, FromBoth))
			continue
		}
		section("<<<<<<< local", local, c.Local)
		section("||||||| base", base, c.Base)
		section("=======", remote, c.Remote)
		sb.WriteString(">>>>>>> remote\n")
	}
	return sb.String()
}
