package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestASingleClickLeavesNoSelection(t *testing.T) {
	a := testApp(t, "testdata/small.go", 80, 24)
	l := a.Layout()
	a.Apply(Action{Kind: ActMouseDown, X: l.ContentX + 3, Y: 0, Button: MouseLeft})
	a.Apply(Action{Kind: ActMouseUp, X: l.ContentX + 3, Y: 0, Button: MouseLeft})
	if a.Sel.IsVisible() {
		t.Errorf("a single click selected %v..%v", a.Sel.Start, a.Sel.End)
	}
}

func TestAGutterClickOnABlankLineSelectsIt(t *testing.T) {
	a := testApp(t, "testdata/small.go", 80, 24)
	// Line 2 of small.go is empty.
	a.Apply(Action{Kind: ActMouseDown, X: 0, Y: 1, Button: MouseLeft})
	a.Apply(Action{Kind: ActMouseUp, X: 0, Y: 1, Button: MouseLeft})
	if !a.Sel.Active {
		t.Fatal("a gutter click on a blank line left no selection")
	}
	if _, _, ok := a.Sel.RowSpan(1, 0); !ok {
		t.Error("the blank line is not painted as selected")
	}
	if _, _, ok := a.Sel.RowSpan(2, a.LineWidth(2)); ok {
		t.Error("the line after the blank one is painted as selected")
	}
	if got := a.Reference(); got != "testdata/small.go:2" {
		t.Errorf("reference = %q, want testdata/small.go:2", got)
	}
}

func TestReferenceNamesTheSelectedLines(t *testing.T) {
	if _, ok := findGitRoot("."); !ok {
		t.Skip("not running inside the repository")
	}
	a := testApp(t, "testdata/small.go", 80, 24)
	if got := a.Reference(); got != "testdata/small.go" {
		t.Errorf("no selection: %q, want the bare path", got)
	}

	cases := []struct {
		start, end Pos
		want       string
	}{
		{Pos{2, 0}, Pos{2, 5}, "testdata/small.go:3"},
		{Pos{2, 3}, Pos{4, 2}, "testdata/small.go:3-5"},
		// Stopping at the start of line 6 takes none of it.
		{Pos{2, 0}, Pos{5, 0}, "testdata/small.go:3-5"},
	}
	for _, c := range cases {
		a.Sel.Begin(c.start)
		a.Sel.Extend(c.end)
		a.Sel.Finish()
		if got := a.Reference(); got != c.want {
			t.Errorf("%v..%v: %q, want %q", c.start, c.end, got, c.want)
		}
	}
}

func TestFindGitRootTakesTheNearestRepository(t *testing.T) {
	base := t.TempDir()
	outer := filepath.Join(base, "outer")
	inner := filepath.Join(outer, "sub")
	deep := filepath.Join(inner, "a", "b")
	if err := os.MkdirAll(filepath.Join(outer, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	if root, ok := findGitRoot(deep); !ok || root != outer {
		t.Errorf("directory .git: got %q %v, want %q", root, ok, outer)
	}

	// A submodule or worktree marks its root with a .git file.
	if err := os.WriteFile(filepath.Join(inner, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if root, ok := findGitRoot(deep); !ok || root != inner {
		t.Errorf("file .git: got %q %v, want %q", root, ok, inner)
	}
}

func TestReferencePathOutsideARepositoryIsAbsolute(t *testing.T) {
	dir := t.TempDir()
	if _, ok := findGitRoot(dir); ok {
		t.Skip("the temp directory is inside a repository")
	}
	path := filepath.Join(dir, "notes.txt")
	got := referencePath(path)
	if got != filepath.ToSlash(path) {
		t.Errorf("got %q, want %q", got, filepath.ToSlash(path))
	}
	if strings.Contains(got, `\`) {
		t.Errorf("%q contains a backslash", got)
	}
}
