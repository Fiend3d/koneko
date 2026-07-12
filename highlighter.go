package main

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

type Highlighter struct {
	lexer       chroma.Lexer
	tokenStyles TokenStyles
	cache       []string
}

func NewHighlighter(filename string, totalLines int, tokenStyles TokenStyles) *Highlighter {
	l := lexers.Match(filename)
	if l == nil {
		l = lexers.Fallback
	}
	l = chroma.Coalesce(l)

	return &Highlighter{
		lexer:       l,
		tokenStyles: tokenStyles,
		cache:       make([]string, totalLines),
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
		if h.cache[n] == "" {
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
		if h.cache[lineNum] != "" {
			continue
		}
		h.cache[lineNum] = strings.ReplaceAll(h.styleLine(lt), "\n", "")
	}
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

func (h *Highlighter) StyledLine(n int) string {
	if n >= 0 && n < len(h.cache) {
		return h.cache[n]
	}
	return ""
}
