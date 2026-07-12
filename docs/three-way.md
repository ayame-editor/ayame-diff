<!-- i18n: language-switcher -->
[English](three-way.md) | [日本語](three-way.ja.md)

# Three-way comparison

Three-way mode compares two derived files against a common base and separates
changes that can be merged automatically from true conflicts.

## Text CLI

```bash
ayame-diff 3way text BASE LEFT RIGHT
ayame-diff 3way text --json BASE LEFT RIGHT
ayame-diff 3way text --format unified BASE LEFT RIGHT
ayame-diff 3way text --choice 2=right --output merged.txt BASE LEFT RIGHT
ayame-diff 3way text --allow-conflicts --output review.txt BASE LEFT RIGHT
```

The implementation runs the bounded-window BASE→LEFT and BASE→RIGHT line diffs
and clusters only overlapping base ranges. Events are classified as left-only,
right-only, the same change on both sides, or conflict. Independent and same
changes merge automatically. Unresolved text conflicts are rejected unless
`--allow-conflicts` is given, which writes standard LEFT/BASE/RIGHT markers.

## Keyed CSV / TSV CLI

```bash
ayame-diff 3way csv --base base.csv --left team-a.csv --right team-b.csv \
  --key id --json

ayame-diff 3way csv --base base.csv --left team-a.csv --right team-b.csv \
  --key id --choice 0123456789abcdef=left --output reconciled.csv
```

An explicit key (or exclude-key set) is required. The command runs two existing
partitioned/external-sort comparisons, then joins only changed key groups in
memory. Saving streams BASE and replaces affected key groups, so unchanged
rows are not materialized. Duplicate rows use multiset semantics. CSV conflicts
without a choice are rejected; after explicit `--allow-conflicts`, BASE rows
are retained because conflict markers are not valid structured records.

The reconciled output is UTF-8. `.csv` / `.csv.gz` outputs use commas and `.tsv`
/ `.tsv.gz` use tabs. Input gzip, Japanese encoding detection, quoting,
multiline cells, column alignment, lazy quotes, and trim-leading-space settings
follow the normal CSV engine.

## GUI

Choose **3-way text** or **3-way csv**, then select BASE, LEFT (OLD), and RIGHT
(NEW). Results use three panes and show a conflict count. Conflict cards offer
BASE / LEFT / RIGHT; all-conflict actions, undo/redo, and atomic save reuse the
two-way merge safety model. Difference navigation works across three-way events;
`Alt+Left` / `Alt+Right` chooses a side and `Alt+B` chooses BASE.

Inputs are never overwritten unless the overwrite option and destructive
confirmation are both supplied. New result paths are written via a temporary
sibling and rename.
