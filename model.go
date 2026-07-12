package main

import (
	"time"

	"koneko/widgets/textinput"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const contextLines = 100

type Model struct {
	filePath       string
	fileBuf        *FileBuffer
	totalLines     int
	yOffset        int
	xOffset        int
	width          int
	height         int
	selection      Selection
	highlighter    *Highlighter
	highlightRange [2]int
	ready          bool
	err            error
	showLineNum    bool
	showScrollbar  bool
	scrollbarDrag  bool
	gutterSelect   bool
	gutterAnchor   int
	highlight      bool
	tabWidth       int
	searchStr      string
	searchMode     bool
	searchInput    textinput.Model
	matchLines     [][2]int
	matchIdx       int
	hasInitSelect  bool
	initSelSR      int
	initSelSC      int
	initSelER      int
	initSelEC      int
	lastClickRow   int
	lastClickCol   int
	lastClickTime  time.Time
	lastWheelTime  time.Time
}

func initialModel(filePath string, tabWidth int, showLineNum bool, showScrollbar bool, highlight bool, searchStr string, hasInitSelect bool, initSelSR, initSelSC, initSelER, initSelEC int) Model {
	si := textinput.New()
	si.Prompt = " search: "
	si.Placeholder = ""
	styles := si.Styles()
	styles.Focused.Text = lipgloss.NewStyle().Background(theme.Background).Foreground(theme.Foreground)
	styles.Focused.Prompt = lipgloss.NewStyle().Background(theme.Background).Foreground(theme.Foreground)
	styles.Cursor.Color = theme.Foreground
	styles.Cursor.Blink = true
	si.SetStyles(styles)
	return Model{
		filePath:      filePath,
		showLineNum:   showLineNum,
		showScrollbar: showScrollbar,
		highlight:     highlight,
		tabWidth:      tabWidth,
		searchStr:     searchStr,
		searchInput:   si,
		hasInitSelect: hasInitSelect,
		initSelSR:     initSelSR,
		initSelSC:     initSelSC,
		initSelER:     initSelER,
		initSelEC:     initSelEC,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			fb, err := OpenFileBuffer(m.filePath)
			if err != nil {
				return errMsg{err}
			}
			return fileLoadedMsg{fb: fb}
		},
		func() tea.Msg { return tea.RequestWindowSize() },
	)
}

type fileLoadedMsg struct {
	fb *FileBuffer
}

type errMsg struct {
	err error
}

type highlightReadyMsg struct{}

func (m Model) visibleLineRange() (int, int) {
	from := m.yOffset
	to := m.yOffset + m.contentHeight()
	if to > m.totalLines {
		to = m.totalLines
	}
	return from, to
}

func (m Model) contentHeight() int {
	h := m.height - statusBarHeight
	if h < 1 {
		return 1
	}
	return h
}
