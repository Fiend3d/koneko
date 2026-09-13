package main

import (
	"fmt"
	"strings"
	"testing"
)

// deletedApp is a test app with a synthetic diff that removed lines, drawn
// inline.
func deletedApp(t *testing.T, path string, w, h uint16, g GitChanges) *App {
	t.Helper()
	a := testApp(t, path, w, h)
	a.ShowGitChanges = true
	a.InstallGitChanges(g)
	a.Apply(Action{Kind: ActToggleDeleted})
	if !a.ShowDeleted {
		t.Fatalf("deleted lines did not turn on: %q", a.StatusMsg)
	}
	return a
}

func lines(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s %d", prefix, i)
	}
	return out
}

func TestRowsInterleaveRemovedLinesWithTheFile(t *testing.T) {
	a := testApp(t, "testdata/small.go", 60, 10)
	end := a.TotalLines
	g := GitChanges{
		Hunks: []Hunk{
			{0, 1, ChangeRemovedAbove}, {4, 5, ChangeRemovedBelow}, {5, 7, ChangeModified}, {end - 1, end, ChangeRemovedBelow},
		},
		Removed: []Removed{
			{0, 0, lines("top", 2)}, {5, 1, lines("mid", 3)}, {5, 2, lines("old", 1)}, {end, 3, lines("tail", 2)},
		},
	}
	a = deletedApp(t, "testdata/small.go", 60, 10, g)

	type at struct{ line, blk, idx int }
	var want []at
	for k := 0; k < 2; k++ {
		want = append(want, at{0, 0, k})
	}
	for n := 0; n < end; n++ {
		if n == 5 {
			want = append(want, at{5, 1, 0}, at{5, 1, 1}, at{5, 1, 2}, at{5, 2, 0})
		}
		want = append(want, at{n, -1, 0})
	}
	want = append(want, at{end, 3, 0}, at{end, 3, 1})

	if got := a.TotalRows(); got != len(want) {
		t.Fatalf("TotalRows = %d, want %d", got, len(want))
	}
	for row, w := range want {
		line, blk, idx := a.rowAt(row)
		if (at{line, blk, idx}) != w {
			t.Errorf("rowAt(%d) = %v, want %v", row, at{line, blk, idx}, w)
		}
		if blk < 0 && a.lineRow(line) != row {
			t.Errorf("lineRow(%d) = %d, want %d", line, a.lineRow(line), row)
		}
	}

	// Hidden, a row is a line again.
	a.Apply(Action{Kind: ActToggleDeleted})
	if a.TotalRows() != end {
		t.Errorf("hidden: TotalRows = %d, want %d", a.TotalRows(), end)
	}
	for _, n := range []int{0, 5, end - 1} {
		if line, blk, _ := a.rowAt(n); line != n || blk != -1 || a.lineRow(n) != n {
			t.Errorf("hidden: row %d maps to line %d blk %d", n, line, blk)
		}
	}
}

func sqliteDiff(t *testing.T) (GitChanges, int) {
	a := testApp(t, "testdata/sqlite3.c", 60, 31)
	end := a.TotalLines
	return GitChanges{
		Hunks:   []Hunk{{100, 101, ChangeRemovedBelow}, {500, 502, ChangeModified}, {end - 1, end, ChangeRemovedBelow}},
		Removed: []Removed{{101, 0, lines("gone", 3)}, {500, 1, lines("old", 2)}, {end, 2, lines("tail", 4)}},
	}, end
}

func TestTogglingDeletedLinesKeepsTheViewInPlace(t *testing.T) {
	const w, h = 60, 31
	g, end := sqliteDiff(t)
	a := deletedApp(t, "testdata/sqlite3.c", w, h, g)

	a.YOff = a.lineRow(700)
	a.Apply(Action{Kind: ActToggleDeleted})
	if a.YOff != 700 {
		t.Errorf("hidden: YOff %d, want 700", a.YOff)
	}
	a.Apply(Action{Kind: ActToggleDeleted})
	if a.YOff != 705 || a.topLine() != 700 {
		t.Errorf("shown: YOff %d top %d, want 705 and 700", a.YOff, a.topLine())
	}
	a.Apply(Action{Kind: ActToggleGitChanges})
	if a.YOff != 700 {
		t.Errorf("markers off: YOff %d, want 700", a.YOff)
	}
	a.Apply(Action{Kind: ActToggleGitChanges})

	// The end of the file is reachable, removed lines below the last line too.
	a.Apply(Action{Kind: ActGoBottom})
	if line, blk, idx := a.rowAt(a.YOff + a.ContentHeight() - 1); line != end || blk != 2 || idx != 3 {
		t.Errorf("bottom row is line %d blk %d idx %d, want the last removed line", line, blk, idx)
	}
	if _, to := a.VisibleRange(); to != end {
		t.Errorf("VisibleRange ends at %d, want %d", to, end)
	}
}

func TestDeletedLinesAreShownByDefault(t *testing.T) {
	const w, h = 60, 31
	g, _ := sqliteDiff(t)
	fb, err := OpenFileBuffer("testdata/sqlite3.c")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fb.Close() })
	opts := testOptions()
	opts.GitChanges, opts.Deleted = true, true
	a := NewApp(fb, opts)
	a.Apply(Action{Kind: ActResize, X: w, Y: h})

	// Before the diff: nothing to show, so s explains and the setting stays on.
	a.Apply(Action{Kind: ActToggleDeleted})
	if !a.ShowDeleted || a.StatusMsg != "git changes are still loading" {
		t.Errorf("before the diff: ShowDeleted %v %q", a.ShowDeleted, a.StatusMsg)
	}

	// The diff arriving while the user reads line 700 does not move them.
	a.YOff = 700
	a.InstallGitChanges(g)
	if !a.phantomsShown() || a.topLine() != 700 || a.YOff != 705 {
		t.Errorf("after the diff: shown %v top %d YOff %d", a.phantomsShown(), a.topLine(), a.YOff)
	}

	a.Apply(Action{Kind: ActToggleDeleted})
	if a.phantomsShown() || a.YOff != 700 {
		t.Errorf("s did not hide them: shown %v YOff %d", a.phantomsShown(), a.YOff)
	}
}

func TestToggleDeletedSaysWhyItCannot(t *testing.T) {
	a := testApp(t, "testdata/small.go", 60, 10)
	for _, c := range []struct {
		setup func()
		msg   string
	}{
		{func() {}, "git changes are off (c to show)"},
		{func() { a.ShowGitChanges = true }, "git changes are still loading"},
		{func() { a.InstallGitChanges(GitChanges{Hunks: []Hunk{{1, 2, ChangeAdded}}}) }, "no deleted lines"},
	} {
		c.setup()
		a.Apply(Action{Kind: ActToggleDeleted})
		if a.ShowDeleted || a.StatusMsg != c.msg {
			t.Errorf("ShowDeleted %v %q, want off %q", a.ShowDeleted, a.StatusMsg, c.msg)
		}
	}
}

func TestJumpingShowsTheRemovedLines(t *testing.T) {
	const w, h = 60, 31 // a jump puts its target 10 rows down
	g, _ := sqliteDiff(t)
	a := testApp(t, "testdata/sqlite3.c", w, h)
	a.ShowGitChanges = true
	a.InstallGitChanges(g)

	a.Apply(Action{Kind: ActNextChange})
	if a.YOff != 90 || a.StatusMsg != "change 1/3 · 3 removed (s to show)" {
		t.Errorf("hidden: YOff %d %q", a.YOff, a.StatusMsg)
	}

	a = deletedApp(t, "testdata/sqlite3.c", w, h, g)
	// A deletion lands on its anchor line, with the removed lines below it.
	a.Apply(Action{Kind: ActNextChange})
	if a.YOff != 90 || a.StatusMsg != "change 1/3" {
		t.Errorf("shown, deletion: YOff %d %q", a.YOff, a.StatusMsg)
	}
	// A modification lands on its old lines, which come before its new ones.
	a.Apply(Action{Kind: ActNextChange})
	if line, blk, idx := a.rowAt(a.YOff + 10); line != 500 || blk != 1 || idx != 0 {
		t.Errorf("shown, modification: anchor row is line %d blk %d idx %d", line, blk, idx)
	}
	// After scrolling by hand, the jump starts from the screen, not from a row
	// count that the removed lines have thrown off.
	a.YOff = a.lineRow(300)
	a.Apply(Action{Kind: ActNextChange})
	if a.StatusMsg != "change 2/3" {
		t.Errorf("next from line 300: %q", a.StatusMsg)
	}
	a.YOff = a.lineRow(400)
	a.Apply(Action{Kind: ActPrevChange})
	if a.StatusMsg != "change 1/3" {
		t.Errorf("prev from line 400: %q", a.StatusMsg)
	}
}

func TestRemovedLinesCannotBeSelected(t *testing.T) {
	const w, h = 60, 10
	g, _ := sqliteDiff(t)
	a := deletedApp(t, "testdata/sqlite3.c", w, h, g)
	a.YOff = a.lineRow(100) // line 100, then its three removed lines, then 101
	x := a.Layout().ContentX + 2

	a.Apply(Action{Kind: ActMouseDown, X: x, Y: 1, Button: MouseLeft})
	if a.Sel.IsVisible() {
		t.Fatalf("a click on a removed line selected %v-%v", a.Sel.Start, a.Sel.End)
	}

	a.Apply(Action{Kind: ActMouseDown, X: x, Y: 0, Button: MouseLeft})
	a.Apply(Action{Kind: ActMouseDrag, X: x, Y: 2, Button: MouseLeft})
	a.Apply(Action{Kind: ActMouseUp, X: x, Y: 2, Button: MouseLeft})
	if a.Sel.Start != (Pos{100, 2}) || a.Sel.End != (Pos{101, 0}) {
		t.Errorf("drag over removed lines selected %v-%v, want 100:2-101:0", a.Sel.Start, a.Sel.End)
	}
	if text := a.SelectedText(); strings.Contains(text, "gone") {
		t.Errorf("copied text includes a removed line: %q", text)
	}
}

func TestRemovedLinesDrawAsTintedRowsWithoutNumbers(t *testing.T) {
	const w, h = 60, 10
	g := GitChanges{
		Hunks:   []Hunk{{2, 3, ChangeModified}},
		Removed: []Removed{{2, 0, []string{"old one", "old\ttwo"}}},
	}
	a := deletedApp(t, "testdata/sqlite3.c", w, h, g)
	buf := frame(t, a, w, h)
	l := a.Layout()
	red := theme.RemovedTint()

	for y, want := range map[uint16]string{2: "old one", 3: "old two"} {
		if got := cellsAcross(buf, l.ContentX, y, Col(len(want))); got != want {
			t.Errorf("row %d shows %q, want %q", y, got, want)
		}
		if got := strings.TrimSpace(cellsAcross(buf, 0, y, Col(l.Gutter-1))); got != "" {
			t.Errorf("row %d has line number %q", y, got)
		}
		if got := buf.CellAt(l.Gutter-1, y).GetSymbol(); got != markerChanged {
			t.Errorf("row %d marker %q, want %q", y, got, markerChanged)
		}
		for _, x := range []uint16{l.ContentX, l.ContentX + l.ContentW - 1} {
			if got := buf.CellAt(x, y).GetStyle().GetBg(); got != red {
				t.Errorf("row %d col %d background %v, want %v", y, x, got, red)
			}
		}
	}
	if got := strings.TrimSpace(cellsAcross(buf, 0, 4, Col(l.Gutter-1))); got != "3" {
		t.Errorf("line after the removed rows is numbered %q, want 3", got)
	}
	if got := buf.CellAt(l.ContentX, 4).GetStyle().GetBg(); got != theme.Palette.Bg {
		t.Errorf("a line not in the current hunk is tinted %v", got)
	}
}

func TestCurrentHunkIsTinted(t *testing.T) {
	const w, h = 60, 10 // 9 content rows: a jump puts its target 3 rows down
	a := changesApp(t, w, h, Hunk{5, 7, ChangeAdded}, Hunk{12, 13, ChangeRemovedBelow})
	l := a.Layout()
	bgAt := func(x, y uint16) any { return frame(t, a, w, h).CellAt(x, y).GetStyle().GetBg() }
	green := theme.ChangeTint(ChangeAdded)

	if got := bgAt(l.ContentX, 5); got != theme.Palette.Bg {
		t.Fatalf("tinted before any jump: %v", got)
	}

	a.Apply(Action{Kind: ActNextChange})
	if a.YOff != 2 {
		t.Fatalf("YOff %d, want 2", a.YOff)
	}
	buf := frame(t, a, w, h)
	for _, y := range []uint16{3, 4} {
		for _, x := range []uint16{l.ContentX, l.ContentX + l.ContentW - 1} {
			if got := buf.CellAt(x, y).GetStyle().GetBg(); got != green {
				t.Errorf("row %d col %d background %v, want %v", y, x, got, green)
			}
		}
		if got := buf.CellAt(l.Gutter-2, y).GetStyle().GetFg(); got != theme.GitMarker(ChangeAdded).GetFg() {
			t.Errorf("row %d number colour %v, want the marker's", y, got)
		}
	}
	for _, y := range []uint16{2, 5} {
		if got := buf.CellAt(l.ContentX, y).GetStyle().GetBg(); got != theme.Palette.Bg {
			t.Errorf("row %d outside the hunk is tinted %v", y, got)
		}
	}

	// The selection shows through the tint.
	a.Sel.Begin(Pos{5, 0})
	a.Sel.Extend(Pos{5, 2})
	a.Sel.Finish()
	if got := bgAt(l.ContentX, 3); got != theme.Palette.SelBg {
		t.Errorf("selected cell in the hunk has background %v, want the selection's", got)
	}
	a.Sel.Clear()

	// A deletion has no lines of its own, so its anchor line takes the tint.
	a.Apply(Action{Kind: ActNextChange})
	if a.YOff != 9 {
		t.Fatalf("YOff %d, want 9", a.YOff)
	}
	if got := bgAt(l.ContentX, 3); got != theme.ChangeTint(ChangeRemovedBelow) {
		t.Errorf("deletion anchor background %v, want red", got)
	}

	// Hiding the markers forgets the current hunk.
	a.Apply(Action{Kind: ActToggleGitChanges})
	a.Apply(Action{Kind: ActToggleGitChanges})
	if got := bgAt(l.ContentX, 3); got != theme.Palette.Bg {
		t.Errorf("still tinted after toggling markers: %v", got)
	}
}

func TestTintsBlendIntoTheBackground(t *testing.T) {
	th := NewTheme(palettes["dracula"])
	kinds := []ChangeKind{ChangeAdded, ChangeModified, ChangeRemovedBelow}
	seen := map[any]bool{}
	for _, k := range kinds {
		c := th.ChangeTint(k)
		if _, _, _, ok := c.RGB(); !ok || c == th.Palette.Bg {
			t.Errorf("kind %v tint %v", k, c)
		}
		seen[c] = true
	}
	if len(seen) != len(kinds) {
		t.Errorf("tints are not distinct: %v", seen)
	}

	base16 := NewTheme(palettes["base16"])
	for _, k := range kinds {
		if _, ok := base16.ChangeTint(k).Index(); !ok {
			t.Errorf("base16 kind %v tint %v, want a 256-colour index", k, base16.ChangeTint(k))
		}
	}
	if th.ChangeTint(ChangeNone).IsSet() {
		t.Error("an unchanged line has a tint")
	}
}
