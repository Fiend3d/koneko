package main

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const contextLines = 100

type Model struct {
	filePath       string
	fileBuf        *FileBuffer
	totalLines     int
	yOffset        int
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
	highlight      bool
	tabWidth       int
	lastClickRow   int
	lastClickCol   int
	lastClickTime  time.Time
}

func initialModel(filePath string, tabWidth int, showLineNum bool, showScrollbar bool, highlight bool) Model {
	return Model{
		filePath:      filePath,
		showLineNum:   showLineNum,
		showScrollbar: showScrollbar,
		highlight:     highlight,
		tabWidth:      tabWidth,
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
