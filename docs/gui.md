# GUI

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

Enter the **OLD** and **NEW** file paths, choose the mode (`text` or `sorted`)
and options (encoding, resync window, ignore-case, whitespace handling, and for
`sorted` the numeric/reverse sort), then **Compare**. The result is shown as a
side-by-side grid with per-hunk headers, line numbers and word-level
highlighting. Choose **patch format** (`normal`, `context`, or `unified`) and a
context-line count, then use **Export patch** to download an applyable
`ayame.patch`. Patch export preserves CRLF and missing-final-newline markers and
rejects binary/NUL input. Export is available in `text` mode only; a patch of a
sorted view would not apply safely to the original file.

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
holding the affected lines (truncated to `maxLines` per side). Errors return an
HTTP 4xx status with a JSON body:

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
