// The search prompt: a one-line editor.
//
// The bubbletea original vendored a whole textinput widget for this. All the
// prompt has to do is hold a string and a cursor, and the cursor has to move by
// grapheme cluster so that arrowing over an emoji or a Devanagari conjunct
// moves one visible character rather than landing inside it.
package main

// PromptKey is an editing action, decoded from a key event.
type PromptKey int

const (
	PromptInsert PromptKey = iota
	PromptBackspace
	PromptDelete
	PromptLeft
	PromptRight
	PromptHome
	PromptEnd
	PromptKill // clear the whole line
)

// Prompt is the text being typed and where the caret sits in it.
type Prompt struct {
	text string
	// cursor is a byte offset into text, always on a cluster boundary.
	cursor int
	table  ClusterTable
}

func (p *Prompt) Text() string { return p.text }

func (p *Prompt) SetText(s string) {
	p.text = s
	p.cursor = len(s)
}

func (p *Prompt) Clear() {
	p.text = ""
	p.cursor = 0
}

// CursorCol is the display column of the caret within the prompt text.
func (p *Prompt) CursorCol() Col {
	p.table.Rebuild(p.text, 1)
	return p.table.ByteToCol(p.cursor)
}

// Apply edits the prompt. r is the rune to insert when key is PromptInsert.
func (p *Prompt) Apply(key PromptKey, r rune) {
	switch key {
	case PromptInsert:
		p.text = p.text[:p.cursor] + string(r) + p.text[p.cursor:]
		p.cursor += len(string(r))

	case PromptBackspace:
		if p.cursor > 0 {
			at := p.prevBoundary(p.cursor)
			p.text = p.text[:at] + p.text[p.cursor:]
			p.cursor = at
		}

	case PromptDelete:
		if p.cursor < len(p.text) {
			p.text = p.text[:p.cursor] + p.text[p.nextBoundary(p.cursor):]
		}

	case PromptLeft:
		p.cursor = p.prevBoundary(p.cursor)

	case PromptRight:
		p.cursor = p.nextBoundary(p.cursor)

	case PromptHome:
		p.cursor = 0

	case PromptEnd:
		p.cursor = len(p.text)

	case PromptKill:
		p.Clear()
	}
}

// prevBoundary is the start of the cluster before byte offset at.
func (p *Prompt) prevBoundary(at int) int {
	if at <= 0 {
		return 0
	}
	p.table.Rebuild(p.text, 1)
	clusters := p.table.Clusters()
	prev := 0
	for _, c := range clusters {
		if int(c.Byte) >= at {
			break
		}
		prev = int(c.Byte)
	}
	return prev
}

// nextBoundary is the start of the cluster after byte offset at.
func (p *Prompt) nextBoundary(at int) int {
	if at >= len(p.text) {
		return len(p.text)
	}
	p.table.Rebuild(p.text, 1)
	for _, c := range p.table.Clusters() {
		if int(c.Byte) > at {
			return int(c.Byte)
		}
	}
	return len(p.text)
}
