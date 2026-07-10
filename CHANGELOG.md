# Changelog

## Unreleased

- **Breaking:** Removed the interactive terminal UI — the setup wizard, the `--interactive` flag, and the `internal/tui` / `internal/interactive` packages. Running with no arguments now prints usage and exits with code 2; pass `--left`, `--right`, and `--out` (plus any key options) directly. The project is moving toward a GUI (see #25 and #10–#14), so the terminal wizard is retired rather than maintained alongside it.
- Removed the now-unused `engine.InspectInputs` header-inspection helper (it existed only to feed the wizard) and the Windows `start-interactive.cmd` launcher.

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
