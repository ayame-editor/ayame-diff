<!-- i18n: language-switcher -->
[English](encoding.md) | [日本語](encoding.ja.md)

# Encoding

The `text` and `sorted` subcommands read files in a range of encodings and
decode them to Unicode before diffing. By default the encoding is detected
automatically; you can override the guess with `--encoding` when detection is
wrong.

The only external dependency involved is `golang.org/x/text`, which provides the
Japanese and UTF-16 decoders.

## Auto-detection

With `--encoding auto` (the default), detection runs in two stages:

1. **BOM first.** A UTF-8, UTF-16LE or UTF-16BE byte-order mark is honoured
   immediately and stripped from the content.
2. **Heuristic.** Without a BOM, the bytes are checked for valid UTF-8 and for
   Japanese encodings (Shift_JIS, EUC-JP, ISO-2022-JP) using a byte-pattern
   heuristic.

Auto-detection covers the common cases, but heuristics can be fooled — a short
file, or one that is valid under more than one encoding, may be classified
wrongly. When that happens, name the encoding explicitly.

## The `--encoding` flag

```text
--encoding auto | utf-8 | utf-16le | utf-16be | shift_jis | euc-jp | iso-2022-jp
```

| Value | Encoding |
|---|---|
| `auto` | Detect from BOM, then heuristics (default). |
| `utf-8` | UTF-8. |
| `utf-16le` | UTF-16, little-endian. |
| `utf-16be` | UTF-16, big-endian. |
| `shift_jis` | Shift_JIS (Japanese). |
| `euc-jp` | EUC-JP (Japanese). |
| `iso-2022-jp` | ISO-2022-JP (Japanese). |

The same flag is available on both `text` and `sorted`, and mirrors the
`encoding` field of the [GUI](gui.md) `/api/diff` request.

## Examples

Force Shift_JIS when a file is misdetected:

```bash
ayame-diff text --encoding shift_jis a.txt b.txt
```

Compare EUC-JP logs:

```bash
ayame-diff text --encoding euc-jp old.log new.log
```

Diff UTF-16LE files (for example exported from a Windows tool):

```bash
ayame-diff text --encoding utf-16le old.txt new.txt
```

Sort and diff Shift_JIS files:

```bash
ayame-diff sorted --encoding shift_jis --numeric a.csv b.csv
```

!!! tip "Japanese files"
    Shift_JIS, EUC-JP, UTF-16 and ISO-2022-JP are all auto-detected (BOM first,
    then a heuristic). If a Japanese file is decoded as garbage, re-run with the
    matching `--encoding` value.

!!! note "Output is Unicode"
    Regardless of the input encoding, diff output is written as Unicode text.
    The `--encoding` flag controls how inputs are *decoded*, not how output is
    re-encoded.
