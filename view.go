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
	v.WindowTitle = "Koneko"
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

	if m.helpMode {
		v.SetContent(renderHelp(m))
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
		lineWidth := 0

		if row > 0 {
			b.WriteByte('\n')
		}

		if row < len(lines) {
			lineNum := from + row
			var styled string
			lineWidth = 0
			cached := false

			if m.highlight {
				var w int
				styled, w, cached = m.highlighter.StyledLine(lineNum)
				if cached {
					lineWidth = w
				}
			}

		if !cached {
			styled = strings.ReplaceAll(lines[row], "\r", "")
			styled = expandTabs(styled, m.tabWidth)
			lineWidth = ansi.StringWidth(styled)
			styled = styleBackground.Render(styled)
		}

			inSelection := false
			if m.selection.Active || m.selection.Selecting {
				sr, sc, er, ec := m.selection.Bounds()
				if lineNum >= sr && lineNum <= er {
					inSelection = true
					styled = applyLineSelection(styled, lineNum, sr, sc, er, ec)
					lineWidth = ansi.StringWidth(styled)
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
			b.WriteString(styleBackground.Render(strings.Repeat(" ", gutter)))
		}

		if m.showScrollbar {
			visWidth := max(0, min(w, lineWidth-m.xOffset))
			if pad := w - visWidth; pad > 0 {
				b.WriteString(styleBackground.Render(strings.Repeat(" ", pad)))
			}
			b.WriteString(scrollbarCharAt(row, contentH, m.yOffset, m.totalLines))
		} else {
			visWidth := max(0, min(w, lineWidth-m.xOffset))
			if pad := w - visWidth; pad > 0 {
				b.WriteString(styleBackground.Render(strings.Repeat(" ", pad)))
			}
		}
	}

	b.WriteByte('\n')
	if m.searchMode {
		m.searchInput.SetWidth(m.width)
		b.WriteString(m.searchInput.View())
	} else {
		b.WriteString(renderStatusBar(m.width, m.filePath, m.yOffset, m.contentHeight(), m.totalLines, m.xOffset, m.selection, m.searchStr, m.matchIdx, len(m.matchLines)))
	}

	v.SetContent(b.String())
	return v
}

func applyLineSelection(styled string, lineNum, sr, sc, er, ec int) string {
	totalWidth := ansi.StringWidth(styled)
	if sc > totalWidth {
		sc = totalWidth
	}
	if ec > totalWidth {
		ec = totalWidth
	}

	if sr == er {
		if sc >= ec {
			return styled
		}
		before := ansi.Cut(styled, 0, sc)
		selected := ansi.Cut(styled, sc, ec)
		after := ansi.Cut(styled, ec, totalWidth)
		styledSelected := styleSelection.Render(ansi.Strip(selected))
		return before + styledSelected + after
	}

	if lineNum == er {
		if ec <= 0 {
			return styled
		}
		if ec >= totalWidth {
			return styleSelection.Render(ansi.Strip(styled))
		}
		selected := ansi.Cut(styled, 0, ec)
		after := ansi.Cut(styled, ec, totalWidth)
		styledSelected := styleSelection.Render(ansi.Strip(selected))
		return styledSelected + after
	}

	if lineNum == sr {
		if sc >= totalWidth {
			return styled
		}
		before := ansi.Cut(styled, 0, sc)
		selected := ansi.Cut(styled, sc, totalWidth)
		styledSelected := styleSelection.Render(ansi.Strip(selected))
		return before + styledSelected
	}

	return styleSelection.Render(ansi.Strip(styled))
}

func expandTabs(s string, tabWidth int) string {
	// Only tabs are rewritten, so a line without them is already expanded and
	// needs none of the grapheme walking below.
	if strings.IndexByte(s, '\t') < 0 {
		return s
	}
	var b strings.Builder
	col := 0
	i := 0
	for i < len(s) {
		if s[i] == '\t' {
			n := tabWidth - (col % tabWidth)
			b.WriteString(strings.Repeat(" ", n))
			col += n
			i++
			continue
		}
		cluster, w := ansi.FirstGraphemeCluster(s[i:], ansi.GraphemeWidth)
		if len(cluster) == 0 {
			break
		}
		b.WriteString(cluster)
		col += w
		i += len(cluster)
	}
	return b.String()
}

func visualToByte(s string, visualPos int) int {
	i := 0
	vis := 0
	for i < len(s) && vis < visualPos {
		cluster, w := ansi.FirstGraphemeCluster(s[i:], ansi.GraphemeWidth)
		if len(cluster) == 0 || w == 0 {
			break
		}
		if vis+w > visualPos {
			return i
		}
		vis += w
		i += len(cluster)
	}
	return i
}

func renderStatusBar(w int, filePath string, yOffset, contentH, totalLines int, xOffset int, sel Selection, searchStr string, matchIdx, matchTotal int) string {
	if w < 2 {
		return ""
	}

	name := filepath.Base(filePath)
	lineInfo := fmt.Sprintf("%d/%d", yOffset+contentH, totalLines)
	if xOffset > 0 {
		lineInfo += fmt.Sprintf(" +%d", xOffset)
	}

	selInfo := ""
	if sel.Active || sel.Selecting {
		sr, sc, er, ec := sel.Bounds()
		selInfo = fmt.Sprintf(" sel %d:%d-%d:%d", sr+1, sc+1, er+1, ec+1)
	}

	searchInfo := ""
	if searchStr != "" && matchTotal > 0 {
		searchInfo = fmt.Sprintf(" %s %d/%d", searchStr, matchIdx+1, matchTotal)
	}

	leftText := name + selInfo + searchInfo
	rightText := lineInfo
	mid := w - 2 - ansi.StringWidth(leftText) - ansi.StringWidth(rightText)

	if mid < 0 {
		if selInfo != "" {
			leftText = name + searchInfo
			mid = w - 2 - ansi.StringWidth(leftText) - ansi.StringWidth(rightText)
		}
		if mid < 0 {
			avail := max(0, w-2-ansi.StringWidth(searchInfo)-ansi.StringWidth(rightText))
			if avail > 3 {
				leftText = truncateString(name, max(0, avail-3)) + "..." + searchInfo
			} else {
				leftText = searchInfo
			}
			mid = w - 2 - ansi.StringWidth(leftText) - ansi.StringWidth(rightText)
		}
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
