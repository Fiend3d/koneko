//go:build !windows

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

type mapping struct {
	data []byte
}

// mapFile maps the whole of f read-only. An empty file maps to a nil slice,
// since mmap rejects a zero length and there is nothing to read from one.
func mapFile(f *os.File, size int64) ([]byte, mapping, error) {
	if size == 0 {
		return nil, mapping{}, nil
	}
	data, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, mapping{}, err
	}
	// The access pattern is a sequential index pass followed by random reads
	// as the user scrolls, so ask for readahead but do not fault it all in.
	_ = unix.Madvise(data, unix.MADV_WILLNEED)
	return data, mapping{data: data}, nil
}

func (m mapping) close() error {
	if m.data == nil {
		return nil
	}
	return unix.Munmap(m.data)
}
