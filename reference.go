// Copying a path:line reference to the clipboard, in the form tools such as
// Claude Code accept: src/app.go:12 or src/app.go:12-18.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
)

// findGitRoot walks up from dir to the nearest directory holding a .git entry.
// A worktree or submodule has a .git file rather than a directory, so either
// counts. Walking the tree rather than asking git keeps the key instant, and it
// works under -no-git, which only concerns the change markers.
func findGitRoot(dir string) (string, bool) {
	for d := dir; ; {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}

// referencePath is path relative to its repository root, or absolute when it is
// not inside one. Separators are always forward slashes, so a reference reads
// the same on every platform.
func referencePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	if root, ok := findGitRoot(filepath.Dir(abs)); ok {
		rel, err := filepath.Rel(root, abs)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(abs)
}

// Reference is the file's path with the selected lines, one-based: path:12 for
// one line, path:12-18 for several, and the bare path with no selection.
func (a *App) Reference() string {
	if a.refPath == "" {
		a.refPath = referencePath(a.fb.Path())
	}
	if !a.Sel.IsVisible() {
		return a.refPath
	}
	first, last := a.Sel.Start.Row+1, a.Sel.End.Row+1
	// A selection that stops at the start of a line takes none of that line,
	// only the newline before it.
	if a.Sel.End.Col == 0 && a.Sel.End.Row > a.Sel.Start.Row {
		last--
	}
	if first == last {
		return fmt.Sprintf("%s:%d", a.refPath, first)
	}
	return fmt.Sprintf("%s:%d-%d", a.refPath, first, last)
}

// copyReference puts Reference on the system clipboard.
func (a *App) copyReference() {
	ref := a.Reference()
	if err := clipboard.WriteAll(ref); err != nil {
		a.StatusMsg = "copy failed: " + err.Error()
		return
	}
	a.StatusMsg = "copied " + ref
}
