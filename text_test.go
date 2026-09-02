package main

import (
	"os"
	"strings"
	"testing"

	"github.com/Fiend3d/catatui"
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
	// How many columns it takes is the terminal's business, not ours; what
	// matters here is that the cluster is not broken up. See
	// TestClusterWidthsAreTheRenderersWidths.
	if want := Col(catatui.StringWidth("👨‍👩‍👧‍👦")); tab.Width() != want {
		t.Fatalf("width %d, want %d", tab.Width(), want)
	}
}

// TestDevanagariConjunctsKeepTheirMarks pins down the shape that breaks naive
// renderers: six code points that must never be walked per rune or per byte.
//
// uniseg segments हिन्दी as three clusters — हि, न्, दी — where Unicode 15.1's
// rule GB9c gives two, binding न्दी into one unit by the virama. uniseg v0.4.7,
// the newest release, does not implement that rule, so the table applies it
// itself; see conjunct.go for why a half-selected conjunct is a visible bug.
// What this test cares about either way is that no mark is ever separated from
// its base and the columns stay contiguous.
func TestDevanagariConjunctsKeepTheirMarks(t *testing.T) {
	line := "हिन्दी"
	tab := tableFor(line, 4)
	if want := Col(catatui.StringWidth(line)); tab.Width() != want {
		t.Fatalf("width %d, want %d", tab.Width(), want)
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

// TestClusterWidthsAreTheRenderersWidths is the rule the Indic scripts turn on.
//
// A cluster is as wide as the terminal makes it, which is not the same as the
// one glyph it is drawn as: a spacing combining mark takes a column of its own,
// so हि covers two however tightly the pair is drawn. Measuring it as one put
// the next cluster on top of its second half and the terminal dropped the pair,
// which is how हिन्दी came out as न्दी. Measuring it long is no better — see
// catatui's clusterWidth for the other half of that story.
//
// The number belongs to catatui, which is what turns clusters into cells, so
// this pins the agreement rather than the number: every cluster in the table
// has to measure exactly what the renderer will measure, or the two disagree
// about where the next glyph goes.
func TestClusterWidthsAreTheRenderersWidths(t *testing.T) {
	for _, line := range []string{
		"हिन्दी परीक्षण पाठ।", // Devanagari
		"বাংলা পরীক্ষা লেখা।", // Bengali
		"தமிழ் சோதனை உரை.", // Tamil
		"తెలుగు పరీక్ష వచనం.", // Telugu
		"ภาษาไทยทดสอบ", // Thai
		"العربية اختبار", // Arabic
		"日本語 한글 ｱｲｳ ﾊﾞ",
		"👨‍👩‍👧‍👦 🏳️‍🌈 á",
	} {
		tab := tableFor(line, 4)
		if got, want := tab.Width(), Col(catatui.StringWidth(line)); got != want {
			t.Errorf("%q: table width %d, renderer width %d", line, got, want)
		}
		var col Col
		for i, c := range tab.Clusters() {
			text := tab.ClusterStr(line, i)
			if Col(c.Col) != col {
				t.Fatalf("%q: cluster %q starts at column %d, want %d", line, text, c.Col, col)
			}
			if got, want := Col(c.Width), Col(catatui.StringWidth(text)); got != want {
				t.Errorf("%q: cluster %q is %d columns here, %d to the renderer",
					line, text, got, want)
			}
			col = c.EndCol()
		}
	}
}

// TestIndicClustersCoverEveryColumnTheTerminalAdvances is the same rule stated
// as the bug it prevents: a cluster the terminal walks two columns for has to
// claim both, or whatever is drawn next lands inside it.
func TestIndicClustersCoverEveryColumnTheTerminalAdvances(t *testing.T) {
	for _, text := range []string{
		"हि", // Devanagari, spacing vowel sign I
		"पा", // Devanagari, spacing vowel sign AA
		"न्", // Devanagari, virama
		"মি", // Bengali
		"সো", // Bengali
		"மி", // Tamil
	} {
		tab := tableFor(text, 4)
		if len(tab.Clusters()) != 1 {
			t.Fatalf("%q split into %d clusters, want 1", text, len(tab.Clusters()))
		}
		if got, want := tab.Width(), Col(catatui.StringWidth(text)); got != want {
			t.Errorf("%q width = %d, want %d", text, got, want)
		}
	}
}

func TestCombiningMarksAttachToTheirBase(t *testing.T) {
	const decomposed = "a\u0301"
	tab := tableFor(decomposed, 4)
	if len(tab.Clusters()) != 1 {
		t.Fatalf("%d clusters, want 1", len(tab.Clusters()))
	}
	if want := Col(catatui.StringWidth(decomposed)); tab.Width() != want {
		t.Fatalf("width %d, want %d", tab.Width(), want)
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

// TestSnapColLandsOnAClusterBoundaryInIndicText is the other side of the width
// policy: a Tamil or Devanagari cluster covers more than one column, so a
// selection edge really can land inside a glyph, and snapping is what keeps it
// out. Every column of the line has to round to a boundary.
func TestSnapColLandsOnAClusterBoundaryInIndicText(t *testing.T) {
	for _, line := range []string{"தமிழ் சோதனை", "हिन्दी परीक्षण"} {
		tab := tableFor(line, 4)
		boundary := map[Col]bool{tab.Width(): true}
		for _, c := range tab.Clusters() {
			boundary[Col(c.Col)] = true
		}
		for col := Col(0); col <= tab.Width(); col++ {
			if got := tab.SnapCol(col); !boundary[got] {
				t.Errorf("%q: SnapCol(%d) = %d, which is inside a cluster", line, col, got)
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
