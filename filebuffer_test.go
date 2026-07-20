package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// expandTabsSlow and visualLineWidthSlow are the original grapheme-walking
// implementations, kept here to check the fast paths against.
func expandTabsSlow(s string, tabWidth int) string {
	var b strings.Builder
	col := 0
	i := 0
	for i < len(s) {
		if s[i] == '\t' {
			n := tabWidth - (col % tabWidth)
			b.WriteString(strings.Repeat(" ", n))
			col += n
			i++
			continue
		}
		cluster, w := ansi.FirstGraphemeCluster(s[i:], ansi.GraphemeWidth)
		if len(cluster) == 0 {
			break
		}
		b.WriteString(cluster)
		col += w
		i += len(cluster)
	}
	return b.String()
}

func visualLineWidthSlow(line string, tabWidth int) int {
	col := 0
	rest := line
	for len(rest) > 0 {
		cluster, w := ansi.FirstGraphemeCluster(rest, ansi.GraphemeWidth)
		if len(cluster) == 0 {
			break
		}
		if cluster == "\t" {
			col += tabWidth - (col % tabWidth)
		} else {
			col += w
		}
		rest = rest[len(cluster):]
	}
	return col
}

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
				got, err := fb.Line(i)
				if err != nil {
					t.Fatalf("Line(%d): %v", i, err)
				}
				if got != want[i] {
					t.Fatalf("Line(%d) = %q, want %q", i, got, want[i])
				}
			}
		})
	}
}

// TestLinesRangeMatchesLine covers the range reads that copying a selection now
// relies on instead of reading the whole file.
func TestLinesRangeMatchesLine(t *testing.T) {
	for _, path := range testdataFiles(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			fb, err := OpenFileBuffer(path)
			if err != nil {
				t.Fatal(err)
			}
			defer fb.Close()
			n := fb.LineCount()
			if n == 0 {
				return
			}
			ranges := [][2]int{
				{0, 1}, {0, n}, {n - 1, n},
			}
			if n > 10 {
				ranges = append(ranges, [2]int{3, 9}, [2]int{n / 2, n/2 + 5}, [2]int{n - 4, n})
			}
			for _, r := range ranges {
				lines, err := fb.Lines(r[0], r[1])
				if err != nil {
					t.Fatalf("Lines%v: %v", r, err)
				}
				if len(lines) != r[1]-r[0] {
					t.Fatalf("Lines%v returned %d lines, want %d", r, len(lines), r[1]-r[0])
				}
				for i, got := range lines {
					want, err := fb.Line(r[0] + i)
					if err != nil {
						t.Fatal(err)
					}
					if got != want {
						t.Fatalf("Lines%v[%d] = %q, want %q", r, i, got, want)
					}
				}
			}
		})
	}
}

func TestIndexFoldMatchesToLower(t *testing.T) {
	cases := []struct{ hay, needle string }{
		{"Hello World", "world"},
		{"HELLO", "hello"},
		{"hello", "hello"},
		{"aaaa", "aa"},
		{"abc", "abcd"},
		{"", "x"},
		{"sqlite3_MALLOC(n)", "sqlite3_malloc"},
		{"no match here", "zzz"},
		{"MiXeD CaSe StRiNg", "case"},
		{"tail", "l"},
	}
	for _, c := range cases {
		want := strings.Index(strings.ToLower(c.hay), c.needle)
		if got := indexFold(c.hay, c.needle); got != want {
			t.Errorf("indexFold(%q, %q) = %d, want %d", c.hay, c.needle, got, want)
		}
	}
}

func TestExpandTabsFastPath(t *testing.T) {
	cases := []string{"", "no tabs here", "\tleading", "a\tb\tc", "unicode → ok", "мир", "tab\tand →"}
	for _, s := range cases {
		if got, want := expandTabs(s, 4), expandTabsSlow(s, 4); got != want {
			t.Errorf("expandTabs(%q) = %q, want %q", s, got, want)
		}
	}
}

func TestAsciiWidthMatchesFullPath(t *testing.T) {
	cases := []string{"", "plain ascii", "a\tb", "\t\t", "wide→arrow", "日本語", "é"}
	for _, s := range cases {
		got := visualLineWidth(s, 4)
		want := visualLineWidthSlow(s, 4)
		if got != want {
			t.Errorf("visualLineWidth(%q) = %d, want %d", s, got, want)
		}
	}
}
