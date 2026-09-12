// The document pane: gutter, text, scrollbar.
package main

import "github.com/Fiend3d/catatui"

// thumb is the scrollbar's filled block.
const thumb = "█"

func renderViewer(buf *catatui.Buffer, a *App, th *Theme) {
	l := a.Layout()
	from, to := a.VisibleRange()

	for screenRow := uint16(0); screenRow < l.ContentH; screenRow++ {
		y := screenRow
		lineNo := from + int(screenRow)

		if lineNo >= to {
			// Past the end of the file: an empty row, gutter included.
			fill(buf, 0, y, l.Gutter+l.ContentW, th.Background())
		} else {
			line, table := a.LineTable(lineNo)
			selFrom, selTo, selOK := a.Sel.RowSpan(lineNo, table.Width())

			if l.Gutter > 0 {
				style := th.LineNum()
				if selOK {
					style = th.LineNumSelected()
				}
				if a.ShowLineNum {
					renderGutter(buf, y, l.Gutter, lineNo+1, style)
				}
				renderMarker(buf, l.Gutter-1, y, a.GitChangeAt(lineNo), th)
			}

			var runs []StyleRun
			if a.Highlight {
				runs = a.Hl.Get(lineNo)
			}
			renderRow(buf, l.ContentX, y, &Row{
				Line:    line,
				Table:   table,
				Runs:    runs,
				SelFrom: selFrom,
				SelTo:   selTo,
				SelOK:   selOK,
				XOff:    a.XOff,
				Width:   l.ContentW,
			}, th)
		}

		if l.HasScrollbar {
			sym := scrollbarSymbol(int(screenRow), int(l.ContentH), from, a.TotalLines)
			buf.SetStringn(l.ScrollbarX, y, sym, 1, th.Scrollbar())
		}
	}
}

// digitChars is sliced one byte at a time to name a digit, so writing a line
// number costs no allocation: a one-byte substring of a constant is not a copy,
// where strconv.Itoa would build a fresh string per row per frame.
const digitChars = "0123456789"

// renderGutter writes a right-aligned line number followed by one space.
func renderGutter(buf *catatui.Buffer, y, width uint16, n int, style catatui.Style) {
	if width == 0 {
		return
	}
	// The number field is the gutter minus its trailing space.
	field := int(width) - 1
	nd := digits(n)

	x := uint16(0)
	if pad := field - nd; pad > 0 {
		x, _ = buf.SetStringn(x, y, spaces[:min(pad, len(spaces))], width, style)
	}
	// Emit digits most significant first, without materialising the number.
	div := 1
	for i := 1; i < nd; i++ {
		div *= 10
	}
	for ; div > 0 && x < width; div /= 10 {
		d := (n / div) % 10
		x, _ = buf.SetStringn(x, y, digitChars[d:d+1], width-x, style)
	}
	if x < width {
		buf.SetStringn(x, y, " ", width-x, style)
	}
}

// Change marker glyphs. They are constants, so drawing one allocates nothing.
const (
	markerChanged      = "▎"
	markerRemovedBelow = "▁"
	markerRemovedAbove = "▔"
)

// renderMarker draws a git change marker in the gutter's last cell, the one
// renderGutter leaves blank after the number. No change draws that blank, which
// is also what fills a marker-only gutter when line numbers are hidden.
func renderMarker(buf *catatui.Buffer, x, y uint16, kind ChangeKind, th *Theme) {
	sym := " "
	switch kind {
	case ChangeAdded, ChangeModified:
		sym = markerChanged
	case ChangeRemovedBelow:
		sym = markerRemovedBelow
	case ChangeRemovedAbove:
		sym = markerRemovedAbove
	}
	buf.SetStringn(x, y, sym, 1, th.GitMarker(kind))
}

// scrollbarSymbol reports whether the thumb covers this row.
func scrollbarSymbol(row, height, yOff, totalLines int) string {
	if totalLines <= height || height == 0 {
		return " "
	}
	maxOffset := max(totalLines-height, 1)
	thumbH := max(height*height/totalLines, 1)
	thumbPos := yOff * (height - thumbH) / maxOffset
	if row >= thumbPos && row < thumbPos+thumbH {
		return thumb
	}
	return " "
}

// fill paints width cells of blank background at (x, y).
func fill(buf *catatui.Buffer, x, y, width uint16, style catatui.Style) {
	for left := int(width); left > 0; {
		take := min(left, len(spaces))
		x, _ = buf.SetStringn(x, y, spaces[:take], uint16(left), style)
		left -= take
	}
}
