// Watching a file that something else edits, and what a reload does to the view.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testPoll is the interval the live-watcher tests run at. Short enough to finish
// quickly, long enough that a tick is a real gap rather than a race.
const testPoll = 30 * time.Millisecond

// testTimeout bounds waiting for a reload: several times the work it takes, so
// a failure is a bug rather than a slow machine.
const testTimeout = 5 * time.Second

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "watched.txt")
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustOpen(t *testing.T, path string) *FileBuffer {
	t.Helper()
	fb, err := OpenFileBuffer(path)
	if err != nil {
		t.Fatal(err)
	}
	return fb
}

func numbered(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

// --- The watcher -------------------------------------------------------------

func TestWatcherFollowsAChangeMadeElsewhere(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, "one\ntwo\nthree\n")

	fb := mustOpen(t, path)
	w := WatchFile(path, fb, testPoll)
	defer w.Stop()

	mustWrite(t, path, "one\nTWO\nthree\nfour\n")

	next := awaitReload(t, w)
	defer next.Close()

	if got := next.Text(0, next.LineCount()); got != "one\nTWO\nthree\nfour\n" {
		t.Errorf("reloaded buffer = %q, want the new file", got)
	}
	// The buffer in view until the reload lands is the one that was there.
	if got := fb.Line(1); got != "two" {
		t.Errorf("old snapshot changed under the viewer: line 1 = %q", got)
	}
}

// TestReloadSupersedesAnEarlierOne is what keeps the queue from backing up: if
// the file is written twice before the viewer takes one reload, the viewer should
// get the newest file, not a stale one followed by a second hitch.
func TestReloadSupersedesAnEarlierOne(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, "first\n")

	fb := mustOpen(t, path)
	w := newFileWatcher(path, fb)

	// Two settled versions, neither of them read by a viewer yet.
	mustWrite(t, path, "second\n")
	w.tick()
	w.tick()
	mustWrite(t, path, "third\n")
	w.tick()
	w.tick()

	next := awaitReload(t, w)
	defer next.Close()
	if got := next.Text(0, next.LineCount()); got != "third\n" {
		t.Fatalf("queued reload = %q, want the newest version", got)
	}
	select {
	case extra := <-w.Events():
		text := extra.Text(0, extra.LineCount())
		extra.Close()
		t.Fatalf("a second reload was queued (%q), want only the newest", text)
	default:
	}
}

func TestTickStaysQuietWhileTheFileIsUnchanged(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, "one\ntwo\n")

	fb := mustOpen(t, path)
	w := newFileWatcher(path, fb)

	for range 8 {
		w.tick()
	}
	assertNoReload(t, w)
}

// TestTickWaitsForAVersionToSettle is the debounce. A half-written file is a
// state a writer passes through on the way to the real one, and the viewer must
// never show it.
func TestTickWaitsForAVersionToSettle(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, numbered(10))

	fb := mustOpen(t, path)
	w := newFileWatcher(path, fb)

	// Truncated: the file is genuinely half a file at this instant.
	mustWrite(t, path, numbered(4))
	w.tick()
	assertNoReload(t, w)

	// The write finishes. The half version was seen once and is never read.
	mustWrite(t, path, numbered(12))
	w.tick()
	assertNoReload(t, w)

	w.tick()
	next := awaitReload(t, w)
	defer next.Close()

	if got := next.Text(0, next.LineCount()); got != numbered(12) {
		t.Errorf("reloaded buffer = %q, want the finished file", got)
	}
}

func TestTickKeepsTheSnapshotWhenTheFileIsGone(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, "one\ntwo\n")

	fb := mustOpen(t, path)
	w := newFileWatcher(path, fb)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	w.tick()
	w.tick()
	assertNoReload(t, w)

	// It comes back under the same name, which is what a rename-over-save does.
	mustWrite(t, path, "replaced\n")
	w.tick()
	w.tick()
	next := awaitReload(t, w)
	defer next.Close()
	if got := next.Text(0, next.LineCount()); got != "replaced\n" {
		t.Errorf("reloaded buffer = %q, want the file that replaced it", got)
	}
}

func TestStopEndsTheWatch(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, "one\n")

	fb := mustOpen(t, path)
	w := WatchFile(path, fb, testPoll)
	w.Stop()

	mustWrite(t, path, "two\n")
	time.Sleep(10 * testPoll)
	assertNoReload(t, w)
}

// TestWatcherDoesNotGetInTheWayOfAnEditor runs the watcher against a file being
// written, renamed and deleted underneath it, which is how an editor or an agent
// replaces a file. Every one of those operations has to succeed while the viewer
// is watching and reloading it.
func TestWatcherDoesNotGetInTheWayOfAnEditor(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, numbered(200))

	fb := mustOpen(t, path)
	w := WatchFile(path, fb, testPoll)
	defer w.Stop()

	for i := range 40 {
		mustWrite(t, path, numbered(i%7+1))
		if err := os.Rename(path, path+".moved"); err != nil {
			t.Fatalf("rename blocked by the watcher: %v", err)
		}
		if err := os.Remove(path + ".moved"); err != nil {
			t.Fatalf("delete blocked by the watcher: %v", err)
		}
	}

	// Whatever reloads happened along the way are the viewer's business; this
	// test is about nobody else being blocked.
	for {
		select {
		case next := <-w.Events():
			next.Close()
			return
		default:
			return
		}
	}
}

func awaitReload(t *testing.T, w *FileWatcher) *FileBuffer {
	t.Helper()
	select {
	case fb := <-w.Events():
		return fb
	case <-time.After(testTimeout):
		t.Fatal("no reload after the file changed")
		return nil
	}
}

func assertNoReload(t *testing.T, w *FileWatcher) {
	t.Helper()
	select {
	case fb := <-w.Events():
		text := fb.Text(0, fb.LineCount())
		fb.Close()
		t.Fatalf("unexpected reload: %q", text)
	default:
	}
}

// --- The reload --------------------------------------------------------------

func newTestApp(t *testing.T, path string) *App {
	t.Helper()
	setTheme("dracula")
	a := NewApp(mustOpen(t, path), Options{TabWidth: 4, LineNumbers: true, Scrollbar: true, Highlight: true, GitChanges: true})
	a.Apply(Action{Kind: ActResize, X: 80, Y: 24})
	return a
}

func TestReloadKeepsTheViewOnTheSameLine(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, numbered(100))

	a := newTestApp(t, path)
	a.Apply(Action{Kind: ActScrollLines, N: 40})
	if got := a.topLine(); got != 40 {
		t.Fatalf("top line = %d, want 40", got)
	}

	mustWrite(t, path, numbered(150))
	a.Reload(mustOpen(t, path))

	if got := a.TotalLines; got != 150 {
		t.Fatalf("TotalLines = %d, want 150", got)
	}
	if got := a.topLine(); got != 40 {
		t.Errorf("top line after reload = %d, want still 40", got)
	}
	if got := a.fb.Line(40); got != "line 41" {
		t.Errorf("line at the top = %q, want %q", got, "line 41")
	}
}

func TestReloadClampsToAShorterFile(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, numbered(100))

	a := newTestApp(t, path)
	a.Apply(Action{Kind: ActGoBottom})
	if a.YOff != 77 {
		t.Fatalf("YOff = %d, want the bottom of a 100 line file on a 23 row pane", a.YOff)
	}

	// Still taller than the screen, so "stay where you were" has to become
	// "stay at the end", not "jump to the top".
	mustWrite(t, path, numbered(40))
	a.Reload(mustOpen(t, path))

	if got := a.TotalLines; got != 40 {
		t.Fatalf("TotalLines = %d, want 40", got)
	}
	if got := a.YOff; got != 17 {
		t.Errorf("YOff = %d, want 17: the last page of the new file", got)
	}
	if got := a.topLine(); got != 17 {
		t.Errorf("top line after reload = %d, want 17", got)
	}

	// A file short enough to fit entirely lands at its top, which is the only
	// place left for it.
	mustWrite(t, path, numbered(10))
	a.Reload(mustOpen(t, path))
	if a.YOff != 0 || a.topLine() != 0 {
		t.Errorf("YOff = %d, top line = %d, want both 0 for a file that fits on screen", a.YOff, a.topLine())
	}
}

// TestReloadDropsEverythingDerivedFromTheOldBytes is the point of the snapshot
// swap: the diff and the highlight runs describe a file that no longer exists.
func TestReloadDropsEverythingDerivedFromTheOldBytes(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, numbered(20))

	a := newTestApp(t, path)

	results := make(chan HlResult, 4)
	h := NewHighlighter(path, results)
	defer h.Close()
	a.AttachHighlighter(h)

	loaded := 0
	askedGen := uint64(0)
	a.AttachGitLoader(func(gen uint64) { loaded++; askedGen = gen })
	a.git = GitChanges{
		Hunks:   []Hunk{{Start: 5, End: 7, Kind: ChangeModified}},
		Removed: []Removed{{Before: 5, Hunk: 0, Lines: []string{"old"}}},
	}
	a.buildRemovedEnd()
	a.gitLoaded, a.gitRequested, a.changeJumped = true, true, true
	a.ShowDeleted = true
	a.Hl.Install(0, [][]StyleRun{{{End: 3, Slot: SlotFg}}})
	a.Gen = 7
	a.hlPending = true

	mustWrite(t, path, numbered(25))
	a.Reload(mustOpen(t, path))

	if a.gitLoaded || len(a.git.Hunks) != 0 {
		t.Error("the diff of the old file survived the reload")
	}
	if a.phantomsShown() {
		t.Error("removed lines from the old diff are still drawn as rows")
	}
	if loaded != 1 {
		t.Errorf("git diff requested %d times, want it started again once", loaded)
	}
	if askedGen != a.GitGen() {
		t.Errorf("the diff was started for generation %d, want %d: a diff of the old file must be recognisable as such", askedGen, a.GitGen())
	}
	if runs := a.Hl.Get(0); runs != nil {
		t.Errorf("highlight runs survived the reload: %v", runs)
	}
	if a.Gen != 8 {
		t.Errorf("Gen = %d, want 8: a bump discards what the worker is still tokenising", a.Gen)
	}
	if a.hlPending {
		t.Error("hlPending still set, so no new highlight would be requested")
	}
	a.RequestHighlight()
	if !a.hlPending {
		t.Error("reload left the viewer with no highlight request in flight")
	}
}

func TestReloadKeepsASelectionThatStillFits(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, numbered(20))

	a := newTestApp(t, path)
	a.SelectRange(2, 0, 4, 6) // zero-based: rows 2 through 4, whole lines

	mustWrite(t, path, numbered(30))
	a.Reload(mustOpen(t, path))

	s, e := a.Sel.Start, a.Sel.End
	if s.Row != 2 || s.Col != 0 || e.Row != 4 || e.Col != 6 {
		t.Errorf("selection = %d:%d-%d:%d, want 2:0-4:6", s.Row, s.Col, e.Row, e.Col)
	}
	if got := a.SelectedText(); got != "line 3\nline 4\nline 5" {
		t.Errorf("SelectedText = %q", got)
	}
}

func TestReloadClampsASelectionPastTheEnd(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, numbered(20))

	a := newTestApp(t, path)
	a.SelectRange(17, 0, 19, 6)

	mustWrite(t, path, "short\nfile\nnow\n")
	a.Reload(mustOpen(t, path))

	if !a.Sel.IsVisible() {
		t.Fatal("the selection was dropped instead of clamped")
	}
	if got := a.Sel.End.Row; got != 2 {
		t.Errorf("selection ends on row %d, want the last row, 2", got)
	}
	if got := a.Sel.End.Col; got > a.LineWidth(2) {
		t.Errorf("selection ends at column %d, past the end of %q", got, a.fb.Line(2))
	}
}

func TestReloadRerunsTheSearch(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, "needle one\nfiller\nneedle two\n")

	a := newTestApp(t, path)
	a.Needle = "needle"
	a.runSearch()
	if got := len(a.Matches); got != 2 {
		t.Fatalf("%d matches, want 2", got)
	}

	mustWrite(t, path, "needle one\nneedle two\nneedle three\nneedle four\n")
	a.Reload(mustOpen(t, path))

	if got := len(a.Matches); got != 4 {
		t.Fatalf("%d matches after reload, want 4", got)
	}

	// A needle the new text no longer contains says so rather than reporting
	// the reload that removed it.
	mustWrite(t, path, "nothing here\n")
	a.Reload(mustOpen(t, path))
	if len(a.Matches) != 0 {
		t.Errorf("%d matches after the needle was deleted", len(a.Matches))
	}
	if a.StatusMsg != "no match: needle" {
		t.Errorf("status = %q, want the search to have the last word", a.StatusMsg)
	}
}

func TestReloadReportsWhatItDid(t *testing.T) {
	path := tempPath(t)
	mustWrite(t, path, numbered(10))

	a := newTestApp(t, path)

	mustWrite(t, path, numbered(13))
	a.Reload(mustOpen(t, path))
	if a.StatusMsg != "reloaded · +3 lines" {
		t.Errorf("status = %q, want %q", a.StatusMsg, "reloaded · +3 lines")
	}

	mustWrite(t, path, numbered(11))
	a.Reload(mustOpen(t, path))
	if a.StatusMsg != "reloaded · -2 lines" {
		t.Errorf("status = %q, want %q", a.StatusMsg, "reloaded · -2 lines")
	}

	mustWrite(t, path, strings.ReplaceAll(numbered(11), "line ", "row "))
	a.Reload(mustOpen(t, path))
	if a.StatusMsg != "reloaded" {
		t.Errorf("status = %q, want %q", a.StatusMsg, "reloaded")
	}
}
