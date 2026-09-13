// Display rows: file lines with the lines HEAD removed shown between them.
//
// While deleted lines are hidden, which is the default, a row is a line and
// none of this does anything. While they are shown, each removed block adds
// phantom rows above the line it was removed before, so YOff counts rows rather
// than lines. Everything that thinks in file lines — selection, search,
// highlighting — keeps doing so, and converts at the edges through lineRow and
// rowAt.
//
// A diff has a handful of blocks however long the file is, so both directions
// are a binary search over a prefix sum and cost nothing per frame.
package main

// phantomsShown reports whether removed lines are drawn as rows.
func (a *App) phantomsShown() bool {
	return a.ShowDeleted && a.hasGitMarkers() && len(a.git.Removed) > 0
}

// TotalRows is how many rows the document occupies.
func (a *App) TotalRows() int {
	if !a.phantomsShown() {
		return a.TotalLines
	}
	return a.TotalLines + a.removedEnd[len(a.removedEnd)-1]
}

// buildRemovedEnd recomputes the prefix sum: removedEnd[k] is how many removed
// lines blocks 0 through k hold between them.
func (a *App) buildRemovedEnd() {
	a.removedEnd = a.removedEnd[:0]
	n := 0
	for _, r := range a.git.Removed {
		n += len(r.Lines)
		a.removedEnd = append(a.removedEnd, n)
	}
}

// phantomsThrough is how many phantom rows sit above line, counting the block
// directly above it.
func (a *App) phantomsThrough(line int) int {
	blocks := a.git.Removed
	lo, hi := 0, len(blocks)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if blocks[mid].Before <= line {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == 0 {
		return 0
	}
	return a.removedEnd[lo-1]
}

// lineRow is the row file line is drawn on.
func (a *App) lineRow(line int) int {
	if !a.phantomsShown() {
		return line
	}
	return line + a.phantomsThrough(line)
}

// blockFirstRow is the row block k's first removed line is drawn on.
func (a *App) blockFirstRow(k int) int {
	if k == 0 {
		return a.git.Removed[0].Before
	}
	return a.git.Removed[k].Before + a.removedEnd[k-1]
}

// rowAt resolves a row. blk is -1 for a file line, which is then line;
// otherwise the row is line idx of removed block blk, and line is the file line
// that block sits above. A row past the end yields a line past the end.
func (a *App) rowAt(row int) (line, blk, idx int) {
	if !a.phantomsShown() {
		return row, -1, 0
	}
	// The last block starting at or above row.
	lo, hi := 0, len(a.git.Removed)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if a.blockFirstRow(mid) <= row {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	k := lo - 1
	if k < 0 {
		return row, -1, 0
	}
	if first := a.blockFirstRow(k); row < first+len(a.git.Removed[k].Lines) {
		return a.git.Removed[k].Before, k, row - first
	}
	return row - a.removedEnd[k], -1, 0
}

// topLine is the file line at the top of the view, or the one the phantom rows
// there sit above.
func (a *App) topLine() int {
	line, _, _ := a.rowAt(a.YOff)
	return line
}

// remap runs change, which may show or hide phantom rows, keeping the same
// file line at the top of the view.
func (a *App) remap(change func()) {
	was := a.phantomsShown()
	top := a.topLine()
	change()
	if was || a.phantomsShown() {
		a.YOff = a.lineRow(top)
		a.clampY()
	}
}

// PhantomTable rebuilds the scratch table for a removed line and returns the
// line with it.
func (a *App) PhantomTable(blk, idx int) (string, *ClusterTable) {
	line := a.git.Removed[blk].Lines[idx]
	a.scratch.Rebuild(line, a.TabWidth)
	return line, &a.scratch
}

// hunkBlock is the removed block that belongs to hunk i, or -1.
func (a *App) hunkBlock(i int) int {
	for k, r := range a.git.Removed {
		if r.Hunk == i {
			return k
		}
	}
	return -1
}
