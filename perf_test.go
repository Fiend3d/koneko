// Hot-path benchmarks, matching nezumi's benches/hot.rs one for one so the two
// implementations can be compared directly.
//
// The corpus is testdata/sqlite3.c (9.8 MB, ~270k lines). Every benchmark skips
// when it is absent.
package main

import (
	"os"
	"testing"

	"github.com/Fiend3d/catatui"
)

const benchFile = "testdata/sqlite3.c"

func benchCorpus(b *testing.B) *FileBuffer {
	b.Helper()
	if _, err := os.Stat(benchFile); err != nil {
		b.Skipf("no benchmark corpus: %v", err)
	}
	fb, err := OpenFileBuffer(benchFile)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { fb.Close() })
	return fb
}

func benchApp(b *testing.B, w, h uint16) *App {
	b.Helper()
	setTheme("dracula")
	fb := benchCorpus(b)
	a := NewApp(fb, Options{TabWidth: 4, LineNumbers: true, Scrollbar: true})
	a.Apply(Action{Kind: ActResize, X: w, Y: h})
	return a
}

// BenchmarkOpenAndIndex builds the line index: one memchr pass over the mapping.
func BenchmarkOpenAndIndex(b *testing.B) {
	if _, err := os.Stat(benchFile); err != nil {
		b.Skipf("no benchmark corpus: %v", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		fb, err := OpenFileBuffer(benchFile)
		if err != nil {
			b.Fatal(err)
		}
		_ = fb.LineCount()
		fb.Close()
	}
}

// BenchmarkLineRandomAccess is a slice of the mapping, not a syscall.
func BenchmarkLineRandomAccess(b *testing.B) {
	fb := benchCorpus(b)
	n := fb.LineCount()
	i := 0
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		i = (i + 7919) % n
		_ = len(fb.Line(i))
	}
}

// BenchmarkClusterTable lays out one line's clusters — done once per visible
// row per frame.
func BenchmarkClusterTable(b *testing.B) {
	cases := []struct{ name, line string }{
		{"ascii", "  static int sqlite3BtreeCursorHasMoved(BtCursor *pCur, int flags){"},
		{"unicode", "東京 (Tokyo) 到 北京 (Beijing) 的 距離 is about 2,100 km."},
		{"tabs", "\tif (x) {\n\t\treturn\t1;\t}"},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			var t ClusterTable
			b.ReportAllocs()
			for b.Loop() {
				t.Rebuild(c.line, 4)
				_ = t.Width()
			}
		})
	}
}

// BenchmarkSearch is a whole-file case-insensitive scan.
func BenchmarkSearch(b *testing.B) {
	fb := benchCorpus(b)
	b.ReportAllocs()
	b.ResetTimer()
	var n int
	for b.Loop() {
		n = len(scanMatches(fb, "sqlite3_malloc", 4))
	}
	b.ReportMetric(float64(n), "matches")
}

// BenchmarkCopySelection reconstructs selected text: the common case of a few
// dozen lines, and the whole file.
func BenchmarkCopySelection(b *testing.B) {
	b.Run("forty_lines", func(b *testing.B) {
		a := benchApp(b, 120, 50)
		mid := a.TotalLines / 2
		a.Sel.Begin(Pos{mid, 0})
		a.Sel.Extend(Pos{mid + 40, 10})
		a.Sel.Finish()
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = len(a.SelectedText())
		}
	})

	b.Run("whole_file", func(b *testing.B) {
		a := benchApp(b, 120, 50)
		a.Apply(Action{Kind: ActSelectAll})
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = len(a.SelectedText())
		}
	})
}

// BenchmarkRenderFrame draws a complete frame: the number that answers "is it
// fast".
func BenchmarkRenderFrame(b *testing.B) {
	for _, size := range [][2]uint16{{80, 24}, {200, 50}} {
		w, h := size[0], size[1]
		b.Run(sizeName(w, h), func(b *testing.B) {
			a := benchApp(b, w, h)
			a.Apply(Action{Kind: ActScrollLines, N: 100000})

			backend := catatui.NewTestBackend(w, h)
			terminal, err := catatui.NewTerminal(backend)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				// Scroll one line each frame so nothing is trivially cached.
				a.Apply(Action{Kind: ActScrollLines, N: 1})
				if err := terminal.Draw(func(f *catatui.Frame) { draw(f, a) }); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkRenderFrameUnicode is the same frame over the torture corpus, where
// every row needs real grapheme segmentation rather than the ASCII path.
func BenchmarkRenderFrameUnicode(b *testing.B) {
	setTheme("dracula")
	fb, err := OpenFileBuffer("testdata/unicode.txt")
	if err != nil {
		b.Skipf("no unicode corpus: %v", err)
	}
	defer fb.Close()

	a := NewApp(fb, Options{TabWidth: 4, LineNumbers: true, Scrollbar: true})
	a.Apply(Action{Kind: ActResize, X: 80, Y: 24})

	backend := catatui.NewTestBackend(80, 24)
	terminal, err := catatui.NewTerminal(backend)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		a.Apply(Action{Kind: ActScrollLines, N: 1})
		if err := terminal.Draw(func(f *catatui.Frame) { draw(f, a) }); err != nil {
			b.Fatal(err)
		}
	}
}

func sizeName(w, h uint16) string {
	return itoa(int(w)) + "x" + itoa(int(h))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
