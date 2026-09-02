// Syntax highlighting.
//
// The bubbletea original rendered highlighting by injecting ANSI escapes into
// the line text, which forced every later operation — horizontal scroll, the
// selection overlay, padding — to slice a string that had escapes in it. Here a
// highlighted line is a list of byte ranges over the raw line, each carrying a
// palette slot. Nothing is ever spliced, and the runs stay valid across a theme
// change because they name slots rather than colours.
//
// The lexer lives entirely on the worker goroutine and only finished results
// cross the channel; the original mutated a shared highlighter from a goroutine
// with no synchronisation at all.
package main

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// contextLines is how far either side of the visible range gets tokenised, so
// small scrolls do not each trigger fresh work.
const contextLines = 100

// StyleRun styles the bytes of a line up to End, exclusive. Runs are sorted by
// End and cover the line from byte zero with no gaps.
type StyleRun struct {
	End  int32
	Slot Slot
}

// HlResult is one finished batch of work from the worker.
type HlResult struct {
	From int
	Runs [][]StyleRun
	Gen  uint64
}

// HlCache holds the runs for a contiguous window of lines.
type HlCache struct {
	from int
	runs [][]StyleRun
}

// Get returns the runs for line n, or nil when it is outside the cached window.
func (c *HlCache) Get(n int) []StyleRun {
	i := n - c.from
	if i < 0 || i >= len(c.runs) {
		return nil
	}
	return c.runs[i]
}

// Covers reports whether [from, to) is entirely within the cached window.
func (c *HlCache) Covers(from, to int) bool {
	return len(c.runs) > 0 && c.from <= from && c.from+len(c.runs) >= to
}

func (c *HlCache) Install(from int, runs [][]StyleRun) {
	c.from, c.runs = from, runs
}

// hlRequest is a window of lines to tokenise.
type hlRequest struct {
	from, to int
	text     string
	gen      uint64
}

// Highlighter is the handle to the worker goroutine. Only requests and results
// cross the boundary; the lexer itself never leaves the worker.
type Highlighter struct {
	reqs chan hlRequest
	done chan struct{}
}

// NewHighlighter starts a worker that tokenises for the lexer matching path and
// sends every result to out.
func NewHighlighter(path string, out chan<- HlResult) *Highlighter {
	lexer := lexers.Match(path)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	h := &Highlighter{
		// Capacity one plus latest-wins replacement in Request: while the
		// worker is busy, a burst of scrolling should leave exactly one job
		// queued — the newest — not a backlog to grind through.
		reqs: make(chan hlRequest, 1),
		done: make(chan struct{}),
	}
	go h.run(lexer, out)
	return h
}

func (h *Highlighter) run(lexer chroma.Lexer, out chan<- HlResult) {
	defer close(h.done)
	for req := range h.reqs {
		runs, ok := tokenise(lexer, req.text, req.to-req.from)
		if !ok {
			continue
		}
		out <- HlResult{From: req.from, Runs: runs, Gen: req.gen}
	}
}

// Request queues a window, replacing any job not yet picked up.
func (h *Highlighter) Request(req hlRequest) {
	for {
		select {
		case h.reqs <- req:
			return
		default:
		}
		// The queue is full: drop the stale job and try again. Losing the race
		// to the worker here just means one extra pass, never a lost request.
		select {
		case <-h.reqs:
		default:
		}
	}
}

func (h *Highlighter) Close() {
	close(h.reqs)
	<-h.done
}

// tokenise turns one window of text into per-line run tables.
//
// Runs are recorded against the bytes of each line as the file stores it, which
// is the same coordinate space ClusterTable uses, so the renderer can walk
// clusters and runs together in one pass.
func tokenise(lexer chroma.Lexer, text string, lineCount int) ([][]StyleRun, bool) {
	iterator, err := lexer.Tokenise(nil, text)
	if err != nil {
		return nil, false
	}
	lineTokens := chroma.SplitTokensIntoLines(iterator.Tokens())

	runs := make([][]StyleRun, 0, max(lineCount, len(lineTokens)))
	for _, lt := range lineTokens {
		var line []StyleRun
		var off int32
		for _, tok := range lt {
			// The newline that ends a token's value is not part of the line's
			// bytes, and neither is a CR the file buffer already strips.
			v := strings.TrimRight(tok.Value, "\r\n")
			if v == "" {
				continue
			}
			off += int32(len(v))
			slot := theme.SlotFor(tok.Type)
			// Coalesce: chroma emits many adjacent tokens of the same type, and
			// a run per token would make the render path's cursor walk longer
			// than the line itself.
			if n := len(line); n > 0 && line[n-1].Slot == slot {
				line[n-1].End = off
				continue
			}
			line = append(line, StyleRun{End: off, Slot: slot})
		}
		runs = append(runs, line)
	}
	// A window can tokenise to fewer lines than it spans when it ends without a
	// trailing newline. Pad so the cache's window length still matches.
	for len(runs) < lineCount {
		runs = append(runs, nil)
	}
	return runs, true
}
