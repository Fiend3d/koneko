package main

import "strings"

type Selection struct {
	StartRow, StartCol int
	EndRow, EndCol     int
	Active             bool
	Selecting          bool
	anchorIsStart      bool
}

func (s *Selection) Begin(row, col int) {
	s.StartRow = row
	s.StartCol = col
	s.EndRow = row
	s.EndCol = col
	s.Selecting = true
	s.Active = false
	s.anchorIsStart = false
}

func (s *Selection) Extend(row, col int) {
	if s.Selecting {
		if s.anchorIsStart {
			s.StartRow = row
			s.StartCol = col
		} else {
			s.EndRow = row
			s.EndCol = col
		}
		return
	}
	if !s.Active {
		s.Begin(row, col)
		return
	}
	sr, sc, _, _ := s.Bounds()
	if row < sr || (row == sr && col < sc) {
		s.StartRow = row
		s.StartCol = col
		s.anchorIsStart = true
	} else {
		s.EndRow = row
		s.EndCol = col
		s.anchorIsStart = false
	}
	s.Selecting = true
}

func (s *Selection) End() {
	if !s.Selecting {
		return
	}
	s.Selecting = false
	if s.StartRow == s.EndRow && s.StartCol == s.EndCol {
		s.Active = false
		return
	}
	s.Active = true
}

func (s *Selection) Bounds() (sr, sc, er, ec int) {
	sr, sc = s.StartRow, s.StartCol
	er, ec = s.EndRow, s.EndCol
	if sr > er || (sr == er && sc > ec) {
		sr, er = er, sr
		sc, ec = ec, sc
	}
	return
}

func extractText(lines []string, sr, sc, er, ec int) string {
	var b strings.Builder
	if sr == er {
		if sr < len(lines) {
			line := lines[sr]
			if sc < len(line) {
				end := ec
				if end > len(line) {
					end = len(line)
				}
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
				end := ec
				if end > len(line) {
					end = len(line)
				}
				if end < 0 {
					end = 0
				}
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
}
