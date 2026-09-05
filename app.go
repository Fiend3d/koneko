package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

// Mode is which input map is live.
type Mode int

const (
	ModeNormal Mode = iota
	ModeSearch
	ModeHelp
)

// doubleClickInterval is how long after a click a second one at the same spot
// still counts as a double click.
const doubleClickInterval = 500 * time.Millisecond

// App is the whole of the viewer's state.
type App struct {
	fb         *FileBuffer
	TotalLines int

	YOff int
	XOff Col

	Width, Height uint16

	Sel  Selection
	Mode Mode

	// Highlighting. Runs arrive from the worker asynchronously; Gen rises with
	// every request so a result for a window the user has already scrolled past
	// can be recognised and dropped.
	Hl          HlCache
	highlighter *Highlighter
	Gen         uint64
	Highlight   bool
	hlPending   bool

	TabWidth      int
	ShowLineNum   bool
	ShowScrollbar bool

	Needle   string
	Matches  []Match
	MatchIdx int
	Prompt   Prompt

	HelpOff int

	// StatusMsg takes over the left of the status bar until the next action.
	StatusMsg string

	scrollbarDrag bool
	lastClick     Pos
	lastClickTime time.Time
	clickCount    int

	// scratch is reused for every row of every frame, so steady-state rendering
	// does not allocate.
	scratch ClusterTable

	Quit bool
}

func NewApp(fb *FileBuffer, opts Options) *App {
	return &App{
		fb:            fb,
		TotalLines:    fb.LineCount(),
		TabWidth:      opts.TabWidth,
		ShowLineNum:   opts.LineNumbers,
		ShowScrollbar: opts.Scrollbar,
		Highlight:     opts.Highlight,
		Needle:        opts.Search,
	}
}

func (a *App) AttachHighlighter(h *Highlighter) { a.highlighter = h }

// Layout is where each part of the screen goes, in cells.
type Layout struct {
	Gutter             uint16
	ContentX, ContentW uint16
	ContentH           uint16
	ScrollbarX         uint16
	HasScrollbar       bool
	StatusY            uint16
}

const statusBarHeight = 1

func (a *App) Layout() Layout {
	var l Layout
	l.ContentH = uint16(a.ContentHeight())
	l.StatusY = 0
	if a.Height > statusBarHeight {
		l.StatusY = a.Height - statusBarHeight
	}

	if a.ShowLineNum {
		l.Gutter = uint16(digits(a.TotalLines) + 1)
	}
	l.ContentX = l.Gutter

	avail := int(a.Width) - int(l.Gutter)
	if a.ShowScrollbar && int(a.Width) > int(l.Gutter) {
		l.HasScrollbar = true
		l.ScrollbarX = a.Width - 1
		avail--
	}
	l.ContentW = uint16(max(avail, 0))
	return l
}

func digits(n int) int {
	d := 1
	for t := n; t >= 10; t /= 10 {
		d++
	}
	return d
}

// ContentHeight is how many document rows fit above the status bar.
func (a *App) ContentHeight() int {
	return max(int(a.Height)-statusBarHeight, 1)
}

// VisibleRange is the half-open range of line indices currently on screen.
func (a *App) VisibleRange() (int, int) {
	return a.YOff, min(a.YOff+a.ContentHeight(), a.TotalLines)
}

// LineTable rebuilds the scratch table for line n and returns the line with it.
func (a *App) LineTable(n int) (string, *ClusterTable) {
	line := a.fb.Line(n)
	a.scratch.Rebuild(line, a.TabWidth)
	return line, &a.scratch
}

// LineWidth is the display width of line n.
func (a *App) LineWidth(n int) Col {
	_, t := a.LineTable(n)
	return t.Width()
}

func (a *App) clampY() {
	maxOffset := max(a.TotalLines-a.ContentHeight(), 0)
	a.YOff = min(max(a.YOff, 0), maxOffset)
}

// clampX bounds the horizontal offset by the widest line on screen, so scrolling
// right cannot run off into empty space. The original incremented without limit.
func (a *App) clampX() {
	if a.XOff <= 0 {
		a.XOff = 0
		return
	}
	from, to := a.VisibleRange()
	var widest Col
	for n := from; n < to; n++ {
		if w := a.LineWidth(n); w > widest {
			widest = w
		}
	}
	// Keep at least one column of text in view.
	if a.XOff > widest {
		a.XOff = max(widest, 0)
	}
}

// scroll moves whichever pane is in front by n lines.
func (a *App) scroll(n int) {
	if a.Mode == ModeHelp {
		a.HelpOff = min(max(a.HelpOff+n, 0), a.helpMaxOffset())
		return
	}
	a.YOff += n
	a.clampY()
}

// --- Actions ---------------------------------------------------------------

type ActionKind int

const (
	ActNone ActionKind = iota
	ActQuit
	ActResize
	ActScrollLines    // N lines, signed
	ActScrollHalfPage // N half-pages, signed
	ActScrollX        // N columns, signed
	ActGoTop
	ActGoBottom
	ActResetX
	ActSelectAll
	ActDeselect
	ActCopy
	ActExtendToLines
	ActToggleLineNumbers
	ActToggleScrollbar
	ActToggleHighlight
	ActOpenHelp
	ActCloseHelp
	ActOpenSearch
	ActNextMatch
	ActPrevMatch
	ActPromptKey
	ActPromptCommit
	ActPromptCancel
	ActMouseDown
	ActMouseDrag
	ActMouseUp
	ActMouseWheel
)

// Action is one intent, decoded from an event by input.go and applied here.
type Action struct {
	Kind   ActionKind
	N      int
	X, Y   uint16
	Button MouseButton
	Rune   rune
	Key    PromptKey
}

// Apply runs one action against the state.
func (a *App) Apply(act Action) {
	// Any deliberate action clears a transient message.
	if act.Kind != ActNone && act.Kind != ActResize {
		a.StatusMsg = ""
	}

	switch act.Kind {
	case ActQuit:
		a.Quit = true

	case ActResize:
		a.Width, a.Height = act.X, act.Y
		a.clampY()

	case ActScrollLines:
		a.scroll(act.N)

	case ActScrollHalfPage:
		a.scroll(act.N * max(a.ContentHeight()/2, 1))

	case ActScrollX:
		a.XOff = max(a.XOff+Col(act.N), 0)
		a.clampX()

	case ActGoTop:
		if a.Mode == ModeHelp {
			a.HelpOff = 0
			return
		}
		a.YOff = 0
		a.clampY()

	case ActGoBottom:
		if a.Mode == ModeHelp {
			a.HelpOff = a.helpMaxOffset()
			return
		}
		a.YOff = a.TotalLines
		a.clampY()

	case ActResetX:
		a.XOff = 0

	case ActSelectAll:
		if a.TotalLines == 0 {
			return
		}
		last := a.TotalLines - 1
		a.Sel.Begin(Pos{0, 0})
		a.Sel.Extend(Pos{last, a.LineWidth(last)})
		a.Sel.Finish()

	case ActDeselect:
		a.Sel.Clear()

	case ActCopy:
		a.copySelection()

	case ActExtendToLines:
		if !a.Sel.IsVisible() {
			return
		}
		start, end := a.Sel.Start, a.Sel.End
		a.beginSelect(SelectLine, Pos{start.Row, 0})
		a.extendSelect(SelectLine, Pos{end.Row, 0})
		a.Sel.Finish()

	case ActToggleLineNumbers:
		a.ShowLineNum = !a.ShowLineNum

	case ActToggleScrollbar:
		a.ShowScrollbar = !a.ShowScrollbar

	case ActToggleHighlight:
		a.Highlight = !a.Highlight
		if a.Highlight {
			a.RequestHighlight()
		}

	case ActOpenHelp:
		a.Mode = ModeHelp
		a.HelpOff = 0

	case ActCloseHelp:
		a.Mode = ModeNormal

	case ActOpenSearch:
		a.Mode = ModeSearch
		a.Prompt.SetText(a.Needle)

	case ActPromptKey:
		a.Prompt.Apply(act.Key, act.Rune)

	case ActPromptCancel:
		a.Mode = ModeNormal
		a.Prompt.Clear()

	case ActPromptCommit:
		a.Mode = ModeNormal
		a.Needle = a.Prompt.Text()
		a.runSearch()

	case ActNextMatch:
		a.stepMatch(1)

	case ActPrevMatch:
		a.stepMatch(-1)

	case ActMouseDown:
		a.mouseDown(act)

	case ActMouseDrag:
		a.mouseDrag(act)

	case ActMouseUp:
		a.scrollbarDrag = false
		a.Sel.Finish()

	case ActMouseWheel:
		a.scroll(act.N)
	}
}

// --- Selection helpers -----------------------------------------------------

// snapCol rounds a column taken from a mouse position to the nearest grapheme
// cluster boundary on the given row, so a selection endpoint is never inside a
// glyph. See ClusterTable.SnapCol for why that matters.
func (a *App) snapCol(row int, col Col) Col {
	// LineTable rebuilds the shared scratch table, so use it and let it go.
	_, t := a.LineTable(row)
	return t.SnapCol(col)
}

// glyphCol identifies the glyph a mouse column landed on, as that cluster's
// first column.
//
// The click counter keys off this rather than off the snapped endpoint: the two
// halves of one wide glyph are a single spot to whoever clicked it, while
// snapping sends them to opposite boundaries, so counting those would never
// register a double click on anything two columns wide.
func (a *App) glyphCol(row int, col Col) Col {
	_, t := a.LineTable(row)
	return t.ClusterStartCol(col)
}

// rangeForPoint expands a position into the range the given granularity covers:
// the position itself, the word under it, or the whole line.
func (a *App) rangeForPoint(mode SelectMode, p Pos) (Pos, Pos) {
	switch mode {
	case SelectLine:
		return Pos{p.Row, 0}, Pos{p.Row, a.LineWidth(p.Row)}
	case SelectWord:
		line, t := a.LineTable(p.Row)
		start, end := FindWordBounds(line, t, p.Col)
		if start < end {
			return Pos{p.Row, start}, Pos{p.Row, end}
		}
		// Symbols (including emoji) and whitespace still have a selectable
		// grapheme, even though they are not alphanumeric words.
		if i, ok := t.IndexAtCol(p.Col); ok {
			c := t.Clusters()[i]
			return Pos{p.Row, Col(c.Col)}, Pos{p.Row, c.EndCol()}
		}
	}
	p.Col = a.snapCol(p.Row, p.Col)
	return p, p
}

func (a *App) beginSelect(mode SelectMode, p Pos) {
	start, end := a.rangeForPoint(mode, p)
	a.Sel.BeginRange(start, end)
	a.Sel.Mode = mode
}

func (a *App) extendSelect(mode SelectMode, p Pos) {
	start, end := a.rangeForPoint(mode, p)
	a.Sel.ExtendRange(start, end)
	a.Sel.Mode = mode
}

// clickCountAt returns 1, 2 or 3 for single, double and triple clicks at the
// same spot within the double-click interval.
func (a *App) clickCountAt(p Pos) int {
	now := time.Now()
	if p == a.lastClick && now.Sub(a.lastClickTime) < doubleClickInterval {
		a.clickCount++
		if a.clickCount > 3 {
			a.clickCount = 1
		}
	} else {
		a.clickCount = 1
	}
	a.lastClick, a.lastClickTime = p, now
	return a.clickCount
}

// copySelection puts the selected text on the system clipboard.
func (a *App) copySelection() {
	if !a.Sel.Active {
		return
	}
	text := a.SelectedText()
	if text == "" {
		return
	}
	if err := clipboard.WriteAll(text); err != nil {
		a.StatusMsg = "copy failed: " + err.Error()
		return
	}
	n := a.Sel.End.Row - a.Sel.Start.Row + 1
	a.StatusMsg = fmt.Sprintf("copied %d line%s", n, plural(n))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// SelectedText reconstructs the selected text from the file.
//
// Columns are converted back to byte offsets through a cluster table, so a
// selection that starts or ends inside a wide glyph cuts on a cluster boundary
// rather than mid-character. Only the first and last rows need that: every row
// between them is taken whole, so building a cluster table for each of them —
// which is what the straightforward version did — is pure waste on a selection
// spanning a large file.
func (a *App) SelectedText() string {
	if !a.Sel.IsVisible() {
		return ""
	}
	start, end := a.Sel.Start, a.Sel.End
	if start.Row >= a.TotalLines {
		return ""
	}
	end.Row = min(end.Row, a.TotalLines-1)

	var t ClusterTable
	first := a.fb.Line(start.Row)
	t.Rebuild(first, a.TabWidth)
	lo := t.ColToByte(start.Col)

	if start.Row == end.Row {
		hi := t.ColToByte(end.Col)
		if lo >= hi {
			return ""
		}
		return first[lo:hi]
	}

	last := a.fb.Line(end.Row)
	t.Rebuild(last, a.TabWidth)
	hi := t.ColToByte(end.Col)

	// Size the buffer from the file's own offsets so the builder never has to
	// grow and copy. The span is an upper bound: it counts the line terminators
	// that are being replaced one-for-one by a newline, plus the parts of the
	// first and last lines that fall outside the selection.
	var b strings.Builder
	b.Grow(a.fb.ByteSpan(start.Row, end.Row+1))

	b.WriteString(first[lo:])
	b.WriteByte('\n')
	for row := start.Row + 1; row < end.Row; row++ {
		b.WriteString(a.fb.Line(row))
		b.WriteByte('\n')
	}
	b.WriteString(last[:hi])
	return b.String()
}

// --- Search ----------------------------------------------------------------

func (a *App) runSearch() {
	a.Matches = nil
	a.MatchIdx = 0
	if a.Needle == "" {
		a.Sel.Clear()
		return
	}
	a.Matches = scanMatches(a.fb, a.Needle, a.TabWidth)
	if len(a.Matches) == 0 {
		a.StatusMsg = "no match: " + a.Needle
		return
	}
	// Resume from where the user is looking rather than always from the top.
	a.MatchIdx = 0
	for i, m := range a.Matches {
		if m.Line >= a.YOff {
			a.MatchIdx = i
			break
		}
	}
	a.selectMatch()
}

func (a *App) stepMatch(delta int) {
	if len(a.Matches) == 0 {
		return
	}
	a.MatchIdx = (a.MatchIdx + delta + len(a.Matches)) % len(a.Matches)
	a.selectMatch()
}

// selectMatch selects the current match and scrolls it into view.
func (a *App) selectMatch() {
	m := a.Matches[a.MatchIdx]
	width := DisplayWidth(a.Needle, a.TabWidth)
	a.Sel.Clear()
	a.Sel.Begin(Pos{m.Line, m.Col})
	a.Sel.Extend(Pos{m.Line, m.Col + width})
	a.Sel.Finish()
	a.ScrollToShow(m.Line)
}

// ScrollToShow puts row a third of the way down the pane, so there is context
// on both sides of it.
func (a *App) ScrollToShow(row int) {
	a.YOff = max(row-a.ContentHeight()/3, 0)
	a.clampY()
}

// --- Mouse -----------------------------------------------------------------

// mouseDown handles a press: the scrollbar, the gutter, or the text.
func (a *App) mouseDown(act Action) {
	l := a.Layout()

	if l.HasScrollbar && act.X == l.ScrollbarX && act.Y < l.ContentH && act.Button == MouseLeft {
		a.scrollbarDrag = true
		a.scrollToRow(int(act.Y))
		return
	}
	if act.Y >= l.ContentH {
		return
	}

	row := a.YOff + int(act.Y)
	if row >= a.TotalLines {
		return
	}

	// A click left of the text is a click in the gutter, which selects by line.
	inGutter := int(act.X) < int(l.Gutter)
	if !inGutter && int(act.X) >= int(l.ContentX)+int(l.ContentW) {
		return
	}
	raw := a.XOff + Col(int(act.X)-int(l.ContentX))

	switch act.Button {
	case MouseLeft:
		if inGutter {
			a.clickCountAt(Pos{row, -1})
			a.beginSelect(SelectLine, Pos{row, 0})
			return
		}
		mode := SelectChar
		switch a.clickCountAt(Pos{row, a.glyphCol(row, raw)}) {
		case 2:
			mode = SelectWord
		case 3:
			mode = SelectLine
		}
		a.beginSelect(mode, Pos{row, raw})

	case MouseRight:
		// Right click extends the existing selection by moving whichever end is
		// nearer, keeping the far end anchored.
		if inGutter {
			a.extendSelect(SelectLine, Pos{row, 0})
		} else {
			a.extendSelect(a.Sel.Mode, Pos{row, raw})
		}
		a.Sel.Finish()
	}
}

// mouseDrag extends the selection, scrolling when dragged past an edge.
func (a *App) mouseDrag(act Action) {
	l := a.Layout()
	if a.scrollbarDrag {
		a.scrollToRow(int(act.Y))
		return
	}
	if !a.Sel.Selecting {
		return
	}

	// A drag that reaches an edge keeps scrolling, so a selection can be
	// extended past what is on screen. Terminal mouse coordinates are unsigned
	// and clamped to the window, so dragging above the pane reports row 0
	// rather than a negative row — the top edge has to be treated as the
	// signal, where the bottom edge can be measured by how far past it is.
	y := int(act.Y)
	switch {
	case y == 0:
		a.YOff--
		a.clampY()
	case y >= int(l.ContentH):
		a.YOff += y - int(l.ContentH) + 1
		a.clampY()
		y = int(l.ContentH) - 1
	}

	row := max(min(a.YOff+y, a.TotalLines-1), 0)
	x := max(min(int(act.X)-int(l.ContentX), int(l.ContentW)), 0)
	a.extendSelect(a.Sel.Mode, Pos{row, a.XOff + Col(x)})
}

// scrollToRow maps a screen row in the scrollbar track to a scroll offset.
func (a *App) scrollToRow(y int) {
	h := a.ContentHeight()
	if a.TotalLines <= h || h <= 1 {
		return
	}
	a.YOff = y * (a.TotalLines - h) / (h - 1)
	a.clampY()
}

// --- Highlighting ----------------------------------------------------------

// RequestHighlight asks the worker for the visible window plus context, unless
// the cache already covers it or a request for the same window is in flight.
func (a *App) RequestHighlight() {
	if !a.Highlight || a.highlighter == nil {
		return
	}
	from, to := a.VisibleRange()
	if from >= to || a.Hl.Covers(from, to) || a.hlPending {
		return
	}
	ctxFrom := max(from-contextLines, 0)
	ctxTo := min(to+contextLines, a.TotalLines)

	a.Gen++
	a.hlPending = true
	a.highlighter.Request(hlRequest{
		from: ctxFrom,
		to:   ctxTo,
		text: a.fb.Text(ctxFrom, ctxTo),
		gen:  a.Gen,
	})
}

// InstallHighlight accepts a worker result, dropping it if the user has already
// scrolled somewhere else.
func (a *App) InstallHighlight(r HlResult) {
	a.hlPending = false
	if r.Gen != a.Gen {
		return
	}
	a.Hl.Install(r.From, r.Runs)
}

// FileName is the base name shown in the status bar.
func (a *App) FileName() string { return filepath.Base(a.fb.Path()) }

// SelectRange selects from (startRow, startChar) to (endRow, endChar), where
// the character indices count grapheme clusters, one-based on the command line
// and zero-based here.
//
// Counting clusters rather than bytes is what lets a caller point at the third
// character of a line without knowing the file's encoding.
func (a *App) SelectRange(startRow, startChar, endRow, endChar int) {
	if a.TotalLines == 0 {
		return
	}
	startRow = min(max(startRow, 0), a.TotalLines-1)
	endRow = min(max(endRow, 0), a.TotalLines-1)

	col := func(row, char int) Col {
		_, t := a.LineTable(row)
		clusters := t.Clusters()
		if char <= 0 {
			return 0
		}
		if char >= len(clusters) {
			return t.Width()
		}
		return Col(clusters[char].Col)
	}

	a.Sel.Begin(Pos{startRow, col(startRow, startChar)})
	a.Sel.Extend(Pos{endRow, col(endRow, endChar)})
	a.Sel.Finish()
	a.ScrollToShow(startRow)

	// Opening on a selection also seeds the search, so n and N step through the
	// other occurrences of whatever was pointed at.
	if text := a.SelectedText(); text != "" && !strings.Contains(text, "\n") {
		a.Needle = text
		a.Matches = scanMatches(a.fb, a.Needle, a.TabWidth)
		for i, m := range a.Matches {
			if m.Line == a.Sel.Start.Row && m.Col == a.Sel.Start.Col {
				a.MatchIdx = i
				break
			}
		}
	}
}
