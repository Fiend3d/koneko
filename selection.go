package main

import "strings"

// SelectMode is the granularity a selection grows by while it is being
// extended: single characters, whole words, or whole lines.
type SelectMode int

const (
	SelectChar SelectMode = iota
	SelectWord
	SelectLine
)

// Selection keeps Start before End in document order at all times. Extending a
// selection unions the anchor range with the range being dragged to, so the
// anchor end stays pinned no matter which direction the mouse moves.
type Selection struct {
	StartRow, StartCol int
	EndRow, EndCol     int
	Active             bool
	Selecting          bool
	Mode               SelectMode

	anchorSR, anchorSC int
	anchorER, anchorEC int
}

// posLess reports whether (r1,c1) comes before (r2,c2) in document order.
func posLess(r1, c1, r2, c2 int) bool {
	if r1 != r2 {
		return r1 < r2
	}
	return c1 < c2
}

// nearer reports whether (ar,ac) is closer to (r,c) than (br,bc) is, comparing
// row distance first so a click stays attached to the end on its own side.
func nearer(r, c, ar, ac, br, bc int) bool {
	dar, dac := iabs(ar-r), iabs(ac-c)
	dbr, dbc := iabs(br-r), iabs(bc-c)
	if dar != dbr {
		return dar < dbr
	}
	return dac < dbc
}

func iabs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// BeginRange starts a fresh selection anchored to the given range.
func (s *Selection) BeginRange(sr, sc, er, ec int) {
	if posLess(er, ec, sr, sc) {
		sr, sc, er, ec = er, ec, sr, sc
	}
	s.anchorSR, s.anchorSC = sr, sc
	s.anchorER, s.anchorEC = er, ec
	s.StartRow, s.StartCol = sr, sc
	s.EndRow, s.EndCol = er, ec
	s.Selecting = true
	s.Active = false
	s.Mode = SelectChar
}

// Begin starts a fresh empty selection at a single position.
func (s *Selection) Begin(row, col int) {
	s.BeginRange(row, col, row, col)
}

// ExtendRange grows the selection so it spans both the anchor range and the
// given range. When no drag is in progress it first re-anchors to whichever end
// of the existing selection is farther away, so the near end is the one that
// moves — this is what makes right-click extend feel right.
func (s *Selection) ExtendRange(sr, sc, er, ec int) {
	if posLess(er, ec, sr, sc) {
		sr, sc, er, ec = er, ec, sr, sc
	}
	if !s.Selecting {
		if !s.Active {
			s.BeginRange(sr, sc, er, ec)
			return
		}
		s.reanchorAwayFrom(sr, sc, er, ec)
		s.Selecting = true
	}

	s.StartRow, s.StartCol = s.anchorSR, s.anchorSC
	s.EndRow, s.EndCol = s.anchorER, s.anchorEC
	if posLess(sr, sc, s.StartRow, s.StartCol) {
		s.StartRow, s.StartCol = sr, sc
	}
	if posLess(s.EndRow, s.EndCol, er, ec) {
		s.EndRow, s.EndCol = er, ec
	}
}

// Extend grows the selection to a single position.
func (s *Selection) Extend(row, col int) {
	s.ExtendRange(row, col, row, col)
}

// reanchorAwayFrom pins the end of the current selection that is farther from
// the incoming range, collapsing the anchor to that single position so the
// selection can shrink as well as grow.
func (s *Selection) reanchorAwayFrom(sr, sc, er, ec int) {
	if nearer(sr, sc, s.StartRow, s.StartCol, s.EndRow, s.EndCol) {
		s.anchorSR, s.anchorSC = s.EndRow, s.EndCol
		s.anchorER, s.anchorEC = s.EndRow, s.EndCol
	} else {
		s.anchorSR, s.anchorSC = s.StartRow, s.StartCol
		s.anchorER, s.anchorEC = s.StartRow, s.StartCol
	}
}

func (s *Selection) End() {
	if !s.Selecting {
		return
	}
	s.Selecting = false
	s.Active = s.StartRow != s.EndRow || s.StartCol != s.EndCol
}

func (s *Selection) Bounds() (sr, sc, er, ec int) {
	return s.StartRow, s.StartCol, s.EndRow, s.EndCol
}

func extractText(lines []string, sr, sc, er, ec int) string {
	var b strings.Builder
	if sr == er {
		if sr < len(lines) {
			line := lines[sr]
			if sc < len(line) {
				end := min(ec, len(line))
				if sc > end {
					sc, end = end, sc
				}
				b.WriteString(line[sc:end])
			}
		}
	} else {
		for i := sr; i <= er; i++ {
			if i >= len(lines) {
				break
			}
			line := lines[i]
			switch i {
			case sr:
				if sc < len(line) {
					b.WriteString(line[sc:])
				}
			case er:
				end := max(min(ec, len(line)), 0)
				if end > 0 {
					b.WriteString(line[:end])
				}
			default:
				b.WriteString(line)
			}
			if i < er {
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func (s *Selection) Clear() {
	s.Active = false
	s.Selecting = false
	s.Mode = SelectChar
}
