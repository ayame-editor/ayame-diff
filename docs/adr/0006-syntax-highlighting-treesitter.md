<!-- i18n: language-switcher -->
[English](0006-syntax-highlighting-treesitter.md) | [日本語](0006-syntax-highlighting-treesitter.ja.md)

# ADR 0006: Syntax highlighting — stateful multi-line approximation now, tree-sitter deferred

- Status: Accepted (2026-09-27)
- Related Issue: hjosugi/ayame-diff#287
- Reference: ADR 0002 (zero-dependency policy), ADR 0003 (dependency exceptions)

## Background

Issue #287 proposes replacing the client-side syntax highlighter
(`internal/server/web/syntax.js`) with tree-sitter, so that multi-line
constructs (block comments, multi-line strings, template literals, heredocs)
are colored accurately, and as a stepping stone to syntax-aware structural
diffs later.

The current highlighter is deliberately line-local: it never carries parser
state and never scans a whole file. It therefore cannot color the second line of
a block comment or multi-line string, and #152 already showed that maintaining a
hand-written tokenizer has a cost.

## Decision

**Do not vendor tree-sitter or a WASM grammar yet.** Keep the standard-library-only
build, the network-free offline GUI, and the small embedded asset budget, and
instead raise the accuracy of the existing highlighter in a bounded way:

- `syntax.js` gains a small state machine (`createHighlighter`) that carries
  state across consecutive lines for the multi-line constructs it models.
- `highlightSpans` — the unchanged line-local pass — remains the **degradation
  path**. Whenever the accurate pass is unavailable (unknown extension,
  data/markup/log format, older front end, an error) or its state cannot be
  trusted (an omitted gap between hunks, context fetched out of order), the view
  falls back to it unchanged.
- The accuracy limit is documented rather than hidden.

## Why tree-sitter is not adopted yet

- **Bundle.** The SPA is embedded in the Go binary (`//go:embed web`, see
  `docs/gui.md`). `web-tree-sitter` plus one grammar `.wasm` per language is
  hundreds of kilobytes to megabytes. There is no bundler or build step — the
  front end is plain `<script>` files — so a runtime and grammars would have to
  be vendored and loaded explicitly, and the growth is paid by every user.
- **Offline/no-network.** Languages are loaded lazily in the proposal, so a
  grammar not shipped in the binary would have to be fetched at runtime, which
  contradicts the offline GUI. Shipping every grammar is the opposite trade.
- **Proof before cost.** The issue's acceptance criteria — correct multi-line
  coloring, no silent breakage on failure, acceptable bundle growth — are met
  for the common constructs by the state machine at essentially zero asset cost.
- **difftastic's own lesson.** Its author notes that structural diff scales
  poorly and uses a lot of memory on files with many changes. That is an argument
  for first having the visible degradation triggers (unknown extension, parse
  error, size/complexity), which the line-local fallback already provides.
- **Maintenance.** Each grammar brings its own version and license, and one more
  thing to update; the project has no toolchain to automate that today.

## Implementation

- `internal/server/web/syntax.js`: `createHighlighter(path)` (state machine with
  `reset`/`state`), `highlightLines(lines, path)`, and `sliceSpans(spans, …)`.
  `highlightSpans` and `languageForPath` are unchanged.
- Constructs: `/* … */` block comments, JavaScript/TypeScript template literals,
  Go raw strings, Python triple-quoted strings, and shell heredocs (`<<`,
  `<<-`, quoted delimiters).
- `internal/server/web/app.js`: highlights each hunk's lines in order per side
  before rendering, drops the carried state at an omitted gap, and uses the
  accurate spans as a lookup that misses into `highlightSpans`.

## Known limits (not implemented)

- Nested delimiters (Rust nested block comments close at the first marker).
- `${…}` interpolation inside a template literal is colored as string.
- Multiple heredocs on one command line; here-strings (`<<<`). Numeric shell
  shifts are excluded, but `<<` followed by an identifier inside `$(( ))`
  arithmetic can still be read as a heredoc opener.
- YAML block scalars, Rust raw strings, and other language-specific forms.
- Syntax-aware structural diff (hunk headers naming the enclosing function,
  folding by syntax block, whitespace-only re-indentation) remains a separate,
  future issue, exactly as #287 itself splits it out.

## Reconsider when

A vendored, offline, toolchain-free tree-sitter runtime with a small grammar set
fits the embedded-asset and licensing budgets, and multi-line coloring is
measurably insufficient in practice. Any adoption must ship the same mandatory
degradation triggers: unknown language, parse failure, and size/complexity
overflow all landing on the unchanged line-local path.
