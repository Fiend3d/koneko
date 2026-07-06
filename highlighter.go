package main

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

type Highlighter struct {
	lexer chroma.Lexer
	style *chroma.Style
	cache []string
}

func NewHighlighter(filename string, totalLines int) *Highlighter {
	l := lexers.Match(filename)
	if l == nil {
		l = lexers.Fallback
	}
	l = chroma.Coalesce(l)

	s := styles.Get("catppuccin-mocha")
	if s == nil {
		s = styles.Fallback
	}

	return &Highlighter{
		lexer: l,
		style: s,
		cache: make([]string, totalLines),
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
		entry := h.style.Get(t.Type)
		if entry.IsZero() {
			b.WriteString(value)
			continue
		}
		open := ansiEscape(entry)
		b.WriteString(open)
		b.WriteString(value)
	}
	return b.String()
}

func ansiEscape(entry chroma.StyleEntry) string {
	var parts []string
	if entry.Bold == chroma.Yes {
		parts = append(parts, "1")
	}
	if entry.Italic == chroma.Yes {
		parts = append(parts, "3")
	}
	if entry.Underline == chroma.Yes {
		parts = append(parts, "4")
	}
	if entry.Colour.IsSet() {
		parts = append(parts, fmt.Sprintf("38;2;%d;%d;%d",
			entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue()))
	}
	if len(parts) == 0 {
		return ""
	}
	return "\033[" + strings.Join(parts, ";") + "m"
}

func (h *Highlighter) StyledLine(n int) string {
	if n >= 0 && n < len(h.cache) {
		return h.cache[n]
	}
	return ""
}
