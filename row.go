// Turning one line of the file into exactly Width terminal cells.
//
// This is the render path the port exists to replace. The bubbletea original
// built a string with ANSI escapes in it and then sliced that string by display
// column — for horizontal scrolling, for the selection overlay, and again for
// padding each row to width. Every splice had to re-measure the result, and
// three separate width computations had to agree; on non-ASCII text they did
// not, so the gutter and the scrollbar column drifted.
//
// Here nothing is ever spliced. Clusters are walked once, styles attach to byte
// ranges, and a single counter tracks how many cells have been emitted. The row
// is structurally guaranteed to occupy exactly Width cells.
package main

import "github.com/Fiend3d/catatui"

// spaces is the padding source; long runs are emitted in chunks of it.
const spaces = "                                                                "

// Row is everything needed to draw one line.
type Row struct {
	// Line is the raw line, exactly as it appears in the file.
	Line string
	// Table is its cluster layout, already rebuilt for this line.
	Table *ClusterTable
	// Runs are syntax runs over Line's bytes, or nil if it is not highlighted.
	Runs []StyleRun
	// SelFrom and SelTo are the half-open column range to draw with the
	// selection background, when SelOK.
	SelFrom, SelTo Col
	SelOK          bool
	XOff           Col
	Width          uint16
}

// emitRow feeds one row's pieces to emit as (text, style) pairs.
//
// The emitted widths always sum to exactly row.Width. emit receives slices of
// row.Line (or of spaces), so this allocates nothing.
func emitRow(row *Row, th *Theme, emit func(string, catatui.Style)) {
	width := Col(row.Width)
	if width == 0 {
		return
	}

	clusters := row.Table.Clusters()
	st := styler{runs: row.Runs, selFrom: row.SelFrom, selTo: row.SelTo, selOK: row.SelOK, theme: th}
	var emitted Col

	i, ok := row.Table.IndexAtCol(row.XOff)
	if !ok {
		i = len(clusters)
	}

	// Left edge: the scroll offset can land inside a wide cluster — the right
	// half of a CJK glyph, or the middle of a tab. Emit the still-visible cells
	// as padding in that cluster's own style, so a selection background carries
	// through the split without the glyph being drawn twice.
	if i < len(clusters) && Col(clusters[i].Col) < row.XOff {
		c := clusters[i]
		visible := c.EndCol() - row.XOff
		take := min(visible, width)
		emitSpaces(take, st.styleOf(c), emit)
		emitted += take
		i++
	}

	// Body: coalesce consecutive clusters that share a style into one span, so
	// a typical source line yields a dozen spans rather than one per grapheme.
	runStart := i
	var runStyle catatui.Style
	haveRun := false
	flush := func(end int) {
		if !haveRun || end <= runStart {
			haveRun = false
			return
		}
		a := int(clusters[runStart].Byte)
		b := row.Table.ByteLen()
		if end < len(clusters) {
			b = int(clusters[end].Byte)
		}
		if a < b {
			emit(row.Line[a:b], runStyle)
		}
		haveRun = false
	}

	for i < len(clusters) && emitted < width {
		c := clusters[i]
		style := st.styleOf(c)
		isTab := row.Line[c.Byte] == '\t'
		remaining := width - emitted

		if Col(c.Width) > remaining {
			// The cluster would straddle the right edge. catatui does not clip
			// a wide grapheme — it drops it and leaves the cell as it was — so
			// it must never be handed one. Pad the remainder instead.
			flush(i)
			emitSpaces(remaining, style, emit)
			emitted = width
			break
		}

		if isTab || (haveRun && runStyle != style) {
			flush(i)
		}

		if isTab {
			// A tab is stored as one cluster whose width is the distance to the
			// next stop; it is only ever rendered as spaces.
			emitSpaces(Col(c.Width), style, emit)
			emitted += Col(c.Width)
			i++
			runStart = i
			continue
		}

		if !haveRun {
			runStart, runStyle, haveRun = i, style, true
		}
		emitted += Col(c.Width)
		i++
	}
	flush(i)

	// Tail: the line ran out before the pane did. Part of it may still be
	// selected — every row but the last includes its trailing newline, drawn as
	// one selected cell past the text, and on an empty line that cell is the
	// whole of the selection.
	if emitted >= width {
		return
	}
	background := th.Background()
	tailStart := row.XOff + emitted
	tailEnd := row.XOff + width

	if row.SelOK && row.SelFrom < tailEnd && row.SelTo > tailStart {
		selFrom := max(row.SelFrom, tailStart)
		selTo := min(row.SelTo, tailEnd)
		emitSpaces(selFrom-tailStart, background, emit)
		emitSpaces(selTo-selFrom, th.Base().Bg(th.Palette.SelBg), emit)
		emitSpaces(tailEnd-selTo, background, emit)
		return
	}
	emitSpaces(width-emitted, background, emit)
}

func emitSpaces(n Col, style catatui.Style, emit func(string, catatui.Style)) {
	for left := int(n); left > 0; {
		take := min(left, len(spaces))
		emit(spaces[:take], style)
		left -= take
	}
}

// styler resolves a cluster's style from the syntax runs and the selection.
type styler struct {
	runs []StyleRun
	// Runs are sorted by End and clusters are visited in increasing byte order,
	// so this only ever moves forward.
	cursor         int
	selFrom, selTo Col
	selOK          bool
	theme          *Theme
}

func (s *styler) styleOf(c Cluster) catatui.Style {
	slot := SlotFg
	if s.runs != nil {
		for s.cursor < len(s.runs) && s.runs[s.cursor].End <= c.Byte {
			s.cursor++
		}
		if s.cursor < len(s.runs) {
			slot = s.runs[s.cursor].Slot
		}
	}

	style := s.theme.Style(slot)
	// Only the background changes. The bubbletea original stripped the syntax
	// colours out of selected text and re-rendered it flat.
	if s.selOK && Col(c.Col) >= s.selFrom && Col(c.Col) < s.selTo {
		return style.Bg(s.theme.Palette.SelBg)
	}
	return style
}

// renderRow draws one row into buf at (x, y), occupying exactly row.Width cells.
func renderRow(buf *catatui.Buffer, x, y uint16, row *Row, th *Theme) {
	cursor := x
	right := x + row.Width
	emitRow(row, th, func(text string, style catatui.Style) {
		if cursor < right {
			cursor, _ = buf.SetStringn(cursor, y, text, right-cursor, style)
		}
	})
}
