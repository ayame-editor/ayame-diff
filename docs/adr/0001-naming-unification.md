<!-- i18n: language-switcher -->
[English](0001-naming-unification.md) | [日本語](0001-naming-unification.ja.md)

# ADR 0001: Standardization of Project Name — `fcsv-diff` → `ayame-diff`

- Status: Accepted (2026-07-10)
- Related Issue: hjosugi/ayame-diff#3
- Related Epic: hjosugi/ayame-diff#26

## Background

The repository name is `ayame-diff`, but the actual implementation remains `fcsv-diff`
(module path `github.com/hjosugi/fcsv-diff`, binary name `fcsv-diff`, `cmd/fcsv-diff/`).
Since it is paired with the sister project ayame-editor, the inconsistency in naming hampers
interlinking, navigation, and distribution coherence.

Reference design guideline (Sindre Sorhus's consistent naming): **Align repository name, product name, and binary name**.
ayame-editor's repo/product/binary (`ayame`) are consistent, making the navigation clearer as a sister project.

## Decision

Unify the product name, module path, and binary name **completely to `ayame-diff`**.

| Item | Before | After |
| --- | --- | --- |
| Module path | `github.com/hjosugi/fcsv-diff` | `github.com/ayame-editor/ayame-diff` |
| Binary name | `fcsv-diff` | `ayame-diff` |
| Entry point | `cmd/fcsv-diff/` | `cmd/ayame-diff/` |
| `go install` path | `.../fcsv-diff/cmd/fcsv-diff@latest` | `.../ayame-diff/cmd/ayame-diff@latest` |

`fcsv` (＝ **f**ast **CSV**) remains only as an internal component name for the **CSV engine used for key comparison** (handled by `internal/engine`).
The product identifier `fcsv-diff` will be discontinued.

## Scope of impact (already implemented)

Replaced the string `fcsv-diff` in 116 places and renamed `cmd/`:

- `go.mod` (module path)
- `cmd/ayame-diff/` (`git mv`) and all import paths
- `Makefile` / `scripts/*` (build-all, build-windows, package-release, smoke-test)
- `.github/workflows/*` (build.yml / release.yml), `.github/ISSUE_TEMPLATE/*`
- `README.md` / `README_WINDOWS.md` / `VALIDATION.md` / `SECURITY.md` / `LICENSE`
- `packaging/windows/start-interactive.cmd`
- `.gitignore`, temporary directory prefixes (`.ayame-diff-output-*`, `ayame-diff-`), version strings, dialog wizard titles

Verification: All `go build ./...` / `go vet ./...` / `go test ./...` succeeded.

## Migration guidance from the old name

- The old binary name `fcsv-diff` will be announced as deprecated in the next release notes.
- `go install github.com/hjosugi/fcsv-diff/...` will no longer work (due to module path change).
  The new path is already listed in the README.
- Since the repository has been renamed to `ayame-diff` before, no additional GitHub redirect handling is necessary.

## Items unaffected

- CLI flags/options (`--left` / `--right` / `--key`, etc.) remain unchanged.
- Behavior and output of CSV comparison remain unchanged (pure identifier renaming).
