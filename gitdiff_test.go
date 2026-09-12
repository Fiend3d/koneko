package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseDiffMapsHunkHeadersToLines(t *testing.T) {
	cases := []struct {
		name string
		diff string
		want []Hunk
	}{
		{"added", "@@ -3,0 +4,2 @@\n+a\n+b\n", []Hunk{{3, 5, ChangeAdded}}},
		{"modified", "@@ -2,2 +2,3 @@ func f() {\n-a\n-b\n+A\n+B\n+C\n", []Hunk{{1, 4, ChangeModified}}},
		{"omitted counts", "@@ -7 +7 @@\n-a\n+b\n", []Hunk{{6, 7, ChangeModified}}},
		{"removed below", "@@ -5,2 +4,0 @@\n-a\n-b\n", []Hunk{{3, 4, ChangeRemovedBelow}}},
		{"removed at top", "@@ -1,2 +0,0 @@\n-a\n-b\n", []Hunk{{0, 1, ChangeRemovedAbove}}},
		{
			"several hunks with file headers",
			"diff --git a/x b/x\nindex 1..2 100644\n--- a/x\n+++ b/x\n" +
				"@@ -1 +1 @@\n-a\n+b\n@@ -10,0 +11 @@\n+c\n@@ -20 +20,0 @@\n-d\n",
			[]Hunk{{0, 1, ChangeModified}, {10, 11, ChangeAdded}, {19, 20, ChangeRemovedBelow}},
		},
		{
			// A content line longer than the reader's 4096-byte buffer comes back
			// in pieces, and here the second piece starts with "@@ -". Only a
			// piece that starts a line may be taken for a header.
			"overlong content line",
			"@@ -1 +1 @@\n-" + strings.Repeat("x", 4095) + "@@ -50 +50 @@\n+y\n@@ -9 +9 @@\n-a\n+b\n",
			[]Hunk{{0, 1, ChangeModified}, {8, 9, ChangeModified}},
		},
		{"deletion on an added line is dropped", "@@ -2,0 +3,2 @@\n+a\n+b\n@@ -3 +4,0 @@\n-c\n", []Hunk{{2, 4, ChangeAdded}}},
		{"empty", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseDiff(strings.NewReader(c.diff)).Hunks
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestGitChangesAtRespectsHunkBounds(t *testing.T) {
	g := GitChanges{Hunks: []Hunk{{2, 4, ChangeAdded}, {4, 5, ChangeRemovedBelow}, {9, 12, ChangeModified}}}
	want := map[int]ChangeKind{
		0: ChangeNone, 1: ChangeNone, 2: ChangeAdded, 3: ChangeAdded, 4: ChangeRemovedBelow,
		5: ChangeNone, 8: ChangeNone, 9: ChangeModified, 11: ChangeModified, 12: ChangeNone,
	}
	for line, kind := range want {
		if got := g.At(line); got != kind {
			t.Errorf("At(%d) = %v, want %v", line, got, kind)
		}
	}
	var empty GitChanges
	if got := empty.At(0); got != ChangeNone {
		t.Errorf("empty At(0) = %v", got)
	}
}

func TestNextAndPrevWrapAround(t *testing.T) {
	g := GitChanges{Hunks: []Hunk{{5, 6, ChangeAdded}, {20, 21, ChangeAdded}, {40, 41, ChangeAdded}}}
	for _, c := range []struct{ row, next, prev int }{
		{0, 0, 2}, {5, 1, 2}, {6, 1, 0}, {20, 2, 0}, {39, 2, 1}, {40, 0, 1}, {100, 0, 2},
	} {
		if got := g.Next(c.row); got != c.next {
			t.Errorf("Next(%d) = %d, want %d", c.row, got, c.next)
		}
		if got := g.Prev(c.row); got != c.prev {
			t.Errorf("Prev(%d) = %d, want %d", c.row, got, c.prev)
		}
	}
	var empty GitChanges
	if empty.Next(0) != -1 || empty.Prev(0) != -1 {
		t.Error("an empty change list should have no next or previous hunk")
	}
}

// git runs a git command in dir, failing the test if it does.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestLoadGitChangesDiffsAWorkingFileAgainstHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	git(t, dir, "init", "-q")
	git(t, dir, "config", "core.autocrlf", "false")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "test")

	if err := os.WriteFile(path, []byte("a\nb\nc\nd\ne\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "f.txt")
	git(t, dir, "commit", "-q", "-m", "init")

	// Modify b, delete d, append f.
	if err := os.WriteFile(path, []byte("a\nB\nc\ne\nf\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := make(chan GitChanges, 1)
	loadGitChanges(context.Background(), path, out)
	got := (<-out).Hunks
	want := []Hunk{{1, 2, ChangeModified}, {2, 3, ChangeRemovedBelow}, {4, 5, ChangeAdded}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadGitChangesOutsideARepositoryIsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := make(chan GitChanges, 1)
	loadGitChanges(context.Background(), path, out)
	if got := (<-out).Hunks; len(got) != 0 {
		t.Errorf("got %v, want no hunks", got)
	}
}

// changesApp is a test app over the sqlite corpus with a synthetic diff
// installed, as if git had reported it.
func changesApp(t *testing.T, w, h uint16, hunks ...Hunk) *App {
	t.Helper()
	a := testApp(t, "testdata/sqlite3.c", w, h)
	a.ShowGitChanges = true
	a.InstallGitChanges(GitChanges{Hunks: hunks})
	return a
}

func TestChangeMarkersDrawInTheGutterSeparator(t *testing.T) {
	const w, h = 60, 10
	a := changesApp(t, w, h,
		Hunk{1, 3, ChangeAdded}, Hunk{4, 5, ChangeModified}, Hunk{6, 7, ChangeRemovedBelow})
	buf := frame(t, a, w, h)
	x := a.Layout().Gutter - 1

	want := []struct {
		sym  string
		kind ChangeKind
	}{
		{" ", ChangeNone}, {markerChanged, ChangeAdded}, {markerChanged, ChangeAdded}, {" ", ChangeNone},
		{markerChanged, ChangeModified}, {" ", ChangeNone}, {markerRemovedBelow, ChangeRemovedBelow},
	}
	for y, cell := range want {
		c := buf.CellAt(x, uint16(y))
		if c.GetSymbol() != cell.sym {
			t.Errorf("row %d: marker %q, want %q", y, c.GetSymbol(), cell.sym)
		}
		if wantFg := theme.GitMarker(cell.kind).GetFg(); cell.kind != ChangeNone && c.GetStyle().GetFg() != wantFg {
			t.Errorf("row %d: marker colour %v, want %v", y, c.GetStyle().GetFg(), wantFg)
		}
	}

	// The number is still there, right-aligned before the marker.
	if got := cellsAcross(buf, 0, 1, Col(x)); strings.TrimSpace(got) != "2" {
		t.Errorf("gutter number = %q, want 2", got)
	}

	a.Apply(Action{Kind: ActToggleGitChanges})
	buf = frame(t, a, w, h)
	if got := buf.CellAt(x, 1).GetSymbol(); got != " " {
		t.Errorf("markers toggled off, but row 1 shows %q", got)
	}
}

func TestChangeMarkersGetAColumnWhenLineNumbersAreHidden(t *testing.T) {
	const w, h = 60, 10
	a := changesApp(t, w, h, Hunk{0, 1, ChangeRemovedAbove})
	a.Apply(Action{Kind: ActToggleLineNumbers})
	if g := a.Layout().Gutter; g != 1 {
		t.Fatalf("gutter = %d, want 1", g)
	}
	buf := frame(t, a, w, h)
	if got := buf.CellAt(0, 0).GetSymbol(); got != markerRemovedAbove {
		t.Errorf("marker = %q, want %q", got, markerRemovedAbove)
	}
	if got := buf.CellAt(0, 1).GetSymbol(); got != " " {
		t.Errorf("unchanged row's marker cell = %q, want blank", got)
	}
	for y := uint16(0); y < h; y++ {
		for x := uint16(0); x < w; x++ {
			if !buf.CellAt(x, y).GetStyle().GetBg().IsSet() {
				t.Fatalf("cell (%d,%d) was never written", x, y)
			}
		}
	}

	// No changes, no column.
	a.InstallGitChanges(GitChanges{})
	if g := a.Layout().Gutter; g != 0 {
		t.Errorf("gutter with no changes = %d, want 0", g)
	}
}

func TestJumpingBetweenChangesScrollsAndWraps(t *testing.T) {
	const w, h = 60, 31 // 30 content rows: a jump puts its target 10 rows down
	a := testApp(t, "testdata/sqlite3.c", w, h)
	end := a.TotalLines
	// The last two hunks both fit on the final screen, where the view cannot
	// scroll any further, so stepping between them must not depend on scrolling.
	a = changesApp(t, w, h,
		Hunk{100, 101, ChangeAdded}, Hunk{500, 510, ChangeModified},
		Hunk{end - 8, end - 7, ChangeAdded}, Hunk{end - 2, end - 1, ChangeRemovedBelow})
	bottom := end - 30

	steps := []struct {
		act  ActionKind
		yOff int
		msg  string
	}{
		{ActNextChange, 90, "change 1/4"},
		{ActNextChange, 490, "change 2/4"},
		{ActNextChange, bottom, "change 3/4"},
		{ActNextChange, bottom, "change 4/4"},
		{ActNextChange, 90, "change 1/4"},
		{ActPrevChange, bottom, "change 4/4"},
		{ActPrevChange, bottom, "change 3/4"},
		{ActPrevChange, 490, "change 2/4"},
	}
	for i, s := range steps {
		a.Apply(Action{Kind: s.act})
		if a.YOff != s.yOff || a.StatusMsg != s.msg {
			t.Fatalf("step %d: YOff %d %q, want %d %q", i, a.YOff, a.StatusMsg, s.yOff, s.msg)
		}
	}

	// After scrolling by hand, jumps are relative to the screen again.
	a.YOff = 300
	a.Apply(Action{Kind: ActNextChange})
	if a.YOff != 490 {
		t.Errorf("next from 300: YOff %d, want 490", a.YOff)
	}
	a.YOff = 300
	a.Apply(Action{Kind: ActPrevChange})
	if a.YOff != 90 {
		t.Errorf("prev from 300: YOff %d, want 90", a.YOff)
	}
}

func TestJumpingReportsWhenThereIsNothingToJumpTo(t *testing.T) {
	a := testApp(t, "testdata/small.go", 60, 10)
	a.Apply(Action{Kind: ActNextChange})
	if a.StatusMsg != "git changes are off (c to show)" {
		t.Errorf("off: %q", a.StatusMsg)
	}

	loads := 0
	a.AttachGitLoader(func() { loads++ })
	a.Apply(Action{Kind: ActToggleGitChanges})
	a.Apply(Action{Kind: ActToggleGitChanges})
	a.Apply(Action{Kind: ActToggleGitChanges})
	if loads != 1 {
		t.Errorf("git was started %d times, want once", loads)
	}
	a.Apply(Action{Kind: ActNextChange})
	if a.StatusMsg != "git changes are still loading" {
		t.Errorf("loading: %q", a.StatusMsg)
	}

	a.InstallGitChanges(GitChanges{})
	a.Apply(Action{Kind: ActNextChange})
	if a.StatusMsg != "no changes" {
		t.Errorf("clean file: %q", a.StatusMsg)
	}
}
