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

var theme = monokaiTheme()

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

func monokaiTheme() Theme {
	bg := lipgloss.Color("#272822")
	fg := lipgloss.Color("#F8F8F2")
	dim := lipgloss.Color("#88846F")
	pink := lipgloss.Color("#F92672")
	green := lipgloss.Color("#A6E22E")
	orange := lipgloss.Color("#FD971F")
	yellow := lipgloss.Color("#E6DB74")
	purple := lipgloss.Color("#AE81FF")
	red := lipgloss.Color("#F92672")

	return Theme{
		Background:  bg,
		Foreground:  fg,
		DimText:     dim,
		SelectionBg: fg,
		SelectionFg: bg,
		TokenStyles: TokenStyles{
			chroma.Text:                   tokenStyle(bg, fg),
			chroma.Whitespace:             tokenStyle(bg, fg),
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
		},
	}
}
