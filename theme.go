package main

import (
	"image/color"

	"github.com/alecthomas/chroma/v2"
	"charm.land/lipgloss/v2"
)

type TokenStyles map[chroma.TokenType]lipgloss.Style

type Theme struct {
	Background    color.Color
	Foreground    color.Color
	DimText       color.Color
	SelectionBg   color.Color
	SelectionFg   color.Color
	TokenStyles   TokenStyles
}

var theme Theme

func setTheme(name string) {
	switch name {
	case "nord":
		theme = nordTheme()
	case "dracula":
		theme = draculaTheme()
	case "tokyonight":
		theme = tokyonightTheme()
	case "github":
		theme = githubTheme()
	case "autumn":
		theme = autumnTheme()
	case "base16":
		theme = base16Theme()
	case "ferra":
		theme = ferraTheme()
	case "monokai":
		theme = monokaiTheme()
	default:
		theme = draculaTheme()
	}
	initStyles()
}

func styleForToken(styles TokenStyles, tt chroma.TokenType) lipgloss.Style {
	if s, ok := styles[tt]; ok {
		return s
	}
	if s, ok := styles[parentTokenType(tt)]; ok {
		return s
	}
	return lipgloss.Style{}
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

func tokenStyle(bg color.Color, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Background(bg).Foreground(fg)
}

func tokenStyleItalic(bg color.Color, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Background(bg).Foreground(fg).Italic(true)
}

func tokenStyleBold(bg color.Color, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Background(bg).Foreground(fg).Bold(true)
}

func makeTokenStyles(bg, fg, dim, pink, green, orange, yellow, purple, red color.Color) TokenStyles {
	return TokenStyles{
		chroma.Text:                   tokenStyle(bg, fg),
		chroma.Whitespace:             tokenStyle(bg, fg),
		chroma.Generic:                tokenStyle(bg, fg),
		chroma.Comment:                tokenStyleItalic(bg, dim),
		chroma.CommentSpecial:         tokenStyleItalic(bg, dim),
		chroma.CommentPreproc:         tokenStyle(bg, green),
		chroma.Keyword:                tokenStyle(bg, pink),
		chroma.KeywordType:            tokenStyle(bg, green),
		chroma.KeywordDeclaration:     tokenStyle(bg, pink),
		chroma.KeywordNamespace:       tokenStyle(bg, pink),
		chroma.KeywordPseudo:          tokenStyle(bg, pink),
		chroma.KeywordReserved:        tokenStyle(bg, pink),
		chroma.KeywordConstant:        tokenStyle(bg, purple),
		chroma.Operator:               tokenStyle(bg, fg),
		chroma.Punctuation:            tokenStyle(bg, fg),
		chroma.Name:                   tokenStyle(bg, fg),
		chroma.Literal:                tokenStyle(bg, fg),
		chroma.LiteralString:          tokenStyle(bg, yellow),
		chroma.LiteralStringAffix:     tokenStyle(bg, yellow),
		chroma.LiteralStringBacktick:  tokenStyle(bg, yellow),
		chroma.LiteralStringChar:      tokenStyle(bg, yellow),
		chroma.LiteralStringDelimiter: tokenStyle(bg, yellow),
		chroma.LiteralStringDoc:       tokenStyleItalic(bg, yellow),
		chroma.LiteralStringDouble:    tokenStyle(bg, yellow),
		chroma.LiteralStringEscape:    tokenStyle(bg, pink),
		chroma.LiteralStringHeredoc:   tokenStyle(bg, yellow),
		chroma.LiteralStringInterpol:  tokenStyle(bg, yellow),
		chroma.LiteralStringOther:     tokenStyle(bg, yellow),
		chroma.LiteralStringRegex:     tokenStyle(bg, orange),
		chroma.LiteralStringSingle:    tokenStyle(bg, yellow),
		chroma.LiteralStringSymbol:    tokenStyle(bg, yellow),
		chroma.LiteralNumber:          tokenStyle(bg, purple),
		chroma.LiteralNumberFloat:     tokenStyle(bg, purple),
		chroma.LiteralNumberHex:       tokenStyle(bg, purple),
		chroma.LiteralNumberInteger:   tokenStyle(bg, purple),
		chroma.LiteralNumberOct:       tokenStyle(bg, purple),
		chroma.NameBuiltin:            tokenStyle(bg, green),
		chroma.NameFunction:           tokenStyle(bg, green),
		chroma.NameClass:              tokenStyle(bg, green),
		chroma.NameNamespace:          tokenStyle(bg, green),
		chroma.NameConstant:           tokenStyle(bg, purple),
		chroma.NameAttribute:          tokenStyle(bg, orange),
		chroma.NameVariable:           tokenStyle(bg, fg),
		chroma.NameException:          tokenStyle(bg, pink),
		chroma.NameDecorator:          tokenStyle(bg, green),
		chroma.NameEntity:             tokenStyle(bg, green),
		chroma.NameLabel:              tokenStyle(bg, green),
		chroma.NameTag:                tokenStyle(bg, pink),
		chroma.NameProperty:           tokenStyle(bg, orange),
		chroma.GenericDeleted:         tokenStyle(bg, red),
		chroma.GenericInserted:        tokenStyle(bg, green),
		chroma.GenericEmph:            tokenStyleItalic(bg, fg),
		chroma.GenericStrong:          tokenStyleBold(bg, fg),
	}
}

func monokaiTheme() Theme {
	bg := lipgloss.Color("#272822")
	fg := lipgloss.Color("#f8f8f2")
	dim := lipgloss.Color("#878b91")
	pink := lipgloss.Color("#F92672")
	green := lipgloss.Color("#a6e22e")
	orange := lipgloss.Color("#fd971f")
	yellow := lipgloss.Color("#e6db74")
	purple := lipgloss.Color("#C586C0")
	red := lipgloss.Color("#f48771")
	return Theme{
		Background:  bg,
		Foreground:  fg,
		DimText:     dim,
		SelectionBg: fg,
		SelectionFg: bg,
		TokenStyles: makeTokenStyles(bg, fg, dim, pink, green, orange, yellow, purple, red),
	}
}

func nordTheme() Theme {
	bg := lipgloss.Color("#2e3440")
	fg := lipgloss.Color("#ECEFF4")
	dim := lipgloss.Color("#4C566A")
	pink := lipgloss.Color("#5E81AC")
	green := lipgloss.Color("#A3BE8C")
	orange := lipgloss.Color("#B48EAD")
	yellow := lipgloss.Color("#88C0D0")
	purple := lipgloss.Color("#81A1C1")
	red := lipgloss.Color("#BF616A")
	return Theme{
		Background:  bg,
		Foreground:  fg,
		DimText:     dim,
		SelectionBg: fg,
		SelectionFg: bg,
		TokenStyles: makeTokenStyles(bg, fg, dim, pink, green, orange, yellow, purple, red),
	}
}

func draculaTheme() Theme {
	bg := lipgloss.Color("#282a36")
	fg := lipgloss.Color("#ffffff")
	dim := lipgloss.Color("#6272a4")
	pink := lipgloss.Color("#ff79c6")
	green := lipgloss.Color("#94d716")
	orange := lipgloss.Color("#ffb86c")
	yellow := lipgloss.Color("#f1fa8c")
	purple := lipgloss.Color("#bd93f9")
	red := lipgloss.Color("#ea1212")
	return Theme{
		Background:  bg,
		Foreground:  fg,
		DimText:     dim,
		SelectionBg: fg,
		SelectionFg: bg,
		TokenStyles: makeTokenStyles(bg, fg, dim, pink, green, orange, yellow, purple, red),
	}
}

func tokyonightTheme() Theme {
	bg := lipgloss.Color("#222436")
	fg := lipgloss.Color("#c8d3f5")
	dim := lipgloss.Color("#636da6")
	pink := lipgloss.Color("#ff966c")
	green := lipgloss.Color("#4fd6be")
	orange := lipgloss.Color("#c099ff")
	yellow := lipgloss.Color("#65bcff")
	purple := lipgloss.Color("#ffc777")
	red := lipgloss.Color("#ff757f")
	return Theme{
		Background:  bg,
		Foreground:  fg,
		DimText:     dim,
		SelectionBg: fg,
		SelectionFg: bg,
		TokenStyles: makeTokenStyles(bg, fg, dim, pink, green, orange, yellow, purple, red),
	}
}

func githubTheme() Theme {
	bg := lipgloss.Color("#22272e")
	fg := lipgloss.Color("#adbac7")
	dim := lipgloss.Color("#768390")
	pink := lipgloss.Color("#c96198")
	green := lipgloss.Color("#57ab5a")
	orange := lipgloss.Color("#f69d50")
	yellow := lipgloss.Color("#eac55f")
	purple := lipgloss.Color("#8256d0")
	red := lipgloss.Color("#e5534b")
	return Theme{
		Background:  bg,
		Foreground:  fg,
		DimText:     dim,
		SelectionBg: fg,
		SelectionFg: bg,
		TokenStyles: makeTokenStyles(bg, fg, dim, pink, green, orange, yellow, purple, red),
	}
}

func autumnTheme() Theme {
	bg := lipgloss.Color("#232323")
	fg := lipgloss.Color("#F3F2CC")
	dim := lipgloss.Color("#646f69")
	pink := lipgloss.Color("#86c1b9")
	green := lipgloss.Color("#99be70")
	orange := lipgloss.Color("#FAD566")
	yellow := lipgloss.Color("#cfba8b")
	purple := lipgloss.Color("#727ca5")
	red := lipgloss.Color("#F05E48")
	return Theme{
		Background:  bg,
		Foreground:  fg,
		DimText:     dim,
		SelectionBg: fg,
		SelectionFg: bg,
		TokenStyles: makeTokenStyles(bg, fg, dim, pink, green, orange, yellow, purple, red),
	}
}

func base16Theme() Theme {
	bg := lipgloss.NoColor{}
	fg := lipgloss.NoColor{}
	dim := lipgloss.BrightBlack
	pink := lipgloss.Cyan
	green := lipgloss.Green
	orange := lipgloss.BrightRed
	yellow := lipgloss.Yellow
	purple := lipgloss.BrightCyan
	red := lipgloss.Red
	return Theme{
		Background:  bg,
		Foreground:  fg,
		DimText:     dim,
		SelectionBg: lipgloss.BrightBlue,
		SelectionFg: lipgloss.NoColor{},
		TokenStyles: makeTokenStyles(bg, fg, dim, pink, green, orange, yellow, purple, red),
	}
}

func ferraTheme() Theme {
	bg := lipgloss.Color("#2b292d")
	fg := lipgloss.Color("#D1D1E0")
	dim := lipgloss.Color("#4d424b")
	pink := lipgloss.Color("#F5D76E")
	green := lipgloss.Color("#B1B695")
	orange := lipgloss.Color("#ffa07a")
	yellow := lipgloss.Color("#fecdb2")
	purple := lipgloss.Color("#F6B6C9")
	red := lipgloss.Color("#e06b75")
	return Theme{
		Background:  bg,
		Foreground:  fg,
		DimText:     dim,
		SelectionBg: fg,
		SelectionFg: bg,
		TokenStyles: makeTokenStyles(bg, fg, dim, pink, green, orange, yellow, purple, red),
	}
}
