<div align="center">

# koneko

**A fast terminal file viewer you can actually select text in.**

Like `less` and `bat`, with mouse selection, clipboard copy, search,
syntax highlighting, git change markers and correct Unicode.

[Features](#features) · [Install](#install) · [Usage](#usage) · [Keys](#key-bindings) · [Performance](#performance)

</div>

---

- **Select with the mouse, copy with <kbd>y</kbd>.** Click a line, drag across
  text, double-click a word. The text goes straight to the system clipboard.
- **Point Claude at code.** Click a line, press <kbd>r</kbd>, and paste
  `src/app.go:12` into Claude Code or any other tool that understands
  `path:line`.
- **Unicode that works.** Emoji, ZWJ sequences, CJK, Devanagari, Bengali,
  Tamil and Thai render, scroll and select as whole characters. No broken cells
  and no leftovers on screen.
- **Fast on big files.** Opens a 9.8 MB, 270k-line file in about 6 ms. Draws a
  frame in about 110 µs with 6 allocations. Uses no CPU while idle.
- **Knows about git.** The gutter marks lines added, modified and deleted since
  `HEAD`, and one key jumps to the next change.

## Features

### Viewing
- The file is memory-mapped, so large files open immediately and scroll smoothly.
- Line numbers and a draggable scrollbar, each togglable.
- Horizontal scrolling that stops at the widest visible line, so you can't
  scroll off into empty space.
- Adjustable tab width (`-tab-width`, default 4).
- Handles CRLF line endings and files without a trailing newline.
- The status bar shows the file name, the selection range, the current search
  match and the scroll position.

### Syntax highlighting
- Hundreds of languages via [chroma](https://github.com/alecthomas/chroma),
  detected from the file name.
- Highlighting runs on a background worker, so scrolling never waits for it.
- Toggle it with <kbd>h</kbd>, or start with `-no-highlight`.

### Themes
Eight built-in themes: **dracula** (default), **monokai**, **nord**,
**tokyonight**, **github**, **autumn**, **ferra** and **base16**. base16 uses
your terminal's own palette. Selected text keeps its syntax colours, and each
theme gets a proper selection background.

### Selection & clipboard
- Click a line to select it. Drag to select exactly the text you want.
  Right-click to extend the selection to the clicked point.
- Double-click selects a word. Keep dragging to extend by word, or from a
  triple-click, by line.
- Click a line number to select the whole line. Right-click one to extend the
  selection to it.
- Dragging past the top or bottom edge scrolls automatically, so selections can
  be longer than the screen.
- <kbd>a</kbd> selects all, <kbd>x</kbd> extends to full lines, <kbd>d</kbd>
  clears, and <kbd>y</kbd> copies and shows how many lines were copied.
- <kbd>r</kbd> copies a reference to the selected lines instead of their text:
  `src/app.go:12`, or `src/app.go:12-18` for several lines. The path is relative
  to the git root, or absolute outside a repository.
- Word and character boundaries follow grapheme clusters, so a double-click
  never splits `हिन्दी`.

### Search
- <kbd>/</kbd> searches the whole file, case-insensitive, starting from where
  you're looking.
- <kbd>n</kbd> / <kbd>N</kbd> step through matches and wrap around. Each match
  is selected and scrolled into view, so <kbd>y</kbd> copies it. The status bar
  shows which match you're on, for example `malloc 3/314`.
- The prompt has proper editing: arrows, <kbd>Home</kbd>/<kbd>End</kbd>,
  <kbd>Del</kbd>, <kbd>Ctrl</kbd>+<kbd>U</kbd>.

### Git change markers
- Inside a git repository, lines changed since `HEAD` get a mark in the gutter:
  a **green** bar for added lines, **yellow** for modified, and a **red** `▁`
  where lines were deleted.
- <kbd>]</kbd> and <kbd>[</kbd> jump to the next and previous change, and
  <kbd>c</kbd> toggles the markers.
- The diff runs in the background, so a file never waits on git to open.

### Built for integration
- `-select=LINE:CHAR-LINE:CHAR` opens a file with a range already selected and
  in view. `CHAR` counts grapheme clusters, so a caller doesn't need to know the
  file's encoding.
- `-search=TEXT` opens a file already jumped to the first match.
- [Modal Commander](https://github.com/Fiend3d/mc) uses these to open its
  search results right on the matching text.
- Going the other way, <kbd>r</kbd> copies a `path:line` reference that you
  can paste into Claude Code, an editor or a chat.

### Help built in
Press <kbd>F1</kbd> for a full-screen, scrollable key reference.

## Install

### With Modal Commander
koneko ships with [**mc**](https://github.com/Fiend3d/mc) and is its default
<kbd>F3</kbd> viewer, so installing mc gets you koneko too.

### From source
Requires Go 1.27 or newer.

```
git clone https://github.com/Fiend3d/koneko
cd koneko
go build            # or .\build.ps1 on Windows
```

On Linux, clipboard copy needs `xclip`, `xsel` or `wl-clipboard`.

## Usage

```
koneko [OPTIONS] <FILE>
```

| Option | Description |
|---|---|
| `-theme=NAME` | Colour theme: `autumn`, `base16`, `dracula`, `ferra`, `github`, `monokai`, `nord`, `tokyonight` (default `dracula`) |
| `-tab-width=N` | Tab display width (default `4`) |
| `-no-line-numbers` | Start with line numbers hidden |
| `-no-scrollbar` | Start with the scrollbar hidden |
| `-no-highlight` | Start with syntax highlighting off |
| `-no-git` | Don't show git change markers |
| `-search=TEXT` | Open with `TEXT` searched and the first match shown |
| `-select=L:C-L:C` | Open with a range selected (1-based line and character) |
| `-v`, `-version` | Print the version and exit |

`-search` and `-select` can't be used together.

```
koneko main.go
koneko -theme=nord -tab-width=8 Makefile
koneko -select=120:5-124:1 server.log
```

## Key bindings

#### Navigation
| Key | Action |
|---|---|
| <kbd>↑</kbd> / <kbd>k</kbd> | Scroll up one line |
| <kbd>↓</kbd> / <kbd>j</kbd> | Scroll down one line |
| <kbd>←</kbd> / <kbd>→</kbd> | Scroll left / right |
| <kbd>PgUp</kbd> / <kbd>PgDn</kbd> | Scroll half a screen |
| <kbd>Home</kbd> / <kbd>g</kbd> | Go to top |
| <kbd>End</kbd> / <kbd>G</kbd> | Go to bottom |
| <kbd>H</kbd> | Reset horizontal scroll |

#### Selection
| Key | Action |
|---|---|
| <kbd>a</kbd> | Select all |
| <kbd>d</kbd> | Deselect |
| <kbd>y</kbd> | Copy selection |
| <kbd>r</kbd> | Copy `path:line` reference |
| <kbd>x</kbd> | Extend selection to full lines |

#### Search
| Key | Action |
|---|---|
| <kbd>/</kbd> | Enter search mode |
| <kbd>Enter</kbd> | Commit search |
| <kbd>Esc</kbd> | Cancel search |
| <kbd>n</kbd> | Next match |
| <kbd>N</kbd> | Previous match |

#### Display
| Key | Action |
|---|---|
| <kbd>l</kbd> | Toggle line numbers |
| <kbd>s</kbd> | Toggle scrollbar |
| <kbd>h</kbd> | Toggle syntax highlighting |

#### Git
| Key | Action |
|---|---|
| <kbd>]</kbd> | Next change |
| <kbd>[</kbd> | Previous change |
| <kbd>c</kbd> | Toggle change markers |

#### Mouse
| Input | Action |
|---|---|
| Left click | Select line (drag to select text) |
| Left drag | Extend selection |
| Right click | Extend selection to clicked position |
| Double click | Select word (drag to extend by word) |
| Triple click | Select line (drag to extend by line) |
| Wheel | Scroll |
| Left click on gutter | Select whole line |
| Right click on gutter | Extend selection to line |
| Drag scrollbar | Jump to position |

#### Other
| Key | Action |
|---|---|
| <kbd>F1</kbd> | Open / close help |
| <kbd>q</kbd> / <kbd>Ctrl</kbd>+<kbd>C</kbd> | Quit |

## Performance

- **~110 µs** per frame at 80×24 and **~480 µs** at 200×50, so drawing is
  never the bottleneck.
- **6 allocations per frame** (about 175 bytes), whatever the terminal size.
- **Zero CPU while idle.** The event loop blocks until something happens.
- **One frame per burst.** A fast wheel spin or drag is drawn once, not once
  per event.

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

## Under the hood

koneko is built on [catatui](https://github.com/Fiend3d/catatui), a Go port of
ratatui: you draw into a `Buffer` of cells, a constraint solver decides where
things go, and the `Terminal` writes only what changed.

<details>
<summary><b>Why the rewrite</b></summary>

<br>

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

</details>

<details>
<summary><b>Behaviour that deliberately differs from the Bubble Tea version</b></summary>

<br>

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

</details>

<details>
<summary><b>Where koneko wins and loses against nezumi</b></summary>

<br>

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

The event loop blocks when idle and drains each burst of input before drawing,
so a fast wheel spin or drag produces one frame rather than one per event — the
Bubble Tea version rendered once per message.

</details>

<details>
<summary><b>Changes made to catatui</b></summary>

<br>

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

</details>

<details>
<summary><b>Caveats</b></summary>

<br>

The file is memory-mapped for the life of the process. It stays open, and
truncating or overwriting it underneath a running `koneko` is undefined. The
Bubble Tea version also held the file open for its whole session, so this is not
a new restriction in practice, but mapping makes the failure mode a fault rather
than a short read.

Unicode segmentation and terminal widths belong to catatui. Koneko uses
`catatui.SegmentGraphemes` to build byte-to-column tables and expand tabs, and
renders through ordinary `Buffer.SetStringn`. There is no separate grapheme
joiner, terminal-width correction, forced-width cell, or custom buffer writer
in Koneko.

catatui keeps Indic conjuncts and Tamil ksha/sri ligatures intact through
measurement, clipping, buffer diffing, and terminal output. Its Windows Terminal
width policy also accounts for Bengali spacing marks and the two-cell limit
per Unicode cluster. Applications targeting another terminal can configure
`catatui.DefaultWidthPolicy` once at startup, before creating buffers.

Regression tests cover the Unicode corpus, forward and backward selections,
clipping, and incremental terminal output over existing text. Core segmentation
and width-policy regressions live in catatui; Koneko retains the viewer and
selection integration tests.

Syntax highlighting tokenises a window around the visible range rather than the
whole file, so a block comment opened far above the viewport can be mis-coloured.
chroma exposes no way to resume a lexer from a checkpoint, which is what would
be needed to fix it.

Git change markers are taken once, when the file is opened, by running
`git diff HEAD` on the file as it is on disk. They do not refresh while the
viewer is open, and an untracked file shows none. Searching non-ASCII text works
but is case-sensitive for the non-ASCII part.

</details>

## Development

```
go test ./...
go build
go test -bench=. ./...    # needs testdata/sqlite3.c; benchmarks skip without it
```

The benchmark corpus `testdata/sqlite3.c` is committed here; the benchmarks skip
if it is missing.

To eyeball a rendered frame without a terminal:

```
KONEKO_DUMP=1 KONEKO_DUMP_SCROLL=72 go test -run TestDumpFrame -v
```
