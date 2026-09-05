// The status bar and the search prompt that replaces it.
package main

import (
	"fmt"

	"github.com/Fiend3d/catatui"
)

const searchPrompt = " search: "

func renderStatus(buf *catatui.Buffer, a *App, th *Theme) {
	if a.Width == 0 {
		return
	}
	y := a.Layout().StatusY
	if a.Mode == ModeSearch {
		renderPrompt(buf, a, th, y)
		return
	}
	renderBar(buf, a, th, y)
}

func renderPrompt(buf *catatui.Buffer, a *App, th *Theme, y uint16) {
	writeBar(buf, y, searchPrompt+a.Prompt.Text(), a.Width, th.StatusBar())
}

// promptCursor is where the terminal cursor should sit while the prompt is
// open, and false when it should stay hidden.
func promptCursor(a *App) (uint16, uint16, bool) {
	if a.Mode != ModeSearch {
		return 0, 0, false
	}
	x := DisplayWidth(searchPrompt, 1) + a.Prompt.CursorCol()
	return uint16(min(int(x), int(a.Width)-1)), a.Layout().StatusY, true
}

// renderBar draws the file name and position readout.
//
// The pieces are written straight into the buffer rather than concatenated into
// one string first: the bar is redrawn on every frame, and building it by
// repeated concatenation allocated once per piece for a row that is mostly
// padding.
func renderBar(buf *catatui.Buffer, a *App, th *Theme, y uint16) {
	style := th.StatusBar()
	width := a.Width

	right := a.rightStatus()
	rightW := uint16(min(int(DisplayWidth(right, 1)), int(width)))

	// Trim the left side first; the position readout is the part you cannot
	// reconstruct by looking at the screen.
	left := TruncateToWidth(a.leftStatus(), Col(width-rightW))

	x := setGraphemeString(buf, 0, y, left, width, style)
	if gap := width - rightW; x < gap {
		fill(buf, x, y, gap-x, style)
		x = gap
	}
	if x < width {
		x = setGraphemeString(buf, x, y, right, width-x, style)
	}
	if x < width {
		fill(buf, x, y, width-x, style)
	}
}

// leftStatus is the transient message if there is one, otherwise the file name
// with the selection and search readouts.
func (a *App) leftStatus() string {
	if a.StatusMsg != "" {
		return " " + a.StatusMsg
	}
	return " " + a.FileName() + selectionInfo(a) + searchInfo(a)
}

// rightStatus is the scroll position, with the horizontal offset when scrolled.
func (a *App) rightStatus() string {
	shown := min(a.YOff+a.ContentHeight(), a.TotalLines)
	if a.XOff > 0 {
		return fmt.Sprintf("+%d %d/%d ", a.XOff, shown, a.TotalLines)
	}
	return fmt.Sprintf("%d/%d ", shown, a.TotalLines)
}

func selectionInfo(a *App) string {
	if !a.Sel.IsVisible() {
		return ""
	}
	s, e := a.Sel.Start, a.Sel.End
	return fmt.Sprintf(" sel %d:%d-%d:%d", s.Row+1, s.Col+1, e.Row+1, e.Col+1)
}

func searchInfo(a *App) string {
	if a.Needle == "" || len(a.Matches) == 0 {
		return ""
	}
	return fmt.Sprintf(" %s %d/%d", a.Needle, a.MatchIdx+1, len(a.Matches))
}

// writeBar draws text at y and pads the rest of the row, so the bar always
// covers its full width even after the text shortens.
func writeBar(buf *catatui.Buffer, y uint16, text string, width uint16, style catatui.Style) {
	x := setGraphemeString(buf, 0, y, text, width, style)
	if x < width {
		fill(buf, x, y, width-x, style)
	}
}
