// Watching the file for changes made somewhere else.
//
// koneko is a viewer, so the file it shows is usually being edited by something
// else — an editor, a formatter, git, a coding agent. Its buffer is a snapshot,
// and a snapshot goes stale. The watcher notices the file changed and hands the
// main loop a fresh one, which is what lets the viewer follow a file it does not
// own.
//
// It polls with Stat rather than subscribing to the OS. The whole check is one
// syscall per tick, it behaves the same on every platform, and it copes with how
// editors actually save: write to a temporary file and rename it over the
// original, which swaps the file behind an unchanged path. Inotify and
// ReadDirectoryChangesW would be faster to react and cost a second copy of this
// file plus a handle held open for the whole session.
package main

import (
	"os"
	"time"
)

// pollInterval is how often the watcher looks at the file. One Stat per interval
// is well below anything measurable, and it is short enough that a change made
// elsewhere shows up before the user has finished looking at the old text.
const pollInterval = 250 * time.Millisecond

// fileIdent is what tells one version of a file from another. Size catches
// every write that changes length; the timestamp catches the rest.
type fileIdent struct {
	mod  time.Time
	size int64
}

func identOf(path string) (fileIdent, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return fileIdent{}, false
	}
	return fileIdent{mod: info.ModTime(), size: info.Size()}, true
}

// FileWatcher reports new versions of a path on a channel.
//
// The reading happens on the watcher's own goroutine: re-reading a 10 MB file
// takes milliseconds, and doing it inline would show up as a hitch in the frame
// the reload lands on.
type FileWatcher struct {
	path    string
	loaded  fileIdent // the version the buffer in view was taken from
	pending fileIdent // the version seen last tick, if it differs from loaded
	stop    chan struct{}
	events  chan *FileBuffer
}

// WatchFile starts watching path, which is currently shown as fb. Stop ends it.
func WatchFile(path string, fb *FileBuffer, interval time.Duration) *FileWatcher {
	w := newFileWatcher(path, fb)

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-w.stop:
				return
			case <-ticker.C:
				w.tick()
			}
		}
	}()
	return w
}

// newFileWatcher builds a watcher without starting its goroutine, so a test can
// drive tick by hand and control what each look at the file sees.
func newFileWatcher(path string, fb *FileBuffer) *FileWatcher {
	return &FileWatcher{
		path:    path,
		loaded:  fb.Ident(),
		pending: fb.Ident(),
		stop:    make(chan struct{}),
		// Capacity one: the viewer takes one reload per loop pass, and the drain
		// in reload keeps only the newest snapshot queued.
		events: make(chan *FileBuffer, 1),
	}
}

// Events is the channel of fresh buffers.
func (w *FileWatcher) Events() <-chan *FileBuffer { return w.events }

// Stop ends the watch. main.go calls it once, from a defer.
func (w *FileWatcher) Stop() { close(w.stop) }

// tick is one look at the file.
//
// A version has to be seen twice before it is read. Writers are not atomic: a
// truncate followed by a write leaves a window where the file is empty or half
// its final length, and a reload landing there would show a broken file until
// the next one. Waiting for the identity to repeat makes the reload land on a
// write that has finished, and costs one tick of latency.
func (w *FileWatcher) tick() {
	cur, ok := identOf(w.path)
	switch {
	case !ok:
		// Deleted, or between a rename and whatever replaces it. Keep showing
		// the snapshot and wait for the path to resolve again.
		w.pending = w.loaded

	case cur == w.loaded:
		// Unchanged. Reset the debounce so a version has to be new, not merely
		// remembered from long ago.
		w.pending = cur

	case cur != w.pending:
		// First tick this version has been seen. Wait one more before reading it.
		w.pending = cur

	default:
		w.reload()
	}
}

// reload reads the file again and publishes it.
func (w *FileWatcher) reload() {
	fb, err := OpenFileBuffer(w.path)
	if err != nil {
		// Mid-write, or a permission change. pending stays put, so the next
		// tick tries again rather than treating the failure as handled.
		return
	}

	// What was actually read, which is the version now in view.
	w.loaded = fb.Ident()
	w.pending = w.loaded

	// A reload supersedes anything still queued: an older snapshot of the same
	// file is worth nothing, and dropping it costs the GC one buffer.
	for draining := true; draining; {
		select {
		case <-w.events:
		default:
			draining = false
		}
	}
	w.events <- fb
}
