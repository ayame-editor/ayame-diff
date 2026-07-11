# ayame-diff

A native command-line diff tool for **huge** CSV / TSV and text files.

`ayame-diff` compares two files and writes the differences. It is designed for
inputs on the order of billions of rows: it never loads every line into memory,
it splits the input by a hash of the key, sorts each partition with a
memory-bounded external merge sort, and compares partitions with several
workers. It ships as a single static binary with only `golang.org/x/text` as a
dependency — no database, no CGO, no runtime to install.

## Highlights

- **CSV/TSV key comparison** (`csv`, the default), plus line-oriented text diff
  (`text`) and sort-then-diff (`sorted`).
- **Encoding auto-detection** — UTF-8, UTF-16 (LE/BE), Shift_JIS, EUC-JP and
  ISO-2022-JP — with a `--encoding` override. See [Encoding](encoding.md).
- **Comparison options** in the spirit of WinMerge: `--ignore-case`,
  `--ignore-whitespace`, per-word highlighting with `--word`, and bounded
  resync via `--window` / `--max-hunks`. See
  [Comparison options](comparison-options.md).
- **Local web GUI** — `serve` and `gui` start an embedded single-page app so you
  can compare files in the browser. See [GUI](gui.md).
- **Single binary** — cross-compiled for Linux, macOS and Windows.

## Install

### From GitHub Releases

If you would rather not install Go, download the archive for your OS and CPU
from the [latest release](https://github.com/hjosugi/ayame-diff/releases/latest):

- Windows x64 / ARM64: `ayame-diff-<version>-windows.zip`
- Linux x64 / ARM64: `ayame-diff-<version>-linux-<arch>.tar.gz`
- macOS Intel / Apple Silicon: `ayame-diff-<version>-darwin-<arch>.tar.gz`
- macOS double-click app: `ayame-diff-<version>-darwin-<arch>-app.zip`

Each release ships a `SHA256SUMS` file so you can verify the download.

### With `go install`

With Go 1.23 or newer you can build straight from source:

```bash
go install github.com/hjosugi/ayame-diff/cmd/ayame-diff@latest
```

## Quickstart

Line-diff two text files in the default unified format:

```bash
ayame-diff text old.txt new.txt
```

Compare two CSV/TSV files by key and write the differing rows to a TSV:

```bash
ayame-diff csv --left old.tsv --right new.csv --key id --out diff.tsv
```

Open the browser GUI and pick a free local port automatically:

```bash
ayame-diff gui
```

For Explorer, Finder, Linux file managers, drag-and-drop, and bare `A B`
invocations, see [File-manager and quick launch](shell-integration.md).

!!! tip "Bare invocation defaults to `csv`"
    Running `ayame-diff --left A --right B --out D` with no subcommand behaves
    exactly like `ayame-diff csv ...`, for backward compatibility.

## Subcommands at a glance

```text
ayame-diff csv    [flags] --left A --right B --out D   # CSV/TSV key comparison (default)
ayame-diff text   [flags] OLD NEW                      # line-oriented text diff
ayame-diff sorted [flags] OLD NEW                      # sort both sides, then diff
ayame-diff serve  [--addr host:port]                   # local web GUI
ayame-diff gui    [--addr host:port] [--no-open]       # serve on a free port and open the browser
```

Full details are in [Usage](usage.md). Every subcommand also prints its own help:

```bash
ayame-diff --help
ayame-diff text --help
```

## Links

- [Ayame family design](design.md) and the sister
  [ayame-editor](https://github.com/hjosugi/ayame-editor)
- [GitHub repository](https://github.com/hjosugi/ayame-diff)
- [Latest release](https://github.com/hjosugi/ayame-diff/releases/latest)
- [Changelog](https://github.com/hjosugi/ayame-diff/blob/main/CHANGELOG.md)
- [Contributing](https://github.com/hjosugi/ayame-diff/blob/main/CONTRIBUTING.md)
