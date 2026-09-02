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
//
// Each of the three is one column, not the two uniseg scores हि and दी: a
// cluster is drawn as one glyph and takes its base character's advance. See
// TestIndicSpacingMarksDoNotClaimAColumn.
func TestDevanagariConjunctsKeepTheirMarks(t *testing.T) {
	line := "हिन्दी"
	tab := tableFor(line, 4)
	if tab.Width() != 3 {
		t.Fatalf("width %d, want 3", tab.Width())
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

// TestIndicSpacingMarksDoNotClaimAColumn is the rule that keeps Indic text from
// being drawn full of holes.
//
// A spacing combining mark is scored one column by Unicode, so uniseg measures
// a consonant carrying one as two. A terminal draws the pair as a single glyph
// in a single cell, and the column left over shows up as a gap beside every
// such consonant — and, since nothing is drawn there, as a hole in any
// selection painted across the line. catatui caps a cluster at its widest rune,
// which is what this measures with.
func TestIndicSpacingMarksDoNotClaimAColumn(t *testing.T) {
	for _, c := range []struct {
		text string
		want Col
	}{
		{"हि", 1}, // Devanagari, spacing vowel sign I
		{"पा", 1}, // Devanagari, spacing vowel sign AA
		{"न्", 1}, // Devanagari, virama: already zero-width to uniseg
		{"मि", 1}, // Tamil
		{"সো", 1}, // Bengali
		{"हिन्दी", 3},
		{"தமிழ்", 3},
	} {
		if got := tableFor(c.text, 4).Width(); got != c.want {
			t.Errorf("%q width = %d, want %d", c.text, got, c.want)
		}
	}

	// The cap must not touch anything whose width really is its own.
	for _, c := range []struct {
		text string
		want Col
	}{
		{"日本語", 6},
		{"한글", 4},
		{"ｱｲｳ", 3},
		{"アイウ", 6},
		{"ﾊﾞ", 2}, // halfwidth katakana plus a sound mark, drawn in two columns
		{"👨‍👩‍👧‍👦", 2},
		{"🏳️‍🌈", 2},
		{"á", 1},
	} {
		if got := tableFor(c.text, 4).Width(); got != c.want {
			t.Errorf("%q width = %d, want %d", c.text, got, c.want)
		}
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

// TestSnapColRoundsToTheNearestClusterBoundary covers the conversion mouse
// selection depends on. In "a日b" the middle cluster is two columns wide, so
// columns 1 and 2 both land inside one glyph and only one of them is a place a
// selection edge may sit.
func TestSnapColRoundsToTheNearestClusterBoundary(t *testing.T) {
	const line = "a日b"
	tab := tableFor(line, 4)
	if tab.Width() != 4 || len(tab.Clusters()) != 3 {
		t.Fatalf("width %d over %d clusters, want 4 over 3", tab.Width(), len(tab.Clusters()))
	}

	for _, c := range []struct{ col, want Col }{
		{0, 0}, // on a boundary already
		{1, 1}, // start of 日
		{2, 3}, // inside 日, past its midpoint, so out to its end
		{3, 3}, // start of b
	} {
		if got := tab.SnapCol(c.col); got != c.want {
			t.Errorf("SnapCol(%d) = %d, want %d", c.col, got, c.want)
		}
	}

	// Past the end the column is left alone: the cell one past the text stands
	// for the newline a selection may include.
	for _, col := range []Col{4, 5, 20} {
		if got := tab.SnapCol(col); got != col {
			t.Errorf("SnapCol(%d) = %d, want it unchanged", col, got)
		}
	}
}

// TestSnapColIsTheIdentityOnIndicText is the other side of the width policy:
// once a consonant and its spacing vowel sign share one column there is nothing
// left inside a Tamil or Devanagari glyph for a selection edge to land in.
func TestSnapColIsTheIdentityOnIndicText(t *testing.T) {
	for _, line := range []string{"தமிழ் சோதனை", "हिन्दी परीक्षण"} {
		tab := tableFor(line, 4)
		for col := Col(0); col <= tab.Width(); col++ {
			if got := tab.SnapCol(col); got != col {
				t.Errorf("%q: SnapCol(%d) = %d, want %d", line, col, got, col)
			}
		}
	}
}

// TestSnapColIsTheIdentityOnASCII keeps the fast path honest: every cluster is
// one column, so every column is already a boundary.
func TestSnapColIsTheIdentityOnASCII(t *testing.T) {
	tab := tableFor("hello world", 4)
	for col := Col(0); col <= tab.Width(); col++ {
		if got := tab.SnapCol(col); got != col {
			t.Errorf("SnapCol(%d) = %d, want %d", col, got, col)
		}
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
