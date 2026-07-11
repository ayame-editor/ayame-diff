<!-- i18n: language-switcher -->
[English](0003-encoding-dependency.en.md) | [日本語](0003-encoding-dependency.md)

# ADR 0003: Example of Character Encoding Support and Dependency Exceptions (golang.org/x/text)

- Status: Accepted (2026-07-11)
- Related Issue: hjosugi/ayame-diff#9
- Reference: ADR 0002 (Zero Dependency Policy and Exception Criteria)

## Background

To handle Japanese files, detection and decoding of non-UTF-8 character encodings (Shift_JIS / EUC-JP / ISO-2022-JP / UTF-16) are necessary (corresponding to WinMerge's codepage support).
Shift_JIS / EUC-JP / ISO-2022-JP require large conversion tables, which are not included in the standard library.
Implementing these accurately on our own is impractical and prone to bugs.

## Decision

**Allow only `golang.org/x/text` as an external dependency.**
This aligns with the exception criteria of ADR 0002 ("limited to areas not present in the standard library and not feasible to implement in-house").

- Depend solely on the `internal/encoding` package (other packages access it via `internal/encoding`).
- Fix the version to **v0.21.0** to maintain compatibility with `go 1.23` (latest `x/text` requires `go 1.25+`, raising the minimum Go version for the module).
  Since Japanese/Unicode codecs are long-term stable, using an older version is acceptable.
- Document BSD-3-Clause license in `THIRD_PARTY_NOTICES.md`.
- Core CSV (`internal/engine`) and diff (`linediff`, etc.) remain dependency-free.

## Implementation

- `internal/encoding`: `Detect(sample, hint)` (detects BOM → explicit encoding → UTF-8 validity → Shift_JIS/EUC-JP heuristics) and `Decoder(r, name)` (streaming decode to UTF-8).
- `internal/linesrc.OpenEncoding(path, hint)`: detects encoding from the first 8KiB sample, decodes with `transform.NewReader`, and reads lines within memory bounds (even for large files).
- CLI: add `--encoding` option to `text` / `sorted` commands (default `auto`).
- GUI: encoding selection dropdown + `/api/diff`'s `encoding` parameter.

Supported encodings: `auto` / `utf-8` / `utf-16le` / `utf-16be` / `shift_jis` / `euc-jp` / `iso-2022-jp`.
UTF-8 BOM is removed; UTF-16 BOM is handled by the decoder.

## Rejected Alternatives

- **Zero dependency (no support for SJIS/EUC-JP)**: Rejected because it cannot fully support Japanese.
- **Implementing conversion tables in-house**: Rejected due to high maintenance cost and bug risk.
- **Latest `x/text`**: Fixed to v0.21.0 because newer versions require `go 1.25+`, raising the minimum Go version for the module.