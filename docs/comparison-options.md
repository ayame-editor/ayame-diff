# Comparison options

The `text` and `sorted` subcommands share a set of WinMerge-style comparison
options. Some of them change how lines are *matched* (normalization); others
control how much output is produced and how the diff engine re-synchronizes
after a difference. Normalization only affects comparison — the output always
shows the original lines.

The same options are exposed through the [GUI](gui.md) `/api/diff` API.

## Normalization (how lines are matched)

### `--ignore-case`

Compare lines case-insensitively. Lines that differ only in letter case are
treated as equal.

```bash
ayame-diff text --ignore-case a.txt b.txt
```

### `--ignore-whitespace`

Control how whitespace is treated when matching lines.

```text
--ignore-whitespace none | change | all
```

| Value | Behaviour |
|---|---|
| `none` | Whitespace is significant (default). |
| `change` | Collapse each run of whitespace to a single space and trim the ends. |
| `all` | Remove all whitespace before comparing. |

```bash
# Treat runs of spaces/tabs as one space, ignore leading/trailing whitespace
ayame-diff text --ignore-whitespace change a.txt b.txt

# Ignore whitespace entirely
ayame-diff text --ignore-whitespace all a.txt b.txt
```

!!! note
    `change` and `all` normalize only for the comparison. The printed lines are
    the untouched originals.

## Word-level highlighting

### `--word`

In the unified (default) output, `--word` highlights just the words that changed
inside a Replace hunk, instead of marking the whole line. Deletions are wrapped
in `[-...-]` and insertions in `{+...+}`.

```bash
ayame-diff text --word old.txt new.txt
```

`--word` applies to the unified format. The [GUI](gui.md) shows equivalent
word-level highlighting in its side-by-side grid.

## Resync and output limits

For huge files, the diff engine looks ahead only a bounded distance to
re-synchronize after lines diverge, and it caps how much it prints. These knobs
tune that behaviour.

### `--window`

The resync look-ahead window used when lines differ (default `128`). A larger
window can re-align after bigger blocks of inserted/deleted lines, at the cost
of more work per mismatch.

```bash
ayame-diff text --window 512 old.txt new.txt
```

### `--max-hunks`

The maximum number of hunks to print (default `200`). Hunks beyond the limit are
still counted — the summary and JSON report the total — but are not printed.

```bash
ayame-diff text --max-hunks 50 old.txt new.txt
```

### `--max-lines`

The maximum number of lines shown per hunk side (default `200`). Longer hunks
are truncated in the output.

```bash
ayame-diff text --max-lines 40 old.txt new.txt
```

### `--width`

Total column width for `--side-by-side` output (default `160`).

```bash
ayame-diff text --side-by-side --width 200 old.txt new.txt
```

## Combining options

The options compose freely:

```bash
ayame-diff text \
  --ignore-case \
  --ignore-whitespace change \
  --word \
  --window 256 \
  --max-hunks 100 \
  old.txt new.txt
```

All of these also work with `sorted`, alongside its `--numeric` / `-n` and
`--reverse` / `-r` sort controls:

```bash
ayame-diff sorted --numeric --ignore-whitespace all a.txt b.txt
```
