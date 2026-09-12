// Decoding terminal events into actions.
//
// Each mode is a closed map. In the bubbletea original, keys the help overlay
// did not handle fell through to the viewer underneath it, so pressing `y`
// inside help copied the selection behind it; here Help returns from its own
// switch and nothing escapes.
package main

import "github.com/Fiend3d/catatui/term"

// MouseButton is which button an event carries.
type MouseButton int

const (
	MouseNone MouseButton = iota
	MouseLeft
	MouseRight
	MouseMiddle
)

// wheelStep is how many lines one notch of the wheel scrolls.
const wheelStep = 3

// Decode turns one event into an action, given the mode it arrives in.
func Decode(ev term.Event, mode Mode) Action {
	switch ev.Kind {
	case term.EventResize:
		return Action{Kind: ActResize, X: ev.Size.Width, Y: ev.Size.Height}
	case term.EventKey:
		switch mode {
		case ModeSearch:
			return decodeSearchKey(ev)
		case ModeHelp:
			return decodeHelpKey(ev)
		default:
			return decodeNormalKey(ev)
		}
	case term.EventMouse:
		return decodeMouse(ev, mode)
	}
	return Action{}
}

func decodeNormalKey(ev term.Event) Action {
	switch {
	case ev.IsRune('q'), ev.IsCtrl('c'):
		return Action{Kind: ActQuit}

	case ev.IsKey(term.KeyUp), ev.IsRune('k'):
		return Action{Kind: ActScrollLines, N: -1}
	case ev.IsKey(term.KeyDown), ev.IsRune('j'):
		return Action{Kind: ActScrollLines, N: 1}
	case ev.IsKey(term.KeyLeft):
		return Action{Kind: ActScrollX, N: -1}
	case ev.IsKey(term.KeyRight):
		return Action{Kind: ActScrollX, N: 1}
	case ev.IsKey(term.KeyPageUp):
		return Action{Kind: ActScrollHalfPage, N: -1}
	case ev.IsKey(term.KeyPageDown):
		return Action{Kind: ActScrollHalfPage, N: 1}
	case ev.IsKey(term.KeyHome), ev.IsRune('g'):
		return Action{Kind: ActGoTop}
	case ev.IsKey(term.KeyEnd), ev.IsRune('G'):
		return Action{Kind: ActGoBottom}
	case ev.IsRune('H'):
		return Action{Kind: ActResetX}

	case ev.IsRune('a'):
		return Action{Kind: ActSelectAll}
	case ev.IsRune('d'):
		return Action{Kind: ActDeselect}
	case ev.IsRune('y'):
		return Action{Kind: ActCopy}
	case ev.IsRune('r'):
		return Action{Kind: ActCopyReference}
	case ev.IsRune('x'), ev.IsRune('X'):
		return Action{Kind: ActExtendToLines}

	case ev.IsRune('l'):
		return Action{Kind: ActToggleLineNumbers}
	case ev.IsRune('s'):
		return Action{Kind: ActToggleScrollbar}
	case ev.IsRune('h'):
		return Action{Kind: ActToggleHighlight}
	case ev.IsRune('c'):
		return Action{Kind: ActToggleGitChanges}
	case ev.IsRune(']'):
		return Action{Kind: ActNextChange}
	case ev.IsRune('['):
		return Action{Kind: ActPrevChange}

	case ev.IsKey(term.KeyF1):
		return Action{Kind: ActOpenHelp}
	case ev.IsRune('/'):
		return Action{Kind: ActOpenSearch}
	case ev.IsRune('n'):
		return Action{Kind: ActNextMatch}
	case ev.IsRune('N'):
		return Action{Kind: ActPrevMatch}
	}
	return Action{}
}

func decodeSearchKey(ev term.Event) Action {
	switch {
	case ev.IsKey(term.KeyEnter):
		return Action{Kind: ActPromptCommit}
	case ev.IsKey(term.KeyEscape), ev.IsCtrl('c'):
		return Action{Kind: ActPromptCancel}
	case ev.IsKey(term.KeyBackspace):
		return Action{Kind: ActPromptKey, Key: PromptBackspace}
	case ev.IsKey(term.KeyDelete):
		return Action{Kind: ActPromptKey, Key: PromptDelete}
	case ev.IsKey(term.KeyLeft):
		return Action{Kind: ActPromptKey, Key: PromptLeft}
	case ev.IsKey(term.KeyRight):
		return Action{Kind: ActPromptKey, Key: PromptRight}
	case ev.IsKey(term.KeyHome):
		return Action{Kind: ActPromptKey, Key: PromptHome}
	case ev.IsKey(term.KeyEnd):
		return Action{Kind: ActPromptKey, Key: PromptEnd}
	case ev.IsCtrl('u'):
		return Action{Kind: ActPromptKey, Key: PromptKill}
	case ev.Key == term.KeyRune && ev.Mods&^term.ModShift == 0:
		return Action{Kind: ActPromptKey, Key: PromptInsert, Rune: ev.Rune}
	}
	return Action{}
}

func decodeHelpKey(ev term.Event) Action {
	switch {
	case ev.IsKey(term.KeyF1), ev.IsKey(term.KeyEscape), ev.IsRune('q'):
		return Action{Kind: ActCloseHelp}
	case ev.IsCtrl('c'):
		return Action{Kind: ActQuit}
	case ev.IsKey(term.KeyUp), ev.IsRune('k'):
		return Action{Kind: ActScrollLines, N: -1}
	case ev.IsKey(term.KeyDown), ev.IsRune('j'):
		return Action{Kind: ActScrollLines, N: 1}
	case ev.IsKey(term.KeyPageUp):
		return Action{Kind: ActScrollHalfPage, N: -1}
	case ev.IsKey(term.KeyPageDown):
		return Action{Kind: ActScrollHalfPage, N: 1}
	case ev.IsKey(term.KeyHome), ev.IsRune('g'):
		return Action{Kind: ActGoTop}
	case ev.IsKey(term.KeyEnd), ev.IsRune('G'):
		return Action{Kind: ActGoBottom}
	}
	// Modal: anything else is swallowed rather than reaching the viewer.
	return Action{}
}

func decodeMouse(ev term.Event, mode Mode) Action {
	switch ev.MouseKind {
	case term.MouseScrollUp:
		return Action{Kind: ActMouseWheel, N: -wheelStep}
	case term.MouseScrollDown:
		return Action{Kind: ActMouseWheel, N: wheelStep}
	}
	if mode != ModeNormal {
		return Action{}
	}
	switch ev.MouseKind {
	case term.MouseDown:
		return Action{Kind: ActMouseDown, X: ev.X, Y: ev.Y, Button: button(ev.Button)}
	case term.MouseDrag:
		return Action{Kind: ActMouseDrag, X: ev.X, Y: ev.Y, Button: button(ev.Button)}
	case term.MouseUp:
		return Action{Kind: ActMouseUp, X: ev.X, Y: ev.Y, Button: button(ev.Button)}
	}
	return Action{}
}

func button(b term.MouseButton) MouseButton {
	switch b {
	case term.MouseButtonLeft:
		return MouseLeft
	case term.MouseButtonRight:
		return MouseRight
	case term.MouseButtonMiddle:
		return MouseMiddle
	}
	return MouseNone
}
