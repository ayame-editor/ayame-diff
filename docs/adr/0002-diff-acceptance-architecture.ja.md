<!-- i18n: language-switcher -->
[English](0002-diff-acceptance-architecture.md) | [日本語](0002-diff-acceptance-architecture.ja.md)

# ADR 0002: ayame-editor の diff / sortdiff 受け入れアーキテクチャ

- ステータス: Accepted（2026-07-10）
- 関連 Issue: hjosugi/ayame-diff#4
- 実装 Issue: hjosugi/ayame-diff#5 #6 #7 #8 #9
- 移管元 Epic: hjosugi/ayame-editor#104

## 背景

ayame-editor（Rust）から diff 関連機能を本プロジェクト（Go・依存ゼロ方針）へ
移管する。移管対象と参照実装：

- `crates/ayame-cli/src/diff.rs`（610 行）
  - `cmd_diff`: **bounded resync window 方式**の行 diff。全行 LCS 行列を
    持たず、アンカー行から前方 `window` 行だけ走査して再同期するため
    **O(n)・メモリ有界**で巨大ファイルに耐える。出力は unified（既定）/
    `--side-by-side` / `--json` / `--summary`。`--max-hunks` `--max-lines`
    `--window` `--width` で制御。
  - `cmd_sortdiff`: 両ファイルを外部ソートで UTF-8 一時ファイルへ書き出し、
    同じ `diff_documents` に通す。`--key/-k` `--delim/-t` `--quote`
    `--numeric/-n` `--reverse/-r` `--csv` `--budget` `--spill-dir` を持つ。
  - データモデル: `DiffResult` / `DiffHunk` / `DiffKind{Insert,Delete,Replace}`。
- `serve/ops.rs:968-1134`（`/api/diff`）、`web/src/search.ts:539-741`（diff ビュー）
  → GUI 側（#10 #11）で受ける。

## 決定

### 1. 移植方式: Go への再実装

Rust→Go の FFI やサブプロセス連携は採らず、**Go で純粋に再実装**する。
依存ゼロ方針（標準ライブラリのみ）と整合し、単一バイナリ配布・`go install`・
クロスコンパイルの容易さを維持する。参照実装のアルゴリズム（bounded resync
window）とデータモデル（Insert/Delete/Replace ハンク）をそのまま踏襲する。

### 2. CLI サーフェス: サブコマンド化（後方互換つき）

```
ayame-diff csv    [flags] --left A --right B   # 既存: CSV/TSV キー比較
ayame-diff text   [flags] OLD NEW              # 新規(#5): 行 diff（resync window）
ayame-diff sorted [flags] OLD NEW              # 新規(#7): 外部ソート後に text diff
ayame-diff        [flags]                      # 無印 = csv 互換（後方互換）
```

> 補足（2026-07-10）: 対話式 TUI ウィザードは #25 で撤去済み。現状の無印・引数なし
> 起動は使い方を表示して終了する。サブコマンド実装（#5）で無印を csv に割り当てる。

**安全なデフォルト + 上級者向け逃げ道**（Sindre Sorhus 流）:

- 既存ユーザーの `ayame-diff --left ... --right ...` は **無印 = csv** に
  ディスパッチして壊さない（サブコマンド実装後）。
- 新機能は明示的なサブコマンドの下に置き、無関係なフラグが混ざらないように
  する（`--mode` フラグ方式を採らない理由：モードごとに有効フラグが違うため、
  サブコマンドで名前空間を分けた方がヘルプ・検証が明快になる）。
- 出力既定は unified（人間可読）。機械可読が要るときだけ `--json`。

サブコマンド・ディスパッチャの実装は #5 で行う（本 ADR では方式のみ確定）。
将来の `serve`（#10）/ `gui`（#14）も同じ第 1 引数サブコマンドとして自然に増設できる。

### 3. 共用エンジンの範囲（小さく焦点の絞れた部品）

参照した部品分割方針（Sindre Sorhus: Small Focused Modules）に沿い、巨大な
単一クラスにせず境界を切る。既存 `internal/engine`（外部ソート・パーティション
基盤）を土台に：

| パッケージ（予定） | 責務 | 由来 |
| --- | --- | --- |
| `internal/engine`（既存 = fcsv） | CSV/TSV パース・キー比較・**外部ソート/パーティション** | 現行 |
| `internal/linediff`（新 #5） | bounded resync window の行 diff・`Hunk{Insert/Delete/Replace}` | diff.rs 移植 |
| `internal/diffout`（新 #6） | unified / side-by-side / JSON / summary の整形（linediff から分離） | diff.rs 移植 |
| `internal/worddiff`（新 #8） | Replace ハンク内の語単位 LCS ハイライト | search.ts 移植 |

- **`sorted` は新規に外部ソートを書かない**。既存 `internal/engine` の
  ソート/スピル基盤を再利用し、その出力を `linediff` に渡す（`cmd_sortdiff`
  と同じ構図）。job-control（並列・バックプレッシャ・キャンセル）も engine 側の
  既存機構を共用する。
- `linediff` は I/O とアルゴリズムを分離し、出力整形（`diffout`）に依存しない
  純粋なコアに保つ（テスト容易性・GUI からの再利用のため）。

### 4. エンコーディング対応

参照実装は UTF-8 / Shift_JIS / EUC-JP / UTF-16 に対応するが、これは
**#9 に分離**する。初期移植（#5〜#8）は UTF-8 前提で進め、非 UTF-8 は #9 で
別途受け入れる。

### 5. 依存ゼロ方針

**維持する。** 標準ライブラリのみ。例外を許容する基準を明文化しておく：

- 追加してよいのは、標準ライブラリに存在せず自前実装が現実的でない領域に
  限る（例: 非 UTF-8 デコード = `golang.org/x/text/encoding` は #9 で
  可否を再検討、GUI の WebView など）。
- 例外を入れる場合は当該 Issue で「なぜ標準ライブラリで不可能か」を記録し、
  `THIRD_PARTY_NOTICES.md` を更新する。
- CLI コア（csv/text/sorted）は依存ゼロを死守する。

## 完了条件（本 ADR で満たすもの）

移植方式（Go 再実装）・CLI 設計（サブコマンド + 後方互換）・共用範囲
（engine 再利用、linediff/diffout/worddiff の分割）が確定し、実装 Issue
（#5 行 diff / #6 出力 / #7 sortdiff / #8 単語 diff / #9 エンコーディング）が
着手可能になった。

## 却下した案

- **`--mode=csv|text|sorted` フラグ方式**: モードごとに有効フラグ集合が
  異なり、ヘルプと検証が複雑化するため却下（サブコマンドで名前空間分離）。
- **Rust バイナリの同梱/サブプロセス呼び出し**: 単一バイナリ配布・依存ゼロ・
  クロスコンパイルの利点を失うため却下。
