package gitconflict

import (
	"os"
	"path/filepath"
	"strings"
)

// OpKind is the operation that stopped with conflicts.
type OpKind string

const (
	// OpNone: conflicts without an operation in progress, e.g. after
	// "git stash pop".
	OpNone       OpKind = "none"
	OpMerge      OpKind = "merge"
	OpRebase     OpKind = "rebase"
	OpCherryPick OpKind = "cherry-pick"
	OpRevert     OpKind = "revert"
)

// Phrase is part of the operation sentence; Strong parts are names
// (branches, commits) the UI shows in bold.
type Phrase struct {
	Text   string `json:"text"`
	Strong bool   `json:"strong"`
}

// Operation describes what is in progress, for the Conflicts window.
type Operation struct {
	Kind OpKind `json:"kind"`
	// Sentence is e.g. "Merging origin/main into feature/checkout".
	Sentence []Phrase `json:"sentence"`
	// YoursLabel and TheirsLabel name the two sides (branch names where
	// possible, else short hashes); TheirsLabel is "" when unknown.
	YoursLabel  string `json:"yoursLabel"`
	TheirsLabel string `json:"theirsLabel"`
	// Next is the command that finishes the operation once every conflict
	// is resolved. mergediff only shows it.
	Next string `json:"next"`
}

// Text returns the sentence as plain text.
func (o Operation) Text() string {
	var b strings.Builder
	for _, p := range o.Sentence {
		b.WriteString(p.Text)
	}
	return b.String()
}

// Sides returns the Yours / Theirs mapping for the operation.
func (o Operation) Sides() Sides { return SidesFor(o.Kind) }

// Operation detects the operation in progress from Git's state files and
// names its sides. It is best effort: a label that cannot be determined
// falls back to a short hash or "HEAD", and it never fails.
func (r *Repo) Operation() Operation {
	names := []string{"rebase-merge", "rebase-apply", "MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD"}
	paths, err := r.gitPaths(names...)
	if err != nil {
		paths = map[string]string{}
	}
	current := r.currentBranch()

	for _, dir := range []string{paths["rebase-merge"], paths["rebase-apply"]} {
		if dir == "" || !isDir(dir) {
			continue
		}
		// rebase-apply is also used by "git am"; "applying" marks that.
		if exists(filepath.Join(dir, "applying")) {
			break
		}
		branch := strings.TrimPrefix(readLine(filepath.Join(dir, "head-name")), "refs/heads/")
		if branch == "" || branch == "detached HEAD" {
			branch = r.label(readLine(filepath.Join(dir, "orig-head")))
		}
		upstream := r.label(readLine(filepath.Join(dir, "onto")))
		return Operation{
			Kind:        OpRebase,
			Sentence:    []Phrase{{"Rebasing ", false}, {branch, true}, {" onto ", false}, {upstream, true}},
			YoursLabel:  branch,
			TheirsLabel: upstream,
			Next:        "git rebase --continue",
		}
	}
	if head := readLine(paths["MERGE_HEAD"]); head != "" {
		theirs := r.label(head)
		return Operation{
			Kind:        OpMerge,
			Sentence:    []Phrase{{"Merging ", false}, {theirs, true}, {" into ", false}, {current, true}},
			YoursLabel:  current,
			TheirsLabel: theirs,
			Next:        "git merge --continue",
		}
	}
	if head := readLine(paths["CHERRY_PICK_HEAD"]); head != "" {
		short, subject := r.commitSummary(head)
		return Operation{
			Kind:        OpCherryPick,
			Sentence:    []Phrase{{"Cherry-picking ", false}, {short, true}, {" " + subject, false}},
			YoursLabel:  current,
			TheirsLabel: short,
			Next:        "git cherry-pick --continue",
		}
	}
	if head := readLine(paths["REVERT_HEAD"]); head != "" {
		short, subject := r.commitSummary(head)
		return Operation{
			Kind:        OpRevert,
			Sentence:    []Phrase{{"Reverting ", false}, {short, true}, {" " + subject, false}},
			YoursLabel:  current,
			TheirsLabel: short,
			Next:        "git revert --continue",
		}
	}
	return Operation{
		Kind:       OpNone,
		Sentence:   []Phrase{{"Resolving conflicts", false}},
		YoursLabel: current,
		Next:       "git commit",
	}
}

// currentBranch is HEAD's branch name, else its short hash, else "HEAD".
func (r *Repo) currentBranch() string {
	if out, err := r.git("symbolic-ref", "--short", "-q", "HEAD"); err == nil {
		if b := firstLine(out); b != "" {
			return b
		}
	}
	if s := r.short("HEAD"); s != "" {
		return s
	}
	return "HEAD"
}

// label names a commit by a branch (or tag) pointing at or near it, else
// by its short hash.
func (r *Repo) label(rev string) string {
	if rev == "" {
		return "HEAD"
	}
	out, err := r.git("name-rev", "--name-only", "--no-undefined", rev)
	if err == nil {
		name := firstLine(out)
		name = strings.TrimSuffix(name, "^0")
		name = strings.TrimPrefix(name, "remotes/")
		name = strings.TrimPrefix(name, "tags/")
		if name != "" {
			return name
		}
	}
	if s := r.short(rev); s != "" {
		return s
	}
	return shortHash(rev)
}

// short returns rev's abbreviated hash, or "".
func (r *Repo) short(rev string) string {
	out, err := r.git("rev-parse", "--short", "--verify", "-q", rev+"^{commit}")
	if err != nil {
		return ""
	}
	return firstLine(out)
}

// commitSummary returns a commit's short hash and subject.
func (r *Repo) commitSummary(rev string) (short, subject string) {
	out, err := r.git("log", "-1", "--no-show-signature", "--format=%h%x00%s", rev, "--")
	if err == nil {
		if h, s, ok := strings.Cut(firstLine(out), "\x00"); ok && h != "" {
			return h, s
		}
	}
	return shortHash(rev), ""
}

func shortHash(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}

// readLine returns the first line of a file, trimmed, or "" if it cannot
// be read.
func readLine(name string) string {
	if name == "" {
		return ""
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(firstLine(data))
}

func isDir(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}

func exists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}
