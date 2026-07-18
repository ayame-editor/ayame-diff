<!-- i18n: language-switcher -->
[English](0002-diff-acceptance-architecture.md) | [日本語](0002-diff-acceptance-architecture.ja.md)

# ADR 0002: Architecture for Accepting diff / sortdiff in ayame-editor

- Status: Accepted (2026-07-10)
- Related Issue: hjosugi/ayame-diff#4
- Implementation Issues: hjosugi/ayame-diff#5 #6 #7 #8 #9
- Origin Epic: hjosugi/ayame-editor#104

## Background

The diff-related features from ayame-editor (Rust) will be migrated to this project (Go, zero dependencies policy).
Targeted reference implementation:

- `crates/ayame-cli/src/diff.rs` (line 610)
  - `cmd_diff`: **bounded resync window** line diff. Does not hold the full line LCS matrix, but scans only the previous `window` lines from anchor lines to resynchronize, making it **O(n) and memory-bounded**, suitable for large files. Output options include unified (default), `--side-by-side`, `--json`, `--summary`. Controlled via `--max-hunks`, `--max-lines`, `--window`, `--width`.
  - `cmd_sortdiff`: External sorts both files into temporary UTF-8 files, then passes them to `diff_documents`. Supports `--key/-k`, `--delim/-t`, `--quote`, `--numeric/-n`, `--reverse/-r`, `--csv`, `--budget`, `--spill-dir`.
  - Data model: `DiffResult` / `DiffHunk` / `DiffKind{Insert,Delete,Replace}`.
- `serve/ops.rs:968-1134` (`/api/diff`), `web/src/search.ts:539-741` (diff view)
  → To be received on the GUI side (#10 #11).

## Decisions

### 1. Migration approach: Re-implementation in Go

No Rust→Go FFI or subprocess linkage; **pure re-implementation in Go**.
Aligns with the zero-dependency (standard library only) policy, maintaining single binary distribution, `go install`, and ease of cross-compilation.
Will directly follow the reference implementation's algorithm (bounded resync window) and data model (Hunks of Insert/Delete/Replace).

### 2. CLI interface: Subcommand-based (with backward compatibility)

```bash
ayame-diff csv    [flags] --left A --right B   # Existing: CSV/TSV key comparison
ayame-diff text   [flags] OLD NEW              # New (#5): line diff (resync window)
ayame-diff sorted [flags] OLD NEW              # New (#7): external sort + text diff
ayame-diff        [flags]                      # Default = CSV compatibility (backward compatible)
```

> Note (2026-07-10): Interactive TUI wizard (#25) has been removed.
> Current default/no-argument invocation shows usage and exits.
> The `#5` subcommand will assign the default/no-argument case to CSV.

**Safe defaults + advanced escape hatch** (Sindre Sorhus style):

- Existing users' `ayame-diff --left ... --right ...` will **dispatch to default = csv** after subcommand implementation.
- New features will be placed under explicit subcommands, avoiding mixing unrelated flags (no `--mode` flag approach).
  Reason: different modes have different valid flags; separating via subcommands makes help and validation clearer.
- Default output is unified (human-readable). Use `--json` only when machine-readable output is needed.

Implementation of the subcommand dispatcher will be done in #5 (only the approach is fixed in this ADR).
Future `serve` (#10) / `gui` (#14) can naturally add their own subcommands under the same first argument.

### 3. Scope of shared engine (small, focused components)

Following the component division principle (Sindre Sorhus: Small Focused Modules), avoid creating a monolithic class; define clear boundaries.
Base on existing `internal/engine` (external sort/partition foundation):

| Package (planned) | Responsibility | Origin |
| --- | --- | --- |
| `internal/engine` (existing = fcsv) | CSV/TSV parsing, key comparison, **external sort/partition** | Current implementation |
| `internal/linediff` (new #5) | line diff with bounded resync window, `Hunk{Insert/Delete/Replace}` | Port from `diff.rs` |
| `internal/diffout` (new #6) | formatting unified/side-by-side/JSON/summary (separated from linediff) | Port from `diff.rs` |
| `internal/worddiff` (new #8) | word-level LCS highlighting within Replace hunks | Port from `search.ts` |

- **`sorted` will not perform external sort itself**. It reuses the existing `internal/engine` sort/spill infrastructure, passing output to `linediff` (similar to `cmd_sortdiff`).
  Job control (parallelism, backpressure, cancellation) will also share existing engine mechanisms.
- `linediff` will separate I/O and algorithm, keeping a pure core that does not depend on output formatting (`diffout`) (for testability and GUI reuse).

### 4. Encoding support

The reference implementation supports UTF-8 / Shift_JIS / EUC-JP / UTF-16, but this will be **separated into #9**.
Initial port (#5-#8) will assume UTF-8, with non-UTF-8 handling deferred to #9.

### 5. Zero dependency policy

**Maintain**. Only standard library.
Exceptions are allowed under clearly documented criteria:

- Only in areas where standard library does not exist and custom implementation is practical (e.g., non-UTF-8 decoding = `golang.org/x/text/encoding` will be reconsidered in #9, or for GUI WebView, etc.).
- When exceptions are made, record "why not possible with standard library" in the relevant Issue, and update `THIRD_PARTY_NOTICES.md`.
- The core CLI (`csv/text/sorted`) will strictly maintain zero dependencies.

## Completion criteria (what this ADR will fulfill)

Once the migration approach (Go re-implementation), CLI design (subcommands + backward compatibility), and shared engine scope (reuse of `linediff`, `diffout`, `worddiff`) are fixed, and implementation issues (#5 line diff, #6 output, #7 sortdiff, #8 word diff, #9 encoding) are ready to start, the work can proceed.

## Rejected proposals

- **`--mode=csv|text|sorted` flag approach**: Rejected because different modes have different valid flag sets, complicating help and validation (subcommands will separate namespace).
- **Including Rust binaries / calling subprocesses**: Rejected because it would lose the benefits of single binary distribution, zero dependencies, and cross-compilation.
