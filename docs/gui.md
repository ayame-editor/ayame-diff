<!-- i18n: language-switcher -->
[English](gui.md) | [日本語](gui.ja.md)

# GUI

## Screenshots

| Setup | Side-by-side diff |
|---|---|
| [![Comparison setup](assets/screenshot-setup.png)](assets/screenshot-setup.png) | [![Syntax-highlighted diff](assets/screenshot-diff.png)](assets/screenshot-diff.png) |

| Folder comparison | Three-way merge |
|---|---|
| [![Folder result tree](assets/screenshot-folder.png)](assets/screenshot-folder.png) | [![Three-way result](assets/screenshot-three-way.png)](assets/screenshot-three-way.png) |

`ayame-diff` bundles a small local web app so you can compare files in the
browser instead of the terminal. The single-page app is embedded in the binary,
so there is nothing extra to install and no network access to the outside world.

Two subcommands start it:

- `serve` — start the server on a fixed address (localhost by default).
- `gui` — start the server on a free localhost port and open your browser.

!!! warning "Local, single-user use only"
    The server opens the file paths you type in the browser, so it binds to
    localhost by default and is meant only for your own local use. Do not expose
    it to a network.

## `serve`

```text
ayame-diff serve [--addr host:port]
```

Starts the web UI and keeps running until you stop it with `Ctrl+C`.

```bash
ayame-diff serve                       # http://127.0.0.1:8080
ayame-diff serve --addr 127.0.0.1:9000
```

| Flag | Default | Meaning |
|---|---|---|
| `--addr` | `127.0.0.1:8080` | Listen address (`host:port`). |

## `gui`

```text
ayame-diff gui [--addr host:port] [--no-open]
```

Same UI as `serve`, but it picks a free localhost port and launches your default
browser — the "double-click to a GUI" experience without a native webview
dependency.

```bash
ayame-diff gui             # pick a free port and open the browser
ayame-diff gui --no-open   # start the server but don't open a browser
```

| Flag | Default | Meaning |
|---|---|---|
| `--addr` | `127.0.0.1:0` | Listen address; port `0` picks a free port. |
| `--no-open` | off | Start the server but do not open the browser. |

## Using the web UI

Enter the **OLD** and **NEW** file paths (or use the server-side file picker),
choose the mode (`text`, `sorted`, or `csv / tsv`)
and options (encoding, resync window, ignore-case, whitespace handling, EOL
controls, repeatable regex filters entered one per line, and for
`sorted` the numeric/reverse sort), then **Compare**. The result is shown as a
side-by-side grid with per-hunk headers, line numbers and word-level
highlighting. **Syntax highlight** adds line-local coloring for common source,
data, markup, and log formats; the file extension selects the language and the
toggle is remembered in the browser. It operates only on rendered diff rows, so
it does not scan or retain the complete input file. Choose **patch format**
(`normal`, `context`, or `unified`) and a
context-line count, then use **Export patch** to download an applyable
`ayame.patch`. Patch export preserves CRLF and missing-final-newline markers and
rejects binary/NUL input. Export is available in `text` mode only; a patch of a
sorted view would not apply safely to the original file.

Applied ignore settings are shown in the result summary. They affect matching
only: rendered lines and exported patches retain the original text.

### Navigating differences

The sticky navigation bar jumps to the first, previous, next, or last hunk and
shows the current/total and unread count. Keyboard shortcuts are `Alt+Down`,
`Alt+Up`, `Alt+End`, and `Alt+Home`; the `?` button shows this list in the UI.
The location bar on the right draws markers directly from hunk indexes, supports
click-to-jump, and overlays the current viewport. Left/right text stays vertically
and horizontally synchronized because each hunk is rendered as one shared grid
and scroll row rather than two independent panes.

Enable **detect moves** to pair exact deleted/inserted blocks. Moved hunks use a
dedicated purple color and an `↔` button jumps to the paired location. Detection
is off by default; **move min lines** and the engine candidate cap prevent the
optional post-processing pass from dominating huge comparisons.

### Manual alignment and ignored differences

When automatic resynchronization picks the wrong lines, click one line on each
side and choose **Add sync**. Sync points split the diff into independent
intervals, are listed as removable `OLD:NEW` chips, and trigger an immediate
recalculation. Points must increase on both sides; the API and CLI validate
bounds and ordering. The CLI equivalent is repeatable `--sync 100:120` using
1-based line numbers.

Each hunk also has **Ignore this difference**. Ignored hunks remain visible as
collapsed dashed headers, are excluded from next/previous navigation and unread
counts, and can be restored. Patch export omits them and records the count in
the `X-Ayame-Ignored-Hunks` response header, so the hidden decision remains
auditable; use declarative line filters (#28) for a permanent rule.

### CSV / TSV setup and table result

Choose `csv / tsv`, then **Inspect headers**. The server reads only the first
logical record and reports detected format/parser and aligned columns. The
searchable column list supports all-column, selected-key, and excluded-key
modes plus Select all / Invert. Parsing, delimiter, compatibility, ignore,
tolerance, resource, temporary-storage, and output settings are available in
the same screen; **Review settings** summarizes the effective run.

The result is paged in groups of 100 logical differences. Changed cells alone
use the modification color, header badges show per-column change counts, and
**changed columns only** hides wide unchanged columns. The server caps the
browser response at 5,000 logical differences; **Run and export** writes the
complete TSV (with `_changed_cols`) or JSON Lines result to a local path.

See the [former TUI parity checklist](gui-setup-parity.md) for the full mapping.

### Folder comparison

The `folder` mode accepts include/exclude globs, hidden-file policy, parallel
worker count, a five-way comparison method, named filter files/sets, and a
boolean metadata filter expression. **Preview filter** reports selected counts
and exposes a path sample before any content comparison. Folder settings can be
saved in a portable `.ayamediff.json` project. Results form an
indented, status-colored tree with status filters. Clicking a changed file
switches to text mode and opens the paired relative paths. Symbolic links are
skipped and `.gz` files compare decompressed content.

## HTTP API

The GUI is a thin client over a small JSON API. You can call it directly.

### `GET /`

Serves the embedded single-page app.

### `GET /api/health`

```json
{ "status": "ok", "version": "..." }
```

### `POST /api/diff`

Request body:

```json
{
  "old": "old.txt",
  "new": "new.txt",
  "mode": "text",
  "encoding": "auto",
  "window": 128,
  "maxHunks": 200,
  "maxLines": 200,
  "numeric": false,
  "reverse": false,
  "ignoreCase": false,
  "whitespace": "none"
}
```

| Field | Type | Notes |
|---|---|---|
| `old`, `new` | string | File paths to compare. |
| `mode` | string | `text` (default) or `sorted`. |
| `encoding` | string | Same values as [`--encoding`](encoding.md). |
| `window` | number | Resync window (default 128 when 0). |
| `maxHunks` | number | Max hunks returned (default 200 when ≤ 0). |
| `maxLines` | number | Max lines per hunk side (default 200 when 0). |
| `numeric`, `reverse` | bool | Sort controls, used when `mode` is `sorted`. |
| `ignoreCase` | bool | Ignore case when comparing. |
| `whitespace` | string | `none`, `change` or `all`. |
| `syncPoints` | array | 0-based `{ "old": N, "new": N }` forced correspondences. |
| `ignoredHunks` | array | Stored hunk indexes omitted from patch/report output. |

Success response:

```json
{
  "old_lines": 120,
  "new_lines": 118,
  "hunks": [
    {
      "kind": "Replace",
      "old_start": 10,
      "old_len": 2,
      "new_start": 10,
      "new_len": 1,
      "old": ["line a", "line b"],
      "new": ["line A"]
    }
  ],
  "hunk_count": 1,
  "omitted_hunks": 0,
  "added": 0,
  "deleted": 1,
  "modified": 1
}
```

Each hunk's `kind` is `Insert`, `Delete` or `Replace`, with `old`/`new` arrays
holding the affected lines (truncated to `maxLines` per side). The optional
`move_detection_skipped: true` field indicates that move detection was
requested but omitted hunks made a complete result impossible. Errors return
an HTTP 4xx status with a JSON body:

```json
{ "error": "..." }
```

### Example request

```bash
curl -s http://127.0.0.1:8080/api/diff \
  -H 'Content-Type: application/json' \
  -d '{"old":"old.txt","new":"new.txt","mode":"text","ignoreCase":true,"whitespace":"change"}'
```

### `POST /api/patch`

Accepts the same path, inline-text, mode, encoding and comparison fields as
`/api/diff`, plus:

```json
{
  "old": "old.txt",
  "new": "new.txt",
  "patchFormat": "unified",
  "context": 3
}
```

The response is `text/x-diff` with `Content-Disposition: attachment`. Valid
formats are `normal`, `context`, and `unified`; `context` is non-negative and
defaults to 3 when omitted.

### CSV and file APIs

- `GET /api/files?path=...` lists up to 2,000 local directory entries for the
  setup file picker.
- `POST /api/csv/inspect` accepts CSV setup JSON and returns first-record schema
  inspection without scanning data rows.
- `POST /api/csv/diff` runs the complete comparison and returns headers,
  summary/ranking, and at most `maxRows` logical JSON cell differences (`500`
  by default, hard cap `5,000`).
- `POST /api/csv/export` uses the same request plus `output`, `outputFormat`
  (`tsv` or `jsonl`), and `outputHeader`, and writes the complete local result.

These endpoints deliberately accept local paths and are subject to the same
localhost-only warning as the rest of the GUI.
