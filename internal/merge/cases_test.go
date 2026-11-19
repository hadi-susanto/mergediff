package merge

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hadi-susanto/mergediff/internal/textio"
)

// mergeCase is one directory under testdata/:
//
//	base      BASE file; may be absent (treated as empty, like Git add/add)
//	local     LOCAL file
//	remote    REMOTE file
//	expected  exact bytes after applying all non-conflicting changes and
//	          writing with the line ending detected from LOCAL (then BASE);
//	          conflicts keep the base text
//	chunks    one chunk per line: "<kind> <base> <local> <remote>", each
//	          range written as start-end (0-based, end exclusive)
type mergeCase struct {
	name                string
	base, local, remote textio.File
	expected            []byte
	chunks              []Chunk
}

func loadCases(t *testing.T) []mergeCase {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join("testdata", "*"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []mergeCase
	for _, dir := range dirs {
		c := mergeCase{name: filepath.Base(dir)}
		var err error
		if c.base, err = textio.ReadOptional(filepath.Join(dir, "base")); err != nil {
			t.Fatal(err)
		}
		if c.local, err = textio.ReadFile(filepath.Join(dir, "local")); err != nil {
			t.Fatal(err)
		}
		if c.remote, err = textio.ReadFile(filepath.Join(dir, "remote")); err != nil {
			t.Fatal(err)
		}
		if c.expected, err = os.ReadFile(filepath.Join(dir, "expected")); err != nil {
			t.Fatal(err)
		}
		spec, err := os.ReadFile(filepath.Join(dir, "chunks"))
		if err != nil {
			t.Fatal(err)
		}
		if c.chunks, err = parseChunks(string(spec)); err != nil {
			t.Fatalf("%s/chunks: %v", dir, err)
		}
		cases = append(cases, c)
	}
	return cases
}

func parseChunks(spec string) ([]Chunk, error) {
	var chunks []Chunk
	for _, line := range strings.Split(strings.TrimSpace(spec), "\n") {
		var c Chunk
		var kind string
		_, err := fmt.Sscanf(line, "%s %d-%d %d-%d %d-%d", &kind,
			&c.Base.Start, &c.Base.End, &c.Local.Start, &c.Local.End, &c.Remote.Start, &c.Remote.End)
		if err != nil {
			return nil, fmt.Errorf("%q: %v", line, err)
		}
		c.Kind = ChunkKind(kind)
		chunks = append(chunks, c)
	}
	return chunks, nil
}

func TestCases(t *testing.T) {
	cases := loadCases(t)
	if len(cases) < 15 {
		t.Fatalf("only %d cases in testdata, want at least 15", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := textio.SplitLines(c.base.Text)
			local := textio.SplitLines(c.local.Text)
			remote := textio.SplitLines(c.remote.Text)
			chunks := Merge(base, local, remote)
			checkChunks(t, base, local, remote, chunks)
			if !reflect.DeepEqual(chunks, c.chunks) {
				t.Errorf("chunks:\n got  %+v\n want %+v", chunks, c.chunks)
			}
			result := ApplyNonConflicting(base, local, remote, chunks, FromBoth)
			got := textio.Encode(result, textio.DetectEOL(c.local, c.base))
			if !bytes.Equal(got, c.expected) {
				t.Errorf("result:\n got  %q\n want %q", got, c.expected)
			}
		})
	}
}
