//go:build windows

package main

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// openForRead opens path with the share mode os.Open does not use.
//
// syscall.Open asks for FILE_SHARE_READ | FILE_SHARE_WRITE and never
// FILE_SHARE_DELETE, and on Windows that omission is a lock: while any handle
// without it is open, the file cannot be renamed or deleted, so a rename-over
// save — how editors and tools replace a file atomically — fails with "the
// process cannot access the file because it is being used by another process".
// The handle a reload holds lasts milliseconds rather than the life of the
// session, which is a far smaller window than a file mapping left open, but it
// is still a window in the middle of someone else's save.
//
// With FILE_SHARE_DELETE the rename goes through, the file becomes delete-
// pending, and this handle keeps reading the bytes it opened until it is closed.
// Nobody waits for anybody.
func openForRead(path string) (*os.File, error) {
	candidate, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}

	handle, err := windows.CreateFile(candidate,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: notExist(err)}
	}
	return os.NewFile(uintptr(handle), path), nil
}

// notExist turns the two ways Windows reports a missing file into the error os
// would have produced, so errors.Is(err, os.ErrNotExist) keeps working.
func notExist(err error) error {
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
		return os.ErrNotExist
	}
	return err
}
