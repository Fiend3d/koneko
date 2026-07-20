package main

import (
	"testing"
)

const benchFile = "testdata/sqlite3.c"

func benchModel(b *testing.B) *Model {
	b.Helper()
	fb, err := OpenFileBuffer(benchFile)
	if err != nil {
		b.Skipf("no benchmark corpus: %v", err)
	}
	b.Cleanup(func() { fb.Close() })
	return &Model{fileBuf: fb, totalLines: fb.LineCount(), tabWidth: 4}
}

func BenchmarkOpenFileBuffer(b *testing.B) {
	for b.Loop() {
		fb, err := OpenFileBuffer(benchFile)
		if err != nil {
			b.Fatal(err)
		}
		fb.Close()
	}
}

func BenchmarkPopulateMatchLines(b *testing.B) {
	m := benchModel(b)
	m.searchStr = "sqlite3_malloc"
	b.ResetTimer()
	for b.Loop() {
		m.populateMatchLines()
	}
	b.ReportMetric(float64(len(m.matchLines)), "matches")
}

func BenchmarkCopySelectionWholeFile(b *testing.B) {
	m := benchModel(b)
	m.selection.Begin(0, 0)
	m.selection.Extend(m.totalLines-1, 0)
	m.selection.End()
	b.ResetTimer()
	for b.Loop() {
		m.copySelection()
	}
}

// BenchmarkCopySelectionSmall is the common case: grabbing a few dozen lines
// out of the middle of a large file.
func BenchmarkCopySelectionSmall(b *testing.B) {
	m := benchModel(b)
	mid := m.totalLines / 2
	m.selection.Begin(mid, 0)
	m.selection.Extend(mid+40, 10)
	m.selection.End()
	b.ResetTimer()
	for b.Loop() {
		m.copySelection()
	}
}

func BenchmarkLineRandomAccess(b *testing.B) {
	m := benchModel(b)
	n := m.totalLines
	i := 0
	b.ResetTimer()
	for b.Loop() {
		m.fileBuf.Line(i % n)
		i += 7919
	}
}

func BenchmarkLineWidthSameRow(b *testing.B) {
	m := benchModel(b)
	b.ResetTimer()
	for b.Loop() {
		m.lineWidth(1000)
	}
}

func BenchmarkExpandTabsNoTabs(b *testing.B) {
	line := "  static int sqlite3BtreeCursorHasMoved(BtCursor *pCur, int flags){"
	b.ResetTimer()
	for b.Loop() {
		expandTabs(line, 4)
	}
}

func BenchmarkVisualLineWidthNoTabs(b *testing.B) {
	line := "  static int sqlite3BtreeCursorHasMoved(BtCursor *pCur, int flags){"
	b.ResetTimer()
	for b.Loop() {
		visualLineWidth(line, 4)
	}
}
