<!-- i18n: language-switcher -->
[English](gui-setup-parity.md) | [日本語](gui-setup-parity.ja.md)

# GUI setup parity checklist

The retired terminal wizard is fully represented by the browser GUI. This
table records the migration explicitly so future changes do not silently drop
an interactive setting.

| Former wizard screen / setting | GUI control |
|---|---|
| Left and right input paths | OLD / NEW inputs and server-side file browser |
| Output path | Output path under Comparison and performance |
| Header present | Header row checkbox |
| Align headers by name / position | Align by name checkbox |
| Lightweight first-record inspection | Inspect headers; detected formats, parsers, reorder state |
| All-column key | Key mode: all columns |
| Include selected keys | Key mode: selected keys + searchable checkbox grid |
| Exclude selected keys | Key mode: exclude selected + searchable checkbox grid |
| Multi-select all / invert / bounds | Select all, Invert; server/engine validation |
| Left/right format | Left format / right format (`auto`, `csv`, `tsv`) |
| Left/right parser | Left parser / right parser (`auto`, `rfc4180`, `simple`) |
| Left/right delimiter override | Left delimiter / right delimiter |
| Lazy quotes | Lazy quotes checkbox |
| Trim leading space | Trim leading space checkbox |
| Memory limit | Memory |
| Temporary directory | Temp directory |
| Hash partitions | Partitions |
| Parallel input readers | Input readers |
| Comparison workers | Workers |
| Merge fan-in | Merge fan-in |
| Partition buffer | Partition buffer |
| Maximum record size | Max record size |
| Keep temporary files | Keep temporary files checkbox |
| Progress | In-page elapsed timer and Cancel control (no terminal progress stream) |
| Output header | Output header checkbox |
| Review before run | Review settings disclosure |
| Error recovery | Inline error status; settings remain populated and editable |
| Start / cancel | Compare / Run and export / Cancel |

The GUI additionally exposes case/whitespace/regex filters, ignored value
columns, global and per-column numeric tolerances, cell-level output, display
row limits, and TSV versus JSON Lines export.
