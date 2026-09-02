package main

import (
	"testing"

	"github.com/Fiend3d/catatui"
)

// TestScrollingLeavesNoStaleCells drives the incremental path the way a user
// does: one Terminal, scrolled a line at a time, so every frame after the first
// is a diff against the last. What ends up on the backend must show the same
// thing a fresh full redraw of that state would.
//
// A mismatch is a cell the diff decided did not need rewriting when it did —
// which on screen is a leftover glyph from an earlier frame.
//
// The comparison steps by display width rather than visiting every cell: the
// column after a double-width glyph is covered by that glyph on a real
// terminal, and TestBackend does not model that, so whatever it holds there is
// not on screen either way.
func TestScrollingLeavesNoStaleCells(t *testing.T) {
	const w, h = 100, 24

	a := testApp(t, "testdata/unicode.txt", w, h)
	backend := catatui.NewTestBackend(w, h)
	terminal, err := catatui.NewTerminal(backend)
	if err != nil {
		t.Fatal(err)
	}
	want := testApp(t, "testdata/unicode.txt", w, h)

	for step := 0; step < a.TotalLines; step++ {
		if err := terminal.Draw(func(f *catatui.Frame) { draw(f, a) }); err != nil {
			t.Fatal(err)
		}

		// What the screen should show for this exact state.
		want.YOff = a.YOff
		fresh := frame(t, want, w, h)
		got := backend.Buffer()

		for y := uint16(0); y < h; y++ {
			for x := uint16(0); x < w; {
				f := fresh.CellAt(x, y)
				sym := f.GetSymbol()
				if g := got.CellAt(x, y).GetSymbol(); g != sym {
					t.Fatalf("at line %d: cell (%d,%d) shows %q, a fresh redraw shows %q\n"+
						"  incremental row: %q\n  fresh row:       %q",
						a.YOff, x, y, g, sym, rowText(got, y, w), rowText(fresh, y, w))
				}
				x += max(f.Width(), 1)
			}
		}
		a.Apply(Action{Kind: ActScrollLines, N: 1})
	}
}
