package main

import (
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
)

var lastWheelTime time.Time

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fileLoadedMsg:
		m.fileBuf = msg.fb
		m.totalLines = m.fileBuf.LineCount()
		m.highlighter = NewHighlighter(m.filePath, m.totalLines)
		m.ready = true
		m.highlightRange = [2]int{-1, -1}
		return m, m.triggerHighlight()

	case errMsg:
		m.err = msg.err
		m.ready = true
		return m, tea.Quit

	case highlightReadyMsg:
		m.ensureHighlighted()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampOffset()
		return m, m.triggerHighlight()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.yOffset > 0 {
				m.yOffset--
				m.clampOffset()
			}
		case "down", "j":
			if m.yOffset < m.totalLines-m.contentHeight() {
				m.yOffset++
				m.clampOffset()
				return m, nil
			}
		case "pgup":
			step := m.contentHeight() / 2
			m.yOffset -= step
			m.clampOffset()
			return m, m.triggerHighlight()
		case "pgdown":
			step := m.contentHeight() / 2
			m.yOffset += step
			m.clampOffset()
			return m, m.triggerHighlight()
		case "home", "g":
			m.yOffset = 0
			m.clampOffset()
			return m, m.triggerHighlight()
		case "end", "G":
			m.yOffset = m.totalLines - m.contentHeight()
			m.clampOffset()
			return m, m.triggerHighlight()
		case "a":
			lastLine, err := m.fileBuf.Line(m.totalLines - 1)
			lastCol := 0
			if err == nil {
				lastCol = visualLineWidth(lastLine, m.tabWidth)
			}
			m.selection.StartRow = 0
			m.selection.StartCol = 0
			m.selection.EndRow = m.totalLines - 1
			m.selection.EndCol = lastCol
			m.selection.Active = true
			m.selection.Selecting = false
			return m, nil
		case "d":
			m.selection.Clear()
			return m, nil
		case "x", "X":
			if m.selection.Active || m.selection.Selecting {
				sr, _, er, _ := m.selection.Bounds()
				m.selection.StartRow = sr
				m.selection.StartCol = 0
				m.selection.EndRow = er
				line, err := m.fileBuf.Line(er)
				if err == nil {
					m.selection.EndCol = visualLineWidth(line, m.tabWidth)
				}
				return m, nil
			}
		case "l":
			m.showLineNum = !m.showLineNum
			return m, nil
		case "s":
			m.showScrollbar = !m.showScrollbar
			return m, nil
		case "h":
			m.highlight = !m.highlight
			if m.highlight {
				return m, m.triggerHighlight()
			}
			return m, nil
		}

	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		row := mouse.Y

		if m.showScrollbar && mouse.X == m.width-1 && row < m.contentHeight() && mouse.Button == tea.MouseLeft {
			m.scrollbarDrag = true
			m.scrollToRow(row)
			return m, m.triggerHighlight()
		}

		col := mouse.X - m.lineNumWidth()
		contentWidth := m.width - m.lineNumWidth()
		if m.showScrollbar {
			contentWidth--
		}
		if row >= m.contentHeight() {
			break
		}
		if col < 0 {
			if mouse.Button == tea.MouseLeft {
				contentRow := m.yOffset + row
				line, err := m.fileBuf.Line(contentRow)
				width := 0
				if err == nil {
					width = visualLineWidth(line, m.tabWidth)
				}
				m.selection.StartRow = contentRow
				m.selection.StartCol = 0
				m.selection.EndRow = contentRow
				m.selection.EndCol = width
				m.selection.Selecting = true
				m.selection.Active = false
				m.gutterSelect = true
				m.lastClickRow = contentRow
				m.lastClickCol = 0
				m.lastClickTime = time.Now()
			}
			break
		}
		if col >= contentWidth {
			break
		}
		contentRow := m.yOffset + row

		if mouse.Button == tea.MouseRight {
			if !m.selection.Active && !m.selection.Selecting {
				m.selection.StartRow = contentRow
				m.selection.StartCol = col
				m.selection.EndRow = contentRow
				m.selection.EndCol = col
			} else {
				sr, sc, er, ec := m.selection.Bounds()
				nearStart := (contentRow-sr)*(contentRow-sr)+(col-sc)*(col-sc) <= (contentRow-er)*(contentRow-er)+(col-ec)*(col-ec)
				if nearStart {
					m.selection.StartRow = contentRow
					m.selection.StartCol = col
				} else {
					m.selection.EndRow = contentRow
					m.selection.EndCol = col
				}
			}
			m.selection.Selecting = false
			m.selection.Active = true
			break
		}

		if mouse.Button == tea.MouseLeft {
			now := time.Now()
			if contentRow == m.lastClickRow && col == m.lastClickCol && now.Sub(m.lastClickTime) < 500*time.Millisecond {
				line, err := m.fileBuf.Line(contentRow)
				if err == nil {
					start, end := findWordBounds(line, col, m.tabWidth)
					if start < end {
						m.selection.Begin(contentRow, start)
						m.selection.Extend(contentRow, end)
						m.selection.End()
					}
				}
				m.lastClickTime = time.Time{}
			} else {
				m.lastClickRow = contentRow
				m.lastClickCol = col
				m.lastClickTime = now
				m.selection.Begin(contentRow, col)
			}
		}

	case tea.MouseMotionMsg:
		if m.scrollbarDrag {
			m.scrollToRow(msg.Mouse().Y)
			return m, m.triggerHighlight()
		}
		if m.selection.Selecting {
			mouse := msg.Mouse()
			row := mouse.Y
			col := mouse.X - m.lineNumWidth()
			contentWidth := m.width - m.lineNumWidth()
			if m.showScrollbar {
				contentWidth--
			}
			if row >= m.contentHeight() {
				row = m.contentHeight() - 1
			}
			if row < 0 {
				row = 0
			}
			contentRow := m.yOffset + row

			if m.gutterSelect {
				sr, _, _, _ := m.selection.Bounds()
				if contentRow < sr {
					m.selection.StartRow = contentRow
					m.selection.StartCol = 0
				} else {
					line, err := m.fileBuf.Line(contentRow)
					width := 0
					if err == nil {
						width = visualLineWidth(line, m.tabWidth)
					}
					m.selection.EndRow = contentRow
					m.selection.EndCol = width
				}
				break
			}

			if col > contentWidth {
				col = contentWidth
			}
			if col < 0 {
				col = 0
			}
			m.selection.Extend(contentRow, col)
		}

	case tea.MouseReleaseMsg:
		m.scrollbarDrag = false
		m.gutterSelect = false
		m.selection.End()

	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		switch mouse.Button {
		case tea.MouseWheelUp:
			step := 3
			m.yOffset -= step
			m.clampOffset()
		case tea.MouseWheelDown:
			step := 3
			m.yOffset += step
			m.clampOffset()
		}
		lastWheelTime = time.Now()
		if !m.hlCoversVisible() {
			return m, m.debouncedHighlight()
		}
	}

	return m, nil
}

func (m Model) hlCoversVisible() bool {
	from, to := m.visibleLineRange()
	return m.highlightRange[0] <= from && m.highlightRange[1] >= to
}

func (m *Model) debouncedHighlight() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(80 * time.Millisecond)
		if time.Since(lastWheelTime) < 80*time.Millisecond {
			return nil
		}
		m.ensureHighlighted()
		return highlightReadyMsg{}
	}
}

func (m *Model) clampOffset() {
	maxOffset := m.totalLines - m.contentHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.yOffset < 0 {
		m.yOffset = 0
	}
	if m.yOffset > maxOffset {
		m.yOffset = maxOffset
	}
}

func (m *Model) scrollToRow(mouseY int) {
	if m.totalLines <= m.contentHeight() {
		return
	}
	maxOffset := m.totalLines - m.contentHeight()
	m.yOffset = mouseY * maxOffset / (m.contentHeight() - 1)
	m.clampOffset()
}

func (m *Model) ensureHighlighted() {
	if !m.highlight {
		return
	}
	from, to := m.visibleLineRange()
	if from >= to {
		return
	}
	if m.highlightRange[0] <= from && m.highlightRange[1] >= to {
		return
	}
	ctxFrom := from - contextLines
	if ctxFrom < 0 {
		ctxFrom = 0
	}
	ctxTo := to + contextLines
	if ctxTo > m.totalLines {
		ctxTo = m.totalLines
	}
	text, err := m.fileBuf.Text(ctxFrom, ctxTo)
	if err != nil {
		return
	}
	m.highlighter.HighlightRange(text, ctxFrom)
	m.highlightRange = [2]int{ctxFrom, ctxTo}
}

func (m *Model) triggerHighlight() tea.Cmd {
	if !m.highlight {
		return nil
	}
	return func() tea.Msg {
		m.ensureHighlighted()
		return highlightReadyMsg{}
	}
}

func (m *Model) copySelection() tea.Cmd {
	allLines, err := m.fileBuf.Lines(0, m.totalLines)
	if err != nil {
		return nil
	}

	sr, sc, er, ec := m.selection.Bounds()

	if sr < len(allLines) {
		sc = visualToRawCol(allLines[sr], sc, m.tabWidth)
	}
	if er < len(allLines) {
		ec = visualToRawCol(allLines[er], ec, m.tabWidth)
	}

	text := extractText(allLines, sr, sc, er, ec)
	if text == "" {
		return nil
	}
	return tea.SetClipboard(text)
}

func visualToRawCol(line string, visualCol int, tabWidth int) int {
	raw := 0
	vis := 0
	for _, ch := range line {
		if vis >= visualCol {
			return raw
		}
		if ch == '\t' {
			tabStop := tabWidth - (vis % tabWidth)
			if vis+tabStop > visualCol {
				return raw
			}
			vis += tabStop
		} else {
			vis++
		}
		raw++
	}
	return raw
}

func visualLineWidth(line string, tabWidth int) int {
	col := 0
	for _, ch := range line {
		if ch == '\t' {
			col += tabWidth - (col % tabWidth)
		} else {
			col++
		}
	}
	return col
}

func findWordBounds(line string, col int, tabWidth int) (int, int) {
	expanded := expandTabs(line, tabWidth)
	if col >= len(expanded) {
		col = len(expanded)
		if col == 0 {
			return 0, 0
		}
		col--
	}
	if col < 0 {
		return 0, 0
	}

	if !isWordChar(rune(expanded[col])) {
		return col, col
	}

	start := col
	for start > 0 && isWordChar(rune(expanded[start-1])) {
		start--
	}
	end := col
	for end < len(expanded) && isWordChar(rune(expanded[end])) {
		end++
	}
	return start, end
}

func isWordChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
