package textio

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		text    string
		eol     EOL
		hasEOL  bool
		wantErr error
	}{
		{name: "empty", in: "", text: "", eol: LF},
		{name: "no newline", in: "abc", text: "abc", eol: LF},
		{name: "lf", in: "a\nb\n", text: "a\nb\n", eol: LF, hasEOL: true},
		{name: "crlf", in: "a\r\nb\r\n", text: "a\nb\n", eol: CRLF, hasEOL: true},
		{name: "crlf no trailing", in: "a\r\nb", text: "a\nb", eol: CRLF, hasEOL: true},
		{name: "mixed mostly crlf", in: "a\r\nb\r\nc\n", text: "a\nb\nc\n", eol: CRLF, hasEOL: true},
		{name: "mixed tie is lf", in: "a\r\nb\n", text: "a\nb\n", eol: LF, hasEOL: true},
		{name: "lone cr kept", in: "a\rb\n", text: "a\rb\n", eol: LF, hasEOL: true},
		{name: "utf8", in: "héllo, 世界\n", text: "héllo, 世界\n", eol: LF, hasEOL: true},
		{name: "nul", in: "a\x00b", wantErr: ErrBinary},
		{name: "invalid utf8", in: "a\xffb", wantErr: ErrNotUTF8},
		{name: "nul after sniff window is not binary", in: strings.Repeat("a", SniffLen) + "\x00",
			text: strings.Repeat("a", SniffLen) + "\x00", eol: LF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Decode([]byte(tt.in))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if f.Text != tt.text || f.EOL != tt.eol || f.HasEOL != tt.hasEOL || !f.Exists {
				t.Fatalf("got %+v, want text=%q eol=%q hasEOL=%v", f, tt.text, tt.eol, tt.hasEOL)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"\n", []string{"\n"}},
		{"a", []string{"a"}},
		{"a\n", []string{"a\n"}},
		{"a\nb", []string{"a\n", "b"}},
		{"a\n\nb\n", []string{"a\n", "\n", "b\n"}},
	}
	for _, tt := range tests {
		got := SplitLines(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("SplitLines(%q) = %q, want %q", tt.in, got, tt.want)
		}
		if strings.Join(got, "") != tt.in {
			t.Errorf("SplitLines(%q) does not join back", tt.in)
		}
	}
}

func TestDetectEOL(t *testing.T) {
	none := File{EOL: LF}
	lf := File{EOL: LF, HasEOL: true}
	crlf := File{EOL: CRLF, HasEOL: true}
	tests := []struct {
		name  string
		files []File
		want  EOL
	}{
		{"local wins", []File{crlf, lf}, CRLF},
		{"local lf wins over crlf base", []File{lf, crlf}, LF},
		{"falls back to base", []File{none, crlf}, CRLF},
		{"default lf", []File{none, none}, LF},
		{"no files", nil, LF},
	}
	for _, tt := range tests {
		if got := DetectEOL(tt.files...); got != tt.want {
			t.Errorf("%s: got %s, want %s", tt.name, got.Name(), tt.want.Name())
		}
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	for _, in := range []string{"", "a", "a\r\nb\r\n", "a\r\nb", "x\ny\n", "x\ny"} {
		src := filepath.Join(dir, "src")
		if err := os.WriteFile(src, []byte(in), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(dir, "dst")
		if err := WriteFile(dst, f.Text, f.EOL); err != nil {
			t.Fatal(err)
		}
		out, err := os.ReadFile(dst)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != in {
			t.Errorf("round trip %q -> %q", in, out)
		}
	}
}

func TestReadOptional(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"", filepath.Join(dir, "missing")} {
		f, err := ReadOptional(p)
		if err != nil || f.Exists || f.Text != "" || f.EOL != LF {
			t.Errorf("ReadOptional(%q) = %+v, %v", p, f, err)
		}
	}
	// A required file must report missing files as errors.
	if _, err := ReadFile(filepath.Join(dir, "missing")); err == nil {
		t.Error("ReadFile on a missing file: want error")
	}
	// Binary files are errors even when optional, and name the file.
	bin := filepath.Join(dir, "bin")
	if err := os.WriteFile(bin, []byte{1, 0, 2}, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadOptional(bin)
	if !errors.Is(err, ErrBinary) || !strings.Contains(err.Error(), bin) {
		t.Errorf("ReadOptional(binary) err = %v", err)
	}
}
