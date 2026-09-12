// Git change markers.
//
// The diff is taken by running git itself rather than linking a git library: it
// honours whatever the user's repository is configured with, adds no dependency,
// and a process that fails — no git, not a repository, an untracked file —
// simply yields no markers. It runs on its own goroutine and reports once, so
// opening a file never waits on it.
package main

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
)

// ChangeKind is what happened to a line relative to HEAD.
type ChangeKind uint8

const (
	ChangeNone ChangeKind = iota
	ChangeAdded
	ChangeModified
	// ChangeRemovedBelow marks the line after which lines were deleted.
	ChangeRemovedBelow
	// ChangeRemovedAbove marks line 0 when the deletion was before the first
	// line, where there is no line above to carry the marker.
	ChangeRemovedAbove
)

// Hunk is a run of changed lines, zero-based and half-open. A deletion has no
// lines of its own, so it is a one-line hunk on its neighbour.
type Hunk struct {
	Start, End int
	Kind       ChangeKind
}

// GitChanges is the file's hunks, sorted by Start and not overlapping.
type GitChanges struct {
	Hunks []Hunk
}

// At is the change on line n. It is a binary search rather than a per-line
// table, so a file with a handful of edits costs a handful of hunks however
// long it is, and a lookup per rendered row allocates nothing.
func (g *GitChanges) At(n int) ChangeKind {
	i, found := slices.BinarySearchFunc(g.Hunks, n, func(h Hunk, n int) int {
		return cmp.Compare(h.Start, n)
	})
	if found {
		return g.Hunks[i].Kind
	}
	if i > 0 && n < g.Hunks[i-1].End {
		return g.Hunks[i-1].Kind
	}
	return ChangeNone
}

// Next is the index of the first hunk starting after row, wrapping to the first
// hunk, or -1 when there are none.
func (g *GitChanges) Next(row int) int {
	if len(g.Hunks) == 0 {
		return -1
	}
	for i, h := range g.Hunks {
		if h.Start > row {
			return i
		}
	}
	return 0
}

// Prev is the index of the last hunk starting before row, wrapping to the last
// hunk, or -1 when there are none.
func (g *GitChanges) Prev(row int) int {
	if len(g.Hunks) == 0 {
		return -1
	}
	for i, h := range slices.Backward(g.Hunks) {
		if h.Start < row {
			return i
		}
	}
	return len(g.Hunks) - 1
}

// add appends a hunk, resolving the one overlap -U0 output can produce: a
// deletion landing on a line an adjacent hunk already marks. The line's own
// addition or modification is the more useful thing to show.
func (g *GitChanges) add(h Hunk) {
	if n := len(g.Hunks); n > 0 {
		prev := &g.Hunks[n-1]
		if h.Start < prev.End {
			switch {
			case isDeletion(h.Kind):
				return
			case isDeletion(prev.Kind):
				*prev = h
				return
			}
		}
	}
	g.Hunks = append(g.Hunks, h)
}

func isDeletion(k ChangeKind) bool {
	return k == ChangeRemovedBelow || k == ChangeRemovedAbove
}

// parseDiff reads `git diff -U0` output and keeps only the hunk headers.
//
// Content lines are skipped without being held: a minified file's single line
// can be megabytes, and ReadSlice hands such a line back in buffer-sized pieces
// rather than growing to fit it. Only a piece that starts a line can be a
// header, and every header fits in the first piece.
func parseDiff(r io.Reader) GitChanges {
	var g GitChanges
	br := bufio.NewReaderSize(r, 4096)
	atLineStart := true
	for {
		chunk, err := br.ReadSlice('\n')
		if atLineStart && bytes.HasPrefix(chunk, []byte("@@ -")) {
			if h, ok := parseHunkHeader(chunk); ok {
				g.add(h)
			}
		}
		if err == bufio.ErrBufferFull {
			atLineStart = false
			continue
		}
		if err != nil {
			break
		}
		atLineStart = true
	}
	return g
}

// parseHunkHeader reads `@@ -a[,b] +c[,d] @@`. A missing count means one line.
func parseHunkHeader(line []byte) (Hunk, bool) {
	rest := line[len("@@ -"):]
	_, oldCount, rest, ok := parseRange(rest)
	if !ok || len(rest) == 0 || rest[0] != ' ' {
		return Hunk{}, false
	}
	rest = rest[1:]
	if len(rest) == 0 || rest[0] != '+' {
		return Hunk{}, false
	}
	newStart, newCount, _, ok := parseRange(rest[1:])
	if !ok {
		return Hunk{}, false
	}

	switch {
	case oldCount == 0:
		return Hunk{Start: newStart - 1, End: newStart - 1 + newCount, Kind: ChangeAdded}, newCount > 0
	case newCount == 0:
		// newStart is the line the deletion follows, zero when it was at the top.
		if newStart == 0 {
			return Hunk{Start: 0, End: 1, Kind: ChangeRemovedAbove}, true
		}
		return Hunk{Start: newStart - 1, End: newStart, Kind: ChangeRemovedBelow}, true
	default:
		return Hunk{Start: newStart - 1, End: newStart - 1 + newCount, Kind: ChangeModified}, true
	}
}

// parseRange reads `start[,count]` and returns what follows it.
func parseRange(b []byte) (start, count int, rest []byte, ok bool) {
	start, b, ok = parseInt(b)
	if !ok {
		return 0, 0, nil, false
	}
	count = 1
	if len(b) > 0 && b[0] == ',' {
		if count, b, ok = parseInt(b[1:]); !ok {
			return 0, 0, nil, false
		}
	}
	return start, count, b, true
}

func parseInt(b []byte) (int, []byte, bool) {
	i := 0
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, b, false
	}
	n, err := strconv.Atoi(string(b[:i]))
	if err != nil {
		return 0, b, false
	}
	return n, b[i:], true
}

// loadGitChanges diffs path against HEAD and sends the result to out exactly
// once. Any failure sends an empty result: a file outside a repository is not
// an error worth a message.
//
// out must have room for the send, so this goroutine can never block on a main
// loop that has already quit. Cancelling ctx kills git.
func loadGitChanges(ctx context.Context, path string, out chan<- GitChanges) {
	out <- diffAgainstHead(ctx, path)
}

func diffAgainstHead(ctx context.Context, path string) GitChanges {
	cmd := exec.CommandContext(ctx, "git",
		"-C", filepath.Dir(path),
		"--no-pager", "diff",
		"--no-color", "--no-ext-diff", "--no-textconv", "-U0",
		"HEAD", "--", filepath.Base(path))
	// A viewer must never take the index lock out from under a git command the
	// user is running at the same time.
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return GitChanges{}
	}
	if err := cmd.Start(); err != nil {
		return GitChanges{}
	}
	// parseDiff reads to EOF, so git is never left blocked on a full pipe.
	g := parseDiff(stdout)
	if err := cmd.Wait(); err != nil {
		return GitChanges{}
	}
	return g
}
