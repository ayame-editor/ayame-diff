# ayame-diff

[English](README.md) | [日本語](README.ja.md)

[![CI](https://github.com/hjosugi/ayame-diff/actions/workflows/build.yml/badge.svg)](https://github.com/hjosugi/ayame-diff/actions/workflows/build.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

行順が異なる巨大な CSV / TSV を比較し、差分行を TSV に出力するネイティブ CLI です。テキストの行 diff やソート済み比較、ローカル Web GUI も同梱しています。

5,000,000,000 行級を想定し、全行をメモリに載せません。入力をキーのハッシュで分割し、各分割をメモリ上限付き外部マージソートで整列した後、複数ワーカーで比較します。

## 主な機能

- CSV/TSV キー比較（`csv`）に加え、テキスト行 diff（`text`）とソート済み比較（`sorted`）
- ブラウザで動くローカル Web GUI（`serve`）
- 文字コード自動判定（UTF-8 / UTF-16 / Shift_JIS / EUC-JP / ISO-2022-JP）、`--encoding` で明示指定も可能
- CSV、TSV、`.csv.gz`、`.tsv.gz` に対応
- キー指定なしなら全列をキーとして比較、`--key` / `--exclude-key` で対象列を調整
- 左右で行順や列順が違っても比較可能
- WinMerge 風の比較オプション（大文字小文字を無視、空白を無視）
- ワード単位ハイライト（変更行の中で変わったワードだけを強調）
- Linux / macOS / Windows 向け単一バイナリ
- 外部データベース、CGO 不要。依存は `golang.org/x/text`（文字コード変換）のみ

## インストール

Go、Python、WSL などを入れずに使う場合は、[GitHub Releases](https://github.com/hjosugi/ayame-diff/releases/latest) から OS と CPU に合うアーカイブを取得してください。

- Windows x64 / ARM64: `ayame-diff-<version>-windows.zip`
- Linux x64 / ARM64: `ayame-diff-<version>-linux-<arch>.tar.gz`
- macOS Intel / Apple Silicon: `ayame-diff-<version>-darwin-<arch>.tar.gz`

各 `.tar.gz` を展開すると `ayame-diff-<version>-<os>-<arch>/` ディレクトリができ、実行ファイル `ayame-diff` と `README.md` / `LICENSE` / `THIRD_PARTY_NOTICES.md` が入っています。Windows 版 `.zip` には `ayame-diff.exe`（x64）と `arm64/ayame-diff.exe` が入っています。

Go 1.23 以降がある場合は、ソースから直接インストールできます。

```bash
go install github.com/hjosugi/ayame-diff/cmd/ayame-diff@latest
```

ダウンロードしたアーカイブと同じ Release にある `SHA256SUMS` で、ファイルの完全性を確認できます。

```bash
sha256sum -c SHA256SUMS
```

## サブコマンド

```
ayame-diff csv    [flags] --left A --right B --out D   # CSV/TSV キー比較（既定）
ayame-diff text   [flags] OLD NEW                      # テキストの行 diff
ayame-diff sorted [flags] OLD NEW                      # ソートしてから行 diff
ayame-diff serve  [--addr host:port]                   # ブラウザ GUI（ローカル Web）
```

サブコマンドを付けずに `--left ... --right ...` と起動した場合は `csv`（後方互換）として動作します。

### `csv` — CSV / TSV キー比較（既定）

行順が異なっていても、キーが一致する行どうしを突き合わせて差分を求めます。出力は常に TSV で、先頭に `_diff`（`LEFT_ONLY` / `RIGHT_ONLY` / `CHANGED`）と `_side`（`left` / `right`）を付け、その後ろに左入力の列順で元の全列を出力します。

キーの指定方法は次の 3 モードです（包含指定と除外指定は混在できません）。

1. オプションなし: 全列をキーにする
2. `--key` / `--key-index`: キーに含める列を指定する
3. `--exclude-key` / `--exclude-key-index`: 全列からキーに含めない列だけを指定する

キーにヘッダー名を指定する例です。

```bash
ayame-diff \
  --left old.tsv \
  --right new.csv \
  --key customer_id \
  --key event_date \
  --out diff.tsv
```

`updated_at` と `checksum` 以外をキーにする例です。除外した列も完全な行比較と差分出力には残るため、残りのキーが同じで除外列だけが違う行は `CHANGED` として出力されます。

```bash
ayame-diff \
  --left old.tsv \
  --right new.tsv \
  --exclude-key updated_at \
  --exclude-key checksum \
  --out diff.tsv
```

gzip 入出力は拡張子で自動的に有効になります。

```bash
ayame-diff --left old.csv.gz --right new.tsv.gz --key id --out diff.tsv.gz
```

キーの選び方や巨大ファイル向けのチューニング（`--memory` / `--partitions` / `--workers` など）は [README.md](README.md) を参照してください。

### `text` — 行 diff

行順どおりに 2 つのテキストファイル（`.gz` 可）を比較します。bounded resync window 方式で、巨大ファイルでも線形・メモリ有界です。

```bash
ayame-diff text old.txt new.txt                 # unified（既定）
ayame-diff text --side-by-side old.txt new.txt  # 2 カラム表示
ayame-diff text --json old.txt new.txt          # 機械可読 JSON
ayame-diff text --summary old.txt new.txt       # サマリ 1 行のみ
```

`--max-hunks` / `--max-lines` / `--window` / `--width` で出力量と再同期幅を調整します。`--word` を付けると、変更行の中で変わったワードだけを `[-削除-]` / `{+追加+}` のマーカーで強調します（ワード単位ハイライト）。

```bash
ayame-diff text --word old.txt new.txt          # ワード単位ハイライト
```

### `sorted` — ソート済み比較

行順が違うだけで同じ内容を持つファイル向けに、両者を行単位でソートしてから比較します。`--numeric` / `-n` で数値としてソート、`--reverse` / `-r` で逆順にできます（v1 はメモリ内ソート、外部ソートは #7）。

```bash
ayame-diff sorted old.txt new.txt
ayame-diff sorted --numeric metrics-a.txt metrics-b.txt   # 数値ソート
ayame-diff sorted --reverse a.txt b.txt                   # 逆順ソート
```

`text` と同じ表示・比較オプション（`--side-by-side` / `--json` / `--summary` / `--word` / `--encoding` / `--ignore-case` / `--ignore-whitespace` など）が使えます。

### `serve` — ブラウザ GUI

ローカル Web アプリを起動し、ブラウザ上で 2 ファイルを比較します。既定で localhost にのみバインドします（入力したパスをそのまま開くため、ローカル利用専用です）。

```bash
ayame-diff serve                       # http://127.0.0.1:8080
ayame-diff serve --addr 127.0.0.1:9000
```

OLD / NEW のパスと `text` / `sorted` モード・オプションを指定して Compare すると、ハンクごとのヘッダー・行番号・ワード単位ハイライト付きの side-by-side グリッドで差分を表示します。

## 文字コード対応

`text` / `sorted` は、次の文字コードを自動判定します（BOM を優先し、その後ヒューリスティックで推定します）。

- UTF-8
- UTF-16 (LE / BE)
- Shift_JIS
- EUC-JP
- ISO-2022-JP

自動判定が誤る場合は、`--encoding` で明示的に上書きしてください。

```bash
ayame-diff text --encoding shift_jis a.txt b.txt   # 文字コードを明示（既定 auto）
```

`--encoding` に指定できる値は `auto`（既定） / `utf-8` / `utf-16le` / `utf-16be` / `shift_jis` / `euc-jp` / `iso-2022-jp` です。

## 比較オプション

WinMerge 風の比較オプションを用意しています。いずれも比較用に正規化するだけで、出力は元の行のまま表示されます。

```bash
ayame-diff text --ignore-case a.txt b.txt              # 大文字小文字を無視
ayame-diff text --ignore-whitespace change a.txt b.txt # 空白の連続を 1 個に圧縮し、行末端をトリム
ayame-diff text --ignore-whitespace all a.txt b.txt    # 空白をすべて無視
```

- `--ignore-case`: 大文字小文字の違いを無視して比較します。
- `--ignore-whitespace none`（既定）: 空白の違いも差分として扱います。
- `--ignore-whitespace change`: 連続する空白を 1 個に圧縮し、行の両端の空白をトリムしてから比較します。
- `--ignore-whitespace all`: 空白をすべて取り除いてから比較します。

これらのオプションは `text` と `sorted` の両方で使えます。

## ライセンス

MIT License（[LICENSE](LICENSE)）。

サードパーティ依存は `golang.org/x/text`（BSD-3-Clause）のみで、それ以外は Go 標準ライブラリです。xxHash64 実装を含むサードパーティ通知は [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) を参照してください。
