package main

import (
	"os"
	"strings"
	"testing"
)

func TestSearchFindsEveryOccurrence(t *testing.T) {
	fb, err := OpenFileBuffer("testdata/sample.go")
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()

	// Count by hand from the file itself, so the expectation cannot drift.
	needle := "func"
	want := 0
	for n := 0; n < fb.LineCount(); n++ {
		want += strings.Count(strings.ToLower(fb.Line(n)), needle)
	}
	if got := len(scanMatches(fb, needle, 4)); got != want {
		t.Errorf("found %d matches, want %d", got, want)
	}
}

func TestSearchIsCaseInsensitive(t *testing.T) {
	fb, err := OpenFileBuffer("testdata/sample.go")
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()
	lower := len(scanMatches(fb, "func", 4))
	upper := len(scanMatches(fb, "FUNC", 4))
	if lower != upper || lower == 0 {
		t.Errorf("case-insensitive search disagreed: %d vs %d", lower, upper)
	}
}

// TestSearchColumnsAreDisplayColumnsNotByteOffsets is the bug the port fixes.
// The bubbletea original recorded a match's column as a byte index, so on a
// line with multi-byte text before the match the highlight landed to the right
// of where it belonged.
func TestSearchColumnsAreDisplayColumnsNotByteOffsets(t *testing.T) {
	dir := t.TempDir() + "/multibyte.txt"
	// 日本語 is nine bytes but six display columns, so a byte index would put
	// the match at column 9 rather than 6.
	writeFile(t, dir, "日本語needle\n")

	fb, err := OpenFileBuffer(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()

	matches := scanMatches(fb, "needle", 4)
	if len(matches) != 1 {
		t.Fatalf("found %d matches, want 1", len(matches))
	}
	if matches[0].Col != 6 {
		t.Errorf("match column = %d, want 6 display columns (9 would be the byte offset)", matches[0].Col)
	}
}

func TestSearchColumnsAccountForTabs(t *testing.T) {
	path := t.TempDir() + "/tabs.txt"
	writeFile(t, path, "\t\tneedle\n")

	fb, err := OpenFileBuffer(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()

	matches := scanMatches(fb, "needle", 4)
	if len(matches) != 1 {
		t.Fatalf("found %d matches, want 1", len(matches))
	}
	// Two tabs at width 4 put the text at column 8, not byte offset 2.
	if matches[0].Col != 8 {
		t.Errorf("match column = %d, want 8", matches[0].Col)
	}
}

func TestOverlappingCandidatesAdvancePastEachMatch(t *testing.T) {
	path := t.TempDir() + "/aaa.txt"
	writeFile(t, path, "aaaa\n")

	fb, err := OpenFileBuffer(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()

	// Non-overlapping: "aa" in "aaaa" is two matches, at columns 0 and 2.
	matches := scanMatches(fb, "aa", 4)
	if len(matches) != 2 || matches[0].Col != 0 || matches[1].Col != 2 {
		t.Errorf("got %v, want matches at columns 0 and 2", matches)
	}
}

func TestAnEmptyNeedleFindsNothing(t *testing.T) {
	fb, err := OpenFileBuffer("testdata/sample.go")
	if err != nil {
		t.Fatal(err)
	}
	defer fb.Close()
	if got := scanMatches(fb, "", 4); got != nil {
		t.Errorf("empty needle found %d matches", len(got))
	}
}

// TestSearchSteppingWrapsBothWays covers n and N at the ends of the list.
func TestSearchSteppingWrapsBothWays(t *testing.T) {
	a := testApp(t, "testdata/sample.go", 80, 24)
	a.Needle = "func"
	a.runSearch()
	if len(a.Matches) < 2 {
		t.Skip("need at least two matches")
	}

	a.MatchIdx = len(a.Matches) - 1
	a.Apply(Action{Kind: ActNextMatch})
	if a.MatchIdx != 0 {
		t.Errorf("next from the last match went to %d, want 0", a.MatchIdx)
	}
	a.Apply(Action{Kind: ActPrevMatch})
	if a.MatchIdx != len(a.Matches)-1 {
		t.Errorf("prev from the first match went to %d, want %d", a.MatchIdx, len(a.Matches)-1)
	}
}

func TestCommittingASearchSelectsTheMatch(t *testing.T) {
	a := testApp(t, "testdata/sample.go", 80, 24)
	a.Apply(Action{Kind: ActOpenSearch})
	for _, r := range "func" {
		a.Apply(Action{Kind: ActPromptKey, Key: PromptInsert, Rune: r})
	}
	a.Apply(Action{Kind: ActPromptCommit})

	if a.Mode != ModeNormal {
		t.Error("committing should leave search mode")
	}
	if len(a.Matches) == 0 {
		t.Fatal("no matches found")
	}
	if !a.Sel.Active {
		t.Error("the current match should be selected")
	}
	m := a.Matches[a.MatchIdx]
	if a.Sel.Start != (Pos{m.Line, m.Col}) {
		t.Errorf("selection starts at %v, want %v", a.Sel.Start, Pos{m.Line, m.Col})
	}
}

func TestAFailedSearchReportsItself(t *testing.T) {
	a := testApp(t, "testdata/sample.go", 80, 24)
	a.Needle = "definitely-not-in-this-file"
	a.runSearch()
	if !strings.Contains(a.StatusMsg, "no match") {
		t.Errorf("status = %q, want a no-match message", a.StatusMsg)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
