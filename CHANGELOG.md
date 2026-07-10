# Changelog

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
