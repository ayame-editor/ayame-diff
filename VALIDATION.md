# Validation report

Validation date: 2026-07-10
Applies to: main after the interactive TUI removal (post-v0.3.2)

## Automated checks

The following commands passed after removing the interactive TUI:

```bash
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
```

Packages checked:

```text
github.com/hjosugi/ayame-diff/cmd/ayame-diff
github.com/hjosugi/ayame-diff/internal/engine
```

The engine tests cover:

- default all-column key selection
- `--exclude-key` and `--exclude-key-index`
- explicit multiple key names and indexes
- mixed CSV and TSV input
- different row order and header column order
- duplicate-key multiset comparison
- RFC 4180 quoted comma, tab, and multiline fields
- gzip input and gzip output
- memory-bounded external merge sorting
- xxHash64 reference vectors

## Larger external-sort regression test

Fixture:

- 250,000 left rows
- 250,000 right rows
- left input TSV
- right input CSV with reversed row order and reordered columns
- all columns except `updated_at` used as the key
- one left-only logical key
- one right-only logical key
- one changed `updated_at` value
- `--memory 16MiB`
- `--workers 1`
- `--partitions 2`
- temporary run files retained and confirmed

Observed result for v0.3.0:

```text
left_rows:      250000
right_rows:     250000
equal_rows:     249998
left_only:      1
right_only:     1
changed_left:   1
changed_right:  1
diff_rows:      4
elapsed:        3.305s
maximum RSS:    59064 KB
```

The timing is only a validation result from the build container. It is not a performance guarantee. Real throughput depends on storage, row width, key distribution, compression, CPU count, memory settings, antivirus scanning, and output size.

## Cross-platform release builds

The following six release targets built successfully with `CGO_ENABLED=0`:

```text
linux/amd64
linux/arm64
darwin/amd64
darwin/arm64
windows/amd64
windows/arm64
```

Binary inspection result:

```text
ayame-diff-linux-amd64:       ELF 64-bit x86-64, statically linked
ayame-diff-linux-arm64:       ELF 64-bit ARM aarch64, statically linked
ayame-diff-darwin-amd64:      Mach-O 64-bit x86_64
ayame-diff-darwin-arm64:      Mach-O 64-bit arm64
ayame-diff-windows-amd64.exe: PE32+ Windows console executable, x86-64
ayame-diff-windows-arm64.exe: PE32+ Windows console executable, ARM64
```

The amd64 executable imports Windows system APIs from `kernel32.dll`; no third-party runtime DLL is required.

The Windows executables were cross-compiled, structurally inspected, and compile-time checked for the Win32 structure ABI in this Linux build environment. They were not executed on a physical or virtual Windows host during this validation.

## Build toolchain

The packaged binaries were built with Go 1.23.2 and `CGO_ENABLED=0`. The module declares Go 1.23 compatibility and has no external Go module dependencies.
