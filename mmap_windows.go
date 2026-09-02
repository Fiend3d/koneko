package main

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type mapping struct {
	handle windows.Handle
	addr   uintptr
}

// mapFile maps the whole of f read-only. An empty file maps to a nil slice:
// Windows refuses to create a zero-length mapping, and there is nothing to read
// from one anyway.
func mapFile(f *os.File, size int64) ([]byte, mapping, error) {
	if size == 0 {
		return nil, mapping{}, nil
	}
	h, err := windows.CreateFileMapping(
		windows.Handle(f.Fd()), nil, windows.PAGE_READONLY,
		uint32(size>>32), uint32(size), nil)
	if err != nil {
		return nil, mapping{}, err
	}
	addr, err := windows.MapViewOfFile(h, windows.FILE_MAP_READ, 0, 0, uintptr(size))
	if err != nil {
		windows.CloseHandle(h)
		return nil, mapping{}, err
	}
	// go vet's unsafeptr check flags this uintptr-to-pointer conversion, and is
	// right to in general: an address held in a uintptr is invisible to the GC,
	// which may move or free what it points at. Here it points at a file
	// mapping the kernel owns, at a fixed address for as long as the view is
	// open, so there is nothing for the GC to move. The mapping outlives every
	// string handed out of it because only Close unmaps it.
	data := unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
	return data, mapping{handle: h, addr: addr}, nil
}

func (m mapping) close() error {
	if m.addr == 0 {
		return nil
	}
	err := windows.UnmapViewOfFile(m.addr)
	if cerr := windows.CloseHandle(m.handle); err == nil {
		err = cerr
	}
	return err
}
