package main

import (
	"errors"
	"io"
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

// TestOpenDoesNotLockTheFile is the reason koneko reads the file instead of
// mapping it: on Windows a mapped section makes every other writer fail —
// os.WriteFile with "a user-mapped section open", rename and delete with "the
// file is being used by another process". A viewer must not hold a file its
// user is editing somewhere else.
func TestOpenDoesNotLockTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "locked.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fb, err := OpenFileBuffer(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()

	if err := os.WriteFile(path, []byte("rewritten\nby an editor\n"), 0o644); err != nil {
		t.Fatalf("overwriting an open file failed: %v", err)
	}
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatalf("renaming an open file failed: %v", err)
	}
	if err := os.Remove(path + ".moved"); err != nil {
		t.Fatalf("deleting an open file failed: %v", err)
	}
}

// TestExternalChangesDoNotCorruptTheView is the other half: a write that does
// get through must not be seen through a line table built from the old byte
// layout. The snapshot keeps showing the file as it was when it opened.
func TestExternalChangesDoNotCorruptTheView(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "changed.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fb, err := OpenFileBuffer(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()

	// Longer, shorter, and a different number of lines: every case that moves
	// the offsets a mapped view would still be reading through.
	for _, replacement := range []string{
		"a much longer line than before, which shifts every offset after it\n",
		"x\n",
		"1\n2\n3\n4\n5\n6\n7\n8\n",
		"",
	} {
		if err := os.WriteFile(path, []byte(replacement), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := fb.LineCount(); got != 3 {
			t.Fatalf("LineCount = %d after an external write, want 3", got)
		}
		for i, want := range []string{"one", "two", "three"} {
			if got := fb.Line(i); got != want {
				t.Fatalf("Line(%d) = %q, want %q", i, got, want)
			}
		}
		if got := fb.Text(0, 3); got != "one\ntwo\nthree\n" {
			t.Errorf("Text = %q, want the original file", got)
		}
	}
}

// TestOpenForReadDoesNotBlockRenameOrDelete pins the share mode the file is
// opened with. os.Open asks for FILE_SHARE_READ | FILE_SHARE_WRITE and never
// FILE_SHARE_DELETE, and on Windows that is enough to make a rename-over save
// fail with "the process cannot access the file because it is being used by
// another process" for as long as the handle lives.
func TestOpenForReadDoesNotBlockRenameOrDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := openForRead(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Hold it the way a reload does: stat, read it all, still open afterwards.
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(f, make([]byte, info.Size())); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("rewritten while open\n"), 0o644); err != nil {
		t.Fatalf("overwriting the open file failed: %v", err)
	}
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatalf("renaming the open file failed: %v", err)
	}
	if err := os.Remove(path + ".moved"); err != nil {
		t.Fatalf("deleting the renamed file failed: %v", err)
	}
}

func TestMissingFileReportsErrNotExist(t *testing.T) {
	_, err := OpenFileBuffer(filepath.Join(t.TempDir(), "absent.txt"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("opening a missing file gave %v, want an os.ErrNotExist", err)
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
