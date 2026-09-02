package main

import (
	"strings"
	"testing"

	"github.com/Fiend3d/catatui"
)

func testTheme() *Theme { return NewTheme(palettes["dracula"]) }

type piece struct {
	text  string
	style catatui.Style
}

// pieces collects a row's emitted spans, for assertions.
func pieces(line string, xOff Col, width uint16) []piece {
	return piecesSel(line, xOff, width, 0, 0, false)
}

func piecesSel(line string, xOff Col, width uint16, from, to Col, ok bool) []piece {
	var out []piece
	emitRow(&Row{
		Line:    line,
		Table:   tableFor(line, 4),
		SelFrom: from,
		SelTo:   to,
		SelOK:   ok,
		XOff:    xOff,
		Width:   width,
	}, testTheme(), func(s string, st catatui.Style) {
		out = append(out, piece{s, st})
	})
	return out
}

func textOf(ps []piece) string {
	var b strings.Builder
	for _, p := range ps {
		b.WriteString(p.text)
	}
	return b.String()
}

func renderedWidth(ps []piece) Col {
	var w Col
	for _, p := range ps {
		w += DisplayWidth(p.text, 4)
	}
	return w
}

func TestAShortASCIILineIsPaddedToWidth(t *testing.T) {
	if got := textOf(pieces("ab", 0, 6)); got != "ab    " {
		t.Errorf("got %q", got)
	}
}

func TestALongLineIsCutAtWidth(t *testing.T) {
	if got := textOf(pieces("abcdefghij", 0, 4)); got != "abcd" {
		t.Errorf("got %q", got)
	}
}

func TestHorizontalScrollSkipsLeadingColumns(t *testing.T) {
	if got := textOf(pieces("abcdefghij", 3, 4)); got != "defg" {
		t.Errorf("got %q", got)
	}
}

func TestScrollingIntoAWideCharPadsItsVisibleHalf(t *testing.T) {
	// 日本語: 日 at cols 0-1, 本 at 2-3, 語 at 4-5. Starting at column 1 means
	// the right half of 日 is visible; it must become one padding cell, not a
	// duplicated glyph.
	if got := textOf(pieces("日本語", 1, 4)); got != " 本 " {
		t.Errorf("got %q, want %q", got, " 本 ")
	}
}

func TestAWideCharIsNeverSplitAcrossTheRightEdge(t *testing.T) {
	// Width 3 cannot fit 日本 (4 cells); the second glyph must become a pad.
	if got := textOf(pieces("日本", 0, 3)); got != "日 " {
		t.Errorf("got %q, want %q", got, "日 ")
	}
}

func TestTabsRenderAsSpacesToTheNextStop(t *testing.T) {
	if got := textOf(pieces("a\tb", 0, 8)); got != "a   b   " {
		t.Errorf("got %q", got)
	}
}

func TestATabAfterAWideCharStopsCorrectly(t *testing.T) {
	// 日 fills cols 0-1, so the tab covers cols 2-3 only.
	if got := textOf(pieces("日\tx", 0, 6)); got != "日  x " {
		t.Errorf("got %q, want %q", got, "日  x ")
	}
}

func TestScrollingIntoTheMiddleOfATabPadsTheRemainder(t *testing.T) {
	if got := textOf(pieces("\tx", 1, 4)); got != "   x" {
		t.Errorf("got %q", got)
	}
}

func TestAnEmptyLineIsAllPadding(t *testing.T) {
	if got := textOf(pieces("", 0, 5)); got != "     " {
		t.Errorf("got %q", got)
	}
}

func TestScrollingPastTheEndOfALineIsAllPadding(t *testing.T) {
	if got := textOf(pieces("abc", 99, 5)); got != "     " {
		t.Errorf("got %q", got)
	}
}

func TestZeroWidthOutputEmitsNothing(t *testing.T) {
	if ps := pieces("abc", 0, 0); len(ps) != 0 {
		t.Errorf("emitted %d pieces, want none", len(ps))
	}
}

// TestWidthIsExactForEveryLineOfTheCorpusAtEveryOffset is the central
// invariant. If this holds, the gutter and the scrollbar column cannot drift —
// the failure the bubbletea original could not shake.
func TestWidthIsExactForEveryLineOfTheCorpusAtEveryOffset(t *testing.T) {
	for _, line := range corpusLines(t) {
		total := DisplayWidth(line, 4)
		for xOff := Col(0); xOff <= total+2; xOff++ {
			for _, width := range []uint16{1, 2, 3, 5, 17, 40, 80} {
				if got := renderedWidth(pieces(line, xOff, width)); got != Col(width) {
					t.Fatalf("line=%q xOff=%d width=%d: rendered %d", line, xOff, width, got)
				}
			}
		}
	}
}

func TestSelectionChangesOnlyTheBackground(t *testing.T) {
	th := testTheme()
	out := piecesSel("abcdef", 0, 6, 2, 4, true)
	baseFg := th.Base().GetFg()
	var selected strings.Builder
	for _, p := range out {
		if p.style.GetFg() != baseFg {
			t.Errorf("selection must not touch fg: %q", p.text)
		}
		if p.style.GetBg() == th.Palette.SelBg {
			selected.WriteString(p.text)
		}
	}
	if selected.String() != "cd" {
		t.Errorf("selected %q, want %q", selected.String(), "cd")
	}
}

func TestAnEmptyLineInASelectionPaintsOneCell(t *testing.T) {
	// An empty line between two selected lines must still show a highlight.
	th := testTheme()
	out := piecesSel("", 0, 10, 0, 1, true)
	var selected Col
	for _, p := range out {
		if p.style.GetBg() == th.Palette.SelBg {
			selected += DisplayWidth(p.text, 4)
		}
	}
	if selected != 1 {
		t.Errorf("selected %d cells, want exactly 1 marking the newline", selected)
	}
	if got := renderedWidth(out); got != 10 {
		t.Errorf("row width %d, want 10", got)
	}
}

func TestAShortLineInASelectionIsPaintedPastItsText(t *testing.T) {
	th := testTheme()
	out := piecesSel("ab", 0, 10, 0, 3, true)
	var selected Col
	for _, p := range out {
		if p.style.GetBg() == th.Palette.SelBg {
			selected += DisplayWidth(p.text, 4)
		}
	}
	if selected != 3 {
		t.Errorf("selected %d cells, want 3 (two text cells plus the newline)", selected)
	}
}

func TestASelectionScrolledOutOfViewPaintsNoTail(t *testing.T) {
	th := testTheme()
	// The selection covers columns 0..3 but the view starts at column 20.
	for _, p := range piecesSel("ab", 20, 6, 0, 3, true) {
		if p.style.GetBg() == th.Palette.SelBg {
			t.Fatalf("nothing selected should be on screen, got %q", p.text)
		}
	}
}

func TestSelectionBackgroundContinuesThroughASplitWideChar(t *testing.T) {
	th := testTheme()
	// Select columns 0..2, then scroll so only 日's right half shows.
	out := piecesSel("日本", 1, 3, 0, 2, true)
	if out[0].style.GetBg() != th.Palette.SelBg {
		t.Error("the visible half lost its selection background")
	}
}

func TestSelectionAcrossAZWJFamilyKeepsTheClusterWhole(t *testing.T) {
	line := "a👨‍👩‍👧‍👦b"
	out := piecesSel(line, 0, 8, 1, 3, true)
	if !strings.Contains(textOf(out), "\U0001f468") {
		t.Error("the family did not survive")
	}
	if got := renderedWidth(out); got != 8 {
		t.Errorf("row width %d, want 8", got)
	}
}

// TestSyntaxRunsColourTheRightBytes checks that runs recorded over raw line
// bytes land on the right clusters, including after multi-byte text.
func TestSyntaxRunsColourTheRightBytes(t *testing.T) {
	th := testTheme()
	line := "日本abc"
	// 日本 is six bytes; colour those green and leave "abc" default.
	runs := []StyleRun{{End: 6, Slot: SlotGreen}, {End: 9, Slot: SlotFg}}

	var out []piece
	emitRow(&Row{
		Line:  line,
		Table: tableFor(line, 4),
		Runs:  runs,
		Width: 10,
	}, th, func(s string, st catatui.Style) { out = append(out, piece{s, st}) })

	var green strings.Builder
	for _, p := range out {
		if p.style.GetFg() == th.Palette.Green {
			green.WriteString(p.text)
		}
	}
	if green.String() != "日本" {
		t.Errorf("green text = %q, want %q", green.String(), "日本")
	}
}
