<!-- i18n: language-switcher -->
[English](design.md) | [日本語](design.ja.md)

# Ayame family design

`ayame-diff` and [ayame-editor](https://github.com/ayame-editor/ayame-editor)
share a visual family while keeping layouts specific to each product. The
editor is a writing surface; diff is a comparison setup and result viewer, so
their screen structures are intentionally not coupled.

## Why the GUI uses a native local server

The GUI is a browser presentation layer over the same native Go executable as
the CLI. `gui` starts an authenticated loopback server, while `serve` exposes
the same UI on an explicitly selected address. Inputs are processed by the
local process; they are not uploaded to an ayame-diff service.

This boundary is intentional:

| Capability | Native local process | Browser-only deployment boundary |
|---|---|---|
| Path access | Opens user-supplied operating-system paths directly. | File access normally starts from a user-selected or previously granted handle and varies by browser. |
| External edits | Watches file-backed two-way and three-way inputs and refreshes after another editor saves them. | There is no portable browser API for watching arbitrary changes made outside the page. |
| Very large inputs | Partitions CSV/TSV data and spills external-sort runs to `--temp-dir` under an explicit memory budget. | Processing is bounded by browser memory and the quotas and persistence rules of browser-managed storage. |
| Desktop workflows | One binary serves the CLI, file-manager registration, scripts, and custom Git tool commands. | Launch and filesystem integration depend on browser- and platform-specific hand-offs. |
| Result parity | CLI commands and GUI handlers call the same internal comparison and merge packages. | A separately deployed web implementation can drift unless it deliberately shares or tests its engine. |

“Browser-only” is a deployment boundary, not a claim that WebAssembly can
never implement these features. Browser APIs such as user-granted file handles
and origin-private storage can cover parts of the table. Ayame-diff chooses the
native boundary because direct paths, portable external-change detection,
disk-backed processing, and operating-system integration are baseline product
requirements rather than optional browser capabilities.

The local privilege also defines the security boundary. Loopback mode uses a
per-run API token and pins the `Host` header. Non-loopback listeners require
the explicit `--allow-remote` option because anyone holding such a URL may be
able to read or write paths available to the process. See the
[GUI security notes](gui.md) and [file-manager and Git integration](shell-integration.md).

## Shared tokens

The embedded GUI's canonical family values live in
`internal/server/web/tokens.css`. They are reviewed copies of the editor's
`crates/ayame-cli/web/style.css` tokens:

- iris neutrals and purple accent;
- the UI and monospace font stacks;
- 10-pixel panel radius, borders and elevation;
- light and dark palettes;
- semantic added, deleted, changed and moved colors.

Controls use the same token vocabulary, while diff-specific colors remain
semantic and retain a color-blind-safe option. Japanese and English labels use
short, direct product language in both applications.

## Sister icons

Both products use the same iris silhouette. `ayame-diff` adds paired green and
red veins to communicate comparison without changing the recognizable family
mark. The SVG source is `internal/server/web/favicon.svg`; packaging raster and
platform icon files are generated from a deterministic code implementation.

## Sync policy

Token synchronization is deliberate rather than automatic. When either
project changes its shared palette, typography, radius, or core control style,
a reviewer checks the sister project and records any intentional divergence in
this document. Product layouts and diff/editor-only semantic colors do not
need to match. This avoids a build-time dependency between independently
released repositories.
