<!-- i18n: language-switcher -->
[English](usage.md) | [日本語](usage.ja.md)

# Usage

`ayame-diff` provides comparison, web GUI, packaging, and maintenance commands.

## Command overview

```text
ayame-diff csv    [flags] --left A --right B --out D   # CSV/TSV key comparison (default)
ayame-diff text   [flags] OLD NEW                      # line-oriented text diff
ayame-diff sorted [flags] OLD NEW                      # sort both sides, then diff
ayame-diff dir    [flags] OLD NEW                      # directory/archive comparison
ayame-diff bin    [flags] OLD NEW                      # binary/hex comparison
ayame-diff 3way   [text|csv] [flags]                   # BASE / LEFT / RIGHT comparison
ayame-diff serve  [--addr host:port] [--allow-remote]  # local web GUI
ayame-diff gui    [flags] [OLD [NEW]]                  # open the GUI, optionally prefilled
ayame-diff update [--check]                            # check for or install the latest release
ayame-diff remove [--yes]                              # uninstall a standalone binary
ayame-diff shell-install                               # file-manager integration
ayame-diff shell-uninstall                             # remove integration
ayame-diff shell-select PATH                           # Windows Explorer integration helper
```

Invoking `ayame-diff` with `--left ... --right ...` and no subcommand runs as
`csv` for backward compatibility. The `serve` and `gui` subcommands are covered
in [GUI](gui.md).

Two bare paths are a quick-launch form: files use `text`, directories use
`dir`, and adding `--gui` opens and immediately runs the browser GUI. See
[File-manager and quick launch](shell-integration.md).

<div class="doc-jump-grid">
  <a class="doc-jump" href="#csv">Compare CSV / TSV</a>
  <a class="doc-jump" href="#text">Compare text</a>
  <a class="doc-jump" href="#sorted">Compare after sorting</a>
  <a class="doc-jump" href="#dir">Compare folders</a>
  <a class="doc-jump" href="#bin">Compare binary files</a>
  <a class="doc-jump" href="#update">Maintain an installation</a>
  <a class="doc-jump" href="#exit-codes">Use in scripts / CI</a>
  <a class="doc-jump" href="../gui/">Prefer the GUI?</a>
</div>

---

## `csv` — CSV/TSV key comparison (default) { #csv }

Compare two CSV/TSV files (including `.csv.gz` and `.tsv.gz`) by key, even when
their row order differs, and write the differing rows to a TSV. Left and right
may use different formats; if the header names match, differing column orders
are aligned automatically.

```bash
ayame-diff csv --left old.tsv --right new.csv --key id --out diff.tsv
```

### Choosing the key

There are three key modes; inclusion and exclusion cannot be mixed.

1. **No key options** — every column is part of the key (multiset row diff).
2. **`--key` / `--key-index`** — name (or 0-based index) the columns to include.
3. **`--exclude-key` / `--exclude-key-index`** — keep every column as key
   *except* the ones you list.

```bash
# All columns are the key (default)
ayame-diff csv --left old.tsv --right new.csv --out diff.tsv

# Named key columns (repeatable)
ayame-diff csv --left old.tsv --right new.csv --key customer_id --key event_date --out diff.tsv

# Key by column index (0-based by default; add --index-base 1 for 1-based)
ayame-diff csv --left old.tsv --right new.tsv --key-index 0 --key-index 3 --out diff.tsv

# Keep every column as key except updated_at and checksum
ayame-diff csv --left old.tsv --right new.tsv \
  --exclude-key updated_at --exclude-key checksum --out diff.tsv
```

Excluded columns still appear in the full row comparison and in the output: when
two rows share the remaining key but differ only in an excluded column, they are
emitted as a `CHANGED` pair.

Use `--ignore-column` / `--ignore-column-index` when a column should be excluded
from value comparison as well. `--tolerance FLOAT` compares numeric value
columns by absolute difference (with an explicit key); repeat
`--column-tolerance NAME=FLOAT` or `--column-tolerance-index N=FLOAT` for
per-column tolerances. Case, whitespace, and regex normalization are available
through `--ignore-case`, `--ignore-whitespace`, and repeatable `--filter-line`.

### Output

Output is TSV by default. Two leading columns, `_diff` and `_side`, are prepended,
followed by the original columns in the left input's column order.

```text
_diff       _side   id  name    amount
LEFT_ONLY   left    10  Alice   100
RIGHT_ONLY  right   20  Bob     200
CHANGED     left    30  Carol   300
CHANGED     right   30  Carol   350
```

| `_diff` | `_side` | Meaning |
|---|---|---|
| `LEFT_ONLY` | `left` | The key exists only on the left. |
| `RIGHT_ONLY` | `right` | The key exists only on the right. |
| `CHANGED` | `left` | Both sides share the key, but this left row could not be cancelled out. |
| `CHANGED` | `right` | Both sides share the key, but this right row could not be cancelled out. |

Identical rows for the same key cancel one for one. Output order follows the
hash-partition and key order — not the input order — so treat the result as a
difference *set*.

Add `--cell-diff` to insert `_changed_cols` after `_side` on `CHANGED` pairs.
The comma-separated header names use the same ignore and numeric-tolerance
rules as row matching. The stderr summary and `--summary-json` rank changed
columns by pair count. The default TSV schema is unchanged when the flag is
absent.

`--json` (alias for `--output-format jsonl --cell-diff`) writes one structured
JSON object per logical difference to `--out`. A paired change contains the
full `old` / `new` rows and typed `changed_columns` entries with index, name,
old value, and new value. JSON Lines remains streamable for huge results.

```bash
ayame-diff csv --left old.csv --right new.csv --key id \
  --cell-diff --out diff.tsv
ayame-diff csv --left old.csv --right new.csv --key id \
  --json --out diff.jsonl
```

!!! note "gzip output"
    A `.gz` output extension turns on gzip automatically, e.g.
    `--out diff.tsv.gz`.

### Selected `csv` options

```text
--left PATH  --right PATH  --out PATH
--key NAME                 (repeatable)
--key-index N              (repeatable)
--exclude-key NAME         (repeatable)
--exclude-key-index N      (repeatable)
--ignore-column NAME       (repeatable)
--ignore-column-index N    (repeatable)
--ignore-case
--ignore-whitespace none|change|all
--filter-line REGEX        (repeatable)
--tolerance FLOAT
--column-tolerance NAME=FLOAT       (repeatable)
--column-tolerance-index N=FLOAT    (repeatable)
--cell-diff
--json
--output-format tsv|jsonl
--index-base 0|1
--header=true|false
--align-columns-by-name=true|false
--left-format auto|csv|tsv     --right-format auto|csv|tsv
--left-parser auto|simple|rfc4180   --right-parser auto|simple|rfc4180
--partitions N   --parse-workers N   --workers N
--memory SIZE    --merge-fan-in N    --temp-dir PATH
--diff-exit-code
```

Run `ayame-diff csv --help` for the complete list, including tuning knobs for
very large inputs (`--memory`, `--partitions`, `--parse-workers`, `--workers`,
`--merge-fan-in`, `--temp-dir`).

Save or replay the full configuration with `--save-project FILE` and
`--project FILE`; see [Comparison projects](projects.md) for the versioned JSON,
relative paths, GUI history, and cron/CI usage.

---

## `text` — line-oriented text diff { #text }

Compare two text files (plain or `.gz`) in row order. A bounded resync window
keeps it linear and memory-bounded even on huge inputs. The differences are
reported as **Insert**, **Delete** and **Replace** hunks.

```bash
ayame-diff text old.txt new.txt                 # unified (default)
ayame-diff text clip: saved.txt                 # OS clipboard vs file
ayame-diff text --side-by-side old.txt new.txt  # two-column (old | new)
ayame-diff text --json old.txt new.txt          # machine-readable JSON
ayame-diff text --summary old.txt new.txt       # one-line summary only
ayame-diff text --format unified -U 3 old.txt new.txt > change.patch
ayame-diff text --format context -C 3 old.txt new.txt > change.patch
ayame-diff text --format normal old.txt new.txt > change.patch
ayame-diff text --detect-moves --move-min-lines 2 old.txt new.txt
ayame-diff text --window 32 --sync 100:120 --sync 5000:5100 old.txt new.txt
```

Use `clip:` (or `clipboard:`) as either input to compare directly against the
OS clipboard. The CLI invokes `pbpaste` on macOS, PowerShell on Windows, and
`wl-paste` or `xclip` on Linux without adding a runtime library dependency.
Clipboard content can also pass through `--pre` like file and stdin input.

### Output formats

| Flag | Output |
|---|---|
| *(none)* | Unified hunks (default). |
| `--side-by-side` (alias `--side`) | Two-column old / new layout; set the total column width with `--width`. |
| `--json` | Structured JSON with hunk kinds, line numbers and counts. |
| `--summary` | A single summary line on stderr. |
| `--format unified` / `-U N` | Applyable unified patch with N context lines (default 3). |
| `--format context` / `-C N` | Applyable context patch with N context lines (default 3). |
| `--format normal` / `--normal` | Classic `NcN`, `NaN`, `NdN` normal patch. |

### `text` flags

```text
--json                       emit the diff as JSON
--side-by-side, --side       two-column (old | new) output
--summary                    print only the one-line summary
--format FORMAT              patch format: normal, context, or unified
--normal                     alias for --format normal
-U N                         unified patch with N context lines
-C N                         context patch with N context lines
--context-lines N            context for --format context/unified (default 3)
--word                       highlight changed words in replace hunks (unified)
--encoding VALUE             auto (default), utf-8, utf-16le, utf-16be, shift_jis, euc-jp, iso-2022-jp
--ignore-case                ignore case when comparing lines
--ignore-whitespace MODE     none (default), change (collapse runs), all (remove)
--ignore-all-space           alias for --ignore-whitespace all
--ignore-space-change        alias for --ignore-whitespace change
--ignore-eol                 ignore CRLF/LF differences
--ignore-trailing-eol        ignore only the final missing line ending
--filter-line REGEX          remove regex matches for comparison (repeatable)
--detect-moves               pair exact delete/insert blocks as moves (default off)
--move-min-lines N           minimum moved-block length (default 2)
--move-max-candidates N      per-side detection guard (default 10000)
--sync OLD:NEW               force corresponding 1-based lines (repeatable)
--max-hunks N                maximum hunks to print; the rest are still counted (default 200)
--max-lines N                maximum lines shown per hunk side (default 200)
--window N                   resync look-ahead window when lines differ (default 128)
--width N                    total width for --side-by-side (default 160)
```

Patch output is never truncated by `--max-hunks` or `--max-lines`. It preserves
LF/CRLF and missing-final-newline markers, rejects decoded binary/NUL input, and
uses locale-independent file-header timestamps. CI applies all three formats
with GNU `patch` and additionally verifies unified output with `git apply`.

See [Encoding](encoding.md) for `--encoding` and
[Comparison options](comparison-options.md) for ignore filters, numeric
tolerance, `--word`, `--window` and `--max-hunks`.

### Input limits

A file argument is streamed through a sliding window, so its size does not
drive memory. Two shapes are bounded explicitly because they cannot stream:

| Limit | Default | Applies to |
| --- | --- | --- |
| `--max-line-bytes` | 64MiB | one logical line; `0` disables the check |
| (built in) | 1GiB | `-` (stdin) and `--pre` output |

A file with no line breaks — minified JSON, a database dump — is a single line,
so the window cannot bound it. Such a file is refused at open time with a
pointer to `--max-line-bytes` or to `bin` mode. Peak memory while accumulating
a long line is a small multiple of the limit, not exactly it.

Standard input and a `--pre` command are pipes: their length is unknown in
advance and their content must be materialized. Past 1GiB they are refused with
a suggestion to write the data to a file, which streams instead.

---

## `sorted` — sort, then diff { #sorted }

For files that hold the same rows in a different order, `sorted` sorts both
inputs line-wise and then runs the same line diff as `text`. It accepts every
display flag from `text` plus sort controls. Patch formats are rejected because
a patch of the sorted view cannot be applied safely to the original file.

```bash
ayame-diff sorted old.txt new.txt
ayame-diff sorted --numeric metrics-a.txt metrics-b.txt
ayame-diff sorted --reverse a.txt b.txt
```

### Extra `sorted` flags

```text
--numeric, -n    sort by leading numeric value
--reverse, -r    reverse the sort order
```

```
--sort-memory SIZE  line data held in memory before spilling (default 256MiB)
--temp-dir DIR      parent directory for spill files (default: TMPDIR)
```

!!! note
    `sorted` is memory-bounded. Input within `--sort-memory` is sorted in
    memory; anything larger spills sorted runs to `--temp-dir` and merges them,
    so files larger than RAM still compare. Comparing two ~370 MB files with
    `--sort-memory 64MiB` peaks at about 114 MiB resident instead of ~1.2 GiB.

!!! warning
    On many Linux systems `TMPDIR` is `/tmp`, which is RAM-backed (tmpfs).
    Spilling there consumes memory rather than saving it, and a large sort can
    fail with "no space left on device" while the disk is empty. Point
    `--temp-dir` (or `TMPDIR`) at a real filesystem when sorting very large
    files.

---

## `dir` — recursive folder / archive comparison { #dir }

`dir OLD NEW` pairs slash-normalized relative paths. Size is checked first;
equal-size candidates are streamed in parallel and compared byte-for-byte.
`--quick` may trust equal size and mtime. Plain `.gz` files compare their
decompressed content, while zip/tar/tar.gz archives compare as folder sources.
Archive expansion is bounded to 64 MiB per selected entry and 256 MiB total per
archive by default. Adjust these safeguards with `--max-archive-entry-bytes` and
`--max-archive-bytes`; oversized or decompression-bomb inputs fail explicitly
instead of exhausting process memory.

A comparison also holds one entry per file — including unchanged ones — so the
file count is bounded by `--max-entries` (default 2,000,000; a negative value
disables the check). Trees past the limit are refused up front, before any file
is read, with a suggestion to narrow the comparison using `--include` /
`--exclude` or to compare a subdirectory.

```bash
ayame-diff dir --include '*.csv' --exclude 'tmp/**' --workers 8 old/ new/
ayame-diff dir --tsv --all old/ new/ > folders.tsv
ayame-diff dir --json --diff-exit-code snapshot-a/ snapshot-b/
ayame-diff dir --html folder-report.html old/ new/
ayame-diff dir --csv folder-summary.csv --all old/ new/
ayame-diff dir --compare-by hash old/ new/
ayame-diff dir --filter "size > 1MiB and name =~ '\\.log$'" old/ new/
ayame-diff dir --filter-set development old/ new/
ayame-diff dir --filter-file filters.json --filter-set audit old/ new/
```

Dotfiles/directories are skipped unless `--hidden` is set. Symbolic links are
always skipped, avoiding loops and ambiguous out-of-tree reads. TSV/JSON include
status, relative path, sizes, and mtimes. In the GUI, choose **folder**, filter
the status tree, and click a changed regular file to drill into text diff.

`--html FILE` writes a self-contained light/dark tree report with status counts,
paths, sizes, and modification times. `--csv FILE` writes the same entry fields
as RFC 4180 CSV for downstream processing. Both write atomically; unchanged
entries are omitted unless `--all` is set. These file outputs are mutually
exclusive with `--json` and `--tsv`.

### Comparison methods

`--compare-by` accepts five explicit methods:

| Method | Behavior |
|---|---|
| `contents` | Stream and compare bytes, short-circuiting on the first difference (default). |
| `quick` | Trust equal size + mtime; otherwise fall back to content comparison. `--quick` is an alias. |
| `hash` | Stream both files through SHA-256 and compare digests. |
| `date` | Compare modification times only. |
| `size` | Compare file sizes only. |

Plain `.gz` content is decompressed for `contents` and `hash`. Metadata-only
methods use the source/archive metadata as presented.

### Filter expressions and reusable sets

`--filter` supports parentheses plus case-insensitive `and`, `or`, and `not`.
Fields are `size`, `name`, `path`, `ext`, and `mtime`. Size/mtime fields accept
`< <= == != >= >`; string fields accept `== != =~ !~`. Sizes accept decimal or
binary units, and mtimes accept RFC 3339 or `YYYY-MM-DD`.

```text
size > 1MiB and (name =~ '\.log$' or ext == '.json') and not path =~ '^vendor/'
```

Bundled sets are `development`, `vcs`, `node`, and `rust`; list them with
`--list-filter-sets`. External JSON files provide named sets:

```json
{
  "version": 1,
  "default": "audit",
  "filters": {
    "audit": {
      "includes": ["**/*.log", "**/*.json"],
      "excludes": ["archive/**"],
      "expression": "size >= 1KiB"
    }
  }
}
```

`--filter-set` is repeatable. Selected sets are combined with direct
`--include`, `--exclude`, and `--filter` arguments. A directory-mode
`.ayamediff.json` can be passed directly as `--filter-file`; when it contains
OLD/NEW paths, those paths may be omitted on the command line.

---

## `bin` — byte-level binary comparison { #bin }

`bin OLD NEW` streams two files and reports each differing region by byte
offset, followed by the old and new bytes in hexadecimal. It is memory-bounded
for large inputs and does not attempt to decode images or other file formats.

```bash
ayame-diff bin firmware-v1.bin firmware-v2.bin
ayame-diff bin --max-regions 20 --max-bytes 64 old.dat new.dat
```

`--max-regions` limits the number of regions printed (default 256), while
`--max-bytes` limits the retained bytes shown on each side of a region (default
32). The summary still reports the total differing byte count and whether the
region list was truncated.

---

## `update` — update a standalone installation { #update }

`update` checks the latest GitHub release, downloads the archive for the current
OS and architecture, verifies it against the release's `SHA256SUMS`, and
atomically replaces the running executable. The executable's directory must be
writable.

```bash
ayame-diff update --check   # report whether a newer release exists
ayame-diff update           # verify and install the latest release
```

For Homebrew, Scoop, Nix, or another managed installation, prefer that package
manager's update command so its package database remains authoritative.

---

## `remove` — uninstall a standalone installation { #remove }

`remove` asks for confirmation and then removes the running standalone binary.
Use `--yes` for a non-interactive invocation. Installs detected under Homebrew,
Scoop, or Nix are left untouched and must be removed with their package manager.

```bash
ayame-diff shell-uninstall  # optional: remove file-manager entries first
ayame-diff remove
ayame-diff remove --yes
```

On Windows, the running executable is renamed with a `.delete-me` suffix; delete
that file after the process exits to finish removal.

---

## `shell-select` — Windows Explorer selection helper { #shell-select }

`shell-select PATH` is the internal bridge installed by `shell-install` for the
Windows Explorer **Compare with Ayame Diff** action. The first invocation stores
one path in the current user's configuration for up to 30 minutes. A second,
different path clears that state and opens both paths in the GUI.

```text
ayame-diff shell-select PATH
```

Users normally invoke the Explorer action instead of running this command
directly. See [File-manager and quick launch](shell-integration.md) for setup
and the cross-platform launch forms.

---

## Exit codes { #exit-codes }

Normally:

- `0` — success
- `2` — usage error: bad flags, arguments, or incompatible options
- `3` — runtime error: I/O, comparison, server, or update failure
- `130` — interrupted or explicitly cancelled (for example, declining `remove`)

With `--diff-exit-code` (`csv` and `dir`):

- `0` — no differences
- `1` — differences found
- `2` / `3` — usage or runtime error, as above

A usage error and a runtime failure are deliberately distinct, so a script can
tell "you called it wrong" from "it could not finish". An internal crash is
reported as `3` with a stack trace on stderr; it never exits `2` and so is never
mistaken for a usage error.

---

## Scope boundaries

`ayame-diff` is intentionally focused on large structured/text data. Image
rendering and web-page screenshot comparison are out of scope: those workflows
need image decoders or a browser engine and are better served by WinMerge or a
dedicated visual-regression tool.

Images and other non-text files can still participate in `dir` comparisons as
binary content. Use `ayame-diff bin LEFT RIGHT` to inspect differing byte
offsets; there is no pixel-level image viewer or DOM/rendered-page comparison.
