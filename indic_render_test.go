package main

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/term"
)

// Inspect the actual VT output, not just the in-memory highlight. Moving the
// terminal cursor between the consonants of a conjunct interrupts shaping.
func TestIndicRedrawEmitsWholeGraphemes(t *testing.T) {
	for _, text := range []string{"हिन्दी", "क्षि", "நி", "கொ", "ஸ்ரீ", "పరీక్ష", "বাংলা পরীক্ষা লেখা।", "আমার নাম রবি।", "বাংলাদেশ একটি সুন্দর দেশ।"} {
		t.Run(text, func(t *testing.T) {
			tab := tableFor(text, 4)
			prev := catatui.NewBuffer(catatui.NewRect(0, 0, 30, 1))
			prev.SetString(0, 0, strings.Repeat("i", 30), testTheme().Base())
			// Paint, select, shrink, reverse, and clear against the previous
			// frame so this also exercises the terminal's incremental diff.
			for _, span := range [][2]Col{{0, 0}, {0, tab.Width()}, {0, 2}, {2, tab.Width()}, {0, 0}} {
				next := catatui.NewBuffer(prev.Area)
				renderRow(next, 0, 0, &Row{Line: text, Table: tab, Width: 30,
					SelFrom: span[0], SelTo: span[1], SelOK: span[0] < span[1]}, testTheme())
				var output bytes.Buffer
				backend := term.NewBackend(&output)
				updates := prev.Diff(next)
				if err := backend.Draw(updates); err != nil {
					t.Fatal(err)
				}
				if err := backend.Flush(); err != nil {
					t.Fatal(err)
				}
				for i, c := range tab.Clusters() {
					glyph := tab.ClusterStr(text, i)
					cell := next.CellAt(uint16(c.Col), 0)
					if cell.GetSymbol() != glyph || Col(cell.Width()) != Col(c.Width) {
						t.Fatalf("cluster %q split in buffer: symbol %q width %d, want %d", glyph, cell.GetSymbol(), cell.Width(), c.Width)
					}
					for _, pc := range updates {
						if Col(pc.X) >= Col(c.Col) && Col(pc.X) < c.EndCol() {
							if Col(pc.X) != Col(c.Col) || pc.Cell.GetSymbol() != glyph || !strings.Contains(output.String(), glyph) {
								t.Fatalf("redraw split %q: cell at %d = %q; VT output %q", glyph, pc.X, pc.Cell.GetSymbol(), output.String())
							}
						}
					}
				}
				prev = next
			}
		})
	}
}

func TestTamilLigaturesHaveTailoredSelectionBoundaries(t *testing.T) {
	for _, glyph := range []string{"க்ஷ", "க்ஷி", "ஶ்ரீ", "ஸ்ரீ", "நி", "கொ", "கௌ"} {
		tab := tableFor(glyph, 4)
		if len(tab.Clusters()) != 1 {
			t.Errorf("%q split into %q", glyph, clusterTexts(glyph))
		}
		for col := Col(0); col <= tab.Width(); col++ {
			if snapped := tab.SnapCol(col); snapped != 0 && snapped != tab.Width() {
				t.Errorf("%q: selection can stop inside the ligature at %d", glyph, snapped)
			}
			if cut := TruncateToWidth(glyph, col); cut != "" && cut != glyph {
				t.Errorf("truncation split %q into %q", glyph, cut)
			}
		}
	}
}

func TestWindowsIndicWidthsLeaveNoPhantomColumns(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows Terminal grapheme width policy")
	}
	// Golden advances from Windows Terminal's grapheme measurement: sum the
	// codepoint widths, capped at two per Unicode cluster. The tailored Tamil
	// kshi selection unit spans two terminal clusters, so it still takes three.
	for _, tc := range []struct {
		text  string
		width Col
	}{{"हिन्दी", 4}, {"न्दी", 2}, {"क्षि", 2}, {"நி", 2}, {"கொ", 2}, {"க்ஷி", 3}, {"ஸ்ரீ", 2}} {
		tab := tableFor(tc.text, 4)
		if tab.Width() != tc.width || DisplayWidth(tc.text, 4) != tc.width {
			t.Errorf("%q: layout width %d, display width %d, want %d", tc.text, tab.Width(), DisplayWidth(tc.text, 4), tc.width)
		}
		prev := catatui.NewBuffer(catatui.NewRect(0, 0, 20, 1))
		prev.SetString(0, 0, strings.Repeat("i", 20), testTheme().Base())
		next := catatui.NewBuffer(prev.Area)
		renderRow(next, 0, 0, &Row{Line: tc.text, Table: tab, Width: 20, SelOK: true, SelTo: tc.width}, testTheme())
		var col Col
		for _, pc := range prev.Diff(next) {
			if Col(pc.X) != col {
				t.Fatalf("%q: unwritten cells at %d before update at %d", tc.text, col, pc.X)
			}
			w := Col(1)
			if col < tc.width {
				w = Col(catatui.StringWidth(pc.Cell.GetSymbol()))
				if pc.Cell.Bg != testTheme().Palette.SelBg {
					t.Errorf("%q: unselected cell at %d", tc.text, col)
				}
			}
			col += w
		}
		if col != 20 {
			t.Errorf("%q: redraw stopped at %d, want 20", tc.text, col)
		}
	}
}
