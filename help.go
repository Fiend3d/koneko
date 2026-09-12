// The help overlay.
package main

import (
	"fmt"
	"strings"

	"github.com/Fiend3d/catatui"
)

const version = "2.0.0"

// helpLines is the overlay's text. A line starting with three spaces is a
// binding — the key, two or more spaces, then its description — and anything
// else is a heading.
var helpLines = []string{
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
	" GIT",
	"   ]                 next change",
	"   [                 previous change",
	"   c                 toggle change markers",
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

// maxKeyWidth is the width the key column is padded to, so descriptions line up.
var maxKeyWidth = computeMaxKeyWidth()

func computeMaxKeyWidth() int {
	w := 0
	for _, line := range helpLines {
		if key, _, ok := splitBinding(line); ok {
			w = max(w, len(key))
		}
	}
	return w
}

// splitBinding pulls the key and description out of a binding line.
func splitBinding(line string) (key, desc string, ok bool) {
	if !strings.HasPrefix(line, "   ") {
		return "", "", false
	}
	rest := line[3:]
	gap := strings.Index(rest, "  ")
	if gap <= 0 {
		return "", "", false
	}
	return rest[:gap], strings.TrimLeft(rest[gap:], " "), true
}

func renderHelp(buf *catatui.Buffer, a *App, th *Theme) {
	l := a.Layout()
	width := a.Width

	keyStyle := th.Style(SlotYellow)
	descStyle := th.Style(SlotDimItalic)
	headStyle := th.Style(SlotGreen)
	plain := th.Base()

	for row := uint16(0); row < l.ContentH; row++ {
		idx := a.HelpOff + int(row)
		if idx < 0 || idx >= len(helpLines) {
			fill(buf, 0, row, width, th.Background())
			continue
		}

		line := helpLines[idx]
		var x uint16
		if key, desc, ok := splitBinding(line); ok {
			x, _ = buf.SetStringn(0, row, "   ", width, plain)
			x, _ = buf.SetStringn(x, row, key, width-x, keyStyle)
			if pad := maxKeyWidth - len(key) + 2; pad > 0 && x < width {
				x, _ = buf.SetStringn(x, row, spaces[:min(pad, len(spaces))], width-x, plain)
			}
			if x < width {
				x, _ = buf.SetStringn(x, row, desc, width-x, descStyle)
			}
		} else if line != "" {
			x, _ = buf.SetStringn(0, row, line, width, headStyle)
		}
		if x < width {
			fill(buf, x, row, width-x, th.Background())
		}
	}

	renderHelpBar(buf, a, th, l)
}

func renderHelpBar(buf *catatui.Buffer, a *App, th *Theme, l Layout) {
	shown := min(a.HelpOff+int(l.ContentH), len(helpLines))
	left := " HELP — F1 or esc to close"
	right := fmt.Sprintf("%d/%d ", shown, len(helpLines))

	width := int(a.Width)
	gap := max(width-int(DisplayWidth(left, 1))-int(DisplayWidth(right, 1)), 0)
	bar := left
	for n := gap; n > 0; {
		take := min(n, len(spaces))
		bar += spaces[:take]
		n -= take
	}
	bar += right

	writeBar(buf, l.StatusY, TruncateToWidth(bar, Col(width)), a.Width, th.StatusBar())
}

// helpMaxOffset is the furthest the overlay can scroll.
func (a *App) helpMaxOffset() int {
	return max(len(helpLines)-a.ContentHeight(), 0)
}
