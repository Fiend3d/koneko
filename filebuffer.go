package main

import (
	"bufio"
	"os"
	"strings"
)

type FileBuffer struct {
	f       *os.File
	offsets []int64
	size    int64
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

	var offsets []int64
	offsets = append(offsets, 0)
	reader := bufio.NewReader(f)
	var pos int64
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			pos += int64(len(line))
			offsets = append(offsets, pos)
		}
		if err != nil {
			break
		}
	}

	return &FileBuffer{f: f, offsets: offsets, size: size}, nil
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
	start := fb.offsets[n]
	end := fb.offsets[n+1]
	buf := make([]byte, end-start)
	_, err := fb.f.ReadAt(buf, start)
	if err != nil {
		return "", err
	}
	s := string(buf)
	s = strings.TrimRight(s, "\r\n")
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
