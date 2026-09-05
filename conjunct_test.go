package main

import (
	"strings"
	"testing"

	"github.com/Fiend3d/catatui"
)

// clusterTexts is the line broken into the clusters the table records.
func clusterTexts(line string) []string {
	tab := tableFor(line, 4)
	out := make([]string, 0, len(tab.Clusters()))
	for i := range tab.Clusters() {
		out = append(out, tab.ClusterStr(line, i))
	}
	return out
}

// TestIndicConjunctsAreOneCluster is the bug this file exists for: selecting
// Telugu did not select whole glyphs.
//
// uniseg stops at GB9b, so it hands a conjunct over in pieces — क् then ष —
// and a selection edge could land between them. Half a ligature would then be
// highlighted, and since the terminal groups cells by their attributes before
// shaping them, the two halves were drawn separately instead of as the
// conjunct. GB9c binds them, and the table is where that has to happen: it is
// what selection endpoints snap to, what word bounds walk, and what the copy
// path converts through.
func TestIndicConjunctsAreOneCluster(t *testing.T) {
	for _, c := range []struct {
		line string
		want []string
	}{
		{"परीक्षण", []string{"प", "री", "क्ष", "ण"}},
		{"हिन्दी", []string{"हि", "न्दी"}},
		{"పరీక్ష", []string{"ప", "రీ", "క్ష"}},
		{"నమస్కారం", []string{"న", "మ", "స్కా", "రం"}},
		{"বাংলা পরীক্ষা", []string{"বাং", "লা", " ", "প", "রী", "ক্ষা"}},
	} {
		got := clusterTexts(c.line)
		if len(got) != len(c.want) {
			t.Errorf("%q split into %d clusters %q, want %d %q",
				c.line, len(got), got, len(c.want), c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q: cluster %d = %q, want %q", c.line, i, got[i], c.want[i])
			}
		}
	}
}

// Joined clusters and the cells sent to the terminal must agree on width.
func TestJoinedConjunctLayoutMatchesRenderedCells(t *testing.T) {
	for _, line := range []string{
		"हिन्दी परीक्षण पाठ।",
		"తెలుగు పరీక్ష వచనం.",
		"నమస్కారం, మీరు ఎలా ఉన్నారు?",
		"বাংলা পরীক্ষা লেখা।",
	} {
		tab := tableFor(line, 4)
		buf := catatui.NewBuffer(catatui.NewRect(0, 0, 80, 1))
		renderRow(buf, 0, 0, &Row{Line: line, Table: tab, Width: 80}, testTheme())
		var col Col
		for _, c := range tab.Clusters() {
			if Col(c.Col) != col {
				t.Fatalf("%q: gap or overlap at column %d", line, c.Col)
			}
			col += Col(buf.CellAt(uint16(c.Col), 0).Width())
		}
		if col != tab.Width() {
			t.Errorf("%q: columns end at %d, width is %d", line, col, tab.Width())
		}
	}
}

// TestALinkerJoinsOnlyAConsonant keeps the rule from swallowing whatever
// follows a virama. A word-final virama is ordinary Tamil and Devanagari.
func TestALinkerJoinsOnlyAConsonant(t *testing.T) {
	for _, c := range []struct {
		line string
		want int // clusters
	}{
		{"तमिल् ", 4}, // virama then a space
		{"क्", 1},     // virama at the end of the line
		{"क् आम", 4},  // virama, space, independent vowel
		{"क्आ", 2},    // virama then an independent vowel, not a consonant
		{"தமிழ்", 3},  // Tamil: its virama is not in the property
		{"ಕನ್ನಡ", 4},  // Kannada: likewise
		{"ਸ੍ਵਰ", 3},   // Gurmukhi: likewise
	} {
		if got := len(clusterTexts(c.line)); got != c.want {
			t.Errorf("%q split into %d clusters %q, want %d",
				c.line, got, clusterTexts(c.line), c.want)
		}
	}
}

// TestASpacingMarkEndsTheLeftContext pins the part of GB9c that is easy to lose
// when the rule is approximated: the run between the consonant and its linker
// admits combining marks but not spacing ones.
func TestASpacingMarkEndsTheLeftContext(t *testing.T) {
	if !joinsConjunct("क्", "ष") {
		t.Error("consonant plus virama should join a following consonant")
	}
	if !joinsConjunct("क्‍", "ष") {
		t.Error("an explicit joiner after the virama should not break the rule")
	}
	if joinsConjunct("का", "ष") {
		t.Error("a cluster with no linker joined anyway")
	}
	if joinsConjunct("का्", "ष") {
		t.Error("a spacing vowel sign before the virama should end the left context")
	}
	if joinsConjunct("ि", "ष") {
		t.Error("a cluster that does not start with a consonant joined anyway")
	}
	if joinsConjunct("क्", "ि") {
		t.Error("a linker joined something that is not a consonant")
	}
}

// TestASelectionEdgeCannotLandInsideAConjunct is the user-visible form: every
// column of a Telugu line, snapped, is a cluster boundary — so a drag can stop
// before or after क्ष but never inside it.
func TestASelectionEdgeCannotLandInsideAConjunct(t *testing.T) {
	for _, line := range []string{
		"తెలుగు పరీక్ష వచనం.",
		"నమస్కారం, మీరు ఎలా ఉన్నారు?",
		"हिन्दी परीक्षण पाठ।",
	} {
		tab := tableFor(line, 4)
		boundary := map[Col]bool{tab.Width(): true}
		for _, c := range tab.Clusters() {
			boundary[Col(c.Col)] = true
		}
		for col := Col(0); col <= tab.Width(); col++ {
			if got := tab.SnapCol(col); !boundary[got] {
				t.Errorf("%q: SnapCol(%d) = %d, which is inside a cluster", line, col, got)
			}
		}
	}
}

// TestWordBoundsCoverAWholeConjunct: a double click on a Telugu word takes the
// whole of it, conjuncts included.
func TestWordBoundsCoverAWholeConjunct(t *testing.T) {
	line := "పరీక్ష వచనం"
	tab := tableFor(line, 4)
	start, end := FindWordBounds(line, tab, 0)
	if start != 0 {
		t.Fatalf("word starts at %d, want 0", start)
	}
	word := line[tab.ColToByte(start):tab.ColToByte(end)]
	if want := "పరీక్ష"; word != want {
		t.Errorf("word = %q, want %q", word, want)
	}
	if strings.ContainsRune(word, ' ') {
		t.Errorf("word %q ran past the space", word)
	}
}

// TestADragStoppingInsideAConjunctTakesAllOfIt is the whole thing end to end:
// press at the start of a Telugu line, release with the pointer over the second
// half of a conjunct, and the selection has to cover the conjunct or none of it
// — never the क् without the ष.
func TestADragStoppingInsideAConjunctTakesAllOfIt(t *testing.T) {
	a := testApp(t, "testdata/unicode.txt", 80, 24)

	row, inside := -1, Col(-1)
	for n := 0; n < a.TotalLines && row < 0; n++ {
		line := a.fb.Line(n)
		if !strings.ContainsFunc(line, func(r rune) bool { return r >= 0x0C00 && r <= 0x0C7F }) {
			continue
		}
		tab := tableFor(line, a.TabWidth)
		for i, c := range tab.Clusters() {
			// A conjunct: more than one uniseg cluster joined into one.
			if c.Width > 1 && strings.ContainsFunc(tab.ClusterStr(line, i), isIndicLinker) {
				row, inside = n, Col(c.Col)+1
				break
			}
		}
	}
	if row < 0 {
		t.Skip("no Telugu conjunct in the corpus")
	}

	a.YOff = max(row-2, 0)
	drag(a, row, 0, uint16(inside))

	line := a.fb.Line(row)
	tab := tableFor(line, a.TabWidth)
	buf := frame(t, a, 80, 24)
	l := a.Layout()
	y := uint16(row - a.YOff)

	// Every conjunct occupies one buffer cell with blank continuation columns.
	// The terminal paints those columns from the complete symbol's leading cell.
	for i, c := range tab.Clusters() {
		var first, known bool
		for col := Col(c.Col); col < c.EndCol(); {
			cell := buf.CellAt(l.ContentX+uint16(col), y)
			sel := cell.GetStyle().GetBg() == theme.Palette.SelBg
			switch {
			case !known:
				first, known = sel, true
			case sel != first:
				t.Errorf("cluster %q at column %d is only half selected",
					tab.ClusterStr(line, i), c.Col)
			}
			col += max(Col(cell.Width()), 1)
		}
	}
	if txt := a.SelectedText(); strings.HasSuffix(txt, "\u0c4d") {
		t.Errorf("the copied text ends on a bare virama: %q", txt)
	}
}
