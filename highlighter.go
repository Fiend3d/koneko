package main

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"
)

type cachedLine struct {
	styled string
	width  int
}

type Highlighter struct {
	lexer       chroma.Lexer
	tokenStyles TokenStyles
	cache       []cachedLine
	tabWidth    int
}

func NewHighlighter(filename string, totalLines int, tokenStyles TokenStyles, tabWidth int) *Highlighter {
	l := lexers.Match(filename)
	if l == nil {
		l = lexers.Fallback
	}
	l = chroma.Coalesce(l)

	return &Highlighter{
		lexer:       l,
		tokenStyles: tokenStyles,
		cache:       make([]cachedLine, totalLines),
		tabWidth:    tabWidth,
	}
}

func (h *Highlighter) HighlightRange(text string, fromLine int) {
	if fromLine < 0 {
		fromLine = 0
	}
	if len(text) == 0 {
		return
	}

	lineCount := 1
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lineCount++
		}
	}

	allCached := true
	for i := 0; i < lineCount; i++ {
		n := fromLine + i
		if n >= len(h.cache) {
			break
		}
		if h.cache[n].styled == "" {
			allCached = false
			break
		}
	}
	if allCached {
		return
	}

	iterator, err := h.lexer.Tokenise(nil, text)
	if err != nil {
		return
	}

	tokens := iterator.Tokens()
	lineTokens := chroma.SplitTokensIntoLines(tokens)

	for i, lt := range lineTokens {
		lineNum := fromLine + i
		if lineNum >= len(h.cache) {
			break
		}
		if h.cache[lineNum].styled != "" {
			continue
		}
		styled := strings.ReplaceAll(h.styleLine(lt), "\n", "")
		expanded := expandTabs(styled, h.tabWidth)
		h.cache[lineNum] = cachedLine{
			styled: expanded,
			width:  ansi.StringWidth(expanded),
		}
	}
}

func (h *Highlighter) StyledLine(n int) (string, int, bool) {
	if n >= 0 && n < len(h.cache) {
		cl := h.cache[n]
		return cl.styled, cl.width, cl.styled != ""
	}
	return "", 0, false
}

func (h *Highlighter) styleLine(tokens []chroma.Token) string {
	var b strings.Builder
	for _, t := range tokens {
		value := strings.ReplaceAll(t.Value, "\r", "")
		if value == "" {
			continue
		}
		s := styleForToken(h.tokenStyles, t.Type)
		b.WriteString(s.Inline(true).Render(value))
	}
	return b.String()
}
