# Usage

`ayame-diff` provides comparison, web GUI, packaging, and maintenance commands.

```text
ayame-diff csv    [flags] --left A --right B --out D   # CSV/TSV key comparison (default)
ayame-diff text   [flags] OLD NEW                      # line-oriented text diff
ayame-diff sorted [flags] OLD NEW                      # sort both sides, then diff
ayame-diff dir    [flags] OLD NEW                      # directory/archive comparison
ayame-diff bin    [flags] OLD NEW                      # binary/hex comparison
ayame-diff 3way   [text|csv] [flags]                   # BASE / LEFT / RIGHT comparison
ayame-diff serve  [--addr host:port]                   # local web GUI
ayame-diff gui    [flags] [OLD [NEW]]                  # open the GUI, optionally prefilled
ayame-diff shell-install                               # file-manager integration
ayame-diff shell-uninstall                             # remove integration
```

Invoking `ayame-diff` with `--left ... --right ...` and no subcommand runs as
`csv` for backward compatibility. The `serve` and `gui` subcommands are covered
in [GUI](gui.md).

Two bare paths are a quick-launch form: files use `text`, directories use
`dir`, and adding `--gui` opens and immediately runs the browser GUI. See
[File-manager and quick launch](shell-integration.md).

---

## `csv` — CSV/TSV key comparison (default)

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

## `text` — line-oriented text diff

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

---

## `sorted` — sort, then diff

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

!!! note
    In v1, `sorted` sorts in memory; an external, memory-bounded line sort is
    tracked in the project's issue tracker.

---

## `dir` — recursive folder / archive comparison

`dir OLD NEW` pairs slash-normalized relative paths. Size is checked first;
equal-size candidates are streamed in parallel and compared byte-for-byte.
`--quick` may trust equal size and mtime. Plain `.gz` files compare their
decompressed content, while zip/tar/tar.gz archives compare as folder sources.
Archive expansion is bounded to 64 MiB per selected entry and 256 MiB total per
archive by default. Adjust these safeguards with `--max-archive-entry-bytes` and
`--max-archive-bytes`; oversized or decompression-bomb inputs fail explicitly
instead of exhausting process memory.

```bash
ayame-diff dir --include '*.csv' --exclude 'tmp/**' --workers 8 old/ new/
ayame-diff dir --tsv --all old/ new/ > folders.tsv
ayame-diff dir --json --diff-exit-code snapshot-a/ snapshot-b/
```

Dotfiles/directories are skipped unless `--hidden` is set. Symbolic links are
always skipped, avoiding loops and ambiguous out-of-tree reads. TSV/JSON include
status, relative path, sizes, and mtimes. In the GUI, choose **folder**, filter
the status tree, and click a changed regular file to drill into text diff.

---

## Exit codes

Normally:

- `0` — success
- `2` — input, configuration or I/O error
- `130` — interrupted or explicitly cancelled (for example, declining `remove`)

With `--diff-exit-code` (`csv` and `dir`):

- `0` — no differences
- `1` — differences found
- `2` — error

---

## Scope boundaries

`ayame-diff` is intentionally focused on large structured/text data. Image
rendering and web-page screenshot comparison are out of scope: those workflows
need image decoders or a browser engine and are better served by WinMerge or a
dedicated visual-regression tool.

Images and other non-text files can still participate in `dir` comparisons as
binary content. Use `ayame-diff bin LEFT RIGHT` to inspect differing byte
offsets; there is no pixel-level image viewer or DOM/rendered-page comparison.
