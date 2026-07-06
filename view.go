package main

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const statusBarHeight = 1

func (m Model) View() tea.View {
	v := tea.NewView("")
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if !m.ready {
		v.SetContent("Loading...")
		return v
	}
	if m.err != nil {
		v.SetContent("Error: " + m.err.Error())
		return v
	}
	if m.totalLines == 0 {
		v.SetContent("(empty file)")
		return v
	}

	from, to := m.visibleLineRange()
	contentH := m.contentHeight()
	gutter := m.lineNumWidth()
	scrollbarCol := 0
	if m.showScrollbar {
		scrollbarCol = 1
	}
	w := m.width - scrollbarCol - gutter

	lines, err := m.fileBuf.Lines(from, to)
	if err != nil {
		v.SetContent("Error reading file: " + err.Error())
		return v
	}

	var b strings.Builder

	for row := 0; row < contentH; row++ {
		var lineContent string

		if row > 0 {
			b.WriteByte('\n')
		}
		if row < len(lines) {
			lineNum := from + row
			var styled string
			if m.highlight {
				styled = m.highlighter.StyledLine(lineNum)
			}
			if styled == "" {
				styled = strings.ReplaceAll(lines[row], "\r", "")
			}
			styled = expandTabs(styled, m.tabWidth)

			inSelection := false
			if m.selection.Active || m.selection.Selecting {
				sr, sc, er, ec := m.selection.Bounds()
				if lineNum >= sr && lineNum <= er {
					inSelection = true
					styled = applyLineSelection(styled, lineNum, sr, sc, er, ec)
				}
			}

			if m.showLineNum {
				numStr := fmt.Sprintf("%*d ", gutter-1, lineNum+1)
				if inSelection {
					b.WriteString(styleLineNumSel.Render(numStr))
				} else {
					b.WriteString(styleLineNum.Render(numStr))
				}
			}

			lineContent = ansi.Cut(styled, m.xOffset, m.xOffset+w)
			b.WriteString(lineContent)
		} else if m.showLineNum {
			b.WriteString(strings.Repeat(" ", gutter))
		}
		if m.showScrollbar {
			lineVis := ansi.StringWidth(ansi.Strip(lineContent))
			if pad := w - lineVis; pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			b.WriteString(scrollbarCharAt(row, contentH, m.yOffset, m.totalLines))
		}
		b.WriteString("\033[0m")
	}

	b.WriteByte('\n')
	b.WriteString(renderStatusBar(m.width, m.filePath, m.yOffset, m.contentHeight(), m.totalLines, m.xOffset, m.selection))

	v.SetContent(b.String())
	return v
}

func applyLineSelection(styled string, lineNum, sr, sc, er, ec int) string {
	totalWidth := ansi.StringWidth(styled)

	if sr == er {
		if sc > totalWidth {
			sc = totalWidth
		}
		if ec > totalWidth {
			ec = totalWidth
		}
		if sc >= ec {
			return styled
		}
		startByte := visualToByte(styled, sc)
		endByte := visualToByte(styled, ec)
		before := styled[:startByte]
		after := styled[endByte:]
		styledSelected := styleSelection.Render(ansi.Strip(styled[startByte:endByte]))
		fgRestore := ansiStateAt(styled, ec)
		return before + styledSelected + fgRestore + after
	}

	if lineNum == er {
		if ec <= 0 {
			return styled
		}
		if ec > totalWidth {
			ec = totalWidth
		}
		endByte := visualToByte(styled, ec)
		if endByte == 0 {
			return styled
		}
		after := styled[endByte:]
		styledSelected := styleSelection.Render(ansi.Strip(styled[:endByte]))
		fgRestore := ansiStateAt(styled, ec)
		return styledSelected + fgRestore + after
	}

	if lineNum == sr {
		if sc >= totalWidth {
			return styled
		}
		startByte := visualToByte(styled, sc)
		before := styled[:startByte]
		styledSelected := styleSelection.Render(ansi.Strip(styled[startByte:]))
		return before + styledSelected
	}

	return styleSelection.Render(ansi.Strip(styled))
}

func expandTabs(s string, tabWidth int) string {
	var b strings.Builder
	col := 0
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			start := i
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			b.WriteString(s[start:i])
			continue
		}
		if s[i] == '\t' {
			n := tabWidth - (col % tabWidth)
			b.WriteString(strings.Repeat(" ", n))
			col += n
		} else {
			b.WriteByte(s[i])
			col++
		}
		i++
	}
	return b.String()
}

func visualToByte(s string, visualPos int) int {
	i := 0
	vis := 0
	for i < len(s) && vis < visualPos {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		vis++
		i++
	}
	return i
}

func ansiStateAt(s string, visualPos int) string {
	var active strings.Builder
	i := 0
	vis := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			start := i
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++
			}
			seq := s[start:i]
			if seq == "\x1b[0m" || seq == "\x1b[m" {
				active.Reset()
			} else {
				active.WriteString(seq)
			}
			continue
		}
		if vis >= visualPos {
			break
		}
		vis++
		i++
	}
	return active.String()
}

func renderStatusBar(w int, filePath string, yOffset, contentH, totalLines int, xOffset int, sel Selection) string {
	name := filepath.Base(filePath)
	lineInfo := fmt.Sprintf("%d/%d", yOffset+contentH, totalLines)
	if xOffset > 0 {
		lineInfo += fmt.Sprintf("  +%d", xOffset)
	}

	selInfo := ""
	if sel.Active || sel.Selecting {
		sr, sc, er, ec := sel.Bounds()
		selInfo = fmt.Sprintf("  sel %d:%d-%d:%d", sr+1, sc+1, er+1, ec+1)
	}

	leftText := name + selInfo
	rightText := lineInfo
	leftText = truncateString(leftText, (w-2)/2)
	rightText = truncateString(rightText, (w-2)/2)
	mid := w - 2 - ansi.StringWidth(leftText) - ansi.StringWidth(rightText)
	if mid < 0 {
		leftText = truncateString(leftText, max(0, w-2-ansi.StringWidth(rightText)-3)) + "..."
		mid = w - 2 - ansi.StringWidth(leftText) - ansi.StringWidth(rightText)
		if mid < 0 {
			rightText = truncateString(rightText, max(0, w-2-ansi.StringWidth(leftText)-3)) + "..."
			mid = w - 2 - ansi.StringWidth(leftText) - ansi.StringWidth(rightText)
		}
	}

	bar := leftText + strings.Repeat(" ", mid) + rightText
	return styleStatusBar.Render(" " + bar + " ")
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w <= maxLen {
		return s
	}
	return ansi.Cut(s, 0, maxLen)
}

func (m Model) lineNumWidth() int {
	if !m.showLineNum {
		return 0
	}
	n := 1
	for t := m.totalLines; t >= 10; t /= 10 {
		n++
	}
	return n + 1
}

func scrollbarCharAt(row, contentH, yOffset, totalLines int) string {
	if totalLines <= contentH {
		return styleScrollbar.Render(" ")
	}
	maxOffset := totalLines - contentH
	thumbH := contentH * contentH / totalLines
	if thumbH < 1 {
		thumbH = 1
	}
	thumbPos := yOffset * (contentH - thumbH) / maxOffset
	if row >= thumbPos && row < thumbPos+thumbH {
		return styleScrollbar.Render("█")
	}
	return styleScrollbar.Render(" ")
}
