package main

import (
	"runtime"
	"testing"

	"github.com/Fiend3d/catatui"
)

func TestBengaliTerminalWidths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Terminal grapheme measurement")
	}
	for _, tc := range []struct {
		text string
		want Col
	}{
		{"বাং", 2}, {"লা", 2}, {"প", 1}, {"রী", 2}, {"ক্ষা", 2},
		{"লে", 2}, {"খা", 2}, {"সু", 1}, {"ন্দ", 2}, {"দে", 2},
		{"টি", 2}, {"রি", 2}, {"বং", 2}, {"বঃ", 2},
		{"মা", 2}, {"না", 2}, {"কো", 2}, {"কো", 2}, {"কৌ", 2}, {"কৌ", 2},
		{"கா", 2}, {"കാ", 2}, {"କା", 2},
	} {
		if got := tableFor(tc.text, 4).Width(); got != tc.want {
			t.Errorf("%q width = %d, want %d", tc.text, got, tc.want)
		}
	}
}

func TestBengaliSelectionTracksTerminalColumns(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Terminal grapheme measurement")
	}
	a := testApp(t, "testdata/unicode.txt", 80, 24)
	const text = "বাংলা পরীক্ষা লেখা।"
	row := -1
	for n := 0; n < a.TotalLines; n++ {
		if a.fb.Line(n) == text {
			row = n
			break
		}
	}
	if row < 0 {
		t.Fatal("Bengali fixture missing")
	}
	a.YOff = row - 1
	// These are terminal columns, independently specified rather than derived
	// from the layout being tested. Exercise both halves of লা and খা, as
	// well as selecting the complete line after the formerly short clusters.
	for _, tc := range []struct {
		from, to uint16
		want     string
	}{{2, 3, "লা"}, {3, 2, "লা"}, {13, 14, "খা"}, {14, 13, "খা"}, {0, 16, text}} {
		drag(a, row, tc.from, tc.to)
		if got := a.SelectedText(); got != tc.want {
			t.Errorf("drag %d->%d: copied %q, want %q", tc.from, tc.to, got, tc.want)
		}
		buf := frame(t, a, 80, 24)
		l := a.Layout()
		for _, glyph := range []struct {
			col, width uint16
			text       string
		}{{0, 2, "বাং"}, {2, 2, "লা"}, {4, 1, " "}, {5, 1, "প"}, {6, 2, "রী"}, {8, 2, "ক্ষা"},
			{10, 1, " "}, {11, 2, "লে"}, {13, 2, "খা"}, {15, 1, "।"}} {
			c := buf.CellAt(l.ContentX+glyph.col, 1)
			if c.GetSymbol() != glyph.text || c.Width() != glyph.width {
				t.Fatalf("terminal column %d: symbol %q, width %d; want %q width %d", glyph.col, c.GetSymbol(), c.Width(), glyph.text, glyph.width)
			}
			wantSelected := tc.want == text || tc.want == glyph.text
			if gotSelected := c.Bg == theme.Palette.SelBg; gotSelected != wantSelected {
				t.Errorf("column %d: selected=%v, want %v", glyph.col, gotSelected, wantSelected)
			}
		}
		// No separate update may overwrite the spacing vowel's continuation.
		blank := catatui.NewBuffer(buf.Area)
		for _, pc := range blank.Diff(buf) {
			if pc.Y == 1 && (pc.X == l.ContentX+3 || pc.X == l.ContentX+14) {
				t.Errorf("redraw writes inside a Bengali vowel cluster at %d", pc.X-l.ContentX)
			}
		}
	}
}
