package main

// SelectMode is the granularity a selection grows by while it is being
// extended: single characters, whole words, or whole lines.
type SelectMode int

const (
	SelectChar SelectMode = iota
	SelectWord
	SelectLine
)

// Pos is a position in the document: a line index and a display column.
type Pos struct {
	Row int
	Col Col
}

// Less reports whether p comes before q in document order.
func (p Pos) Less(q Pos) bool {
	if p.Row != q.Row {
		return p.Row < q.Row
	}
	return p.Col < q.Col
}

// Selection keeps Start before End in document order at all times. Extending a
// selection unions the anchor range with the range being dragged to, so the
// anchor end stays pinned no matter which direction the mouse moves.
type Selection struct {
	Start, End Pos
	Active     bool
	Selecting  bool
	Mode       SelectMode

	anchorStart, anchorEnd Pos
}

// IsVisible reports whether there is a selection worth drawing.
func (s *Selection) IsVisible() bool { return s.Active || s.Selecting }

// BeginRange starts a fresh selection anchored to the given range.
func (s *Selection) BeginRange(start, end Pos) {
	if end.Less(start) {
		start, end = end, start
	}
	s.anchorStart, s.anchorEnd = start, end
	s.Start, s.End = start, end
	s.Selecting = true
	s.Active = false
	s.Mode = SelectChar
}

// Begin starts a fresh empty selection at a single position.
func (s *Selection) Begin(p Pos) { s.BeginRange(p, p) }

// ExtendRange grows the selection so it spans both the anchor range and the
// given range. When no drag is in progress it first re-anchors to whichever end
// of the existing selection is farther away, so the near end is the one that
// moves — this is what makes right-click extend feel right.
func (s *Selection) ExtendRange(start, end Pos) {
	if end.Less(start) {
		start, end = end, start
	}
	if !s.Selecting {
		if !s.Active {
			s.BeginRange(start, end)
			return
		}
		s.reanchorAwayFrom(start)
		s.Selecting = true
	}

	s.Start, s.End = s.anchorStart, s.anchorEnd
	if start.Less(s.Start) {
		s.Start = start
	}
	if s.End.Less(end) {
		s.End = end
	}
}

// Extend grows the selection to a single position.
func (s *Selection) Extend(p Pos) { s.ExtendRange(p, p) }

// reanchorAwayFrom pins the end of the current selection that is farther from
// p, collapsing the anchor to that single position so the selection can shrink
// as well as grow.
func (s *Selection) reanchorAwayFrom(p Pos) {
	far := s.Start
	if nearer(p, s.Start, s.End) {
		far = s.End
	}
	s.anchorStart, s.anchorEnd = far, far
}

// nearer reports whether a is closer to p than b is, comparing row distance
// first so a click stays attached to the end on its own side.
func nearer(p, a, b Pos) bool {
	dar, dac := iabs(a.Row-p.Row), iabs(int(a.Col-p.Col))
	dbr, dbc := iabs(b.Row-p.Row), iabs(int(b.Col-p.Col))
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

// Finish ends a drag. A selection that never moved off its anchor is discarded,
// so a plain click does not leave a zero-width selection behind.
func (s *Selection) Finish() {
	if !s.Selecting {
		return
	}
	s.Selecting = false
	s.Active = s.Start != s.End
}

func (s *Selection) Clear() {
	s.Active = false
	s.Selecting = false
	s.Mode = SelectChar
}

// RowSpan is the half-open column range to paint on line row, given that line's
// display width, or ok=false when the row is outside the selection.
//
// Every row but the last carries one extra column past its text: that cell
// stands for the newline the selection includes, and on an empty line inside a
// multi-line selection it is the only thing marking the row as selected at all.
func (s *Selection) RowSpan(row int, lineWidth Col) (from, to Col, ok bool) {
	if !s.IsVisible() || row < s.Start.Row || row > s.End.Row {
		return 0, 0, false
	}

	from = 0
	if row == s.Start.Row {
		from = s.Start.Col
	}

	if row == s.End.Row {
		to = s.End.Col
	} else {
		// Not the last row, so the selection runs through the line ending.
		to = lineWidth + 1
	}

	if to > lineWidth+1 {
		to = lineWidth + 1
	}
	if from >= to {
		return 0, 0, false
	}
	return from, to, true
}
