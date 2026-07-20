package main

import (
	"bytes"
	"os"
	"strings"
)

// avgLineGuess is the bytes-per-line estimate used to size the offset table up
// front, so indexing a large file does not repeatedly grow and copy the slice.
const avgLineGuess = 32

type FileBuffer struct {
	f       *os.File
	offsets []int64

	// Single-entry cache: the view and mouse handling ask for the same line
	// many times in a row, and each miss costs a read syscall.
	cachedNum  int
	cachedLine string
	cachedOK   bool
}

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

	offsets := make([]int64, 1, size/avgLineGuess+16)
	buf := make([]byte, 1<<20)
	var base int64
	for {
		n, err := f.Read(buf)
		chunk := buf[:n]
		off := 0
		for {
			j := bytes.IndexByte(chunk[off:], '\n')
			if j < 0 {
				break
			}
			off += j + 1
			offsets = append(offsets, base+int64(off))
		}
		base += int64(n)
		if err != nil {
			break
		}
	}
	// A final line with no trailing newline still counts as a line.
	if base > 0 && offsets[len(offsets)-1] != base {
		offsets = append(offsets, base)
	}

	return &FileBuffer{f: f, offsets: offsets}, nil
}

func (fb *FileBuffer) LineCount() int {
	if len(fb.offsets) == 0 {
		return 0
	}
	return len(fb.offsets) - 1
}

func (fb *FileBuffer) Line(n int) (string, error) {
	if n < 0 || n >= len(fb.offsets)-1 {
		return "", nil
	}
	if fb.cachedOK && fb.cachedNum == n {
		return fb.cachedLine, nil
	}
	start := fb.offsets[n]
	end := fb.offsets[n+1]
	buf := make([]byte, end-start)
	_, err := fb.f.ReadAt(buf, start)
	if err != nil {
		return "", err
	}
	s := string(buf)
	s = strings.TrimRight(s, "\r\n")
	fb.cachedNum, fb.cachedLine, fb.cachedOK = n, s, true
	return s, nil
}

func (fb *FileBuffer) Lines(from, to int) ([]string, error) {
	if from < 0 {
		from = 0
	}
	if to > len(fb.offsets)-1 {
		to = len(fb.offsets) - 1
	}
	if from >= to {
		return nil, nil
	}
	start := fb.offsets[from]
	end := fb.offsets[to]
	buf := make([]byte, end-start)
	_, err := fb.f.ReadAt(buf, start)
	if err != nil {
		return nil, err
	}
	text := string(buf)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines, nil
}

func (fb *FileBuffer) Text(from, to int) (string, error) {
	if from < 0 {
		from = 0
	}
	if to > len(fb.offsets)-1 {
		to = len(fb.offsets) - 1
	}
	if from >= to {
		return "", nil
	}
	start := fb.offsets[from]
	end := fb.offsets[to]
	buf := make([]byte, end-start)
	_, err := fb.f.ReadAt(buf, start)
	if err != nil {
		return "", err
	}
	text := string(buf)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text, nil
}

func (fb *FileBuffer) Close() error {
	return fb.f.Close()
}
