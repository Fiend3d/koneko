// Colour themes.
//
// Every theme is nine colours fanned out over ~50 syntax token types. A Slot
// names which of the nine a token gets; the highlighter caches slots, never
// resolved colours, so switching theme costs nothing and the cache stays valid.
package main

import (
	"github.com/Fiend3d/catatui"
	"github.com/alecthomas/chroma/v2"
)

// Slot is which palette entry a piece of syntax uses, plus the few emphasis
// variants.
type Slot uint8

const (
	SlotFg Slot = iota
	SlotDim
	SlotPink
	SlotGreen
	SlotOrange
	SlotYellow
	SlotPurple
	SlotRed
	SlotDimItalic
	SlotYellowItalic
	SlotFgItalic
	SlotFgBold

	slotCount
)

// Palette is the nine colours a theme is built from, plus the selection
// background derived from them.
type Palette struct {
	Bg, Fg, Dim                 catatui.Color
	Pink, Green, Orange, Yellow catatui.Color
	Purple, Red                 catatui.Color
	// SelBg is the background for selected text.
	//
	// The bubbletea original inverted the whole cell (selection bg = fg)
	// because it stripped the syntax colours out of selected text first. We
	// keep the syntax foreground and change only the background, so this has to
	// be a real, subtle highlight rather than the foreground colour.
	SelBg catatui.Color
}

// Theme is a palette with its per-slot styles precomputed, so the render path
// is an array index rather than a lookup.
type Theme struct {
	Palette Palette
	styles  [slotCount]catatui.Style
	slots   map[chroma.TokenType]Slot
}

var theme *Theme

func NewTheme(p Palette) *Theme {
	plain := func(c catatui.Color) catatui.Style {
		return catatui.NewStyle().Bg(p.Bg).Fg(c)
	}
	italic := func(c catatui.Color) catatui.Style {
		return plain(c).AddModifier(catatui.ModifierItalic)
	}
	bold := func(c catatui.Color) catatui.Style {
		return plain(c).AddModifier(catatui.ModifierBold)
	}

	t := &Theme{Palette: p, slots: tokenSlots}
	t.styles[SlotFg] = plain(p.Fg)
	t.styles[SlotDim] = plain(p.Dim)
	t.styles[SlotPink] = plain(p.Pink)
	t.styles[SlotGreen] = plain(p.Green)
	t.styles[SlotOrange] = plain(p.Orange)
	t.styles[SlotYellow] = plain(p.Yellow)
	t.styles[SlotPurple] = plain(p.Purple)
	t.styles[SlotRed] = plain(p.Red)
	t.styles[SlotDimItalic] = italic(p.Dim)
	t.styles[SlotYellowItalic] = italic(p.Yellow)
	t.styles[SlotFgItalic] = italic(p.Fg)
	t.styles[SlotFgBold] = bold(p.Fg)
	return t
}

func (t *Theme) Style(s Slot) catatui.Style { return t.styles[s] }

// Base is unstyled text: the background with the default foreground.
func (t *Theme) Base() catatui.Style { return t.styles[SlotFg] }

// Background is a background fill with no text in it.
func (t *Theme) Background() catatui.Style { return catatui.NewStyle().Bg(t.Palette.Bg) }

func (t *Theme) StatusBar() catatui.Style {
	return catatui.NewStyle().Bg(t.Palette.Bg).Fg(t.Palette.Fg)
}

func (t *Theme) LineNum() catatui.Style {
	return catatui.NewStyle().Bg(t.Palette.Bg).Fg(t.Palette.Dim)
}

// LineNumSelected is the gutter style for a line that intersects the selection.
func (t *Theme) LineNumSelected() catatui.Style {
	return catatui.NewStyle().Bg(t.Palette.Bg).Fg(t.Palette.Fg)
}

func (t *Theme) Scrollbar() catatui.Style {
	return catatui.NewStyle().Bg(t.Palette.Bg).Fg(t.Palette.Dim)
}

// SlotFor maps a chroma token type to a palette slot, walking up to the parent
// type when the exact type has no entry of its own.
func (t *Theme) SlotFor(tt chroma.TokenType) Slot {
	if s, ok := t.slots[tt]; ok {
		return s
	}
	if s, ok := t.slots[parentTokenType(tt)]; ok {
		return s
	}
	return SlotFg
}

func parentTokenType(tt chroma.TokenType) chroma.TokenType {
	switch {
	case tt >= chroma.Generic:
		return chroma.Generic
	case tt >= chroma.Comment:
		return chroma.Comment
	case tt >= chroma.Punctuation:
		return chroma.Punctuation
	case tt >= chroma.Operator:
		return chroma.Operator
	case tt >= chroma.Literal:
		return chroma.Literal
	case tt >= chroma.Name:
		return chroma.Name
	case tt >= chroma.Keyword:
		return chroma.Keyword
	}
	return chroma.Text
}

// tokenSlots is the token-type-to-slot mapping, shared by every theme. It is
// the bubbletea original's makeTokenStyles table with the colours factored out.
var tokenSlots = map[chroma.TokenType]Slot{
	chroma.Text:                   SlotFg,
	chroma.Whitespace:             SlotFg,
	chroma.Generic:                SlotFg,
	chroma.Comment:                SlotDimItalic,
	chroma.CommentSpecial:         SlotDimItalic,
	chroma.CommentPreproc:         SlotGreen,
	chroma.Keyword:                SlotPink,
	chroma.KeywordType:            SlotGreen,
	chroma.KeywordDeclaration:     SlotPink,
	chroma.KeywordNamespace:       SlotPink,
	chroma.KeywordPseudo:          SlotPink,
	chroma.KeywordReserved:        SlotPink,
	chroma.KeywordConstant:        SlotPurple,
	chroma.Operator:               SlotFg,
	chroma.Punctuation:            SlotFg,
	chroma.Name:                   SlotFg,
	chroma.Literal:                SlotFg,
	chroma.LiteralString:          SlotYellow,
	chroma.LiteralStringAffix:     SlotYellow,
	chroma.LiteralStringBacktick:  SlotYellow,
	chroma.LiteralStringChar:      SlotYellow,
	chroma.LiteralStringDelimiter: SlotYellow,
	chroma.LiteralStringDoc:       SlotYellowItalic,
	chroma.LiteralStringDouble:    SlotYellow,
	chroma.LiteralStringEscape:    SlotPink,
	chroma.LiteralStringHeredoc:   SlotYellow,
	chroma.LiteralStringInterpol:  SlotYellow,
	chroma.LiteralStringOther:     SlotYellow,
	chroma.LiteralStringRegex:     SlotOrange,
	chroma.LiteralStringSingle:    SlotYellow,
	chroma.LiteralStringSymbol:    SlotYellow,
	chroma.LiteralNumber:          SlotPurple,
	chroma.LiteralNumberFloat:     SlotPurple,
	chroma.LiteralNumberHex:       SlotPurple,
	chroma.LiteralNumberInteger:   SlotPurple,
	chroma.LiteralNumberOct:       SlotPurple,
	chroma.NameBuiltin:            SlotGreen,
	chroma.NameFunction:           SlotGreen,
	chroma.NameClass:              SlotGreen,
	chroma.NameNamespace:          SlotGreen,
	chroma.NameConstant:           SlotPurple,
	chroma.NameAttribute:          SlotOrange,
	chroma.NameVariable:           SlotFg,
	chroma.NameException:          SlotPink,
	chroma.NameDecorator:          SlotGreen,
	chroma.NameEntity:             SlotGreen,
	chroma.NameLabel:              SlotGreen,
	chroma.NameTag:                SlotPink,
	chroma.NameProperty:           SlotOrange,
	chroma.GenericDeleted:         SlotRed,
	chroma.GenericInserted:        SlotGreen,
	chroma.GenericEmph:            SlotFgItalic,
	chroma.GenericStrong:          SlotFgBold,
}

func rgb(hex uint32) catatui.Color { return catatui.RgbFromU32(hex) }

// ThemeNames is the list offered on the command line, in the order help shows.
var ThemeNames = []string{
	"autumn", "base16", "dracula", "ferra",
	"github", "monokai", "nord", "tokyonight",
}

const defaultTheme = "dracula"

// palettes are the nine colours per theme. SelBg is a subtle highlight rather
// than the foreground: selected text keeps its syntax colours here, so an
// inverted cell would make it unreadable.
var palettes = map[string]Palette{
	"monokai": {
		Bg: rgb(0x272822), Fg: rgb(0xf8f8f2), Dim: rgb(0x878b91),
		Pink: rgb(0xF92672), Green: rgb(0xa6e22e), Orange: rgb(0xfd971f),
		Yellow: rgb(0xe6db74), Purple: rgb(0xC586C0), Red: rgb(0xf48771),
		SelBg: rgb(0x49483e),
	},
	"nord": {
		Bg: rgb(0x2e3440), Fg: rgb(0xECEFF4), Dim: rgb(0x4C566A),
		Pink: rgb(0x5E81AC), Green: rgb(0xA3BE8C), Orange: rgb(0xB48EAD),
		Yellow: rgb(0x88C0D0), Purple: rgb(0x81A1C1), Red: rgb(0xBF616A),
		SelBg: rgb(0x434c5e),
	},
	"dracula": {
		Bg: rgb(0x282a36), Fg: rgb(0xffffff), Dim: rgb(0x6272a4),
		Pink: rgb(0xff79c6), Green: rgb(0x94d716), Orange: rgb(0xffb86c),
		Yellow: rgb(0xf1fa8c), Purple: rgb(0xbd93f9), Red: rgb(0xea1212),
		SelBg: rgb(0x44475a),
	},
	"tokyonight": {
		Bg: rgb(0x222436), Fg: rgb(0xc8d3f5), Dim: rgb(0x636da6),
		Pink: rgb(0xff966c), Green: rgb(0x4fd6be), Orange: rgb(0xc099ff),
		Yellow: rgb(0x65bcff), Purple: rgb(0xffc777), Red: rgb(0xff757f),
		SelBg: rgb(0x2d3f76),
	},
	"github": {
		Bg: rgb(0x22272e), Fg: rgb(0xadbac7), Dim: rgb(0x768390),
		Pink: rgb(0xc96198), Green: rgb(0x57ab5a), Orange: rgb(0xf69d50),
		Yellow: rgb(0xeac55f), Purple: rgb(0x8256d0), Red: rgb(0xe5534b),
		SelBg: rgb(0x3c4450),
	},
	"autumn": {
		Bg: rgb(0x232323), Fg: rgb(0xF3F2CC), Dim: rgb(0x646f69),
		Pink: rgb(0x86c1b9), Green: rgb(0x99be70), Orange: rgb(0xFAD566),
		Yellow: rgb(0xcfba8b), Purple: rgb(0x727ca5), Red: rgb(0xF05E48),
		SelBg: rgb(0x3f3f3f),
	},
	"ferra": {
		Bg: rgb(0x2b292d), Fg: rgb(0xD1D1E0), Dim: rgb(0x4d424b),
		Pink: rgb(0xF5D76E), Green: rgb(0xB1B695), Orange: rgb(0xffa07a),
		Yellow: rgb(0xfecdb2), Purple: rgb(0xF6B6C9), Red: rgb(0xe06b75),
		SelBg: rgb(0x433f45),
	},
	// base16 uses the terminal's own palette, so it inherits whatever colours
	// the user has configured rather than imposing hex values.
	"base16": {
		Bg: catatui.ColorReset, Fg: catatui.ColorReset, Dim: catatui.ColorDarkGray,
		Pink: catatui.ColorCyan, Green: catatui.ColorGreen, Orange: catatui.ColorLightRed,
		Yellow: catatui.ColorYellow, Purple: catatui.ColorLightCyan, Red: catatui.ColorRed,
		SelBg: catatui.ColorBlue,
	},
}

// setTheme installs the named theme, falling back to the default.
func setTheme(name string) {
	p, ok := palettes[name]
	if !ok {
		p = palettes[defaultTheme]
	}
	theme = NewTheme(p)
}
