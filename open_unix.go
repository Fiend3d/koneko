//go:build !windows

package main

import "os"

// openForRead is plain os.Open. A Unix file descriptor is a reference to an
// inode, not a lock on it: the file can be written, renamed or unlinked while
// it is open, and the read keeps seeing the bytes it started with.
func openForRead(path string) (*os.File, error) { return os.Open(path) }
