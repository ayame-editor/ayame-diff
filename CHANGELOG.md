# Changelog

## Unreleased

- Added search inside the comparison result (Ctrl+F): every match is
  highlighted with a running count, Enter and Shift+Enter step between them, and
  the scope can be narrowed to changed lines, with match-case and regular
  expression toggles. It works on side-by-side, three-way and CSV results, and
  it matches against each line's whole text rather than the fragments syntax
  highlighting splits it into, so anchors and patterns spanning several tokens
  behave as written. The line-number gutter is excluded, so a numeric query
  matches content rather than line numbers. (#118)
- Made the inline word diff allocate one reusable table instead of one array
  per token per call, worth 1.64x on worst-case sized lines, and throttled the
  resize handler to a frame like the scroll handler beside it. (#155)
- Stopped the merge UI from searching the whole document for every row on
  every click. Both merge views resolved each row with a document-wide
  attribute selector, once per event, and a three-way result has no cap on its
  event count: with 3,000 conflicts a single merge click cost 661ms, now 4.5ms.
  The key-column search is also debounced and works from a cached list rather
  than re-reading and re-lowercasing every label on each keystroke. (#154)
- Removed two allocation hotspots the engine paid on every large comparison.
  Counting a file's lines built and discarded a string per line; counting now
  scans the decoded bytes instead, which on 200,000 lines is 90ms and 300,006
  allocations down to 62ms and 100,006. And the CSV difference count kept every
  difference ID in a set that nothing bounded — roughly 70MB per million
  differing rows, sized by the input rather than by the server — replaced by an
  exact constant-memory count, which is possible because the engine emits
  identical rows consecutively. (#156)
- Stopped a comparison with any ignore option from re-normalizing the same
  lines over and over. The resync scan holds one side fixed while sweeping the
  other, so each line was normalized once per candidate; a small window-sized
  cache now does it once. On 400 churn-heavy lines with ignore-case that is
  169ms and 172,590 allocations down to 2.3ms and 819 — the normalizing path is
  now as fast as the plain one, where before it was 62x slower. (#156)
- Made long GUI operations mutually exclusive, so a comparison and an export no
  longer run at once and race their results into the same view. The lock wraps
  the flows rather than the buttons, which is what covers the callers that never
  touch one — drag and drop, folder-entry clicks, sync-point edits, and the
  Enter key — and a superseded response is now discarded instead of painting
  over a newer one. Three-way comparison gained the AbortController, elapsed
  counter, and Cancel button the other paths already had, and folder comparison
  gained the counter, so every comparison reports progress and can be stopped.
  (#128)
- Stopped a large diff from freezing the GUI. Every result was built in one
  synchronous loop, so 20,000 rows — reachable at the default caps via large
  insertion blocks — blocked the main thread for ~15.7s with a blank result
  area and a stalled elapsed counter. Rendering is now sliced with yields
  between batches, off-screen hunks skip layout, and the whitespace markers and
  word-diff spans that were built even when their option was off (82% of the
  DOM: 660,000 of 801,400 elements) are only built when shown. The same diff
  now takes 758ms with a longest main-thread block of 356ms, and a placeholder
  fills the result area while the comparison runs. (#127)

## v0.8.0 - 2026-07-18

**Breaking:** the local GUI server now requires an API token on every `/api`
call. The URL that `gui` opens and `serve` prints carries it, so the browser
flow is unchanged, but a script calling the API directly must send the token in
an `X-Ayame-Token` header. See [the GUI guide](docs/gui.md) for details. New
input limits (`--max-line-bytes`, `--max-entries`, and the 1GiB cap on stdin and
`--pre`) can also refuse inputs that previously ran until they exhausted memory.

- Stopped the diff minimap from rendering as a tall empty bar next to a diff
  that already fits the screen: it now appears only when the result actually
  scrolls, and its column is reclaimed when hidden so a short diff is not
  indented by an empty gutter. (#102)
- Hid the per-hunk "use left / use right" merge buttons and the merge panel
  behind an opt-in merge-mode toggle, so the default view stays focused on
  reading diffs instead of merge controls. (#100)
- Preserved the base file's encoding, BOM, line endings, and final-newline
  state when saving a three-way text merge, so a Shift_JIS/CRLF (or UTF-8-BOM,
  UTF-16, EUC-JP, ISO-2022-JP) file round-trips instead of being rewritten as
  BOM-less UTF-8 with LF and a forced trailing newline. (#159)
- Closed the local GUI server to everything but the browser it launched: every
  run generates an API token that the opened or printed URL carries, and every
  `/api` call now requires it in a header, so a website the user happens to be
  visiting can no longer read or overwrite local files through the server or
  enumerate directories via `/api/files`. Because the credential is a header
  rather than a cookie, another origin cannot attach it without a CORS preflight
  that the server refuses, which also closes CSRF. A loopback server
  additionally pins the `Host` header, refusing DNS-rebinding requests even when
  they carry a valid token. The embedded UI, its assets, and `/api/health` stay
  open, as none exposes user data. (#108)
- Bounded the remaining inputs that could not stream, so an oversized one is
  refused with a way forward instead of exhausting memory: a single line past
  `--max-line-bytes` (default 64MiB) is rejected at open time, since a file with
  no line breaks defeats the sliding window; standard input and `--pre` output
  are capped at 1GiB with a pointer to passing a file instead; and a directory
  comparison past `--max-entries` (default 2,000,000 files) is refused before
  any file is read, since a result holds one entry per file. (#137)
- Made the `sorted` comparison memory-bounded: input within `--sort-memory`
  (default 256MiB) still sorts in memory, and anything larger spills sorted runs
  to `--temp-dir` and merges them with a bounded fan-in, so files far larger
  than RAM compare instead of being OOM-killed. Two ~370MB files at
  `--sort-memory 64MiB` now peak at 114MiB resident instead of 1.2GiB, with
  identical output. A failed spill explains that the default temporary directory
  is RAM-backed on many systems rather than reporting a bare "no space left on
  device". (#7, #137)
- Contained panics so a bug in one comparison no longer kills the process: the
  local GUI server answers with a 500 and keeps serving, the CLI reports the
  documented runtime-failure code 3 with a reportable trace instead of colliding
  with the usage code 2, and panics on partition, parallel-parse, progress, and
  directory-comparison worker goroutines — which no caller-side recover could
  reach — now fail their own unit of work. Corrected the documented exit codes,
  which still described the pre-#113 scheme. (#137)
- Fixed three-way CSV merge correctness: non-ASCII keys no longer fail to match
  (inputs are decoded before the byte-oriented engine, which reported raw bytes
  through JSONL and lost them to U+FFFD), the reconciled file keeps the base's
  encoding/BOM/line terminator instead of being rewritten as UTF-8, independent
  edits to different rows of a duplicated key group now auto-merge as `merged`
  rather than conflicting and dropping one side's edit, and a replacement row is
  written where the base row it replaces sat. (#160)
- Required `--allow-remote` before the local GUI can bind beyond loopback,
  printed usable loopback URLs for wildcard listeners, bounded browser drops
  per file and session with atomic cleanup, and added defensive HTTP transport
  timeouts without cutting off long comparisons or downloads. (#109)

## v0.7.16 - 2026-07-14

- Added folder-comparison report export so directory diffs can be saved and
  shared. (#191)
- Systematized CLI exit codes so scripts can distinguish a clean run, a found
  difference, a usage error, a runtime failure, and an interrupt. (#113)
- Surfaced each side's auto-detected encoding on file diffs, and localized the
  mode/status selects, the CSV/folder/three-way summaries, and byte counts.
  (#130, #125)
- Auto-detected ISO-2022-JP and BOM-less UTF-16 input, and split CJK text per
  character so inline word highlighting works for Japanese. (#158, #161)
- Rejected cross-origin state-changing requests and added HTTP hardening
  headers to the local GUI server. (#145, #146)
- Propagated request cancellation so directory, text, patch, and three-way
  diffs abort promptly when the browser disconnects instead of finishing the
  full comparison. (#169)
- Capped client-controlled memory and archive limits, bounded gzip
  decompression and JSON request bodies, and gated expensive comparisons behind
  a concurrency limit. (#170, #147)
- Excluded live sessions from browser-drop cleanup and moved the filesystem
  scan out of the lock, so an in-flight upload is never deleted mid-request.
  (#168)
- Made the CSV-merge overwrite atomic on Windows and capped worker counts.
  (#171)
- Corrected duplicate-key cell attribution and numeric/NaN sort ordering in the
  comparison engine. (#165)
- Detected binary and directory inputs on the text path and steered users to
  the binary or folder modes instead of emitting a garbled diff. (#166)
- Disabled controls that do not apply to the current mode and tied the
  move-min-lines input to move detection. (#124)
- Kept syntax tokens legible on changed rows by pulling same-hue colors toward
  the body text color, and padded the inline word-diff highlights. (#150)
- Gave every GUI control hover and active feedback with short, reduced-motion-
  aware transitions so the interface no longer feels inert. (#149)
- Tokenized syntax highlighting per line without O(L²) substring allocation.
  (#152)
- Hardened release engineering: a minimum-Go build/test job, a pinned
  staticcheck, de-duplicated lint runs, reproducible release archives, and a
  drift guard for the generated package manifests. (#172)
- Broadened server handler test coverage across error and guard branches, and
  redesigned the documentation navigation. (#141)

## v0.7.15 - 2026-07-13

- Fixed appending to a file without a final newline so the unchanged former
  last line is no longer reported as a false replacement; LF, CRLF, and CR
  inputs now produce only the actual insertion. (#162)
- Made truncated move detection explicit in CLI summaries, JSON/API responses,
  and the localized GUI instead of leaving an ambiguous zero-move result.
  (#164)
- Unified current/selected outlines behind a shared 2px token, added an active
  busy spinner with reduced-motion handling, and standardized empty-result
  cards across text, CSV, and three-way views. (#151)
- Added positive complete-match cards with the compared row/line/column scope,
  distinguished matches under comparison rules, and prevented truncated CSV
  results from claiming complete verification. (#122)

## v0.7.14 - 2026-07-13

- Added complete top-level CLI help covering every subcommand and GUI entry
  point while preserving explicit CSV help and existing script compatibility.
  (#95)
- Completed keyboard and screen-reader support for focus rings, the minimap,
  sync-point cells, and live status announcements, and localized the remaining
  client-side validation messages. (#123, #143)
- Made HTTP failures accurately distinguish invalid requests, missing files,
  permission failures, timeouts, unresolved merges, and internal errors; health
  checks now reject unsupported methods. (#112)
- Rejected invalid sync points throughout the diff engine and all text API
  paths instead of silently falling back to an unanchored comparison. (#163)
- Added field-specific validation for numeric API controls without leaking Go
  struct or type names for negative, zero, fractional, or oversized values.
  (#167)
- Added line-level comparison and exact terminator preservation for classic Mac
  CR-only files across in-memory, plain-file, gzip, and decoded inputs. (#157)

## v0.7.13 - 2026-07-12

- Completed the bilingual user and contributor documentation, including GUI
  workflows, comparison options, packaging guidance, and current screenshots.
  (#62)
- Streamlined the initial GUI workflow with OLD-field autofocus, Enter and
  Ctrl/Cmd+Enter comparison, a single primary Compare action, visible setup and
  drag-and-drop guidance, and patch export tied to the currently displayed
  result. (#87, #88, #89)
- Made navigation and language controls fully accessible and locale-aware:
  symbolic controls now have translated names and tooltips, language switching
  shows both the current and target language, and static title, aria-label, and
  natural-language placeholder attributes update with the locale. (#101, #131,
  #142)
- Added direct CSV page-number navigation with bounded input, localized
  previous/next controls, Enter activation, and keyboard focus preservation.
  (#136)

## v0.7.12 - 2026-07-12

- Centralized text, CSV, project, and engine output writes behind a shared
  atomic-write helper that stages a sibling file, syncs it, preserves requested
  permissions, and cleans up safely on failure. (#71)
- Added `clip:` / `clipboard:` pseudo paths to compare a file directly with the
  desktop clipboard on macOS, Windows, Wayland, and X11, including `--pre`
  preprocessing and deterministic backend fallback. (#76)
- Added dependency-free, line-local syntax highlighting to the side-by-side GUI
  for common source, data, markup, and log formats, with a persistent Japanese /
  English display toggle and semantic theme colors. (#78)

## v0.7.11 - 2026-07-12

- Kept binary/hex comparison memory-bounded even for dense differences by
  capping the retained bytes per region; `--max-bytes` now controls both
  retention and display, and scanning still reports exact sizes and differing
  byte totals after output truncation. (#68)
- Added true recursive `**` glob matching for folder and archive include/exclude
  filters, including zero-directory and deeply nested matches. (#69)
- Protected archive comparison from oversized entries and decompression bombs
  with configurable per-entry and per-archive expansion limits in both the CLI
  and directory-diff API. (#70)

## v0.7.10 - 2026-07-12

- Made a no-argument invocation print help and exit successfully so WinGet and
  other portable package managers can probe the installed command alias
  without receiving a false application failure.
- Added a pre-publication Windows package gate that extracts the exact release
  ZIP, executes its x64 binary, verifies no-argument/help/version/text flows,
  checks the ARM64 payload, and binds both WinGet manifest entries to the ZIP's
  actual SHA-256. GitHub Releases are now created only after this gate passes.

## v0.7.9 - 2026-07-12

- Added two-way and three-way merge workflows for text and keyed CSV, including
  per-hunk/per-row choices, undo/redo, conflict tracking, and safe result
  writing.
- Completed structured CSV comparison: cell-level results, full GUI setup,
  pagination, tolerance and ignore controls, reusable project files, and
  portable export settings.
- Added applyable normal, context, and unified patch output with external
  `patch`/`git apply` compatibility checks.
- Expanded diff review in the GUI with first/previous/next/last navigation, a
  location map, moved-block detection, sync points, and auditable ignored
  hunks.
- Completed scalable folder comparison and consolidated the record-processing
  and CLI exit/stream pipelines; binary record buffers are now reused to reduce
  allocation pressure.
- Added native application icons and launchers, file-manager shell integration,
  generated WinGet/Scoop/Homebrew metadata, and a VirusTotal release gate.
- Unified GUI typography, the CJK-capable monospace stack, control radii, and
  semantic color tokens with Ayame Editor. The colorblind scheme now keeps the
  same translucent Ayame treatment instead of reverting to the legacy solid
  palette. Added drift tests for the shared tokens. (#63, #64, #65, #66, #67)
- Added a release gate requiring every pushed version tag to have an exact
  CHANGELOG section, preventing the release notes drift found in #79.

## v0.7.8 - 2026-07-11

- Web UI: **scratch / paste comparison** — toggle "paste text" to diff two
  pasted/typed texts directly, without saving files first. Backed by an inline
  mode on `/api/diff` (`inline` + `oldText`/`newText`). (#55, scratch part)

## v0.7.7 - 2026-07-11

- Web UI: added a **show-whitespace** toggle that renders spaces as `·` and tabs
  as `→` (dimmed), making whitespace-only differences visible; applies
  immediately and persists. Completes the display-options set (wrap, colors,
  whitespace). (#36)

## v0.7.6 - 2026-07-11

- Web UI: added a **colorblind-safe color scheme** (blue = added, orange =
  deleted) selectable in the UI, plus a line-wrap toggle. Both persist across
  visits and work in light and dark. (#58, #36 partial)

## v0.7.5 - 2026-07-11

- Added `--pre <command>` to `text`/`sorted`: preprocess each input through a
  shell command before diffing (WinMerge's unpacker/prediffer, Unix-style) —
  e.g. `--pre 'jq -S .'` to canonicalize JSON, `--pre 'sort'`, `--pre 'tr A-Z a-z'`.
  The command's output is then encoding-detected and diffed. (#56)

## v0.7.4 - 2026-07-11

- `dir` now compares archives too: pass a `.zip`, `.tar`, `.tar.gz`, or `.tgz`
  on either side and it is compared like a folder (archive vs archive, or
  archive vs directory). Archive contents are read into memory. (#53)

## v0.7.3 - 2026-07-11

- Added `--html <file>` to `text`/`sorted`: write a self-contained HTML diff
  report (inline CSS, light/dark, side-by-side with word highlighting) you can
  share or archive. (#33)

## v0.7.2 - 2026-07-11

- Added `--normal` to `text`/`sorted`: GNU normal-diff (patch) output
  (`<n>c<n>` / `<n>a<n>` / `<n>d<n>` with `< `/`> ` lines) — byte-identical to
  `diff`. (#54, normal format; applyable unified/context patches with
  patch/git-apply CI verification remain.)

## v0.7.1 - 2026-07-11

- Added a `bin` subcommand: byte-level (binary/hex) diff of two files. Streams
  both files and prints each differing region as its offset with the old/new
  bytes in hex; nearby differences coalesce into one region. (#57)

## v0.7.0 - 2026-07-11

- Added a `dir` subcommand: recursively compare two directory trees **by file
  content** and report added (`+`), removed (`-`), and changed (`~`) files
  (WinMerge-style folder comparison). `--all` also lists unchanged files,
  `--json` emits structured output, and `--exclude <glob>` skips paths. (#52)

## v0.6.4 - 2026-07-11

- `text` and `sorted` now accept `-` as OLD or NEW to read from standard input,
  so you can compare piped data (`... | ayame-diff text - file`). Stdin is
  encoding-detected and BOM-stripped like a file. (#55, stdin part)

## v0.6.3 - 2026-07-11

- Added self-update: `ayame-diff update` downloads the latest GitHub release,
  verifies its SHA-256 against `SHA256SUMS`, and replaces the running binary in
  place (`--check` only reports). `ayame-diff remove` uninstalls a standalone
  binary and leaves Homebrew/Scoop/Nix installs to their package manager. (#20)

## v0.6.2 - 2026-07-11

- Added "open the GUI without a terminal" launchers: `start-gui.cmd` (bundled in
  the Windows zip), a Linux `.desktop` file, and a macOS `.app` builder
  (`packaging/macos/build-app.sh`). All run `ayame-diff gui`. (#23, partial —
  app icon and the release.yml `.app` step are follow-ups.)

## v0.6.1 - 2026-07-11

- Web UI: elapsed-time display and a Cancel button that aborts an in-flight
  compare. (#13)
- Added a MkDocs (Material) documentation site under `docs/` with a GitHub Pages
  deploy workflow. (#17)
- Expanded server/API test coverage (sorted mode, Shift_JIS decoding via the
  API, ignore-case/whitespace, max-hunks/max-lines capping). (#16)

## v0.6.0 - 2026-07-11

- Added a `gui` subcommand: starts the local web UI on a free localhost port and
  opens it in your browser (`--no-open` to skip, `--addr` to pin). It's the
  "double-click to a GUI" experience without a native-webview dependency, so the
  binary stays a single static cross-compiled executable. (#14)

## v0.5.2 - 2026-07-11

- Display width is now grapheme-cluster / emoji / East-Asian-width aware, so
  `--side-by-side` columns align for emoji (including ZWJ sequences and flags)
  and CJK text.
- The external-merge sort now raises the process open-file limit to fit the
  partition/fan-in configuration, failing early with a clear message when the
  hard limit is too low.
- Added a Japanese README (`README.ja.md`) and an EN/JA language switcher. (#18)
- Added installers and package manifests: `scripts/install.sh` (Linux/macOS),
  `scripts/install.ps1` (Windows), plus Scoop and Homebrew manifests. (#19)
- Added a `lint` CI workflow (gofmt, `go vet`, staticcheck) and strengthened the
  release gate to run gofmt + vet + `go test -race` before publishing. (#22)

## v0.5.1 - 2026-07-11

- Added WinMerge-style comparison options to `text` and `sorted` (and the web
  UI): `--ignore-case` and `--ignore-whitespace <none|change|all>` (change
  collapses runs of whitespace and trims ends; all removes whitespace). These
  normalize only the text used for comparison — the output still shows the
  original lines.
- Version embedding is now derived from `git describe` in the Makefile and CI,
  instead of a hard-coded (and stale) string. (#21)

## v0.5.0 - 2026-07-11 — full Japanese encoding support

- Added character-encoding detection and decoding for non-UTF-8 input (#9):
  **UTF-8, UTF-16 (LE/BE, BOM-aware), Shift_JIS, EUC-JP, and ISO-2022-JP**. The
  `text` and `sorted` subcommands auto-detect the encoding (BOM first, then a
  UTF-8 / Japanese heuristic) and decode to UTF-8 while streaming with bounded
  memory. Override with `--encoding <name>`; the web UI has an encoding
  dropdown. Modeled on WinMerge's codepage support.
- **New dependency:** `golang.org/x/text` (pinned v0.21.0), confined to
  `internal/encoding`, for the vetted Japanese/UTF-16 codec tables. It is the
  project's only dependency beyond the standard library; the CSV and diff cores
  stay standard-library-only. See ADR 0003 and `THIRD_PARTY_NOTICES.md`.
- Web UI: encoding selector; Japanese label reads 「ワードハイライト」.

## v0.4.0 - 2026-07-10 — GUI (web) debut

- Added a `serve` subcommand and local web UI (#10, #11). `ayame-diff serve`
  starts a localhost web app for comparing files in the browser: enter two
  paths, pick `text` or `sorted` mode plus options, and browse the diff as a
  side-by-side grid with per-hunk headers, line numbers, and word-level
  highlighting. Backed by a JSON `/api/diff` endpoint and an embedded,
  framework-free frontend (dependency-zero).
- Bilingual (Japanese / English) UI with a language toggle, defaulting to the
  browser locale and remembered across visits. (#15)
- Extracted the line-sort logic into `internal/linesort`, now shared by the
  `sorted` subcommand and the server.

Follow-ups tracked for the GUI: native desktop window (#14), progress/cancel
for long runs (#13), and GUI packaging/docs (#23, #17).

## v0.3.4 - 2026-07-10

- Added `--word` to the `text` and `sorted` subcommands: in unified output it
  highlights the changed words inside a Replace hunk with git-style
  `[-removed-]` / `{+added+}` markers, using the new `worddiff` LCS engine.
  Unchanged words stay plain; very large or identical lines fall back to plain
  `-`/`+`. (#8)
- `text` / `sorted` now strip a leading UTF-8 byte-order mark (BOM) so the first
  line is not prefixed with a stray marker. (#9, partial — Shift_JIS / EUC-JP /
  UTF-16 support is pending a dependency decision on #9.)

## v0.3.3 - 2026-07-10

### Added — line diff (migrated from ayame-editor)

- Added a `text` subcommand: line-level diff of two text files (plain or `.gz`)
  by row order, using a bounded resync window that stays linear and
  memory-bounded on huge inputs (no O(n·m) LCS matrix). Output as unified
  (default), `--side-by-side`, `--json`, or `--summary`, controlled by
  `--max-hunks`, `--max-lines`, `--window`, `--width`. (#5, #6)
- Added a `sorted` subcommand: sort both files line-wise (`--numeric`,
  `--reverse`) then diff — for files that hold the same rows in a different
  order. v1 sorts in memory; a memory-bounded external sort is tracked in #7.
- Introduced subcommands `csv` / `text` / `sorted`. A bare invocation, or one
  that starts with a flag, stays on the existing CSV/TSV key comparison for
  backward compatibility (ADR 0002).
- New dependency-free internal packages ported from ayame-editor: `linediff`
  (diff engine + parity tests), `diffout` (unified/side-by-side/JSON/summary),
  `linesrc` (bounded-memory plain/gzip line source), `worddiff` (LCS word diff,
  #8 — CLI rendering to follow).

### Changed

- **Breaking:** Removed the interactive terminal UI — the setup wizard, the
  `--interactive` flag, and the `internal/tui` / `internal/interactive`
  packages. A bare invocation now prints usage and exits 2; pass `--left`,
  `--right`, `--out` (plus key options) directly, or use `text` / `sorted`.
  `--interactive` prints a migration pointer. The project is moving to a GUI
  (#10–#14, #37). (#25, #37)
- Removed the now-unused `engine.InspectInputs` header-inspection helper and the
  Windows `start-interactive.cmd` launcher.
- Preserved the removed TUI's wcwidth/CJK display-width logic as the new
  `internal/textwidth` package, now used for `--side-by-side` alignment. (#6, #37)

## v0.3.2 - 2026-07-10

- Renamed the project to `ayame-diff` to align with its sister project ayame-editor. The Go module path is now `github.com/hjosugi/ayame-diff`, the binary is `ayame-diff`, and the entry point is `cmd/ayame-diff`.
- **Breaking:** `go install github.com/hjosugi/fcsv-diff/cmd/fcsv-diff@latest` no longer works. Use `go install github.com/hjosugi/ayame-diff/cmd/ayame-diff@latest`. The `fcsv-diff` binary name is deprecated; the `fcsv` name is retained only for the internal CSV engine.
- No change to CLI flags, CSV/TSV comparison behavior, or output — this is a pure identifier rename.
- Recorded the naming and diff/sortdiff acceptance-architecture decisions as ADRs under `docs/adr/`.

## v0.3.1 - 2026-07-10

- Fixed interactive startup in WezTerm and other ConPTY hosts when stdout is not a console screen-buffer handle.
- Added `CONIN$` / `CONOUT$` fallback handles for redirected Windows standard input and output.
- Preserved the native Unicode Win32 input and drawing path after resolving the active console devices.

## v0.3.0 - 2026-07-10

- Added a full-screen interactive setup wizard, launched with no arguments or `--interactive`.
- Added first-record header inspection for CSV, TSV, CSV.GZ, and TSV.GZ without scanning the full input.
- Added Space-key multi-selection for included and excluded key columns.
- Added case-insensitive header search, select-visible, clear-visible, invert-visible, paging, and jump navigation.
- Added interactive editing for format, delimiter, parser mode, memory, temporary storage, partitioning, and worker settings.
- Added native Unicode Windows console input/output using Win32 Console APIs with no third-party DLLs.
- Added Japanese/CJK display-width handling and long-path horizontal scrolling.
- Added a double-clickable Windows interactive launcher.
- Preserved all v0.2.0 command-line behavior.

## v0.2.0 - 2026-07-10

- Changed the default key selection to all columns when no key option is given.
- Added repeatable `--exclude-key` and `--exclude-key-index` options.
- Kept excluded columns in full-row comparison and diff output.
- Rejected mixed include-key and exclude-key modes.
- Added a single-copy storage path for the default full-row key to reduce temporary disk I/O.
- Added a Windows-only build script and Windows-native package documentation.

## v0.1.0 - 2026-07-09

- Initial release.
- Added mixed CSV/TSV and gzip input support.
- Added multiple header-name and column-index keys.
- Added header-based column alignment.
- Added parallel simple parser, hash partitioning, external merge sort, and parallel comparison.
- Added multiset duplicate-key semantics.
- Added TSV and TSV.GZ difference output.
- Added Linux, macOS, and Windows cross-build scripts.
