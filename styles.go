package main

import (
	"charm.land/lipgloss/v2"
)

var (
	styleStatusBar = lipgloss.NewStyle().
			Background(lipgloss.NoColor{}).
			Foreground(lipgloss.NoColor{})

	styleSelection = lipgloss.NewStyle().
			Background(lipgloss.White).
			Foreground(lipgloss.Black)

	styleLineNum = lipgloss.NewStyle().Foreground(lipgloss.BrightBlack)

	styleScrollbar = lipgloss.NewStyle().Foreground(lipgloss.White)
)
