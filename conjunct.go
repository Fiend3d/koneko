// Unicode 15.1's rule GB9c: do not break an Indic conjunct.
//
// uniseg v0.4.7, the newest release, implements the grapheme rules through
// GB9b, so it breaks a conjunct in two: पर|ीक्|ष rather than the ligature the
// terminal draws. Everything downstream of the cluster table inherits that
// split — a selection edge can land in the middle of क्ष, and then half a glyph
// is highlighted and the terminal, which groups cells by their attributes
// before shaping them, draws the two halves separately instead of the conjunct.
// Telugu and Devanagari are written almost entirely in conjuncts, so this is
// most of the text.
//
// The rule is:
//
//	Consonant (Extend | Linker)* Linker (Extend | Linker)* × Consonant
//
// where Consonant, Extend and Linker are the Indic_Conjunct_Break property
// values. Applied on top of uniseg's own segmentation it comes to joining two
// adjacent clusters, since uniseg has already gathered each base with its
// marks: the left one has to start with a consonant and carry a linker, and the
// right one has to start with a consonant.
package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// indicLinkers are the code points with Indic_Conjunct_Break=Linker in Unicode
// 15.1 — the viramas of the six scripts the rule covers. Taken from
// DerivedCoreProperties.txt; the whole property is six code points.
var indicLinkers = [...]rune{
	0x094D, // DEVANAGARI SIGN VIRAMA
	0x09CD, // BENGALI SIGN VIRAMA
	0x0ACD, // GUJARATI SIGN VIRAMA
	0x0B4D, // ORIYA SIGN VIRAMA
	0x0C4D, // TELUGU SIGN VIRAMA
	0x0D4D, // MALAYALAM SIGN VIRAMA
}

// indicConsonants are the ranges with Indic_Conjunct_Break=Consonant in Unicode
// 15.1, from DerivedCoreProperties.txt. Kannada, Tamil, Gurmukhi and Sinhala
// have viramas of their own but are not part of the property, and so are not
// here: their conjuncts break like any other cluster.
var indicConsonants = [...]struct{ lo, hi rune }{
	{0x0915, 0x0939}, // DEVANAGARI LETTER KA..HA
	{0x0958, 0x095F}, // DEVANAGARI LETTER QA..YYA
	{0x0978, 0x097F}, // DEVANAGARI LETTER MARWARI DDA..BBA
	{0x0995, 0x09A8}, // BENGALI LETTER KA..NA
	{0x09AA, 0x09B0}, // BENGALI LETTER PA..RA
	{0x09B2, 0x09B2}, // BENGALI LETTER LA
	{0x09B6, 0x09B9}, // BENGALI LETTER SHA..HA
	{0x09DC, 0x09DD}, // BENGALI LETTER RRA..RHA
	{0x09DF, 0x09DF}, // BENGALI LETTER YYA
	{0x09F0, 0x09F1}, // BENGALI LETTER RA WITH MIDDLE..LOWER DIAGONAL
	{0x0A95, 0x0AA8}, // GUJARATI LETTER KA..NA
	{0x0AAA, 0x0AB0}, // GUJARATI LETTER PA..RA
	{0x0AB2, 0x0AB3}, // GUJARATI LETTER LA..LLA
	{0x0AB5, 0x0AB9}, // GUJARATI LETTER VA..HA
	{0x0AF9, 0x0AF9}, // GUJARATI LETTER ZHA
	{0x0B15, 0x0B28}, // ORIYA LETTER KA..NA
	{0x0B2A, 0x0B30}, // ORIYA LETTER PA..RA
	{0x0B32, 0x0B33}, // ORIYA LETTER LA..LLA
	{0x0B35, 0x0B39}, // ORIYA LETTER VA..HA
	{0x0B5C, 0x0B5D}, // ORIYA LETTER RRA..RHA
	{0x0B5F, 0x0B5F}, // ORIYA LETTER YYA
	{0x0B71, 0x0B71}, // ORIYA LETTER WA
	{0x0C15, 0x0C28}, // TELUGU LETTER KA..NA
	{0x0C2A, 0x0C39}, // TELUGU LETTER PA..HA
	{0x0C58, 0x0C5A}, // TELUGU LETTER TSA..RRRA
	{0x0D15, 0x0D3A}, // MALAYALAM LETTER KA..TTTA
}

// zeroWidthJoiner is Indic_Conjunct_Break=Extend as well as a joiner, since a
// conjunct may be written with an explicit ZWJ after the virama.
const zeroWidthJoiner = '‍'

func isIndicLinker(r rune) bool {
	// Six values, all in the same corner of the BMP: a scan beats a search, and
	// the range check keeps every non-Indic rune out of the loop entirely.
	if r < 0x0900 || r > 0x0D4D {
		return false
	}
	for _, l := range indicLinkers {
		if r == l {
			return true
		}
	}
	return false
}

func isIndicConsonant(r rune) bool {
	if r < 0x0915 || r > 0x0D3A {
		return false
	}
	lo, hi := 0, len(indicConsonants)-1
	for lo <= hi {
		mid := int(uint(lo+hi) >> 1)
		switch {
		case r < indicConsonants[mid].lo:
			hi = mid - 1
		case r > indicConsonants[mid].hi:
			lo = mid + 1
		default:
			return true
		}
	}
	return false
}

// isIndicExtend reports Indic_Conjunct_Break=Extend, which is the grapheme
// extenders and the zero-width joiner, less the linkers themselves. Spacing
// marks are deliberately not extenders: a vowel sign between the consonant and
// the virama ends the conjunct.
func isIndicExtend(r rune) bool {
	if r == zeroWidthJoiner {
		return true
	}
	return !isIndicLinker(r) && (unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r))
}

// joinsConjunct reports whether GB9c forbids a break between two clusters that
// uniseg has already segmented, prev before next.
//
// prev has to be the left context of the rule in full — a consonant, then
// extenders and linkers with at least one linker among them, to the end of the
// cluster — and next has to begin with a consonant.
func joinsConjunct(prev, next string) bool {
	if prev == "" || next == "" {
		return false
	}
	head, _ := utf8.DecodeRuneInString(next)
	if !isIndicConsonant(head) {
		return false
	}

	base, n := utf8.DecodeRuneInString(prev)
	if !isIndicConsonant(base) {
		return false
	}
	var linked bool
	for _, r := range prev[n:] {
		switch {
		case isIndicLinker(r):
			linked = true
		case isIndicExtend(r):
			// Extenders are allowed on either side of the linker.
		default:
			return false
		}
	}
	return linked
}

// Tamil's ksha and sri ligatures need tailored selection boundaries: default
// extended grapheme clusters split them at the pulli. Other Tamil consonant
// sequences retain their explicit pulli and remain separate.
// https://www.w3.org/TR/2020/WD-ilreq-taml-20200616/#h_grapheme_boundaries
func joinsTamilLigature(prev, next string) bool {
	if prev == "க்" {
		r, _ := utf8.DecodeRuneInString(next)
		return r == 'ஷ'
	}
	return (prev == "ஶ்" || prev == "ஸ்") && strings.HasPrefix(next, "ரீ")
}
