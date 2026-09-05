# koneko

A fast terminal file viewer — `less`/`bat` with mouse-driven text selection,
clipboard copy, search and syntax highlighting.

```
koneko [OPTIONS] <FILE>
```

Press <kbd>F1</kbd> inside the viewer for the full key list.

Built on [catatui](https://github.com/Fiend3d/catatui), a Go port of ratatui:
you draw into a `Buffer` of cells, a constraint solver decides where things go,
and the `Terminal` writes only what changed.

## Why the rewrite

koneko was previously written with Bubble Tea and Lip Gloss, and it could not
render unicode correctly. The cause was architectural rather than a bug waiting
on an upstream fix.

It rendered by building a string with ANSI escape sequences embedded in it, then
**slicing that string by display column** — for horizontal scrolling, for the
selection overlay, and again for padding each row to width. Every splice had to
re-measure the result, and four different width implementations were in play
(`ansi.StringWidth`, `uniseg.StringWidth`, `go-runewidth.RuneWidth` measuring
per rune rather than per cluster, and `lipgloss.Width`). They disagreed on ZWJ
sequences, emoji presentation selectors, and East-Asian-ambiguous characters. On
top of that, three coordinate spaces — display columns, byte offsets, and
grapheme clusters — were converted between by five separate helpers, and one of
them silently mixed a byte index with a display column.

koneko now never splices text. A line is walked once into a cluster table
(`text.go`) that records, for each grapheme, its byte offset, its starting
column, and its width; syntax styles attach to byte ranges; and a single counter
guarantees each row occupies exactly the width it was given. Widths come from
`uniseg`, the same package catatui measures with, so our measurements and the
renderer's cannot disagree.

The regression test for all of this renders every line of `testdata/unicode.txt`
at every horizontal offset and at seven pane widths, and asserts each row is
exactly the requested number of cells. A second test asserts that every cell of
a rendered frame was actually written — a gap would mean a row came up short.

## Behaviour that deliberately differs from the Bubble Tea version

These are places where the old code was wrong, not places where the rewrite took
liberties.

- **Search columns.** A match's column was recorded as a byte index into a
  tab-expanded string and then used as a display column, so on any line with
  multi-byte text before the match the highlight landed in the wrong place.
- **Selected text keeps its syntax colours.** The old code stripped the styling
  out of selected text and re-rendered it flat. Each theme therefore gains an
  explicit selection background; the old code inverted the whole cell, which
  only worked because the foreground had been thrown away.
- **No highlighter data race.** A shared highlighter was mutated from a
  goroutine with no synchronisation. The lexer now lives entirely on one worker
  goroutine and only finished results cross the channel.
- **Word selection handles clusters.** The old code walked backwards rune by
  rune while measuring width per cluster, so a double-click could split `हिन्दी`.
- **Horizontal scroll is bounded.** The offset was incremented without limit.
- **Help is modal.** Keys the help overlay did not handle fell through, so
  pressing `y` inside the help overlay copied the selection underneath it.
- **Selection spans blank lines visibly.** An empty line inside a selection now
  paints the one cell that stands for its newline.

`-select=LINE:CHAR-LINE:CHAR` counts `CHAR` in **grapheme clusters**, so a
caller does not need to know the file's encoding to point at the third character
of a line. The flags keep Go's single-dash style, unchanged from before.

## Performance

Measured against [nezumi](https://github.com/Fiend3d/nezumi), the Rust/ratatui
implementation of the same program, on the same machine (i5-12400F, Windows) and
the same 9.8 MB / 270k-line corpus (`sqlite3.c`). Both suites run the same
benchmarks; nezumi's are criterion, koneko's are `go test -bench`.

| | koneko (Go) | nezumi (Rust) | |
|---|---|---|---|
| Random line access | 8.5 ns | 62.7 ns | **7.4× faster** |
| Cluster table, ASCII line | 101 ns | 1080 ns | **10.7× faster** |
| Cluster table, unicode line | 790 ns | 849 ns | **1.1× faster** |
| Copy 40 lines | 1.00 µs | 2.33 µs | **2.3× faster** |
| Copy whole file | 4.19 ms | 9.60 ms | **2.3× faster** |
| Open and index | 5.84 ms | 4.92 ms | 1.2× slower |
| Frame at 80×24 | 110 µs | 84 µs | 1.3× slower |
| Frame at 200×50 | 480 µs | 336 µs | 1.4× slower |
| Search `sqlite3_malloc` | 7.43 ms | 5.44 ms | 1.4× slower |

Both implementations find the same 314 matches, which the test suite asserts.

Where koneko wins, it is mostly not the language. Line access is faster because
Go strings carry no UTF-8 validity requirement, so a line is a slice of the
mapping where Rust pays `from_utf8_lossy`; invalid bytes still render as
replacement characters, they are just resolved at segmentation time rather than
up front. The cluster table is faster because it has an ASCII fast path that
nezumi does not. Copying is faster because only the first and last rows of a
selection need a cluster table, and the buffer is sized from the file's own
offset index so it is allocated exactly once.

Where it loses, it is mostly the language: the render path and the search scan
are both tight byte loops where Rust's codegen and `memchr` are ahead.

A frame renders in ~110 µs at 80×24 and ~480 µs at 200×50, so drawing is never
the bottleneck. The event loop blocks when idle and drains each burst of input
before drawing, so a fast wheel spin or drag produces one frame rather than one
per event — the Bubble Tea version rendered once per message.

Steady-state rendering costs **6 allocations per frame** (about 175 bytes),
independent of terminal size.

```
go test -bench=. ./...    # needs testdata/sqlite3.c; benchmarks skip without it
```

## Changes made to catatui

Profiling koneko's render path found two hot spots in the library, both fixed
upstream in the local checkout:

- `Graphemes` allocated a slice of every cluster in the string on each call, and
  segmented through `uniseg.StepString`, which also computes line-break state
  that a renderer never needs. `AllGraphemes` is the same iteration as a
  range-over-func with no allocation, reusing the width uniseg already measured
  while segmenting. `Buffer.SetStringn`, `StringWidth` and `TrimLeftColumns` now
  go through it.
- `Terminal` allocated a fresh update slice for every frame's diff.
  `Buffer.DiffInto` appends into a caller-owned slice, and the terminal keeps
  one across frames.

Together these took a frame at 80×24 from 348 µs and 178 KB of garbage to 110 µs
and 175 bytes.

A third fix is a correctness one. The backend skipped a cursor move whenever it
believed the cursor was already in place, predicting how far printing a symbol
had moved it with `uniseg.StringWidth`. How far the cursor actually moves is the
terminal's decision, though, and it need not match Unicode's tables: Windows
Terminal shapes Devanagari, Bengali, Tamil, Telugu, Thai and Arabic to widths
that do not agree. One wrong prediction desynchronised the writer from the real
cursor, and every later cell in that row was written to the wrong column, which
left fragments of previous frames on screen while scrolling. The cursor is now
tracked only through ASCII, where every terminal advances exactly one column;
after any other symbol the position is forgotten and the next cell re-anchors
with an absolute move. Ordinary text pays nothing. `cellColumns` also measured
with `uniseg` where the `Buffer` measures with `catatui.StringWidth`, a second
width implementation of the kind the library exists to avoid; it now uses the
library's own.

catatui's suite — including the 677 generated layout cases and the `cell_width`
conformance tests — passes unchanged. Tests were added pinning the grapheme
iterator to `cellWidth` and asserting it does not allocate, and `term` gained a
small VT interpreter that replays the backend's output against a simulated
terminal which *deliberately disagrees* about cluster widths, so the re-anchoring
rule is verified rather than assumed.

## Caveats

The file is memory-mapped for the life of the process. It stays open, and
truncating or overwriting it underneath a running `koneko` is undefined. The
Bubble Tea version also held the file open for its whole session, so this is not
a new restriction in practice, but mapping makes the failure mode a fault rather
than a short read.

`uniseg` v0.4.7 — the newest release — implements the grapheme rules through
GB9b, so it breaks an Indic conjunct in two: `परीक्षण` comes out as `प री क् ष
ण` rather than `प री क्ष ण`. That is Unicode 15.1's rule GB9c, and koneko
applies it over uniseg's segmentation in `conjunct.go`, from the
`Indic_Conjunct_Break` property. Without it a selection edge could stop inside a
conjunct: half the ligature was highlighted, and because a terminal groups cells
by their attributes before shaping them, the two halves were then drawn as
separate glyphs instead of the conjunct. Telugu and Devanagari are written
almost entirely in conjuncts, so it is most of the text rather than a corner of
it. The renderer preserves these joined clusters as complete buffer symbols,
so incremental redraws cannot insert cursor moves inside a conjunct.

The property covers six scripts: Devanagari, Bengali, Gujarati, Oriya, Telugu
and Malayalam. Tamil additionally uses tailored selection boundaries for the
ligatures `க்ஷ`, `ஶ்ரீ`, and `ஸ்ரீ`, as described in the
[W3C Tamil layout requirements](https://www.w3.org/TR/2020/WD-ilreq-taml-20200616/).
Other Tamil sequences with explicit pulli remain separate.

Layout, selection, truncation, and drawing share `textGraphemes`. Widths start
with catatui's measurement. On Windows they follow Windows Terminal's grapheme
mode, which [caps a Unicode cluster at two cells](https://github.com/microsoft/terminal/blob/main/src/types/CodepointWidthDetector.cpp).
The cap is applied after joining Indic conjuncts: `न्दी` takes two columns,
not the three obtained by adding the separate pieces. Tailored Tamil selection
units retain the sum of their terminal clusters' widths. The buffer receives
explicit width overrides where its own measurement differs.

Windows spacing marks also need a correction before that cap: uniseg treats
some marks with the grapheme property `Extend` as zero-width, while Windows
Terminal assigns them a column. This includes Bengali AA (`া`), so `লা` and
`খা` occupy two columns. The same correction covers corresponding marks in
Tamil, Malayalam, and Odia, keeping pointer positions and redraws aligned.

Getting that number wrong corrupts the frame in one of two ways, and both were
seen on the way here. Measure a cluster short and the next cell is written into
the middle of a glyph the terminal has already laid down, which took every
consonant carrying a spacing vowel sign off the screen and turned `हिन्दी` into
`न्दी`. Measure it long and the extra columns are never written at all — a wide
cell covers its own trailing columns, so the diff skips them — and the previous
frame's text stays there, which put stray characters beside every combining mark
and every joined emoji.

The Windows width policy assumes Windows Terminal's grapheme measurement mode;
terminal emulators using a different measurement mode may need a different
policy. Regression tests inspect the VT output and incremental redraws over
existing text, in addition to checking the selection and buffer contents.

Syntax highlighting tokenises a window around the visible range rather than the
whole file, so a block comment opened far above the viewport can be mis-coloured.
chroma exposes no way to resume a lexer from a checkpoint, which is what would
be needed to fix it.

## Development

```
go test ./...
go build
```

The benchmark corpus `testdata/sqlite3.c` is committed here; the benchmarks skip
if it is missing.

To eyeball a rendered frame without a terminal:

```
KONEKO_DUMP=1 KONEKO_DUMP_SCROLL=72 go test -run TestDumpFrame -v
```
