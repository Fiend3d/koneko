// Display-column arithmetic over grapheme clusters.
//
// This is the module the port exists for. The bubbletea original measured text
// with four different width implementations (ansi.StringWidth,
// uniseg.StringWidth, runewidth.RuneWidth per rune rather than per cluster, and
// lipgloss.Width) that disagreed on ZWJ sequences and emoji presentation
// selectors, and it freely mixed byte offsets with display columns. Everything
// here goes through one pass of uniseg's grapheme stepper, which is what
// catatui.StringWidth measures with, so our widths and the renderer's can never
// disagree.
package main

import (
	"unicode"

	"github.com/rivo/uniseg"
)

// Col is a display column: one terminal cell, measured after tab expansion.
//
// A named type rather than a bare int because the original routinely mixed
// display columns with byte offsets — search matches were recorded as byte
// indices into a tab-expanded string and then used as columns, silently
// misplacing every highlight on a line containing multi-byte text. Byte offsets
// stay int; anything measured in cells is a Col.
type Col int

// Cluster is one grapheme cluster, located in both coordinate spaces at once.
type Cluster struct {
	// Byte is the offset of the cluster in the raw, un-expanded line.
	Byte int32
	// Col is the first display column the cluster occupies.
	Col int32
	// Width is the cells occupied. A tab is already expanded to its next stop,
	// so this is the only place tab width is ever applied.
	Width int32
}

// EndCol is one past the last column this cluster covers.
func (c Cluster) EndCol() Col { return Col(c.Col + c.Width) }

// ClusterTable is the cluster layout of a single line, built once and reused
// across frames. Reuse one table and call Rebuild per line: the backing slice
// keeps its capacity, so steady-state rendering does not allocate.
type ClusterTable struct {
	clusters   []Cluster
	totalWidth Col
	byteLen    int
}

// Rebuild lays out line, expanding tabs to tabWidth stops.
//
// Tab expansion happens here and nowhere else. The original expanded in two
// places — once on raw text and once on text that already had ANSI escapes
// injected — which is why its column counter drifted.
func (t *ClusterTable) Rebuild(line string, tabWidth int) {
	t.clusters = t.clusters[:0]
	t.byteLen = len(line)

	tab := int32(max(tabWidth, 1))
	var col int32

	// The ASCII fast path: every printable byte is its own cluster, one cell
	// wide. Source code is overwhelmingly ASCII and this is the render path.
	if i := firstNonASCII(line); i == len(line) {
		for j := 0; j < len(line); j++ {
			var w int32 = 1
			if line[j] == '\t' {
				w = tab - (col % tab)
			}
			t.clusters = append(t.clusters, Cluster{Byte: int32(j), Col: col, Width: w})
			col += w
		}
		t.totalWidth = Col(col)
		return
	}

	state := -1
	rest := line
	byteOff := 0
	for len(rest) > 0 {
		var cluster string
		var w int
		cluster, rest, w, state = uniseg.FirstGraphemeClusterInString(rest, state)
		width := int32(w)
		if cluster == "\t" {
			width = tab - (col % tab)
		}
		t.clusters = append(t.clusters, Cluster{Byte: int32(byteOff), Col: col, Width: width})
		col += width
		byteOff += len(cluster)
	}

	t.totalWidth = Col(col)
}

// firstNonASCII returns the index of the first byte needing real grapheme
// measurement, or len(s) when the string is entirely printable ASCII.
func firstNonASCII(s string) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\t' && (c < 0x20 || c >= 0x7f) {
			return i
		}
	}
	return len(s)
}

func (t *ClusterTable) Clusters() []Cluster { return t.clusters }

// Width is the total display width of the line.
func (t *ClusterTable) Width() Col { return t.totalWidth }

func (t *ClusterTable) ByteLen() int { return t.byteLen }

// IndexAtCol returns the index of the cluster covering col, and false past the
// end of the line.
//
// A column landing inside a double-width cluster resolves to that cluster,
// which is what lets the renderer detect a half-visible wide character.
func (t *ClusterTable) IndexAtCol(col Col) (int, bool) {
	if col >= t.totalWidth || col < 0 {
		return 0, false
	}
	// Clusters are sorted by column. Zero-width clusters share a column with
	// whatever follows them, so the partition point lands after that run and
	// stepping back picks the first cluster starting at or before col.
	i := partitionPoint(len(t.clusters), func(k int) bool {
		return Col(t.clusters[k].Col) <= col
	})
	if i == 0 {
		return 0, false
	}
	return i - 1, true
}

// ColToByte is the byte offset of the cluster containing col, rounded down to a
// cluster boundary so the result is always a valid string index.
func (t *ClusterTable) ColToByte(col Col) int {
	if i, ok := t.IndexAtCol(col); ok {
		return int(t.clusters[i].Byte)
	}
	if col < 0 {
		return 0
	}
	return t.byteLen
}

// ByteToCol is the display column at which the cluster containing byte begins.
func (t *ClusterTable) ByteToCol(b int) Col {
	if b >= t.byteLen {
		return t.totalWidth
	}
	if b <= 0 {
		return 0
	}
	i := partitionPoint(len(t.clusters), func(k int) bool {
		return int(t.clusters[k].Byte) <= b
	})
	if i == 0 {
		return 0
	}
	return Col(t.clusters[i-1].Col)
}

// ClusterStr is the text of cluster idx, sliced from the raw line it was built
// from.
func (t *ClusterTable) ClusterStr(line string, idx int) string {
	start := int(t.clusters[idx].Byte)
	end := t.byteLen
	if idx+1 < len(t.clusters) {
		end = int(t.clusters[idx+1].Byte)
	}
	if start > len(line) {
		return ""
	}
	return line[start:min(end, len(line))]
}

// partitionPoint returns the first index in [0,n) where pred stops holding,
// assuming pred is true for a prefix and false thereafter.
func partitionPoint(n int, pred func(int) bool) int {
	lo, hi := 0, n
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if pred(mid) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// DisplayWidth is the width of a standalone string with tabs expanded from
// column zero, for status-bar and help text where a full ClusterTable is
// overkill.
func DisplayWidth(s string, tabWidth int) Col {
	if w, ok := asciiWidth(s, tabWidth); ok {
		return w
	}
	tab := int32(max(tabWidth, 1))
	var col int32
	state := -1
	rest := s
	for len(rest) > 0 {
		var cluster string
		var w int
		cluster, rest, w, state = uniseg.FirstGraphemeClusterInString(rest, state)
		if cluster == "\t" {
			col += tab - (col % tab)
		} else {
			col += int32(w)
		}
	}
	return Col(col)
}

// asciiWidth is the fast path for strings that are entirely printable ASCII,
// where every byte is exactly one cell. ok is false when real grapheme
// measurement is needed.
func asciiWidth(s string, tabWidth int) (Col, bool) {
	tab := int32(max(tabWidth, 1))
	var col int32
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\t':
			col += tab - (col % tab)
		case c >= 0x20 && c < 0x7f:
			col++
		default:
			return 0, false
		}
	}
	return Col(col), true
}

// TruncateToWidth cuts s to at most max display columns, on a cluster boundary.
func TruncateToWidth(s string, maxWidth Col) string {
	if maxWidth <= 0 {
		return ""
	}
	var col Col
	state := -1
	rest := s
	byteOff := 0
	for len(rest) > 0 {
		var cluster string
		var w int
		cluster, rest, w, state = uniseg.FirstGraphemeClusterInString(rest, state)
		if col+Col(w) > maxWidth {
			return s[:byteOff]
		}
		col += Col(w)
		byteOff += len(cluster)
	}
	return s
}

// isWordCluster reports whether a grapheme cluster counts as part of a word for
// double-click selection, judged by its base character.
//
// Classifying the cluster rather than each rune is what makes this work for
// Devanagari conjuncts and ZWJ emoji: the original walked backwards rune by rune
// while measuring width per cluster, so the two disagreed and a word boundary
// could land inside a cluster such as हिन्दी.
func isWordCluster(g string) bool {
	if g == "" {
		return false
	}
	for _, r := range g {
		return r == '_' || isAlphanumeric(r)
	}
	return false
}

func isAlphanumeric(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r < 0x80:
		return false
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// FindWordBounds is the column range of the word under col, as [start, end).
// It returns an empty range when the position is not inside a word.
func FindWordBounds(line string, t *ClusterTable, col Col) (Col, Col) {
	clusters := t.Clusters()
	if len(clusters) == 0 {
		return 0, 0
	}

	// A click past the end of the line attaches to the last cluster, matching
	// the original's behaviour.
	idx, ok := t.IndexAtCol(col)
	if !ok {
		idx = len(clusters) - 1
	}

	if !isWordCluster(t.ClusterStr(line, idx)) {
		at := Col(clusters[idx].Col)
		return at, at
	}

	start := idx
	for start > 0 && isWordCluster(t.ClusterStr(line, start-1)) {
		start--
	}
	end := idx
	for end+1 < len(clusters) && isWordCluster(t.ClusterStr(line, end+1)) {
		end++
	}
	return Col(clusters[start].Col), clusters[end].EndCol()
}
