package gitconflict

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// editedOf lists the repository's conflicts and returns whether path is
// edited.
func editedOf(t *testing.T, tr *testRepo, path string) bool {
	t.Helper()
	r := tr.open("")
	cs, err := r.List(nil, r.Operation().Sides())
	if err != nil {
		t.Fatal(err)
	}
	for i := range cs {
		if cs[i].Path == path {
			e, err := r.Edited(&cs[i])
			if err != nil {
				t.Fatalf("Edited(%s): %v", path, err)
			}
			return e
		}
	}
	t.Fatalf("%s is not conflicted", path)
	return false
}

// twoHunks has two conflicting hunks in a.txt, so one can be fixed by hand.
func twoHunks(t *testing.T) *testRepo {
	file := func(a, b string) func(tr *testRepo) {
		return func(tr *testRepo) { tr.write("a.txt", "1\n"+a+"\n3\n4\n5\n6\n7\n"+b+"\n9\n") }
	}
	return divergent(t, file("2", "8"), file("M2", "M8"), file("F2", "F8"))
}

func TestEditedTextConflict(t *testing.T) {
	for _, style := range []string{"", "diff3", "zdiff3"} {
		t.Run("style="+style, func(t *testing.T) {
			tr := twoHunks(t)
			if style != "" {
				tr.git("config", "merge.conflictStyle", style)
			}
			mergeFeature(tr)
			if editedOf(t, tr, "a.txt") {
				t.Fatalf("untouched conflict reported as edited:\n%s", tr.read("a.txt"))
			}
			// Fix the first hunk by hand.
			text := tr.read("a.txt")
			end := strings.Index(text, ">>>>>>> ")
			nl := strings.Index(text[end:], "\n")
			tr.write("a.txt", "1\nfixed\n"+text[end+nl+1:])
			if !editedOf(t, tr, "a.txt") {
				t.Errorf("one hunk fixed: not reported as edited:\n%s", tr.read("a.txt"))
			}
		})
	}
}

func TestEditedRebaseAndMarkersRemoved(t *testing.T) {
	tr := simpleConflict(t)
	rebaseFeature(tr)
	if editedOf(t, tr, "a.txt") {
		t.Fatalf("untouched rebase conflict reported as edited:\n%s", tr.read("a.txt"))
	}
	tr.write("a.txt", "1\nM\nF\n3\n")
	if !editedOf(t, tr, "a.txt") {
		t.Error("markers removed: not reported as edited")
	}
	// Equal to a stage (git checkout --theirs, no add): not edited.
	tr.git("checkout", "--theirs", "--", "a.txt")
	if editedOf(t, tr, "a.txt") {
		t.Error("file equal to a stage reported as edited")
	}
	os.Remove(tr.path("a.txt"))
	if !editedOf(t, tr, "a.txt") {
		t.Error("deleted file not reported as edited")
	}
}

func TestEditedCRLF(t *testing.T) {
	tr := divergent(t,
		func(tr *testRepo) {
			tr.write(".gitattributes", "*.txt text eol=crlf\n")
			editLine2("a.txt", "2")(tr)
		},
		editLine2("a.txt", "M"), editLine2("a.txt", "F"))
	mergeFeature(tr)
	if !strings.Contains(tr.read("a.txt"), "\r\n") {
		t.Fatalf("checkout did not produce CRLF: %q", tr.read("a.txt"))
	}
	if editedOf(t, tr, "a.txt") {
		t.Errorf("untouched CRLF conflict reported as edited:\n%q", tr.read("a.txt"))
	}
}

func TestEditedBinaryAndModifyDelete(t *testing.T) {
	tr := divergent(t,
		func(tr *testRepo) {
			tr.write("bin", "base\x00")
			tr.write("gone.txt", "base\n")
		},
		func(tr *testRepo) {
			tr.write("bin", "main\x00")
			tr.write("gone.txt", "main\n")
		},
		func(tr *testRepo) {
			tr.write("bin", "feature\x00")
			tr.remove("gone.txt")
		})
	mergeFeature(tr)
	if editedOf(t, tr, "bin") || editedOf(t, tr, "gone.txt") {
		t.Fatal("untouched binary or modify/delete reported as edited")
	}
	tr.write("bin", "hand\x00")
	tr.write("gone.txt", "main\nmore\n")
	if !editedOf(t, tr, "bin") || !editedOf(t, tr, "gone.txt") {
		t.Error("edited binary or modify/delete not reported")
	}
}

func TestEditedSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	tr := divergent(t,
		func(tr *testRepo) { mustSymlink(t, "base-target", tr.path("link")) },
		func(tr *testRepo) { os.Remove(tr.path("link")); mustSymlink(t, "main-target", tr.path("link")) },
		func(tr *testRepo) { os.Remove(tr.path("link")); mustSymlink(t, "feature-target", tr.path("link")) })
	mergeFeature(tr)
	if editedOf(t, tr, "link") {
		t.Fatal("untouched symlink reported as edited")
	}
	os.Remove(tr.path("link"))
	mustSymlink(t, "elsewhere", tr.path("link"))
	if !editedOf(t, tr, "link") {
		t.Error("retargeted symlink not reported as edited")
	}
}

func TestNormalizeMarkers(t *testing.T) {
	in := "a\r\n<<<<<<< HEAD\r\nx\n||||||| base\n=======\ny\n>>>>>>> feature\n======== not a marker? \n<<<<<<<<<< long\n<<<<<< short\n"
	want := "a\n<\nx\n|\n=\ny\n>\n=\n<\n<<<<<< short\n"
	if got := string(normalizeMarkers([]byte(in))); got != want {
		t.Errorf("normalizeMarkers =\n%q\nwant\n%q", got, want)
	}
}
