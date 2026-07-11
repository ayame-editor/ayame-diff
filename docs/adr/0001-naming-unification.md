<!-- i18n: language-switcher -->
[English](0001-naming-unification.en.md) | [日本語](0001-naming-unification.md)

# ADR 0001: プロジェクト名の統一 — `fcsv-diff` → `ayame-diff`

- ステータス: Accepted（2026-07-10）
- 関連 Issue: hjosugi/ayame-diff#3
- 関連 Epic: hjosugi/ayame-diff#26

## 背景

本リポジトリ名は `ayame-diff` だが、実体は `fcsv-diff` のままだった
（モジュールパス `github.com/hjosugi/fcsv-diff`、バイナリ名 `fcsv-diff`、
`cmd/fcsv-diff/`）。姉妹プロジェクト ayame-editor と対になるにあたり、
名称の不一致は相互リンク・導線・配布物の一貫性を損なう。

参考にした設計指針（Sindre Sorhus の一貫命名）: **リポジトリ名・製品名・
バイナリ名を一致させる**。ayame-editor は repo/製品/バイナリ（`ayame`）が
一貫しており、姉妹プロジェクトとして名称が揃っていると導線が明快になる。

## 決定

製品名・モジュールパス・バイナリ名を **`ayame-diff` に完全統一**する。

| 項目 | 変更前 | 変更後 |
| --- | --- | --- |
| モジュールパス | `github.com/hjosugi/fcsv-diff` | `github.com/hjosugi/ayame-diff` |
| バイナリ名 | `fcsv-diff` | `ayame-diff` |
| エントリポイント | `cmd/fcsv-diff/` | `cmd/ayame-diff/` |
| `go install` パス | `.../fcsv-diff/cmd/fcsv-diff@latest` | `.../ayame-diff/cmd/ayame-diff@latest` |

`fcsv`（＝ **f**ast **CSV**）は、キー比較を行う **内部の CSV エンジンの
コンポーネント名** としてのみ残す（`internal/engine` が担う領域の呼称）。
製品識別子としての `fcsv-diff` は廃止する。

## 影響範囲（実施済み）

`fcsv-diff` の文字列 116 箇所を一括置換し、`cmd/` を改名した：

- `go.mod`（module path）
- `cmd/ayame-diff/`（`git mv`）と全 import パス
- `Makefile` / `scripts/*`（build-all, build-windows, package-release, smoke-test）
- `.github/workflows/*`（build.yml / release.yml）、`.github/ISSUE_TEMPLATE/*`
- `README.md` / `README_WINDOWS.md` / `VALIDATION.md` / `SECURITY.md` / `LICENSE`
- `packaging/windows/start-interactive.cmd`
- `.gitignore`、一時ディレクトリ接頭辞（`.ayame-diff-output-*`, `ayame-diff-`）、
  バージョン文字列、対話ウィザードのタイトル

検証: `go build ./...` / `go vet ./...` / `go test ./...` すべて成功。

## 旧名からの移行案内

- 旧バイナリ名 `fcsv-diff` は次のリリースノートで deprecation を告知する。
- `go install github.com/hjosugi/fcsv-diff/...` は動作しなくなる（モジュール
  パス変更のため）。README に新パスを記載済み。
- リポジトリはリネーム前から `ayame-diff` のため、GitHub リダイレクトの
  追加対応は不要。

## 影響しないもの

- CLI フラグ・オプション（`--left` / `--right` / `--key` など）は不変。
- CSV 比較の挙動・出力は不変（純粋な識別子リネーム）。
