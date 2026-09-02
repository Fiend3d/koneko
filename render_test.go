package main

import (
	"os"
	"strings"
	"testing"

	"github.com/Fiend3d/catatui"
)

func testOptions() Options {
	return Options{TabWidth: 4, LineNumbers: true, Scrollbar: true, Theme: "dracula"}
}

// testApp opens a file and sizes the app, with highlighting off unless a test
// turns it on.
func testApp(t *testing.T, path string, w, h uint16) *App {
	t.Helper()
	setTheme("dracula")
	fb, err := OpenFileBuffer(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { fb.Close() })
	a := NewApp(fb, testOptions())
	a.Apply(Action{Kind: ActResize, X: w, Y: h})
	return a
}

// frame renders one frame and returns the resulting buffer.
func frame(t *testing.T, a *App, w, h uint16) *catatui.Buffer {
	t.Helper()
	backend := catatui.NewTestBackend(w, h)
	terminal, err := catatui.NewTerminal(backend)
	if err != nil {
		t.Fatal(err)
	}
	if err := terminal.Draw(func(f *catatui.Frame) { draw(f, a) }); err != nil {
		t.Fatal(err)
	}
	return backend.Buffer()
}

// rowText reconstructs a row's logical text. A double-width glyph occupies one
// cell plus a blanked continuation cell, so stepping by display width is what
// turns the buffer back into what is actually on screen.
func rowText(buf *catatui.Buffer, y, w uint16) string {
	var b strings.Builder
	for x := uint16(0); x < w; {
		sym := buf.CellAt(x, y).GetSymbol()
		b.WriteString(sym)
		x += max(uint16(catatui.StringWidth(sym)), 1)
	}
	return b.String()
}

// TestEveryCellOfTheFrameIsWritten is the screen-level form of the width
// invariant. A cell left at its default style means a row did not fill its
// width — the drift the bubbletea original suffered. catatui drops a wide
// grapheme that would overflow rather than clipping it, so a gap here would be
// silent at runtime.
func TestEveryCellOfTheFrameIsWritten(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)
	buf := frame(t, a, 80, 24)
	for y := uint16(0); y < 24; y++ {
		for x := uint16(0); x < 80; x++ {
			if !buf.CellAt(x, y).GetStyle().GetBg().IsSet() {
				t.Fatalf("cell (%d,%d) was never written", x, y)
			}
		}
	}
}

func TestEveryRowOfEveryFrameIsExactlyTheTerminalWidth(t *testing.T) {
	// Sweep sizes, including awkward narrow ones, over the torture corpus.
	for _, size := range [][2]uint16{{80, 24}, {40, 10}, {13, 5}, {200, 50}, {3, 3}} {
		w, h := size[0], size[1]
		a := testApp(t, "testdata/unicode.txt", w, h)
		a.Apply(Action{Kind: ActScrollHalfPage, N: 1})
		buf := frame(t, a, w, h)
		for y := uint16(0); y < h; y++ {
			for x := uint16(0); x < w; x++ {
				if !buf.CellAt(x, y).GetStyle().GetBg().IsSet() {
					t.Fatalf("%dx%d: gap at (%d,%d)", w, h, x, y)
				}
			}
		}
	}
}

// TestTheScrollbarColumnHoldsOnlyTrackOrThumb sweeps the horizontal offset
// across CJK, emoji and Devanagari text. Content must never be pushed into the
// scrollbar column.
func TestTheScrollbarColumnHoldsOnlyTrackOrThumb(t *testing.T) {
	for xOff := 0; xOff < 60; xOff++ {
		a := testApp(t, "testdata/unicode.txt", 80, 24)
		for i := 0; i < xOff; i++ {
			a.Apply(Action{Kind: ActScrollX, N: 1})
		}
		a.Apply(Action{Kind: ActScrollHalfPage, N: 1})
		buf := frame(t, a, 80, 24)
		for y := uint16(0); y < 23; y++ {
			if sym := buf.CellAt(79, y).GetSymbol(); sym != " " && sym != thumb {
				t.Fatalf("xOff %d, row %d: scrollbar column holds %q", xOff, y, sym)
			}
		}
	}
}

func TestTheGutterShowsLineNumbersAlignedRight(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)
	buf := frame(t, a, 80, 24)
	if got := rowText(buf, 0, 5); !strings.HasPrefix(got, "  1 ") {
		t.Errorf("row 0 gutter = %q", got)
	}
	if got := rowText(buf, 9, 5); !strings.HasPrefix(got, " 10 ") {
		t.Errorf("row 9 gutter = %q", got)
	}
}

func TestHidingTheGutterReclaimsTheColumn(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)
	a.Apply(Action{Kind: ActToggleLineNumbers})
	buf := frame(t, a, 80, 24)
	if got := rowText(buf, 0, 20); !strings.HasPrefix(got, "Unicode") {
		t.Errorf("row 0 = %q, want the text at column 0", got)
	}
}

func TestACJKLineRendersItsCharactersNotReplacementMarks(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)
	buf := frame(t, a, 80, 24)
	found := false
	for y := uint16(0); y < 23; y++ {
		row := rowText(buf, y, 80)
		if strings.Contains(row, "�") {
			t.Fatalf("replacement character on row %d: %q", y, row)
		}
		if strings.ContainsAny(row, "日本語") {
			found = true
		}
	}
	if !found {
		t.Error("expected Japanese text somewhere in the frame")
	}
}

func TestTheStatusBarShowsTheFileAndPosition(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)
	buf := frame(t, a, 80, 24)
	status := rowText(buf, 23, 80)
	if !strings.Contains(status, "unicode.txt") {
		t.Errorf("status bar = %q", status)
	}
	if !strings.Contains(status, "/") {
		t.Errorf("status bar has no line count: %q", status)
	}
}

func TestTheSearchPromptReplacesTheStatusBar(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)
	a.Apply(Action{Kind: ActOpenSearch})
	a.Prompt.SetText("日本")
	buf := frame(t, a, 80, 24)
	if got := rowText(buf, 23, 80); !strings.HasPrefix(got, " search: 日本") {
		t.Errorf("prompt row = %q", got)
	}
}

func TestTheHelpOverlayCoversTheWholeScreen(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)
	a.Apply(Action{Kind: ActOpenHelp})
	buf := frame(t, a, 80, 24)
	if got := rowText(buf, 0, 20); !strings.Contains(got, "Koneko") {
		t.Errorf("help row 0 = %q", got)
	}
	if got := rowText(buf, 23, 80); !strings.Contains(got, "HELP") {
		t.Errorf("help status = %q", got)
	}
	for y := uint16(0); y < 24; y++ {
		for x := uint16(0); x < 80; x++ {
			if !buf.CellAt(x, y).GetStyle().GetBg().IsSet() {
				t.Fatalf("help gap at (%d,%d)", x, y)
			}
		}
	}
}

func TestSelectedRowsRenderWithTheSelectionBackground(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)
	a.Apply(Action{Kind: ActSelectAll})
	buf := frame(t, a, 80, 24)
	found := false
	for x := uint16(0); x < 80; x++ {
		if buf.CellAt(x, 0).GetStyle().GetBg() == theme.Palette.SelBg {
			found = true
		}
	}
	if !found {
		t.Error("no selection background on the first row")
	}
}

// TestAnEmptyLineBetweenSelectedLinesIsHighlightedOnScreen is the end-to-end
// form of the empty-line case, at the buffer level.
func TestAnEmptyLineBetweenSelectedLinesIsHighlightedOnScreen(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)

	// Find a blank line with text on both sides, and select across it.
	blank := -1
	for n := 1; n < min(a.TotalLines-1, 60); n++ {
		if a.LineWidth(n) == 0 && a.LineWidth(n-1) > 0 && a.LineWidth(n+1) > 0 {
			blank = n
			break
		}
	}
	if blank < 0 {
		t.Skip("no blank line surrounded by text in the corpus")
	}

	a.Sel.Begin(Pos{blank - 1, 0})
	a.Sel.Extend(Pos{blank + 1, 1})
	a.Sel.Finish()
	buf := frame(t, a, 80, 24)

	l := a.Layout()
	painted := 0
	for x := l.ContentX; x < l.ContentX+l.ContentW; x++ {
		if buf.CellAt(x, uint16(blank-a.YOff)).GetStyle().GetBg() == theme.Palette.SelBg {
			painted++
		}
	}
	if painted != 1 {
		t.Errorf("blank line shows %d selected cells, want exactly 1", painted)
	}

	// Its neighbours are still highlighted, so the run is contiguous.
	for _, row := range []int{blank - 1, blank + 1} {
		any := false
		for x := l.ContentX; x < l.ContentX+l.ContentW; x++ {
			if buf.CellAt(x, uint16(row-a.YOff)).GetStyle().GetBg() == theme.Palette.SelBg {
				any = true
			}
		}
		if !any {
			t.Errorf("row %d lost its highlight", row)
		}
	}
}

// TestSyntaxColoursReachTheScreen drives the whole highlight path: the worker
// goroutine, the channel, the cache, then the render.
func TestSyntaxColoursReachTheScreen(t *testing.T) {
	setTheme("dracula")
	fb, err := OpenFileBuffer("testdata/sample.go")
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()

	opts := testOptions()
	opts.Highlight = true
	a := NewApp(fb, opts)
	a.Apply(Action{Kind: ActResize, X: 80, Y: 24})

	results := make(chan HlResult, 1)
	h := NewHighlighter("testdata/sample.go", results)
	defer h.Close()
	a.AttachHighlighter(h)
	a.RequestHighlight()
	a.InstallHighlight(<-results)

	buf := frame(t, a, 80, 24)

	// More than a couple of foreground colours must appear, or nothing was
	// actually highlighted.
	colours := map[catatui.Color]bool{}
	for y := uint16(0); y < 10; y++ {
		for x := uint16(5); x < 70; x++ {
			colours[buf.CellAt(x, y).GetStyle().GetFg()] = true
		}
	}
	if len(colours) <= 2 {
		t.Errorf("expected several syntax colours, saw %d", len(colours))
	}
}

func TestAStaleHighlightResultIsDiscarded(t *testing.T) {
	a := testApp(t, "testdata/sample.go", 80, 24)
	a.Highlight = true
	// Generation 99 never matches the app's current generation of 0.
	a.InstallHighlight(HlResult{From: 0, Runs: [][]StyleRun{{{End: 5, Slot: SlotPink}}}, Gen: 99})
	if a.Hl.Get(0) != nil {
		t.Error("a stale result must not be cached")
	}
}

// TestDumpFrame prints a rendered frame for eyeballing. Not an assertion; run it
// with:
//
//	go test -run TestDumpFrame -v
//	KONEKO_DUMP_FILE=testdata/sample.go KONEKO_DUMP_SCROLL=10 go test -run TestDumpFrame -v
func TestDumpFrame(t *testing.T) {
	if os.Getenv("KONEKO_DUMP") == "" {
		t.Skip("set KONEKO_DUMP=1 to print a frame")
	}
	path := envOr("KONEKO_DUMP_FILE", "testdata/unicode.txt")
	w, h := uint16(80), uint16(24)

	a := testApp(t, path, w, h)
	a.Highlight = false
	for i := 0; i < atoiOr(os.Getenv("KONEKO_DUMP_SCROLL"), 0); i++ {
		a.Apply(Action{Kind: ActScrollLines, N: 1})
	}
	for i := 0; i < atoiOr(os.Getenv("KONEKO_DUMP_X"), 0); i++ {
		a.Apply(Action{Kind: ActScrollX, N: 1})
	}
	buf := frame(t, a, w, h)

	t.Logf("+%s+", strings.Repeat("-", int(w)))
	for y := uint16(0); y < h; y++ {
		t.Logf("|%s|", rowText(buf, y, w))
	}
	t.Logf("+%s+", strings.Repeat("-", int(w)))
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func atoiOr(s string, fallback int) int {
	n := 0
	if s == "" {
		return fallback
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return fallback
		}
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// TestSelectingIndicTextLeavesNoGaps is the bug this corpus was opened to find:
// selecting the Hindi block painted a highlight with holes in it, one beside
// every consonant carrying a spacing vowel sign.
//
// The cause was width, not selection. uniseg scores a spacing combining mark a
// column of its own, so हि measured two columns; the terminal shapes it into
// one glyph and draws it in one, and the column left over was never painted by
// anyone. Every cluster of these scripts is one column now, so a selection
// across one of their lines covers all of it.
//
// The corpus does put a stray 。 at the end of two Tamil lines. That one really
// is two columns and its trailing cell really is unstyled in the buffer, but a
// terminal paints both columns of a double-width glyph from its leading cell,
// so nothing shows. Only the column a cluster starts at is checked, which is
// the column that had the hole.
func TestSelectingIndicTextLeavesNoGaps(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)

	// Devanagari, Bengali, Tamil and Telugu.
	isIndic := func(r rune) bool {
		switch {
		case r >= 0x0900 && r <= 0x097F, // Devanagari
			r >= 0x0980 && r <= 0x09FF, // Bengali
			r >= 0x0B80 && r <= 0x0BFF, // Tamil
			r >= 0x0C00 && r <= 0x0C7F: // Telugu
			return true
		}
		return false
	}

	var checked int
	for row := 0; row < a.TotalLines; row++ {
		line := a.fb.Line(row)
		if !strings.ContainsFunc(line, isIndic) {
			continue
		}
		// A table of its own: LineTable hands out the App's shared scratch, and
		// rendering a frame rebuilds it for every line on screen.
		tab := tableFor(line, a.TabWidth)

		width := tab.Width()
		a.YOff = max(row-2, 0)
		a.Sel.Clear()
		a.Sel.Begin(Pos{row, 0})
		a.Sel.Extend(Pos{row, width})
		a.Sel.Finish()

		buf := frame(t, a, 80, 24)
		l := a.Layout()
		y := uint16(row - a.YOff)

		for i, c := range tab.Clusters() {
			text := tab.ClusterStr(line, i)
			if c.Width > 1 && strings.ContainsFunc(text, isIndic) {
				t.Errorf("line %d: cluster %q spans %d columns, want 1",
					row+1, text, c.Width)
			}
			cell := buf.CellAt(l.ContentX+uint16(c.Col), y)
			if cell.GetStyle().GetBg() != theme.Palette.SelBg {
				t.Fatalf("line %d %q: cluster %q at column %d is not selected",
					row+1, line, text, c.Col)
			}
		}
		checked++
	}
	if checked < 8 {
		t.Fatalf("only %d Indic lines checked, expected the corpus to have more", checked)
	}
}
