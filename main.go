// Koneko is a terminal file viewer: less or bat with mouse-driven text
// selection, clipboard copy, search and syntax highlighting.
//
// Press F1 inside the viewer for the full key list.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/term"
)

// Options is the command line, parsed.
type Options struct {
	TabWidth    int
	LineNumbers bool
	Scrollbar   bool
	Highlight   bool
	GitChanges  bool
	Search      string
	Select      string
	Theme       string
	File        string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "koneko:", err)
		os.Exit(1)
	}
}

func run() error {
	opts, ok := parseFlags()
	if !ok {
		os.Exit(1)
	}
	setTheme(opts.Theme)

	fb, err := OpenFileBuffer(opts.File)
	if err != nil {
		return err
	}
	defer fb.Close()

	app := NewApp(fb, opts)

	// Git runs alongside terminal setup rather than after it, and reports
	// whenever it finishes; nothing waits on it. Cancelling on exit kills a git
	// still running, and the buffered channel lets its goroutine finish either
	// way.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gitResults := make(chan GitChanges, 1)
	app.AttachGitLoader(func() { go loadGitChanges(ctx, opts.File, gitResults) })
	app.RequestGitChanges()

	// RecoverAndRestore puts the terminal back if anything below panics, so a
	// crash leaves a usable shell and a readable stack trace.
	defer term.RecoverAndRestore()

	terminal, restore, err := term.Init(term.WithMouse())
	if err != nil {
		return err
	}
	defer restore()

	events := term.NewEventReader(os.Stdin, os.Stdout)
	defer events.Close()

	// Highlight results arrive asynchronously; the buffered channel keeps the
	// worker from blocking on a send while the main loop is busy drawing. The
	// worker starts even under -no-highlight so that toggling highlighting on
	// with `h` works — it costs one idle goroutine and does nothing until a
	// request arrives.
	hlResults := make(chan HlResult, 4)
	h := NewHighlighter(opts.File, hlResults)
	defer h.Close()
	app.AttachHighlighter(h)

	if size, err := terminal.Size(); err == nil {
		app.Apply(Action{Kind: ActResize, X: size.Width, Y: size.Height})
	}
	applyInitialSelection(app, opts)
	app.RequestHighlight()

	for !app.Quit {
		if err := terminal.Draw(func(f *catatui.Frame) { draw(f, app) }); err != nil {
			return err
		}

		// Block until something happens. An idle viewer costs no CPU at all.
		select {
		case ev, open := <-events.Events():
			if !open {
				return events.Err()
			}
			app.Apply(Decode(ev, app.Mode))
		case r := <-hlResults:
			app.InstallHighlight(r)
		case c := <-gitResults:
			app.InstallGitChanges(c)
		}

		// Drain whatever else is already queued before drawing again. A fast
		// wheel spin or a mouse drag produces dozens of events, and collapsing
		// them into one frame is what keeps the UI immediate — the bubbletea
		// original rendered once per message.
		for draining := true; draining && !app.Quit; {
			select {
			case ev, open := <-events.Events():
				if !open {
					app.Quit = true
				} else {
					app.Apply(Decode(ev, app.Mode))
				}
			case r := <-hlResults:
				app.InstallHighlight(r)
			case c := <-gitResults:
				app.InstallGitChanges(c)
			default:
				draining = false
			}
		}

		app.RequestHighlight()
	}
	return events.Err()
}

// draw renders one frame. Help is a full-screen overlay, so it replaces the
// viewer entirely rather than drawing on top of it.
func draw(f *catatui.Frame, app *App) {
	area := f.Area()
	app.Width, app.Height = area.Width, area.Height

	buf := f.Buffer()
	if app.Mode == ModeHelp {
		renderHelp(buf, app, theme)
		return
	}
	renderViewer(buf, app, theme)
	renderStatus(buf, app, theme)

	if x, y, ok := promptCursor(app); ok {
		f.SetCursor(x, y)
	}
}

func parseFlags() (Options, bool) {
	var o Options
	flag.IntVar(&o.TabWidth, "tab-width", 4, "tab display width")
	noLineNumbers := flag.Bool("no-line-numbers", false, "hide line numbers")
	noScrollbar := flag.Bool("no-scrollbar", false, "hide scrollbar")
	noHighlight := flag.Bool("no-highlight", false, "disable syntax highlighting")
	noGit := flag.Bool("no-git", false, "disable git change markers")
	flag.StringVar(&o.Search, "search", "", "search string")
	flag.StringVar(&o.Select, "select", "", "selection range (e.g. 1:7-1:10)")
	flag.StringVar(&o.Theme, "theme", defaultTheme,
		"color theme ("+strings.Join(ThemeNames, ", ")+")")
	var showVersion bool
	flag.BoolVar(&showVersion, "v", false, "show version")
	flag.BoolVar(&showVersion, "version", false, "show version")
	flag.Parse()

	if showVersion {
		fmt.Println("koneko version " + version)
		os.Exit(0)
	}

	o.LineNumbers = !*noLineNumbers
	o.Scrollbar = !*noScrollbar
	o.Highlight = !*noHighlight
	o.GitChanges = !*noGit

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: koneko [options] <file>\n\n")
		flag.PrintDefaults()
		return o, false
	}
	if o.Search != "" && o.Select != "" {
		fmt.Fprintln(os.Stderr, "error: -search and -select cannot be used together")
		return o, false
	}
	o.File = flag.Arg(0)
	return o, true
}

// applyInitialSelection acts on -select and -search, which give a script a way
// to open the file with something already highlighted.
func applyInitialSelection(app *App, opts Options) {
	if opts.Select != "" {
		if sel, ok := parseSelectRange(opts.Select); ok {
			app.SelectRange(sel[0], sel[1], sel[2], sel[3])
		}
		return
	}
	if opts.Search != "" {
		app.Needle = opts.Search
		app.runSearch()
	}
}

// parseSelectRange reads LINE:CHAR-LINE:CHAR, one-based, and returns zero-based
// values. CHAR counts grapheme clusters, so a caller does not need to know the
// file's encoding to point at the third character of a line.
func parseSelectRange(s string) ([4]int, bool) {
	var out [4]int
	start, end, ok := strings.Cut(s, "-")
	if !ok {
		fmt.Fprintf(os.Stderr, "invalid -select %q (expected LINE:CHAR-LINE:CHAR)\n", s)
		return out, false
	}
	for i, part := range [2]string{start, end} {
		lineStr, charStr, ok := strings.Cut(part, ":")
		if !ok {
			fmt.Fprintf(os.Stderr, "invalid -select %q (expected LINE:CHAR-LINE:CHAR)\n", s)
			return out, false
		}
		line, err1 := strconv.Atoi(lineStr)
		char, err2 := strconv.Atoi(charStr)
		if err1 != nil || err2 != nil {
			fmt.Fprintf(os.Stderr, "invalid -select %q: line and char must be numbers\n", s)
			return out, false
		}
		out[i*2], out[i*2+1] = line-1, char-1
	}
	return out, true
}
