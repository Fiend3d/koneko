// Random access to a file's lines.
//
// The file is memory-mapped and indexed once into a table of line offsets. A
// line is then a slice of the mapping — no syscall, no allocation, no cache —
// which is what makes scrolling and searching a 10 MB file cost nothing. The
// bubbletea original issued a ReadAt and allocated a string per line, and had
// to keep a single-entry cache to make the view bearable.
package main

import (
	"bytes"
	"os"
	"strings"
	"unsafe"
)

// avgLineGuess is the bytes-per-line estimate used to size the offset table up
// front, so indexing a large file does not repeatedly grow and copy the slice.
const avgLineGuess = 32

// FileBuffer holds a mapped file and the offset of every line start.
//
// offsets has one entry per line plus a terminating entry at the end of the
// data, so line n spans [offsets[n], offsets[n+1]).
type FileBuffer struct {
	f       *os.File
	data    []byte
	mapping mapping
	offsets []int64
	path    string
}

// OpenFileBuffer maps path and indexes its line starts.
func OpenFileBuffer(path string) (*FileBuffer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	size := info.Size()

	data, m, err := mapFile(f, size)
	if err != nil {
		f.Close()
		return nil, err
	}

	fb := &FileBuffer{f: f, data: data, mapping: m, path: path}
	fb.index()
	return fb, nil
}

// index builds the offset table with one pass of memchr over the mapping.
func (fb *FileBuffer) index() {
	n := int64(len(fb.data))
	fb.offsets = make([]int64, 1, n/avgLineGuess+16)

	rest := fb.data
	var base int64
	for {
		j := bytes.IndexByte(rest, '\n')
		if j < 0 {
			break
		}
		base += int64(j) + 1
		fb.offsets = append(fb.offsets, base)
		rest = rest[j+1:]
	}
	// A final line with no trailing newline still counts as a line.
	if n > 0 && fb.offsets[len(fb.offsets)-1] != n {
		fb.offsets = append(fb.offsets, n)
	}
}

func (fb *FileBuffer) Path() string { return fb.path }

func (fb *FileBuffer) LineCount() int {
	if len(fb.offsets) == 0 {
		return 0
	}
	return len(fb.offsets) - 1
}

// Line returns line n with its line terminator stripped.
//
// The result aliases the mapping and stays valid for the life of the
// FileBuffer. It must not be modified, and it must not outlive Close.
func (fb *FileBuffer) Line(n int) string {
	if n < 0 || n >= len(fb.offsets)-1 {
		return ""
	}
	start, end := fb.offsets[n], fb.offsets[n+1]
	b := fb.data[start:end]
	// Trailing \n, then a \r before it: the two forms of line ending, checked
	// in the order they appear so a lone \r inside the line is left alone.
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	if len(b) > 0 && b[len(b)-1] == '\r' {
		b = b[:len(b)-1]
	}
	return bytesToString(b)
}

// Bytes returns the raw bytes of lines [from, to), terminators included.
func (fb *FileBuffer) Bytes(from, to int) []byte {
	from = max(from, 0)
	to = min(to, len(fb.offsets)-1)
	if from >= to {
		return nil
	}
	return fb.data[fb.offsets[from]:fb.offsets[to]]
}

// ByteSpan is the number of bytes lines [from, to) occupy in the file,
// terminators included. It sizes a buffer without reading anything.
func (fb *FileBuffer) ByteSpan(from, to int) int {
	from = max(from, 0)
	to = min(to, len(fb.offsets)-1)
	if from >= to {
		return 0
	}
	return int(fb.offsets[to] - fb.offsets[from])
}

// All returns the whole mapping, for a scan that wants to run over the file in
// one pass rather than line by line.
func (fb *FileBuffer) All() []byte { return fb.data }

// LineAt returns the index of the line containing byte offset off, and the
// offset at which that line starts.
func (fb *FileBuffer) LineAt(off int64) (line int, start int64) {
	// The last entry is the end of the data, so the search space is the line
	// starts proper.
	i := partitionPoint(len(fb.offsets), func(k int) bool { return fb.offsets[k] <= off })
	if i == 0 {
		return 0, 0
	}
	return min(i-1, fb.LineCount()-1), fb.offsets[i-1]
}

// LineEnd is the offset one past the last byte of line n, excluding its
// terminator.
func (fb *FileBuffer) LineEnd(n int) int64 {
	if n < 0 || n >= len(fb.offsets)-1 {
		return 0
	}
	end := fb.offsets[n+1]
	if end > fb.offsets[n] && fb.data[end-1] == '\n' {
		end--
	}
	if end > fb.offsets[n] && fb.data[end-1] == '\r' {
		end--
	}
	return end
}

// Text returns lines [from, to) as one string with line endings normalised to
// \n, which is what the highlighter and the clipboard both want.
func (fb *FileBuffer) Text(from, to int) string {
	b := fb.Bytes(from, to)
	if len(b) == 0 {
		return ""
	}
	if bytes.IndexByte(b, '\r') < 0 {
		return string(b)
	}
	s := string(b)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func (fb *FileBuffer) Close() error {
	err := fb.mapping.close()
	fb.data = nil
	if cerr := fb.f.Close(); err == nil {
		err = cerr
	}
	return err
}

// bytesToString views b as a string without copying. Safe here because the
// mapping is read-only and outlives every string handed out of it.
func bytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}
