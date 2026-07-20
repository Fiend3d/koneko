package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	helpLines   = buildHelpLines()
	maxKeyWidth int
)

func buildHelpLines() []string {
	lines := []string{
		" Koneko v" + version,
		"",
		" NAVIGATION",
		"   up/k              scroll up 1 line",
		"   down/j            scroll down 1 line",
		"   left/right        scroll left/right",
		"   pgup/pgdn         scroll 1/2 screen",
		"   home/g            go to top",
		"   end/G             go to bottom",
		"   H                 reset horizontal scroll",
		"",
		" SELECTION",
		"   mouse click       set cursor position",
		"   mouse drag        select text",
		"   a                 select all",
		"   d                 deselect",
		"   y                 copy selection",
		"   x                 extend selection to full lines",
		"",
		" SEARCH",
		"   /                 enter search mode",
		"   n                 next match",
		"   N                 previous match",
		"   enter             commit search",
		"   esc               cancel search",
		"",
		" DISPLAY",
		"   l                 toggle line numbers",
		"   s                 toggle scrollbar",
		"   h                 toggle syntax highlighting",
		"",
		" MOUSE",
		"   left click        set cursor / start selection",
		"   left drag         extend selection",
		"   right click       extend selection to clicked pos",
		"   left dbl-click    select word (drag by word)",
		"   left tpl-click    select line (drag by line)",
		"   wheel             scroll",
		"   gutter l-click    select whole line",
		"   gutter r-click    extend selection to line",
		"   scrollbar drag    jump to position",
		"",
		" QUIT",
		"   q                 quit",
		"   ctrl+c            quit",
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "   ") {
			rest := line[3:]
			gapStart := strings.Index(rest, "  ")
			if gapStart > 0 {
				key := rest[:gapStart]
				if len(key) > maxKeyWidth {
					maxKeyWidth = len(key)
				}
			}
		}
	}
	return lines
}

func helpLineStyle(line string) string {
	bg := theme.Background
	defStyle := lipgloss.NewStyle().Background(bg).Foreground(theme.Foreground)
	if strings.HasPrefix(line, "   ") {
		rest := line[3:]
		gapStart := strings.Index(rest, "  ")
		if gapStart > 0 {
			key := rest[:gapStart]
			desc := strings.TrimLeft(rest[gapStart:], " ")
			keyStyle := theme.TokenStyles[chroma.LiteralString].Background(bg)
			descStyle := theme.TokenStyles[chroma.Comment].Background(bg)
			keyPadded := key + strings.Repeat(" ", maxKeyWidth-len(key))
			return defStyle.Render("   ") +
				keyStyle.Render(keyPadded) +
				descStyle.Render("  "+desc)
		}
		return defStyle.Render(line)
	}

	return theme.TokenStyles[chroma.NameFunction].Background(bg).Render(line)
}

func renderHelp(m Model) string {
	contentH := m.contentHeight()
	from := m.helpOffset
	to := m.helpOffset + contentH
	if to > len(helpLines) {
		to = len(helpLines)
	}

	bg := styleBackground

	var b strings.Builder
	for row := 0; row < contentH; row++ {
		if row > 0 {
			b.WriteByte('\n')
		}
		absIdx := from + row
		if absIdx < len(helpLines) {
			line := helpLines[absIdx]
			if line == "" {
				b.WriteString(bg.Render(strings.Repeat(" ", m.width)))
			} else {
				styled := helpLineStyle(line)
				if pad := m.width - ansi.StringWidth(styled); pad > 0 {
					styled += bg.Render(strings.Repeat(" ", pad))
				}
				b.WriteString(styled)
			}
		} else {
			b.WriteString(bg.Render(strings.Repeat(" ", m.width)))
		}
	}

	b.WriteByte('\n')
	b.WriteString(renderHelpStatusBar(m, from, to, contentH))

	return b.String()
}

func renderHelpStatusBar(m Model, from, to, contentH int) string {
	total := len(helpLines)
	leftText := "HELP"
	rightText := fmt.Sprintf("%d/%d", to, total)
	if m.helpOffset > 0 {
		rightText = fmt.Sprintf("+%d  ", m.helpOffset) + rightText
	}

	mid := m.width - 2 - ansi.StringWidth(leftText) - ansi.StringWidth(rightText)
	if mid < 0 {
		rightText = fmt.Sprintf("%d/%d", to, total)
		mid = m.width - 2 - ansi.StringWidth(leftText) - ansi.StringWidth(rightText)
	}
	if mid < 0 {
		mid = 0
	}

	bar := " " + leftText + strings.Repeat(" ", mid) + rightText + " "
	return styleStatusBar.Render(bar)
}
