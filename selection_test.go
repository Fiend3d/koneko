package main

import "testing"

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
	a := testApp(t, "testdata/unicode.txt", 80, 24)
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
