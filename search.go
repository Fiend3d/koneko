// Case-insensitive substring search over the whole file.
package main

import (
	"bytes"
	"strings"
)

// Match is one occurrence, located at a display column.
//
// The bubbletea original recorded a match's column as a byte index into a
// tab-expanded string and then used it as a display column, so on any line with
// multi-byte text before the match the highlight landed in the wrong place.
// Here the search runs over the file's raw bytes and the byte offset is
// converted through the line's cluster table exactly once, and only on lines
// that actually matched.
type Match struct {
	Line int
	Col  Col
}

// scanMatches returns every occurrence of needle, in document order.
//
// The scan runs over the whole mapping in one pass rather than line by line,
// finding candidates with IndexByte — which is a vectorised assembly routine —
// and only then verifying and locating them. A hand-rolled byte loop over 10 MB
// costs several cycles per byte; this costs a fraction of one.
func scanMatches(fb *FileBuffer, needle string, tabWidth int) []Match {
	if needle == "" {
		return nil
	}
	data := fb.All()
	lower := strings.ToLower(needle)
	if len(lower) > len(data) {
		return nil
	}

	// Candidates are located by the first byte, in either case. Two cursors are
	// kept so each IndexByte resumes just past its own previous hit, which
	// makes the whole scan two linear passes rather than a rescan per match.
	lo := lower[0]
	up := lo
	if lo >= 'a' && lo <= 'z' {
		up = lo - 'a' + 'A'
	}
	nextLo := bytes.IndexByte(data, lo)
	nextUp := -1
	if up != lo {
		nextUp = bytes.IndexByte(data, up)
	}

	var out []Match
	var table ClusterTable
	// Matches come out in increasing offset order, so the line cursor only ever
	// moves forward.
	line, lineStart, lineEnd := -1, int64(0), int64(0)
	// Matches do not overlap: once one is taken, the scan resumes past its end,
	// so "aa" finds two matches in "aaaa" rather than three.
	skipUntil := 0

	for {
		at := nextLo
		if at < 0 || (nextUp >= 0 && nextUp < at) {
			at = nextUp
		}
		if at < 0 {
			break
		}
		if nextLo == at {
			nextLo = indexByteFrom(data, lo, at+1)
		}
		if nextUp == at {
			nextUp = indexByteFrom(data, up, at+1)
		}

		if at < skipUntil || at+len(lower) > len(data) {
			continue
		}
		if !equalFoldASCII(bytesToString(data[at:at+len(lower)]), lower) {
			continue
		}

		// Locate the line, advancing the cursor rather than searching again.
		if line < 0 || int64(at) >= lineEnd {
			var n int
			n, lineStart = fb.LineAt(int64(at))
			if n != line {
				line = n
				lineEnd = fb.LineEnd(n)
			}
		}
		// A match may not run past the end of its line, which is the same rule
		// the line-by-line scan enforced implicitly.
		if int64(at+len(lower)) > lineEnd {
			continue
		}

		within := at - int(lineStart)
		col := Col(within)
		text := fb.Line(line)
		// A line of printable ASCII with no tabs has one cell per byte, so the
		// byte offset is already the column and the table is not worth building.
		if !isPlainASCII(text) {
			table.Rebuild(text, tabWidth)
			col = table.ByteToCol(within)
		}
		out = append(out, Match{Line: line, Col: col})
		skipUntil = at + len(lower)
	}
	return out
}

// indexByteFrom is IndexByte resumed at from, returning an absolute index.
func indexByteFrom(data []byte, c byte, from int) int {
	if from >= len(data) {
		return -1
	}
	i := bytes.IndexByte(data[from:], c)
	if i < 0 {
		return -1
	}
	return from + i
}

// isPlainASCII reports whether every byte of s is a printable ASCII character
// other than a tab, so that one byte is exactly one display column.
func isPlainASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c >= 0x7f {
			return false
		}
	}
	return true
}

// equalFoldASCII compares against an already-lowercased needle.
//
// A non-ASCII byte can only be equal to itself here: Unicode case folding can
// change a string's length, so folding per byte would misreport the match's
// extent. Searching for non-ASCII text still works, it is just case-sensitive
// for the non-ASCII part.
func equalFoldASCII(s, lower string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != lower[i] {
			return false
		}
	}
	return true
}
