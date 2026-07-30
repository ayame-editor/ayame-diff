<!-- i18n: language-switcher -->
[English](projects.md) | [日本語](projects.ja.md)

# Comparison projects

A `.ayamediff.json` project stores one repeatable CSV/TSV comparison. GUI and
CLI use the same versioned JSON, and relative paths are resolved from the
project file's directory. This makes projects suitable for committing beside a
repository's test data.

Folder projects use `"mode": "dir"` and a `directory` object. They preserve
LEFT/RIGHT paths, include/exclude globs, the filter expression, selected comparison
method, hidden-file policy, and worker count. Named external/bundled filter sets
are flattened into the saved project, so reopening does not depend on the
original filter file.

```json
{
  "version": 1,
  "mode": "dir",
  "directory": {
    "old": "../snapshots/old",
    "new": "../snapshots/new",
    "excludes": [".git/**", "node_modules/**", "target/**"],
    "filter": "size >= 1KiB and not name =~ '\\.tmp$'",
    "compare_by": "hash",
    "hidden": false,
    "workers": 8
  }
}
```

## Schema version 1

```json
{
  "version": 1,
  "mode": "csv",
  "csv": {
    "LeftPath": "../fixtures/old.csv",
    "RightPath": "../fixtures/new.csv",
    "OutputPath": "../reports/diff.tsv",
    "KeyNames": ["id"],
    "IgnoreColumnNames": ["updated_at"],
    "Tolerance": 0.0001,
    "ToleranceSet": true,
    "HasHeader": true,
    "AlignColumnsByName": true,
    "LeftFormat": "auto",
    "RightFormat": "auto",
    "LeftParser": "auto",
    "RightParser": "auto",
    "Partitions": 256,
    "ParseWorkers": 8,
    "Workers": 8,
    "MemoryText": "2GiB",
    "PartitionBufferText": "256KiB",
    "MergeFanIn": 32,
    "MaxRecordText": "256MiB",
    "OutputHeader": true
  },
  "report": {
    "cell_diff": true,
    "output_format": "tsv"
  }
}
```

`csv` is the serializable `engine.Config`: paths, keys, parser/resource
settings, declarative ignore rules, numeric tolerances, cell report settings,
and output settings are retained. Runtime writers/callbacks are deliberately
excluded. Unknown fields and versions other than `1` fail closed.

## CLI

```bash
# Save the effective flags, then perform the comparison
ayame-diff csv --left data/a.csv --right data/b.csv --key id \
  --cell-diff --out reports/diff.tsv --save-project jobs/daily.ayamediff.json

# Re-run the same project (paths resolve relative to jobs/)
ayame-diff csv --project jobs/daily.ayamediff.json --diff-exit-code
```

When loading, project comparison settings win; process behavior such as
`--diff-exit-code` and `--summary-json` remains controlled by the invocation.

## GUI and recent history

The CSV setup review includes project path, **Open project**, and **Save
project** controls. Saving requires an output path so the result can also run
headlessly. The browser keeps the ten most recent CSV configurations in local
storage; selecting one re-inspects headers before restoring key selections.

## Cron / CI example

```cron
15 2 * * * /usr/local/bin/ayame-diff csv --project /srv/audit/jobs/daily.ayamediff.json --diff-exit-code --summary-json /srv/audit/reports/summary.json
```

In CI, exit `0` means equal, `1` means differences were written, and `2` means
configuration or processing failed. Archive the configured TSV/JSONL output
and summary as job artifacts.
