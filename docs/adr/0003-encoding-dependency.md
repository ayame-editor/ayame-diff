# ADR 0003: 文字コード対応と依存性の例外（golang.org/x/text）

- ステータス: Accepted（2026-07-11）
- 関連 Issue: hjosugi/ayame-diff#9
- 参照: ADR 0002（依存ゼロ方針と例外基準）

## 背景

日本語ファイルを扱うため、非 UTF-8 の文字コード（Shift_JIS / EUC-JP /
ISO-2022-JP / UTF-16）の検出とデコードが必要（WinMerge の codepage 対応に相当）。
Shift_JIS / EUC-JP / ISO-2022-JP は大きな変換表を要し、標準ライブラリには無い。
自前で正確に再実装するのは非現実的でバグの温床になる。

## 決定

**`golang.org/x/text` を唯一の外部依存として許可する。** ADR 0002 の例外基準
（「標準ライブラリに存在せず自前実装が現実的でない領域に限る」）に合致する。

- `internal/encoding` パッケージにのみ依存を閉じ込める（他パッケージは
  `internal/encoding` 経由で利用）。
- バージョンは **v0.21.0** に固定。これは `go 1.23` 互換を維持するため（最新の
  x/text は go 1.25+ を要求し、モジュールの最低 Go バージョンを引き上げてしまう）。
  Japanese/Unicode コーデックは長期安定のため旧バージョンで問題ない。
- `THIRD_PARTY_NOTICES.md` に BSD-3-Clause を記載。
- CSV コア（`internal/engine`）・diff コア（`linediff` 等）は引き続き依存ゼロ。

## 実装

- `internal/encoding`: `Detect(sample, hint)`（BOM → 明示 → UTF-8 妥当性 →
  Shift_JIS/EUC-JP ヒューリスティック）と `Decoder(r, name)`（ストリーミング
  デコードで UTF-8 化）。
- `internal/linesrc.OpenEncoding(path, hint)`: 先頭 8KiB のサンプルで検出し、
  `transform.NewReader` で復号しつつ既存のメモリ有界な行読み取りに載せる
  （巨大ファイルでもメモリ有界を維持）。
- CLI: `text` / `sorted` に `--encoding`（既定 `auto`）。
- GUI: エンコーディング選択ドロップダウン + `/api/diff` の `encoding`。

対応: `auto` / `utf-8` / `utf-16le` / `utf-16be` / `shift_jis` / `euc-jp` /
`iso-2022-jp`。UTF-8 BOM は除去、UTF-16 BOM は復号器が消費。

## 却下した案

- **依存ゼロ死守（SJIS/EUC-JP 非対応）**: 「日本語も完全対応」という要件を
  満たせないため却下。
- **変換表の自前実装**: 保守コスト大・バグリスク大で却下。
- **最新 x/text**: `go 1.25+` を要求しモジュールの最低 Go を上げるため v0.21.0 に固定。
