package main

import (
	"charm.land/lipgloss/v2"
)

var (
	styleStatusBar = lipgloss.NewStyle().
			Background(lipgloss.BrightBlack).
			Foreground(lipgloss.White)

	styleSelection = lipgloss.NewStyle().
			Background(lipgloss.White).
			Foreground(lipgloss.Black)

	styleLineNum = lipgloss.NewStyle().Foreground(lipgloss.BrightBlack)

	styleLineNumSel = lipgloss.NewStyle().Foreground(lipgloss.White)

	styleScrollbar = lipgloss.NewStyle().
			Background(lipgloss.BrightBlack).
			Foreground(lipgloss.White)
)
