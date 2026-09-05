package main

import (
	"strings"
	"testing"
)

// Exercise every cell, including the right half of a wide glyph and the
// interior of expanded tabs. Keep the corpus on screen so mouse actions are
// not silently ignored or interpreted as autoscrolling.
func TestUnicodeDoubleClickSelectsTheGlyphUnderThePointer(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 256, 256)
	for row := 0; row < a.TotalLines; row++ {
		line := a.fb.Line(row)
		tab := tableFor(line, a.TabWidth)
		for i, c := range tab.Clusters() {
			start, end := FindWordBounds(line, tab, Col(c.Col))
			if start == end {
				start, end = Col(c.Col), c.EndCol()
			}
			want := line[tab.ColToByte(start):tab.ColToByte(end)]
			for col := Col(c.Col); col < c.EndCol(); col++ {
				a.Sel.Clear()
				a.clickCount = 0
				x := a.Layout().ContentX + uint16(col)
				for click := 0; click < 2; click++ {
					a.Apply(Action{Kind: ActMouseDown, X: x, Y: uint16(row), Button: MouseLeft})
					a.Apply(Action{Kind: ActMouseUp, X: x, Y: uint16(row), Button: MouseLeft})
				}
				if got := a.SelectedText(); got != want {
					t.Fatalf("row %d cluster %d col %d: got %q, want %q", row, i, col, got, want)
				}
			}
		}
	}
}

func TestUnicodeDragsCopyExactlyTheHighlightedGraphemes(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 256, 256)
	for row := 0; row < a.TotalLines; row++ {
		line := a.fb.Line(row)
		tab := tableFor(line, a.TabWidth)
		for from := Col(0); from <= tab.Width(); from++ {
			for _, delta := range []Col{-7, -2, -1, 1, 2, 7} {
				to := max(Col(0), min(from+delta, tab.Width()))
				a.Sel.Clear()
				drag(a, row, uint16(from), uint16(to))
				lo, hi := tab.SnapCol(from), tab.SnapCol(to)
				if lo > hi {
					lo, hi = hi, lo
				}
				want := line[tab.ColToByte(lo):tab.ColToByte(hi)]
				if got := a.SelectedText(); got != want {
					t.Fatalf("row %d drag %d->%d: got %q, want %q", row, from, to, got, want)
				}
				var painted strings.Builder
				for _, p := range piecesSel(line, 0, 256, a.Sel.Start.Col, a.Sel.End.Col, a.Sel.IsVisible()) {
					if p.style.GetBg() == theme.Palette.SelBg {
						painted.WriteString(p.text)
					}
				}
				var expanded strings.Builder
				for i, c := range tab.Clusters() {
					if Col(c.Col) >= lo && Col(c.Col) < hi {
						g := tab.ClusterStr(line, i)
						if g == "\t" {
							g = strings.Repeat(" ", int(c.Width))
						}
						expanded.WriteString(g)
					}
				}
				if painted.String() != expanded.String() {
					t.Fatalf("row %d drag %d->%d: highlighted %q, want %q", row, from, to, painted.String(), expanded.String())
				}
			}
		}
	}
}

func TestWordSelectionUsesRawColumnsWhenScrolledAndExtended(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 40, 12)
	for row := 1; row < a.TotalLines; row++ {
		line := a.fb.Line(row)
		tab := tableFor(line, a.TabWidth)
		for _, c := range tab.Clusters() {
			if c.Width < 2 {
				continue
			}
			lo, hi := FindWordBounds(line, tab, Col(c.Col))
			if lo == hi {
				lo, hi = Col(c.Col), c.EndCol()
			}
			// Put the final cell of this grapheme at the left of the pane.
			a.YOff, a.XOff = row-1, c.EndCol()-1
			x := a.Layout().ContentX
			a.Sel.Clear()
			a.clickCount = 0
			for click := 0; click < 2; click++ {
				a.Apply(Action{Kind: ActMouseDown, X: x, Y: 1, Button: MouseLeft})
				a.Apply(Action{Kind: ActMouseUp, X: x, Y: 1, Button: MouseLeft})
			}
			if a.Sel.Start != (Pos{row, lo}) || a.Sel.End != (Pos{row, hi}) {
				t.Fatalf("row %d scrolled double click: %v..%v, want %d..%d", row, a.Sel.Start, a.Sel.End, lo, hi)
			}
			// Both dragging and right-click extension must identify the same
			// target as double-clicking, even at an interior display column.
			for _, kind := range []ActionKind{ActMouseDrag, ActMouseDown} {
				a.beginSelect(SelectWord, Pos{row, 0})
				anchor := a.Sel.Start
				button := MouseLeft
				if kind == ActMouseDown {
					a.Sel.Finish()
					button = MouseRight
				}
				a.Apply(Action{Kind: kind, X: x, Y: 1, Button: button})
				if a.Sel.Start != anchor || a.Sel.End != (Pos{row, hi}) {
					t.Fatalf("row %d extension %v: %v..%v, want %v..%v", row, kind, a.Sel.Start, a.Sel.End, anchor, Pos{row, hi})
				}
			}
		}
	}
}

func TestBeginAndExtendOrdersPositions(t *testing.T) {
	var s Selection
	s.Begin(Pos{5, 10})
	s.Extend(Pos{2, 3})
	s.Finish()
	if s.Start != (Pos{2, 3}) || s.End != (Pos{5, 10}) {
		t.Errorf("got %v-%v, want {2 3}-{5 10}", s.Start, s.End)
	}
}

func TestAClickThatNeverMovesLeavesNoSelection(t *testing.T) {
	var s Selection
	s.Begin(Pos{3, 4})
	s.Finish()
	if s.Active {
		t.Error("a zero-width selection should not stay active")
	}
}

// TestExtendMovesTheNearEnd is what makes right-click extend feel right: the
// far end stays pinned so the selection grows or shrinks from the side clicked.
func TestExtendMovesTheNearEnd(t *testing.T) {
	var s Selection
	s.Begin(Pos{10, 0})
	s.Extend(Pos{20, 0})
	s.Finish()

	// Clicking just past the end moves the end, leaving the start alone.
	s.Extend(Pos{22, 0})
	s.Finish()
	if s.Start != (Pos{10, 0}) || s.End != (Pos{22, 0}) {
		t.Errorf("got %v-%v, want {10 0}-{22 0}", s.Start, s.End)
	}

	// Clicking near the start moves the start instead.
	s.Extend(Pos{8, 0})
	s.Finish()
	if s.Start != (Pos{8, 0}) || s.End != (Pos{22, 0}) {
		t.Errorf("got %v-%v, want {8 0}-{22 0}", s.Start, s.End)
	}
}

// TestExtendCanShrinkASelection follows from re-anchoring: a click inside the
// selection collapses it toward the far end rather than always growing it.
func TestExtendCanShrinkASelection(t *testing.T) {
	var s Selection
	s.Begin(Pos{10, 0})
	s.Extend(Pos{20, 0})
	s.Finish()

	s.Extend(Pos{18, 0})
	s.Finish()
	if s.Start != (Pos{10, 0}) || s.End != (Pos{18, 0}) {
		t.Errorf("got %v-%v, want {10 0}-{18 0}", s.Start, s.End)
	}
}

func TestDraggingBackwardKeepsTheAnchorPinned(t *testing.T) {
	var s Selection
	s.Begin(Pos{10, 5})
	s.Extend(Pos{4, 2})
	if s.Start != (Pos{4, 2}) || s.End != (Pos{10, 5}) {
		t.Errorf("got %v-%v, want {4 2}-{10 5}", s.Start, s.End)
	}
	// Dragging back past the anchor flips the direction without losing it.
	s.Extend(Pos{14, 1})
	if s.Start != (Pos{10, 5}) || s.End != (Pos{14, 1}) {
		t.Errorf("got %v-%v, want {10 5}-{14 1}", s.Start, s.End)
	}
}

func TestRowSpanCoversWholeInteriorLines(t *testing.T) {
	var s Selection
	s.Begin(Pos{2, 3})
	s.Extend(Pos{5, 4})
	s.Finish()

	// An interior line runs from column 0 through its newline cell.
	from, to, ok := s.RowSpan(3, 10)
	if !ok || from != 0 || to != 11 {
		t.Errorf("interior row: (%d, %d, %v), want (0, 11, true)", from, to, ok)
	}
	// The first line starts at the anchor column.
	from, to, ok = s.RowSpan(2, 10)
	if !ok || from != 3 || to != 11 {
		t.Errorf("first row: (%d, %d, %v), want (3, 11, true)", from, to, ok)
	}
	// The last line stops at the end column and takes no newline cell.
	from, to, ok = s.RowSpan(5, 10)
	if !ok || from != 0 || to != 4 {
		t.Errorf("last row: (%d, %d, %v), want (0, 4, true)", from, to, ok)
	}
}

func TestRowSpanRejectsRowsOutsideTheSelection(t *testing.T) {
	var s Selection
	s.Begin(Pos{2, 0})
	s.Extend(Pos{4, 0})
	s.Finish()
	for _, row := range []int{0, 1, 5, 99} {
		if _, _, ok := s.RowSpan(row, 10); ok {
			t.Errorf("row %d should be outside the selection", row)
		}
	}
}

// TestRowSpanMarksAnEmptyInteriorLine is the empty-line case: a blank row
// inside a selection still gets its one newline cell.
func TestRowSpanMarksAnEmptyInteriorLine(t *testing.T) {
	var s Selection
	s.Begin(Pos{2, 0})
	s.Extend(Pos{6, 0})
	s.Finish()
	from, to, ok := s.RowSpan(4, 0)
	if !ok || from != 0 || to != 1 {
		t.Errorf("(%d, %d, %v), want (0, 1, true)", from, to, ok)
	}
}

func TestRowSpanClampsPastTheEndOfALine(t *testing.T) {
	var s Selection
	s.Begin(Pos{1, 0})
	s.Extend(Pos{1, 999})
	s.Finish()
	_, to, ok := s.RowSpan(1, 5)
	if !ok || to != 6 {
		t.Errorf("to = %d (ok=%v), want 6 — the text plus its newline cell", to, ok)
	}
}

func TestClearDropsEverything(t *testing.T) {
	var s Selection
	s.Begin(Pos{1, 1})
	s.Extend(Pos{2, 2})
	s.Finish()
	s.Clear()
	if s.IsVisible() {
		t.Error("a cleared selection should not be visible")
	}
}

// TestSelectedTextCutsOnClusterBoundaries checks the column-to-byte conversion
// the copy path depends on.
func TestSelectedTextCutsOnClusterBoundaries(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)

	// Find a line with wide characters and select its first two columns, which
	// is one cluster.
	row := -1
	for n := 0; n < a.TotalLines; n++ {
		line, tab := a.LineTable(n)
		if len(tab.Clusters()) > 2 && tab.Clusters()[0].Width == 2 && len(line) > 0 {
			row = n
			break
		}
	}
	if row < 0 {
		t.Skip("no wide-character line in the corpus")
	}

	a.Sel.Begin(Pos{row, 0})
	a.Sel.Extend(Pos{row, 1}) // half of a double-width cluster
	a.Sel.Finish()
	// Rounding down to the cluster start means an empty, not a broken, result.
	if got := a.SelectedText(); got != "" {
		t.Errorf("half a wide cluster gave %q, want empty", got)
	}

	a.Sel.Begin(Pos{row, 0})
	a.Sel.Extend(Pos{row, 2})
	a.Sel.Finish()
	line, tab := a.LineTable(row)
	want := tab.ClusterStr(line, 0)
	if got := a.SelectedText(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSelectAllCoversTheWholeFile(t *testing.T) {
	a := testApp(t, "testdata/small.go", 80, 24)
	a.Apply(Action{Kind: ActSelectAll})
	if a.Sel.Start != (Pos{0, 0}) {
		t.Errorf("start = %v, want {0 0}", a.Sel.Start)
	}
	if a.Sel.End.Row != a.TotalLines-1 {
		t.Errorf("end row = %d, want %d", a.Sel.End.Row, a.TotalLines-1)
	}

	// The text must round-trip to the file's own contents.
	var want string
	for n := 0; n < a.TotalLines; n++ {
		if n > 0 {
			want += "\n"
		}
		want += a.fb.Line(n)
	}
	if got := a.SelectedText(); got != want {
		t.Errorf("select-all text did not round-trip\ngot  %q\nwant %q", got, want)
	}
}

// tamilRow finds a line of the corpus that mixes one- and two-column clusters,
// which is what makes Tamil the case that exposed the rounding: a consonant
// plus a spacing vowel sign is a single two-column cluster sitting between
// one-column ones.
func mixedWidthRow(t *testing.T, a *App) int {
	t.Helper()
	for n := 0; n < a.TotalLines; n++ {
		_, tab := a.LineTable(n)
		var narrow, wide int
		for _, c := range tab.Clusters() {
			switch c.Width {
			case 1:
				narrow++
			case 2:
				wide++
			}
		}
		if narrow > 2 && wide > 2 {
			return n
		}
	}
	t.Skip("no line mixing one- and two-column clusters in the corpus")
	return -1
}

// drag drives a press, a move and a release across one row, in screen columns.
func drag(a *App, row int, fromX, toX uint16) {
	a.clickCount = 0 // Each simulated drag starts a new click sequence.
	l := a.Layout()
	y := uint16(row - a.YOff)
	a.Apply(Action{Kind: ActMouseDown, X: l.ContentX + fromX, Y: y, Button: MouseLeft})
	a.Apply(Action{Kind: ActMouseDrag, X: l.ContentX + toX, Y: y, Button: MouseLeft})
	a.Apply(Action{Kind: ActMouseUp, X: l.ContentX + toX, Y: y, Button: MouseLeft})
}

// TestADragPaintsExactlyWhatItCopies is the invariant the two consumers of a
// selection column used to break. The renderer paints a cluster when its start
// column falls inside the range, rounding both ends up; SelectedText converts
// through ColToByte, rounding both ends down. Unless the endpoints are cluster
// boundaries the two cover different text, which on a line of Tamil is most
// drags.
func TestADragPaintsExactlyWhatItCopies(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 256, 256)
	row := mixedWidthRow(t, a)
	width := a.LineWidth(row)

	for from := uint16(0); from < uint16(width); from++ {
		for to := from + 1; to <= uint16(width); to++ {
			a.Sel.Clear()
			drag(a, row, from, to)

			painted := Col(0)
			line, tab := a.LineTable(row)
			selFrom, selTo, ok := a.Sel.RowSpan(row, tab.Width())
			if ok {
				for _, p := range piecesSel(line, 0, uint16(a.Width), selFrom, selTo, true) {
					if p.style.GetBg() == theme.Palette.SelBg {
						painted += DisplayWidth(p.text, a.TabWidth)
					}
				}
			}

			copied := DisplayWidth(a.SelectedText(), a.TabWidth)
			if painted != copied {
				t.Fatalf("drag %d->%d on line %d: painted %d columns, copied %d (%q)",
					from, to, row, painted, copied, a.SelectedText())
			}
		}
	}
}

// TestTwoClicksOnOneWideGlyphAreADoubleClick covers the other half of the same
// gap: the click counter keys off the column, so before the endpoints were
// snapped, clicking the two halves of one glyph looked like two different spots
// and word select never fired.
func TestTwoClicksOnOneWideGlyphAreADoubleClick(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)

	// A line whose first cluster is two columns wide and part of a word.
	row := -1
	for n := 0; n < a.TotalLines; n++ {
		line, tab := a.LineTable(n)
		cs := tab.Clusters()
		if len(cs) > 2 && cs[0].Width == 2 && isWordCluster(tab.ClusterStr(line, 0)) {
			row = n
			break
		}
	}
	if row < 0 {
		t.Skip("no line starting with a wide word cluster")
	}

	l := a.Layout()
	y := uint16(row - a.YOff)
	// Left half, then right half, of the same glyph.
	a.Apply(Action{Kind: ActMouseDown, X: l.ContentX, Y: y, Button: MouseLeft})
	a.Apply(Action{Kind: ActMouseUp, X: l.ContentX, Y: y, Button: MouseLeft})
	a.Apply(Action{Kind: ActMouseDown, X: l.ContentX + 1, Y: y, Button: MouseLeft})
	a.Apply(Action{Kind: ActMouseUp, X: l.ContentX + 1, Y: y, Button: MouseLeft})

	if a.Sel.Mode != SelectWord {
		t.Fatalf("select mode = %v, want SelectWord", a.Sel.Mode)
	}
	line, tab := a.LineTable(row)
	wantStart, wantEnd := FindWordBounds(line, tab, 0)
	if a.Sel.Start.Col != wantStart || a.Sel.End.Col != wantEnd {
		t.Errorf("selected columns %d..%d, want the word at %d..%d",
			a.Sel.Start.Col, a.Sel.End.Col, wantStart, wantEnd)
	}
}
