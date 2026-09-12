package main

import (
	"testing"

	"github.com/Fiend3d/catatui/term"
)

func keyRune(r rune) term.Event {
	return term.Event{Kind: term.EventKey, Key: term.KeyRune, Rune: r}
}

func key(k term.KeyCode) term.Event {
	return term.Event{Kind: term.EventKey, Key: k}
}

// TestHelpIsModal is the bug the port fixes. In the bubbletea original, keys the
// help overlay did not handle fell through to the viewer, so pressing y inside
// help copied the selection underneath it.
func TestHelpIsModal(t *testing.T) {
	swallowed := []term.Event{
		keyRune('y'), keyRune('a'), keyRune('d'), keyRune('x'),
		keyRune('l'), keyRune('s'), keyRune('h'), keyRune('/'),
		keyRune('n'), keyRune('N'), keyRune('H'),
	}
	for _, ev := range swallowed {
		if act := Decode(ev, ModeHelp); act.Kind != ActNone {
			t.Errorf("%q in help decoded to action %d, want none", ev.Rune, act.Kind)
		}
	}
}

func TestHelpClosesOnItsOwnKeys(t *testing.T) {
	for _, ev := range []term.Event{key(term.KeyF1), key(term.KeyEscape), keyRune('q')} {
		if act := Decode(ev, ModeHelp); act.Kind != ActCloseHelp {
			t.Errorf("event %v decoded to %d, want ActCloseHelp", ev, act.Kind)
		}
	}
}

func TestHelpStillScrolls(t *testing.T) {
	if act := Decode(keyRune('j'), ModeHelp); act.Kind != ActScrollLines || act.N != 1 {
		t.Errorf("j in help = %+v, want a one-line scroll", act)
	}
	if act := Decode(keyRune('k'), ModeHelp); act.Kind != ActScrollLines || act.N != -1 {
		t.Errorf("k in help = %+v, want a one-line scroll up", act)
	}
}

// TestHelpScrollingDoesNotMoveTheDocument keeps the two offsets separate.
func TestHelpScrollingDoesNotMoveTheDocument(t *testing.T) {
	a := testApp(t, "testdata/sample.go", 80, 24)
	a.Apply(Action{Kind: ActScrollLines, N: 5})
	before := a.YOff

	a.Apply(Action{Kind: ActOpenHelp})
	for i := 0; i < 10; i++ {
		a.Apply(Decode(keyRune('j'), a.Mode))
	}
	if a.YOff != before {
		t.Errorf("document scrolled to %d while help was open, want %d", a.YOff, before)
	}
	if a.HelpOff == 0 {
		t.Error("the help overlay did not scroll")
	}
}

func TestHelpScrollingStopsAtTheEnd(t *testing.T) {
	a := testApp(t, "testdata/sample.go", 80, 24)
	a.Apply(Action{Kind: ActOpenHelp})
	for i := 0; i < len(helpLines)*2; i++ {
		a.Apply(Action{Kind: ActScrollLines, N: 1})
	}
	if a.HelpOff != a.helpMaxOffset() {
		t.Errorf("help offset = %d, want %d", a.HelpOff, a.helpMaxOffset())
	}
}

// TestTypingInSearchDoesNotTriggerCommands checks the other modal boundary: the
// letters that are commands in normal mode are plain text in the prompt.
func TestTypingInSearchDoesNotTriggerCommands(t *testing.T) {
	for _, r := range []rune{'y', 'a', 'q', 'j', 'k', 'n', '/'} {
		act := Decode(keyRune(r), ModeSearch)
		if act.Kind != ActPromptKey || act.Key != PromptInsert || act.Rune != r {
			t.Errorf("%q in search decoded to %+v, want an insert", r, act)
		}
	}
}

func TestSearchModeHandlesEditingKeys(t *testing.T) {
	cases := []struct {
		ev   term.Event
		kind ActionKind
		key  PromptKey
	}{
		{key(term.KeyEnter), ActPromptCommit, 0},
		{key(term.KeyEscape), ActPromptCancel, 0},
		{key(term.KeyBackspace), ActPromptKey, PromptBackspace},
		{key(term.KeyDelete), ActPromptKey, PromptDelete},
		{key(term.KeyLeft), ActPromptKey, PromptLeft},
		{key(term.KeyRight), ActPromptKey, PromptRight},
		{key(term.KeyHome), ActPromptKey, PromptHome},
		{key(term.KeyEnd), ActPromptKey, PromptEnd},
	}
	for _, c := range cases {
		act := Decode(c.ev, ModeSearch)
		if act.Kind != c.kind || (c.kind == ActPromptKey && act.Key != c.key) {
			t.Errorf("event %v decoded to %+v, want kind %d key %d", c.ev, act, c.kind, c.key)
		}
	}
}

func TestRCopiesAReferenceOnlyInNormalMode(t *testing.T) {
	if act := Decode(keyRune('r'), ModeNormal); act.Kind != ActCopyReference {
		t.Errorf("r in normal mode = %d, want ActCopyReference", act.Kind)
	}
	if act := Decode(keyRune('r'), ModeSearch); act.Kind == ActCopyReference {
		t.Error("r must be typed, not copy a reference, while searching")
	}
}

func TestQuitOnlyFromNormalAndCtrlC(t *testing.T) {
	if act := Decode(keyRune('q'), ModeNormal); act.Kind != ActQuit {
		t.Error("q should quit in normal mode")
	}
	// q inside the prompt is a character, not a quit.
	if act := Decode(keyRune('q'), ModeSearch); act.Kind == ActQuit {
		t.Error("q must not quit while typing a search")
	}
	ctrlC := term.Event{Kind: term.EventKey, Key: term.KeyRune, Rune: 'c', Mods: term.ModCtrl}
	if act := Decode(ctrlC, ModeHelp); act.Kind != ActQuit {
		t.Error("ctrl+c should quit even from help")
	}
}

func TestTheWheelScrollsInEveryMode(t *testing.T) {
	up := term.Event{Kind: term.EventMouse, MouseKind: term.MouseScrollUp}
	down := term.Event{Kind: term.EventMouse, MouseKind: term.MouseScrollDown}
	for _, mode := range []Mode{ModeNormal, ModeSearch, ModeHelp} {
		if act := Decode(up, mode); act.Kind != ActMouseWheel || act.N != -wheelStep {
			t.Errorf("mode %d: wheel up = %+v", mode, act)
		}
		if act := Decode(down, mode); act.Kind != ActMouseWheel || act.N != wheelStep {
			t.Errorf("mode %d: wheel down = %+v", mode, act)
		}
	}
}

// TestClicksAreIgnoredOutsideNormalMode keeps a stray click from moving a
// selection the user cannot see.
func TestClicksAreIgnoredOutsideNormalMode(t *testing.T) {
	click := term.Event{
		Kind: term.EventMouse, MouseKind: term.MouseDown,
		Button: term.MouseButtonLeft, X: 10, Y: 5,
	}
	for _, mode := range []Mode{ModeSearch, ModeHelp} {
		if act := Decode(click, mode); act.Kind != ActNone {
			t.Errorf("mode %d: click decoded to %d, want none", mode, act.Kind)
		}
	}
	if act := Decode(click, ModeNormal); act.Kind != ActMouseDown {
		t.Errorf("normal mode click = %d, want ActMouseDown", act.Kind)
	}
}

func TestResizeIsHandledInEveryMode(t *testing.T) {
	ev := term.Event{Kind: term.EventResize}
	ev.Size.Width, ev.Size.Height = 100, 40
	for _, mode := range []Mode{ModeNormal, ModeSearch, ModeHelp} {
		act := Decode(ev, mode)
		if act.Kind != ActResize || act.X != 100 || act.Y != 40 {
			t.Errorf("mode %d: resize = %+v", mode, act)
		}
	}
}

// TestHorizontalScrollIsBounded is another original bug: the offset was
// incremented without limit, so holding the right arrow scrolled the text off
// the screen entirely.
func TestHorizontalScrollIsBounded(t *testing.T) {
	a := testApp(t, "testdata/sample.go", 80, 24)
	for i := 0; i < 5000; i++ {
		a.Apply(Action{Kind: ActScrollX, N: 1})
	}
	var widest Col
	from, to := a.VisibleRange()
	for n := from; n < to; n++ {
		widest = max(widest, a.LineWidth(n))
	}
	if a.XOff > widest {
		t.Errorf("x offset ran to %d past the widest visible line (%d)", a.XOff, widest)
	}
}

func TestPromptEditsMoveByCluster(t *testing.T) {
	var p Prompt
	p.SetText("a日本b")
	// One left arrow must step over the whole b, then the whole 本.
	p.Apply(PromptLeft, 0)
	p.Apply(PromptBackspace, 0)
	if got := p.Text(); got != "a日b" {
		t.Errorf("got %q, want %q — backspace ate part of a cluster", got, "a日b")
	}
}

func TestPromptCursorColumnIsADisplayColumn(t *testing.T) {
	var p Prompt
	p.SetText("日本")
	// Two double-width clusters put the caret at column 4, not byte 6.
	if got := p.CursorCol(); got != 4 {
		t.Errorf("cursor column = %d, want 4", got)
	}
}
