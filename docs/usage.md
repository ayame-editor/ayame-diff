# Usage

`ayame-diff` has three comparison subcommands plus two that launch the web GUI.

```text
ayame-diff csv    [flags] --left A --right B --out D   # CSV/TSV key comparison (default)
ayame-diff text   [flags] OLD NEW                      # line-oriented text diff
ayame-diff sorted [flags] OLD NEW                      # sort both sides, then diff
ayame-diff serve  [--addr host:port]                   # local web GUI
ayame-diff gui    [--addr host:port] [--no-open]       # serve on a free port and open the browser
```

Invoking `ayame-diff` with `--left ... --right ...` and no subcommand runs as
`csv` for backward compatibility. The `serve` and `gui` subcommands are covered
in [GUI](gui.md).

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

### Output

Output is always TSV. Two leading columns, `_diff` and `_side`, are prepended,
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

---

## `text` — line-oriented text diff

Compare two text files (plain or `.gz`) in row order. A bounded resync window
keeps it linear and memory-bounded even on huge inputs. The differences are
reported as **Insert**, **Delete** and **Replace** hunks.

```bash
ayame-diff text old.txt new.txt                 # unified (default)
ayame-diff text --side-by-side old.txt new.txt  # two-column (old | new)
ayame-diff text --json old.txt new.txt          # machine-readable JSON
ayame-diff text --summary old.txt new.txt       # one-line summary only
```

### Output formats

| Flag | Output |
|---|---|
| *(none)* | Unified hunks (default). |
| `--side-by-side` (alias `--side`) | Two-column old / new layout; set the total column width with `--width`. |
| `--json` | Structured JSON with hunk kinds, line numbers and counts. |
| `--summary` | A single summary line on stderr. |

### `text` flags

```text
--json                       emit the diff as JSON
--side-by-side, --side       two-column (old | new) output
--summary                    print only the one-line summary
--word                       highlight changed words in replace hunks (unified)
--encoding VALUE             auto (default), utf-8, utf-16le, utf-16be, shift_jis, euc-jp, iso-2022-jp
--ignore-case                ignore case when comparing lines
--ignore-whitespace MODE     none (default), change (collapse runs), all (remove)
--max-hunks N                maximum hunks to print; the rest are still counted (default 200)
--max-lines N                maximum lines shown per hunk side (default 200)
--window N                   resync look-ahead window when lines differ (default 128)
--width N                    total width for --side-by-side (default 160)
```

See [Encoding](encoding.md) for `--encoding` and
[Comparison options](comparison-options.md) for `--ignore-case`,
`--ignore-whitespace`, `--word`, `--window` and `--max-hunks`.

---

## `sorted` — sort, then diff

For files that hold the same rows in a different order, `sorted` sorts both
inputs line-wise and then runs the same line diff as `text`. It accepts every
`text` flag plus sort controls.

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

## Exit codes

Normally:

- `0` — success
- `2` — input, configuration or I/O error

With `--diff-exit-code` (`csv`):

- `0` — no differences
- `1` — differences found
- `2` — error
