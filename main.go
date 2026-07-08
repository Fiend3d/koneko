package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func main() {
	tabWidth := flag.Int("tab-width", 4, "tab display width")
	noLineNumbers := flag.Bool("no-line-numbers", false, "hide line numbers")
	noScrollbar := flag.Bool("no-scrollbar", false, "hide scrollbar")
	noHighlight := flag.Bool("no-highlight", false, "disable syntax highlighting")
	searchStr := flag.String("search", "", "search string")
	selectRange := flag.String("select", "", "selection range (e.g. 1:7-1:10)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: koneko [-tab-width=N] [-no-line-numbers] [-no-scrollbar] [-no-highlight] [-search=STRING] [-select=LINE:CHAR-LINE:CHAR] <file>\n")
		os.Exit(1)
	}
	if *searchStr != "" && *selectRange != "" {
		fmt.Fprintf(os.Stderr, "error: -search and -select cannot be used together\n")
		os.Exit(1)
	}
	filePath := flag.Arg(0)

	sr, sc, er, ec, hasSel := parseSelectRange(*selectRange)

	p := tea.NewProgram(initialModel(filePath, *tabWidth, !*noLineNumbers, !*noScrollbar, !*noHighlight, *searchStr, hasSel, sr, sc, er, ec))

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseSelectRange(s string) (sr, sc, er, ec int, ok bool) {
	if s == "" {
		return
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "invalid -select format %q (expected LINE:CHAR-LINE:CHAR)\n", s)
		return
	}
	start := strings.SplitN(parts[0], ":", 2)
	end := strings.SplitN(parts[1], ":", 2)
	if len(start) != 2 || len(end) != 2 {
		fmt.Fprintf(os.Stderr, "invalid -select format %q (expected LINE:CHAR-LINE:CHAR)\n", s)
		return
	}
	var a, b, c, d int
	var err error
	if a, err = strconv.Atoi(start[0]); err != nil {
		fmt.Fprintf(os.Stderr, "invalid -select start line %q\n", start[0])
		return
	}
	if b, err = strconv.Atoi(start[1]); err != nil {
		fmt.Fprintf(os.Stderr, "invalid -select start char %q\n", start[1])
		return
	}
	if c, err = strconv.Atoi(end[0]); err != nil {
		fmt.Fprintf(os.Stderr, "invalid -select end line %q\n", end[0])
		return
	}
	if d, err = strconv.Atoi(end[1]); err != nil {
		fmt.Fprintf(os.Stderr, "invalid -select end char %q\n", end[1])
		return
	}
	return a - 1, b - 1, c - 1, d - 1, true
}
