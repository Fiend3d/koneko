package main

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fileLoadedMsg:
		m.fileBuf = msg.fb
		m.totalLines = m.fileBuf.LineCount()
		m.highlighter = NewHighlighter(m.filePath, m.totalLines, theme.TokenStyles, m.tabWidth)
		m.ready = true
		m.highlightRange = [2]int{-1, -1}

		if m.hasInitSelect {
			allLines, err := m.fileBuf.Lines(0, m.totalLines)
			if err == nil {
				sr, sc, er, ec := m.initSelSR, m.initSelSC, m.initSelER, m.initSelEC
				if sr < len(allLines) {
					sc = rawToVisualCol(allLines[sr], sc, m.tabWidth)
				}
				if er < len(allLines) {
					ec = rawToVisualCol(allLines[er], ec, m.tabWidth)
				}
				m.selection.Begin(sr, sc)
				m.selection.Extend(er, ec)
				m.selection.End()
				m.scrollToShowMatch(m.initSelSR)

				visSr, visSc, _, _ := m.selection.Bounds()
				sr, sc = visSr, visSc
				if sr < len(allLines) {
					sc = visualToRawCol(allLines[sr], sc, m.tabWidth)
				}
				if er < len(allLines) {
					ec = visualToRawCol(allLines[er], ec, m.tabWidth)
				}
				m.searchStr = extractText(allLines, sr, sc, er, ec)
				if m.searchStr != "" {
					m.populateMatchLines()
					for i, ml := range m.matchLines {
						if ml[0] == visSr && ml[1] == visSc {
							m.matchIdx = i
							break
						}
					}
				}
			}
		}
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
		if m.searchMode {
			switch msg.String() {
			case "enter":
				m.searchStr = m.searchInput.Value()
				m.searchInput.Blur()
				m.searchMode = false
				m.matchLines = nil
				m.matchIdx = 0
				if m.searchStr != "" && m.fileBuf != nil {
					m.populateMatchLines()
					if len(m.matchLines) > 0 {
						m.matchIdx = 0
						if m.yOffset > 0 {
							for i, ml := range m.matchLines {
								if ml[0] >= m.yOffset {
									m.matchIdx = i
									break
								}
							}
						}
						row, col := m.matchLines[m.matchIdx][0], m.matchLines[m.matchIdx][1]
						m.selection.Clear()
						m.selection.Begin(row, col)
						m.selection.Extend(row, col+ansi.StringWidth(m.searchStr))
						m.selection.End()
						m.scrollToShowMatch(row)
						return m, m.triggerHighlight()
					}
				}
				return m, nil
			case "esc":
				m.searchInput.Blur()
				m.searchInput.Reset()
				m.searchMode = false
				m.matchLines = nil
				m.matchIdx = 0
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd
			}
		}

		if m.helpMode {
			switch msg.String() {
			case "f1", "esc":
				m.helpMode = false
				return m, nil
			case "up", "k":
				if m.helpOffset > 0 {
					m.helpOffset--
				}
				return m, nil
			case "down", "j":
				maxOff := len(helpLines) - m.contentHeight()
				if maxOff < 0 {
					maxOff = 0
				}
				if m.helpOffset < maxOff {
					m.helpOffset++
				}
				return m, nil
			case "pgup":
				m.helpOffset -= m.contentHeight() / 2
				if m.helpOffset < 0 {
					m.helpOffset = 0
				}
				return m, nil
			case "pgdown":
				m.helpOffset += m.contentHeight() / 2
				maxOff := len(helpLines) - m.contentHeight()
				if maxOff < 0 {
					maxOff = 0
				}
				if m.helpOffset > maxOff {
					m.helpOffset = maxOff
				}
				return m, nil
			case "home", "g":
				m.helpOffset = 0
				return m, nil
			case "end", "G":
				m.helpOffset = len(helpLines) - m.contentHeight()
				if m.helpOffset < 0 {
					m.helpOffset = 0
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			if m.fileBuf != nil {
				m.fileBuf.Close()
			}
			return m, tea.Quit
		case "up", "k":
			if m.yOffset > 0 {
				m.yOffset--
				m.clampOffset()
				return m, m.triggerHighlight()
			}
			return m, nil
		case "down", "j":
			if m.yOffset < m.totalLines-m.contentHeight() {
				m.yOffset++
				m.clampOffset()
				return m, m.triggerHighlight()
			}
			return m, nil
		case "left":
			if m.xOffset > 0 {
				m.xOffset--
			}
			return m, nil
		case "right":
			m.xOffset++
			return m, nil
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
			m.selection.Begin(0, 0)
			m.selection.Extend(m.totalLines-1, lastCol)
			m.selection.End()
			return m, nil
		case "d":
			m.selection.Clear()
			return m, nil
		case "y":
			if m.selection.Active {
				return m, m.copySelection()
			}
			return m, nil
		case "x", "X":
			if m.selection.Active || m.selection.Selecting {
				sr, _, er, _ := m.selection.Bounds()
				m.selection.Begin(sr, 0)
				line, err := m.fileBuf.Line(er)
				if err == nil {
					m.selection.Extend(er, visualLineWidth(line, m.tabWidth))
				}
				m.selection.End()
				return m, nil
			}
			return m, nil
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
		case "H":
			m.xOffset = 0
			return m, nil
		case "f1":
			m.helpMode = true
			m.helpOffset = 0
			return m, nil
		case "/":
			m.searchMode = true
			m.searchInput.SetValue(m.searchStr)
			m.searchInput.SetWidth(m.width - 2)
			cmd := m.searchInput.Focus()
			return m, cmd
		case "n":
			if len(m.matchLines) == 0 {
				return m, nil
			}
			m.matchIdx++
			if m.matchIdx >= len(m.matchLines) {
				m.matchIdx = 0
			}
			row, col := m.matchLines[m.matchIdx][0], m.matchLines[m.matchIdx][1]
			m.selection.Clear()
			m.selection.Begin(row, col)
			m.selection.Extend(row, col+ansi.StringWidth(m.searchStr))
			m.selection.End()
			m.scrollToShowMatch(row)
			return m, m.triggerHighlight()
		case "N":
			if len(m.matchLines) == 0 {
				return m, nil
			}
			m.matchIdx--
			if m.matchIdx < 0 {
				m.matchIdx = len(m.matchLines) - 1
			}
			row, col := m.matchLines[m.matchIdx][0], m.matchLines[m.matchIdx][1]
			m.selection.Clear()
			m.selection.Begin(row, col)
			m.selection.Extend(row, col+ansi.StringWidth(m.searchStr))
			m.selection.End()
			m.scrollToShowMatch(row)
			return m, m.triggerHighlight()
		}

	case tea.MouseClickMsg:
		if m.helpMode {
			return m, nil
		}
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
			return m, nil
		}

		if col < 0 {
			contentRow := m.yOffset + row
			if contentRow >= m.totalLines {
				return m, nil
			}

			if mouse.Button == tea.MouseLeft {
				line, err := m.fileBuf.Line(contentRow)
				width := 0
				if err == nil {
					width = visualLineWidth(line, m.tabWidth)
				}
				m.selection.Begin(contentRow, 0)
				m.selection.EndRow = contentRow
				m.selection.EndCol = width
				m.selection.Selecting = true
				m.selection.Active = false
				m.gutterSelect = true
				m.gutterAnchor = contentRow
				m.lastClickRow = contentRow
				m.lastClickCol = 0
				m.lastClickTime = time.Now()
			}

			if mouse.Button == tea.MouseRight {
				line, err := m.fileBuf.Line(contentRow)
				width := 0
				if err == nil {
					width = visualLineWidth(line, m.tabWidth)
				}
				if m.selection.Active || m.selection.Selecting {
					sr, _, er, _ := m.selection.Bounds()
					if contentRow < sr {
						m.selection.StartRow = contentRow
						m.selection.StartCol = 0
					} else if contentRow > er {
						m.selection.EndRow = contentRow
						m.selection.EndCol = width
					} else {
						m.selection.EndRow = contentRow
						m.selection.EndCol = width
					}
				} else {
					m.selection.Begin(contentRow, 0)
					m.selection.EndRow = contentRow
					m.selection.EndCol = width
				}
				m.selection.Selecting = false
				m.selection.Active = true
				return m, nil
			}
			return m, nil
		}

		if col >= contentWidth {
			return m, nil
		}

		contentRow := m.yOffset + row
		if contentRow >= m.totalLines {
			return m, nil
		}
		contentCol := col + m.xOffset

		if mouse.Button == tea.MouseRight {
			if !m.selection.Active && !m.selection.Selecting {
				m.selection.Begin(contentRow, contentCol)
			} else {
				m.selection.Extend(contentRow, contentCol)
			}
			m.selection.End()
			return m, nil
		}

		if mouse.Button == tea.MouseLeft {
			now := time.Now()
			if contentRow == m.lastClickRow && contentCol == m.lastClickCol && now.Sub(m.lastClickTime) < 500*time.Millisecond {
				line, err := m.fileBuf.Line(contentRow)
				if err == nil {
					start, end := findWordBounds(line, contentCol, m.tabWidth)
					if start < end {
						m.selection.Begin(contentRow, start)
						m.selection.Extend(contentRow, end)
						m.selection.End()
					}
				}
				m.lastClickTime = time.Time{}
			} else {
				m.lastClickRow = contentRow
				m.lastClickCol = contentCol
				m.lastClickTime = now
				m.selection.Begin(contentRow, contentCol)
			}
		}

	case tea.MouseMotionMsg:
		if m.helpMode {
			return m, nil
		}
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
			if m.totalLines > 0 {
				contentRow = min(contentRow, m.totalLines-1)
			}

			if m.gutterSelect {
				if contentRow < m.gutterAnchor {
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
			contentCol := col + m.xOffset
			m.selection.Extend(contentRow, contentCol)
		}

	case tea.MouseReleaseMsg:
		if m.helpMode {
			return m, nil
		}
		m.scrollbarDrag = false
		m.gutterSelect = false
		m.selection.End()

	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		if m.helpMode {
			switch mouse.Button {
			case tea.MouseWheelUp:
				m.helpOffset -= 3
			case tea.MouseWheelDown:
				m.helpOffset += 3
			}
			if m.helpOffset < 0 {
				m.helpOffset = 0
			}
			maxOff := len(helpLines) - m.contentHeight()
			if maxOff < 0 {
				maxOff = 0
			}
			if m.helpOffset > maxOff {
				m.helpOffset = maxOff
			}
			return m, nil
		}

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
		m.lastWheelTime = time.Now()
		if !m.hlCoversVisible() {
			return m, m.debouncedHighlight()
		}
	}

	if m.searchMode {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) hlCoversVisible() bool {
	from, to := m.visibleLineRange()
	return m.highlightRange[0] <= from && m.highlightRange[1] >= to
}

func (m *Model) debouncedHighlight() tea.Cmd {
	from, to := m.visibleLineRange()
	if from >= to {
		return nil
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
		return nil
	}
	return func() tea.Msg {
		time.Sleep(80 * time.Millisecond)
		if time.Since(m.lastWheelTime) < 80*time.Millisecond {
			return nil
		}
		m.highlighter.HighlightRange(text, ctxFrom)
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
	from, to := m.visibleLineRange()
	if from >= to {
		return nil
	}
	if m.highlightRange[0] <= from && m.highlightRange[1] >= to {
		return nil
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
		return nil
	}
	return func() tea.Msg {
		m.highlighter.HighlightRange(text, ctxFrom)
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

func rawToVisualCol(line string, rawCol int, tabWidth int) int {
	vis := 0
	bytePos := 0
	rest := line
	for len(rest) > 0 {
		if bytePos >= rawCol {
			return vis
		}
		cluster, w := ansi.FirstGraphemeCluster(rest, ansi.GraphemeWidth)
		if len(cluster) == 0 {
			break
		}
		if cluster == "\t" {
			vis += tabWidth - (vis % tabWidth)
		} else {
			vis += w
		}
		bytePos += len(cluster)
		rest = rest[len(cluster):]
	}
	return vis
}

func visualToRawCol(line string, visualCol int, tabWidth int) int {
	vis := 0
	byteStart := 0
	rest := line
	for len(rest) > 0 {
		cluster, w := ansi.FirstGraphemeCluster(rest, ansi.GraphemeWidth)
		if len(cluster) == 0 {
			break
		}
		if cluster == "\t" {
			tabStop := tabWidth - (vis % tabWidth)
			if vis+tabStop > visualCol {
				return byteStart
			}
			vis += tabStop
		} else {
			if vis+w > visualCol {
				return byteStart
			}
			vis += w
		}
		byteStart += len(cluster)
		rest = rest[len(cluster):]
	}
	return byteStart
}

func (m *Model) findNext(fromRow, fromCol int) (int, int, bool) {
	allLines, err := m.fileBuf.Lines(0, m.totalLines)
	if err != nil || m.searchStr == "" {
		return 0, 0, false
	}
	searchLower := strings.ToLower(m.searchStr)
	for row := fromRow; row < len(allLines); row++ {
		line := expandTabs(allLines[row], m.tabWidth)
		searchIn := line
		offset := 0
		if row == fromRow {
			if fromCol >= len(line) {
				continue
			}
			searchIn = line[fromCol:]
			offset = fromCol
		}
		lineLower := strings.ToLower(searchIn)
		idx := strings.Index(lineLower, searchLower)
		if idx != -1 {
			return row, offset + idx, true
		}
	}
	return 0, 0, false
}

func (m *Model) findPrev(fromRow, fromCol int) (int, int, bool) {
	if fromCol < 0 {
		fromCol = 0
	}
	allLines, err := m.fileBuf.Lines(0, m.totalLines)
	if err != nil || m.searchStr == "" {
		return 0, 0, false
	}
	searchLower := strings.ToLower(m.searchStr)
	for row := fromRow; row >= 0; row-- {
		line := expandTabs(allLines[row], m.tabWidth)
		searchIn := line
		if row == fromRow {
			if fromCol > len(line) {
				fromCol = len(line)
			}
			searchIn = line[:fromCol]
		}
		lineLower := strings.ToLower(searchIn)
		idx := strings.LastIndex(lineLower, searchLower)
		if idx != -1 {
			return row, idx, true
		}
	}
	return 0, 0, false
}

func (m *Model) scrollToShowMatch(row int) {
	targetY := row - m.contentHeight()/3
	if targetY < 0 {
		targetY = 0
	}
	m.yOffset = targetY
	m.clampOffset()
}

func (m *Model) populateMatchLines() {
	m.matchLines = nil
	if m.searchStr == "" {
		return
	}
	allLines, err := m.fileBuf.Lines(0, m.totalLines)
	if err != nil {
		return
	}
	searchLower := strings.ToLower(m.searchStr)
	for row, line := range allLines {
		expanded := expandTabs(line, m.tabWidth)
		lineLower := strings.ToLower(expanded)
		start := 0
		for {
			idx := strings.Index(lineLower[start:], searchLower)
			if idx == -1 {
				break
			}
			col := start + idx
			m.matchLines = append(m.matchLines, [2]int{row, col})
			start = col + len(searchLower)
		}
	}
}

func visualLineWidth(line string, tabWidth int) int {
	col := 0
	rest := line
	for len(rest) > 0 {
		cluster, w := ansi.FirstGraphemeCluster(rest, ansi.GraphemeWidth)
		if len(cluster) == 0 {
			break
		}
		if cluster == "\t" {
			col += tabWidth - (col % tabWidth)
		} else {
			col += w
		}
		rest = rest[len(cluster):]
	}
	return col
}

func findWordBounds(line string, col int, tabWidth int) (int, int) {
	expanded := ansi.Strip(expandTabs(line, tabWidth))
	if col < 0 {
		return 0, 0
	}
	totalVis := ansi.StringWidth(expanded)
	if col >= totalVis {
		col = totalVis
		if col == 0 {
			return 0, 0
		}
		col--
	}
	bytePos := visualToByte(expanded, col)
	if bytePos >= len(expanded) {
		bytePos = len(expanded)
	}
	r, _ := utf8.DecodeLastRuneInString(expanded[:bytePos])
	if !isWordChar(r) {
		return col, col
	}
	start := col
	startByte := bytePos
	for startByte > 0 {
		r, size := utf8.DecodeLastRuneInString(expanded[:startByte])
		if !isWordChar(r) {
			break
		}
		_, w := ansi.FirstGraphemeCluster(expanded[startByte-size:startByte], ansi.GraphemeWidth)
		start -= w
		startByte -= size
	}
	if start < 0 {
		start = 0
	}
	end := col
	endByte := bytePos
	for endByte < len(expanded) {
		r, size := utf8.DecodeRuneInString(expanded[endByte:])
		if !isWordChar(r) {
			break
		}
		_, w := ansi.FirstGraphemeCluster(expanded[endByte:endByte+size], ansi.GraphemeWidth)
		end += w
		endByte += size
	}
	return start, end
}

// isWordChar reports whether r should be treated as part of a "word" for
// double-click selection purposes.
//
// FIX: Devanagari (and most other Indic-script) dependent vowel signs
// (matras) and the virama are Unicode *combining marks* (category Mn/Mc),
// not letters, so unicode.IsLetter alone returns false for them. That
// caused findWordBounds to stop at every matra, fragmenting Hindi words
// on double-click. Including unicode.IsMark keeps a base consonant and
// its combining marks together as a single "word" for selection purposes.
func isWordChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}
