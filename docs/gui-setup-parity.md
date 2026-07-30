<!-- i18n: language-switcher -->
[English](gui-setup-parity.md) | [日本語](gui-setup-parity.ja.md)

# GUI setup reachability and placement policy

The retired terminal wizard's capabilities remain reachable from the browser
GUI. Reachability does not require a permanently visible control, however, and
this document must not be used to justify promoting every new setting onto the
comparison screen.

When a setting is added or moved, use the least prominent placement that still
makes the intended task clear:

- **Input rail** — information needed to identify the inputs and start the
  first comparison. After a successful comparison, editable pane headers take
  over the path controls.
- **Result toolbar** — frequent actions and display preferences used while
  reading a result. Infrequent choices belong in a one-level menu.
- **Settings dialog** — comparison semantics, format overrides, and export
  choices that must remain reachable but need not occupy result space.
- **Automatic** — bounded operational tuning that the application can derive
  safely. Normal users should not have to choose implementation parameters;
  diagnostics or a deliberate expert escape hatch may expose them when needed.

The table records both today's access route and the intended placement. A
transition note means reachability is preserved until the linked replacement is
implemented; it is not a requirement to retain that control permanently.

| Former wizard screen / setting | Current access route | Placement policy |
|---|---|---|
| Left and right input paths | Initial LEFT / RIGHT rail; editable sticky pane headers and server-side file browser after comparison | **Input rail** initially; **Result toolbar** pane headers after comparison |
| Output path | CSV **Comparison and performance** disclosure | **Settings dialog**, grouped with export choices |
| Header present | CSV setup: **Header row** | **Settings dialog** |
| Align headers by name / position | CSV setup: **Align by name** | **Settings dialog** |
| Lightweight first-record inspection | **Inspect headers**; comparison also inspects when needed | **Automatic** by default; report detected format in setup/status and retain an explicit retry action |
| All-column key | Key mode: **All columns** | **Settings dialog** |
| Include selected keys | Key mode: **Selected keys** and searchable column grid | **Settings dialog** |
| Exclude selected keys | Key mode: **Exclude selected** and searchable column grid | **Settings dialog** |
| Multi-select all / invert / bounds | **Select all**, **Invert**, and server/engine validation | **Settings dialog**; bounds remain **Automatic** validation |
| Left/right format | Left/right format (`auto`, `csv`, `tsv`) under **File format** | **Settings dialog**; `auto` remains the default |
| Left/right parser | Left/right parser (`auto`, `rfc4180`, `simple`) under **File format** | **Settings dialog**; `auto` remains the default |
| Left/right delimiter override | Left/right delimiter under **File format** | **Settings dialog** |
| Lazy quotes | **Lazy quotes** under **File format** | **Settings dialog** |
| Trim leading space | **Trim leading space** under **File format** | **Settings dialog** |
| Memory limit | **Memory** under **Comparison and performance** | **Automatic** target; current expert control remains during the #261 migration |
| Temporary directory | **Temp directory** under **Comparison and performance** | **Automatic** target; current expert control remains during the #261 migration |
| Hash partitions | **Partitions** under **Comparison and performance** | **Automatic** target; current expert control remains during the #261 migration |
| Parallel input readers | **Input readers** under **Comparison and performance** | **Automatic** target; current expert control remains during the #261 migration |
| Comparison workers | **Workers** under **Comparison and performance** | **Automatic** target; current expert control remains during the #261 migration |
| Merge fan-in | **Merge fan-in** under **Comparison and performance** | **Automatic** target; current expert control remains during the #261 migration |
| Partition buffer | **Partition buffer** under **Comparison and performance** | **Automatic** target; current expert control remains during the #261 migration |
| Maximum record size | **Max record size** under **Comparison and performance** | **Automatic** target with a safe hard limit; current expert control remains during the #261 migration |
| Keep temporary files | **Keep temporary files** under **Comparison and performance** | **Automatic** cleanup by default; a diagnostics-only override may remain in **Settings dialog** |
| Progress | In-page stage/elapsed status and **Cancel** | **Result toolbar** and adjacent status area |
| Output header | **Output header** under **Comparison and performance** | **Settings dialog**, grouped with export choices |
| Review before run | **Review settings** disclosure | **Settings dialog** summary; do not add another nesting level |
| Error recovery | Inline error status; settings remain populated and editable | Adjacent to the **Input rail** before a result and the **Result toolbar** afterward |
| Start / cancel | Initial **Compare**, persistent **Re-compare**, and **Cancel** | **Input rail** for the first run; **Result toolbar** afterward |

GUI-only capabilities follow the same policy:

| Capability | Placement policy |
|---|---|
| Case, whitespace, EOL, regular-expression filters, ignored columns, and numeric tolerances | **Settings dialog** because they change comparison meaning |
| Word highlighting, wrapping, syntax highlighting, whitespace display, theme, and colour scheme | **Result toolbar** View menu because they only change presentation |
| Auto-reload after external saves | **Result toolbar** View menu; the preference persists globally |
| Comparison URL copy and browser-history navigation | **Result toolbar**; copied URLs remove the API token and disclose that local paths remain |
| Display row limits, changed-column output, output format, and output headers | **Settings dialog**, grouped with result/export choices |

## Review rule

A future parity review asks whether each supported task still has a documented,
tested route from the GUI. It does **not** ask for one visible control per engine
option. Before adding a control:

1. prefer a safe automatic value for operational tuning;
2. put comparison meaning in the settings dialog and presentation in the
   result toolbar;
3. keep the input rail limited to what is required to begin; and
4. avoid more than one disclosure layer inside a dialog or menu.

Removing or automating a control still requires tests, documentation, and a
migration path when saved projects contain the old field. Silent loss of a
capability remains a regression; reducing permanent chrome does not.
