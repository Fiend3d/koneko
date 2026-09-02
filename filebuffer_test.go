package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// referenceLines is the obvious, slow way to split a file into lines, used to
// check the offset table the buffer builds while indexing.
func referenceLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	var out []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, strings.TrimRight(s, "\r\n"))
			break
		}
		out = append(out, strings.TrimRight(s[:i+1], "\r\n"))
		s = s[i+1:]
	}
	return out
}

func testdataFiles(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("testdata/*")
	if err != nil || len(paths) == 0 {
		t.Skip("no testdata")
	}
	return paths
}

func TestFileBufferMatchesReference(t *testing.T) {
	for _, path := range testdataFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			want := referenceLines(t, path)
			fb, err := OpenFileBuffer(path)
			if err != nil {
				t.Fatal(err)
			}
			defer fb.Close()

			if got := fb.LineCount(); got != len(want) {
				t.Fatalf("LineCount = %d, want %d", got, len(want))
			}
			for i := range want {
				if got := fb.Line(i); got != want[i] {
					t.Fatalf("Line(%d) = %q, want %q", i, got, want[i])
				}
			}
		})
	}
}

func TestLineOutOfRangeIsEmpty(t *testing.T) {
	fb, err := OpenFileBuffer("testdata/small.go")
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()
	for _, n := range []int{-1, fb.LineCount(), fb.LineCount() + 10} {
		if got := fb.Line(n); got != "" {
			t.Errorf("Line(%d) = %q, want empty", n, got)
		}
	}
}

// TestTextKeepsLinesAlignedWithLine is what lets the highlighter record runs
// against raw line bytes: a byte offset into Text must name the same character
// as the same offset into the corresponding Line.
func TestTextKeepsLinesAlignedWithLine(t *testing.T) {
	for _, path := range testdataFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			fb, err := OpenFileBuffer(path)
			if err != nil {
				t.Fatal(err)
			}
			defer fb.Close()
			if fb.LineCount() == 0 {
				t.Skip("empty file")
			}
			to := min(fb.LineCount(), 200)
			got := strings.Split(fb.Text(0, to), "\n")
			for i := 0; i < to; i++ {
				if i >= len(got) {
					t.Fatalf("Text produced %d lines, want at least %d", len(got), to)
				}
				if got[i] != fb.Line(i) {
					t.Fatalf("line %d: Text has %q, Line has %q", i, got[i], fb.Line(i))
				}
			}
		})
	}
}

func TestAnEmptyFileHasNoLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	fb, err := OpenFileBuffer(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()
	if got := fb.LineCount(); got != 0 {
		t.Errorf("LineCount = %d, want 0", got)
	}
	if got := fb.Text(0, 1); got != "" {
		t.Errorf("Text = %q, want empty", got)
	}
}

func TestAFileWithNoTrailingNewlineKeepsItsLastLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nolf.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc"), 0o644); err != nil {
		t.Fatal(err)
	}
	fb, err := OpenFileBuffer(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()
	if got := fb.LineCount(); got != 3 {
		t.Fatalf("LineCount = %d, want 3", got)
	}
	if got := fb.Line(2); got != "c" {
		t.Errorf("Line(2) = %q, want %q", got, "c")
	}
}

func TestCRLFEndingsAreStripped(t *testing.T) {
	fb, err := OpenFileBuffer("testdata/crlf.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()
	for i := 0; i < fb.LineCount(); i++ {
		if strings.ContainsAny(fb.Line(i), "\r\n") {
			t.Fatalf("Line(%d) = %q still has a terminator", i, fb.Line(i))
		}
	}
}
