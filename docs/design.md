<!-- i18n: language-switcher -->
[English](design.md) | [日本語](design.ja.md)

# Ayame family design

`ayame-diff` and [ayame-editor](https://github.com/hjosugi/ayame-editor)
share a visual family while keeping layouts specific to each product. The
editor is a writing surface; diff is a comparison setup and result viewer, so
their screen structures are intentionally not coupled.

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
