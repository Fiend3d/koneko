// Random access to a file's lines.
//
// The file is read once into a private buffer and indexed into a table of line
// offsets. A line is then a slice of that buffer — no syscall, no allocation,
// no cache — which is what makes scrolling and searching a 10 MB file cost
// nothing. The bubbletea original issued a ReadAt and allocated a string per
// line, and had to keep a single-entry cache to make the view bearable.
//
// The buffer is a snapshot, not a window onto the file. Mapping the file
// instead was cheaper to open and cost two things that turned out to matter:
// on Windows a mapped section blocks every other writer (an editor, git, a
// coding agent) with "the file is being used by another process", and a write
// that does get through changes the bytes under a line table built from the
// old layout, which garbles the text. Reading the file once and closing the
// handle removes both, and the only price is open time and one buffer the size
// of the file.
package main

import (
	"bytes"
	"io"
	"strings"
	"unsafe"
)

// avgLineGuess is the bytes-per-line estimate used to size the offset table up
// front, so indexing a large file does not repeatedly grow and copy the slice.
const avgLineGuess = 32

// FileBuffer holds a snapshot of a file and the offset of every line start.
//
// offsets has one entry per line plus a terminating entry at the end of the
// data, so line n spans [offsets[n], offsets[n+1]).
type FileBuffer struct {
	data    []byte
	offsets []int64
	path    string
	ident   fileIdent
}

// OpenFileBuffer reads path and indexes its line starts. The file is closed
// before OpenFileBuffer returns, so koneko holds no lock on it for the rest of
// the session.
func OpenFileBuffer(path string) (*FileBuffer, error) {
	data, ident, err := readWholeFile(path)
	if err != nil {
		return nil, err
	}
	fb := &FileBuffer{data: data, path: path, ident: ident}
	fb.index()
	return fb, nil
}

// Ident is the file's size and modification time as they were when the snapshot
// was taken, which is what FileWatcher compares against to see whether the file
// has moved on.
func (fb *FileBuffer) Ident() fileIdent { return fb.ident }

// readWholeFile reads path into a single buffer. The size from Stat sizes the
// buffer so the read is one pass; a file that reports no size (a pipe, a
// device) falls back to reading until EOF.
func readWholeFile(path string) ([]byte, fileIdent, error) {
	f, err := openForRead(path)
	if err != nil {
		return nil, fileIdent{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fileIdent{}, err
	}
	ident := fileIdent{mod: info.ModTime(), size: info.Size()}

	if ident.size <= 0 {
		data, err := io.ReadAll(f)
		return data, ident, err
	}

	data := make([]byte, ident.size)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, fileIdent{}, err
	}
	return data, ident, nil
}

// index builds the offset table with one pass of memchr over the buffer.
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
// The result aliases the buffer and stays valid for the life of the
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

// All returns the whole buffer, for a scan that wants to run over the file in
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

// Close releases the snapshot. Nothing handed out by Line, Bytes or All is
// meant to outlive it.
func (fb *FileBuffer) Close() error {
	fb.data = nil
	fb.offsets = nil
	return nil
}

// bytesToString views b as a string without copying. Safe here because the
// buffer is never written to and outlives every string handed out of it.
func bytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}
