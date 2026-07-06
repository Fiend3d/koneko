package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	tabWidth := flag.Int("tab-width", 4, "tab display width")
	noLineNumbers := flag.Bool("no-line-numbers", false, "hide line numbers")
	noScrollbar := flag.Bool("no-scrollbar", false, "hide scrollbar")
	noHighlight := flag.Bool("no-highlight", false, "disable syntax highlighting")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: koneko [-tab-width=N] [-no-line-numbers] [-no-scrollbar] [-no-highlight] <file>\n")
		os.Exit(1)
	}
	filePath := flag.Arg(0)

	p := tea.NewProgram(initialModel(filePath, *tabWidth, !*noLineNumbers, !*noScrollbar, !*noHighlight))

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
