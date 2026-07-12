package main

import (
	"charm.land/lipgloss/v2"
)

var (
	styleStatusBar = lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground)

	styleSelection = lipgloss.NewStyle().
			Background(theme.SelectionBg).
			Foreground(theme.SelectionFg)

	styleLineNum = lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.DimText)

	styleLineNumSel = lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground)

	styleScrollbar = lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.Foreground)

	styleBackground = lipgloss.NewStyle().Background(theme.Background)
)
