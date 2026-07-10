# Changelog

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
