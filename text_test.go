package main

import (
	"os"
	"strings"
	"testing"
)

func tableFor(line string, tabWidth int) *ClusterTable {
	var t ClusterTable
	t.Rebuild(line, tabWidth)
	return &t
}

func TestASCIIWidthsMatchByteCounts(t *testing.T) {
	tab := tableFor("hello world", 4)
	if tab.Width() != 11 || len(tab.Clusters()) != 11 {
		t.Fatalf("width %d, %d clusters", tab.Width(), len(tab.Clusters()))
	}
}

func TestCJKIsTwoCellsPerCluster(t *testing.T) {
	tab := tableFor("日本語", 4)
	if len(tab.Clusters()) != 3 || tab.Width() != 6 {
		t.Fatalf("width %d, %d clusters", tab.Width(), len(tab.Clusters()))
	}
	for _, c := range tab.Clusters() {
		if c.Width != 2 {
			t.Fatalf("cluster width %d, want 2", c.Width)
		}
	}
}

func TestHangulIsTwoCells(t *testing.T) {
	tab := tableFor("한글", 4)
	if len(tab.Clusters()) != 2 || tab.Width() != 4 {
		t.Fatalf("width %d, %d clusters", tab.Width(), len(tab.Clusters()))
	}
}

func TestHalfwidthKanaIsOneCellFullwidthIsTwo(t *testing.T) {
	if got := DisplayWidth("ｱｲｳｴｵ", 4); got != 5 {
		t.Errorf("halfwidth = %d, want 5", got)
	}
	if got := DisplayWidth("アイウエオ", 4); got != 10 {
		t.Errorf("fullwidth = %d, want 10", got)
	}
}

func TestZWJFamilyIsASingleCluster(t *testing.T) {
	// A four-person family joined by zero-width joiners must not fracture.
	tab := tableFor("👨‍👩‍👧‍👦", 4)
	if len(tab.Clusters()) != 1 {
		t.Fatalf("%d clusters, want 1", len(tab.Clusters()))
	}
	if tab.Width() != 2 {
		t.Fatalf("width %d, want 2", tab.Width())
	}
}

// TestDevanagariConjunctsKeepTheirMarks pins down the shape that breaks naive
// renderers: six code points that must never be walked per rune or per byte.
//
// uniseg segments हिन्दी as three clusters — हि, न्, दी — where Rust's
// unicode-segmentation gives two, binding न्दी into one unit by the virama.
// That is Unicode 15.1's rule GB9c, which uniseg v0.4.7 (the newest release)
// does not yet implement. It costs nothing here: catatui measures with the same
// uniseg, so our columns and the renderer's agree either way, and what the
// render path needs is that every mark stays attached to its base and the
// columns are contiguous. Both hold.
func TestDevanagariConjunctsKeepTheirMarks(t *testing.T) {
	line := "हिन्दी"
	tab := tableFor(line, 4)
	if tab.Width() != 5 {
		t.Fatalf("width %d, want 5", tab.Width())
	}
	// No cluster may begin with a combining mark: a mark that started its own
	// cluster would be one the base character had lost.
	for i := range tab.Clusters() {
		for _, r := range tab.ClusterStr(line, i) {
			if r == 0x093F || r == 0x0940 || r == 0x094D {
				t.Errorf("cluster %d begins with a combining mark %U", i, r)
			}
			break
		}
	}
	// Columns are contiguous and end at the total width.
	var expected Col
	for _, c := range tab.Clusters() {
		if Col(c.Col) != expected {
			t.Fatalf("gap at column %d", c.Col)
		}
		expected = c.EndCol()
	}
	if expected != tab.Width() {
		t.Fatalf("columns end at %d, width is %d", expected, tab.Width())
	}
}

func TestCombiningMarksAttachToTheirBase(t *testing.T) {
	tab := tableFor("á", 4)
	if len(tab.Clusters()) != 1 || tab.Width() != 1 {
		t.Fatalf("width %d, %d clusters", tab.Width(), len(tab.Clusters()))
	}
}

func TestTabsExpandToTheNextStop(t *testing.T) {
	tab := tableFor("\tx", 4)
	if tab.Clusters()[0].Width != 4 || tab.Clusters()[1].Col != 4 {
		t.Fatalf("got width %d at col %d", tab.Clusters()[0].Width, tab.Clusters()[1].Col)
	}
	tab = tableFor("ab\tx", 4)
	if tab.Clusters()[2].Width != 2 {
		t.Fatalf("tab from col 2 filled %d, want 2", tab.Clusters()[2].Width)
	}
	if tab.Clusters()[3].Col != 4 {
		t.Fatalf("next cluster at col %d, want 4", tab.Clusters()[3].Col)
	}
}

func TestTabStopsAccountForPrecedingWideCharacters(t *testing.T) {
	// 日 occupies columns 0-1, so the tab must fill only columns 2-3.
	tab := tableFor("日\tx", 4)
	if tab.Clusters()[1].Width != 2 || tab.Clusters()[2].Col != 4 {
		t.Fatalf("tab width %d, next col %d", tab.Clusters()[1].Width, tab.Clusters()[2].Col)
	}
}

func TestColAndByteRoundTripOnClusterBoundaries(t *testing.T) {
	line := "日本語 abc 👨‍💻 हिन्दी"
	tab := tableFor(line, 4)
	for _, c := range tab.Clusters() {
		if got := tab.ColToByte(Col(c.Col)); got != int(c.Byte) {
			t.Fatalf("ColToByte(%d) = %d, want %d", c.Col, got, c.Byte)
		}
		if got := tab.ByteToCol(int(c.Byte)); got != Col(c.Col) {
			t.Fatalf("ByteToCol(%d) = %d, want %d", c.Byte, got, c.Col)
		}
	}
}

func TestColToByteRoundsDownInsideAWideCluster(t *testing.T) {
	tab := tableFor("日本", 4)
	// Column 1 is the right half of 日; it must resolve to 日's start.
	if got := tab.ColToByte(1); got != 0 {
		t.Errorf("ColToByte(1) = %d, want 0", got)
	}
	if got := tab.ColToByte(2); got != 3 {
		t.Errorf("ColToByte(2) = %d, want 3", got)
	}
}

func TestColToBytePastTheEndYieldsTheLineLength(t *testing.T) {
	line := "日本"
	if got := tableFor(line, 4).ColToByte(99); got != len(line) {
		t.Errorf("ColToByte(99) = %d, want %d", got, len(line))
	}
}

func TestWordBoundsDoNotSplitADevanagariCluster(t *testing.T) {
	line := "हिन्दी परीक्षण"
	tab := tableFor(line, 4)
	start, end := FindWordBounds(line, tab, 0)
	if start != 0 {
		t.Errorf("start = %d, want 0", start)
	}
	// The boundary must land on a real cluster start, never mid-cluster.
	found := false
	for _, c := range tab.Clusters() {
		if c.EndCol() == end {
			found = true
		}
	}
	if !found {
		t.Errorf("word end %d is not a cluster boundary", end)
	}
}

func TestWordBoundsTreatAZWJEmojiAsOneUnit(t *testing.T) {
	line := "ab 👨‍💻 cd"
	start, end := FindWordBounds(line, tableFor(line, 4), 0)
	if start != 0 || end != 2 {
		t.Errorf("got (%d, %d), want (0, 2) selecting only `ab`", start, end)
	}
}

func TestWordBoundsAreEmptyOnWhitespace(t *testing.T) {
	line := "ab cd"
	if start, end := FindWordBounds(line, tableFor(line, 4), 2); start != end {
		t.Errorf("got (%d, %d), want an empty range", start, end)
	}
}

func TestTruncateNeverSplitsAWideCluster(t *testing.T) {
	// Column 1 would bisect 日, so nothing fits.
	for _, tc := range []struct {
		max  Col
		want string
	}{{1, ""}, {2, "日"}, {3, "日"}, {4, "日本"}} {
		if got := TruncateToWidth("日本語", tc.max); got != tc.want {
			t.Errorf("TruncateToWidth(_, %d) = %q, want %q", tc.max, got, tc.want)
		}
	}
}

func TestASCIIFastPathAgreesWithTheGraphemeWalk(t *testing.T) {
	for _, line := range []string{"", "abc", "a\tb", "\t\t", "  spaces  "} {
		if got, want := DisplayWidth(line, 4), tableFor(line, 4).Width(); got != want {
			t.Errorf("%q: DisplayWidth = %d, table width = %d", line, got, want)
		}
	}
}

// TestEveryLineOfTheUnicodeCorpusIsSelfConsistent checks the two independent
// width implementations against each other, and checks that cluster columns are
// contiguous with no gaps — the property the whole render path rests on.
func TestEveryLineOfTheUnicodeCorpusIsSelfConsistent(t *testing.T) {
	for _, line := range corpusLines(t) {
		tab := tableFor(line, 4)
		if got, want := DisplayWidth(line, 4), tab.Width(); got != want {
			t.Fatalf("width mismatch on %q: %d vs %d", line, got, want)
		}
		var expected Col
		for _, c := range tab.Clusters() {
			if Col(c.Col) != expected {
				t.Fatalf("gap in columns on %q at %d", line, c.Col)
			}
			expected = c.EndCol()
		}
		if expected != tab.Width() {
			t.Fatalf("columns end at %d but width is %d on %q", expected, tab.Width(), line)
		}
	}
}

func corpusLines(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("testdata/unicode.txt")
	if err != nil {
		t.Skipf("no unicode corpus: %v", err)
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
}
