# ayame-diff

[English](README.md) | [日本語](README.ja.md)

[![CI](https://github.com/hjosugi/ayame-diff/actions/workflows/build.yml/badge.svg)](https://github.com/hjosugi/ayame-diff/actions/workflows/build.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

行順が異なる巨大な CSV / TSV を比較し、差分行を TSV に出力するネイティブ CLI です。

5,000,000,000 行級を想定し、全行をメモリに載せません。入力をキーのハッシュで分割し、各分割をメモリ上限付き外部マージソートで整列した後、複数ワーカーで比較します。

## 主な機能

- CSV/TSV キー比較（`csv`）に加え、テキスト行 diff（`text`）とソート済み比較（`sorted`）
- 文字コード自動判定（UTF-8 / UTF-16 / Shift_JIS / EUC-JP / ISO-2022-JP）、`--encoding` で明示指定も可能
- CSV、TSV、`.csv.gz`、`.tsv.gz` に対応
- 左右で形式が異なる組み合わせにも対応
- キー指定なしなら全列をキーとして比較
- 複数のヘッダー名、または複数の列番号をキーに指定
- `--exclude-key` / `--exclude-key-index` で全列キーから除外する列だけを指定
- 左右で行順が違っても比較可能
- ヘッダー名が同じなら左右の列順が違っても自動整列
- 重複キーを行の多重集合として厳密に比較
- CSV セル単位差分（`_changed_cols` / JSON Lines）、数値許容差、列別変更ランキング
- Web GUI のヘッダー検査、検索可能なキー列選択、解析/性能設定、セル強調・完全結果書き出し
- 引用符、カンマ、タブ、改行を含む RFC 4180 系 CSV/TSV に対応
- 単純TSV/CSV向け高速並列パーサー
- メモリ上限付き外部ソート
- ハッシュ分割単位の並列比較
- gzip 入出力
- Linux / macOS / Windows 向け単一バイナリをクロスビルド可能
- 外部データベース、CGO、外部Goモジュール不要

## インストール

Go、Python、WSLなどを入れずに使う場合は、[GitHub Releases](https://github.com/hjosugi/ayame-diff/releases/latest) からOSとCPUに合うアーカイブを取得してください。

- Windows x64 / ARM64: `ayame-diff-<version>-windows.zip`
- Linux x64 / ARM64: `ayame-diff-<version>-linux-<arch>.tar.gz`
- macOS Intel / Apple Silicon: `ayame-diff-<version>-darwin-<arch>.tar.gz`

Go 1.23以降がある場合は、ソースから直接インストールできます。

```bash
go install github.com/hjosugi/ayame-diff/cmd/ayame-diff@latest
```

ダウンロードしたアーカイブと同じReleaseにある `SHA256SUMS` で、ファイルの完全性を確認できます。

## サブコマンド

```
ayame-diff csv    [flags] --left A --right B --out D   # CSV/TSV キー比較（既定）
ayame-diff text   [flags] OLD NEW                      # テキストの行 diff
ayame-diff sorted [flags] OLD NEW                      # ソートしてから行 diff
ayame-diff dir    [flags] OLD NEW                      # フォルダ/アーカイブ(zip,tar.gz)を再帰比較
ayame-diff bin    [flags] OLD NEW                      # バイナリ/hex 差分
ayame-diff 3way   [text|csv] [flags]                   # BASE / LEFT / RIGHT の3-way比較
ayame-diff serve  [--addr host:port]                   # ブラウザ GUI（ローカル Web）
ayame-diff gui    [--addr host:port] [--no-open]       # GUI を空きポートで起動しブラウザを開く
ayame-diff shell-install                               # ファイルマネージャ連携を登録
ayame-diff shell-uninstall                             # ファイルマネージャ連携を解除
ayame-diff update [--check]                            # 最新リリースへ自己更新（SHA256 検証）
ayame-diff remove [--yes]                              # スタンドアロン版をアンインストール
```

サブコマンドを付けずに `--left ... --right ...` と起動した場合は `csv`（後方互換）として動作します。
2つの裸パス `ayame-diff A B` はファイルなら text、ディレクトリなら dir
を自動選択します。`ayame-diff --gui A B` は GUI を開いて即比較します。
GUIへの2項目ドロップと Explorer / Finder / Linux ファイルマネージャ連携は
`ayame-diff shell-install` で有効にできます（解除は `shell-uninstall`）。

### `text` — 行 diff

行順どおりに 2 つのテキストファイル（`.gz` 可）を比較します。bounded resync window 方式で、巨大ファイルでも線形・メモリ有界です。

```bash
ayame-diff text old.txt new.txt                 # unified（既定）
ayame-diff text --side-by-side old.txt new.txt  # 2 カラム表示
ayame-diff text --json old.txt new.txt          # 機械可読 JSON
ayame-diff text --summary old.txt new.txt       # サマリ 1 行のみ
ayame-diff text --normal old.txt new.txt        # GNU normal-diff（パッチ）
ayame-diff text --format unified -U 3 old.txt new.txt > change.patch
ayame-diff text --format context -C 3 old.txt new.txt > change.patch
ayame-diff text --detect-moves old.txt new.txt   # 移動ブロックを対応付け
ayame-diff text --sync 100:120 old.txt new.txt   # 対応行を手動指定
ayame-diff text --html report.html old.txt new.txt  # 自己完結 HTML レポート
ayame-diff text --pre "jq -S ." a.json b.json   # 前処理してから diff（prediffer）
ayame-diff text --encoding shift_jis a.txt b.txt  # 文字コードを明示（既定 auto）
```

Shift_JIS / EUC-JP / UTF-16 / ISO-2022-JP は自動判定されます（BOM 優先、その後ヒューリスティック）。誤判定時は `--encoding` で上書きしてください。

WinMerge 風の比較オプションもあります（比較用に正規化するだけで、出力は元の行のまま）:

```bash
ayame-diff text --ignore-case a.txt b.txt              # 大文字小文字を無視
ayame-diff text --ignore-whitespace change a.txt b.txt # 空白の連続を 1 個に圧縮・端をトリム
ayame-diff text --ignore-whitespace all a.txt b.txt    # 空白をすべて無視
ayame-diff text --ignore-eol a.txt b.txt               # CRLF/LF 差を無視
ayame-diff text --ignore-trailing-eol a.txt b.txt      # 末尾改行の有無だけを無視
ayame-diff text --filter-line 'timestamp=\S+' a.log b.log # 可変部分を正規表現で除外
```

CSV では同じ正規化をキーと値に適用でき、値比較からの列除外と数値許容差にも対応します。

```bash
ayame-diff csv --left a.csv --right b.csv --key id \
  --ignore-column updated_at --tolerance 0.0001 --out diff.tsv
ayame-diff csv --left a.csv --right b.csv \
  --column-tolerance price=0.01 --out diff.tsv
ayame-diff csv --left a.csv --right b.csv --key id \
  --cell-diff --out cells.tsv       # _changed_cols と列別ランキング
ayame-diff csv --left a.csv --right b.csv --key id \
  --json --out cells.jsonl          # old/new セル値を構造化出力
ayame-diff csv --project jobs/daily.ayamediff.json --diff-exit-code
```

`--max-hunks` / `--max-lines` / `--window` / `--width` で出力量と再同期幅を調整します。`--word` を付けると、変更行の中で変わったワードだけを `[-削除-]` / `{+追加+}` のマーカーで強調します。

### `sorted` — ソート済み比較

行順が違うだけで同じ内容を持つファイル向けに、両者を行単位でソートしてから比較します。`--numeric` / `-n`、`--reverse` / `-r` に対応（v1 はメモリ内ソート、外部ソートは #7）。

```bash
ayame-diff sorted old.txt new.txt
ayame-diff sorted --numeric metrics-a.txt metrics-b.txt
```

### `serve` — ブラウザ GUI

ローカル Web アプリを起動し、ブラウザ上で 2 ファイルを比較します。既定で localhost にのみバインドします（入力したパスをそのまま開くため、ローカル利用専用）。

```bash
ayame-diff serve                       # http://127.0.0.1:8080
ayame-diff serve --addr 127.0.0.1:9000
```

OLD / NEW のパスと `text` / `sorted` モード・オプションを指定して Compare すると、ハンクごとのヘッダー・行番号・ワード単位ハイライト付きの side-by-side グリッドで差分を表示します。

## キーの選び方

キー指定方法は次の3モードです。

1. オプションなし: 全列をキーにする
2. `--key` / `--key-index`: キーに含める列を指定する
3. `--exclude-key` / `--exclude-key-index`: 全列からキーに含めない列だけを指定する

包含指定と除外指定は混在できません。

### 既定: 全列をキーにする

キーオプションを省略すると、すべての列をキーとして行の多重集合差分を取ります。

```powershell
.\ayame-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.csv" `
  --out "D:\data\diff.tsv"
```

全列がキーのため、1列でも値が違う行は `LEFT_ONLY` と `RIGHT_ONLY` になります。全列キー時は、同じ行をキーと行データとして二重保存せず、一時ファイルには1回だけ保存します。

### キーから除外する列だけを指定する

たとえば `updated_at` と `checksum` 以外をキーにする場合です。除外列も完全な行比較と差分出力には残ります。同じ残存キーで除外列だけが違えば、左右の行を `CHANGED` として出力します。

```powershell
.\ayame-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --exclude-key updated_at `
  --exclude-key checksum `
  --out "D:\data\diff.tsv"
```

列番号で除外する場合です。

```powershell
.\ayame-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --exclude-key-index 3 `
  --exclude-key-index 7 `
  --out "D:\data\diff.tsv"
```

### キーに含める列を指定する

ヘッダー名を複数指定します。

```bash
ayame-diff \
  --left old.tsv \
  --right new.csv \
  --key customer_id \
  --key event_date \
  --out diff.tsv
```

列番号を複数指定する場合です。列番号は既定で 0 始まりです。

```bash
ayame-diff \
  --left old.tsv \
  --right new.tsv \
  --key-index 0 \
  --key-index 3 \
  --out diff.tsv
```

1 始まりにする場合です。

```bash
ayame-diff \
  --left old.csv \
  --right new.csv \
  --key-index 1 \
  --key-index 4 \
  --index-base 1 \
  --out diff.tsv
```

ヘッダーがない場合です。

```bash
ayame-diff \
  --left old.tsv \
  --right new.tsv \
  --header=false \
  --key-index 0 \
  --key-index 2 \
  --out diff.tsv
```

gzip 出力は拡張子で有効になります。

```bash
ayame-diff --left old.csv.gz --right new.tsv.gz --key id --out diff.tsv.gz
```

## Windowsネイティブ実行

配布ZIP内の `ayame-diff.exe` は Windows x64 用のネイティブコンソールEXEです。Go、Python、WSL、Java、外部DLLの追加インストールは不要です。

PowerShell:

```powershell
.\ayame-diff.exe --version
.\ayame-diff.exe --help
```

コマンドプロンプト:

```bat
ayame-diff.exe --version
ayame-diff.exe --help
```

ARM64 Windows では `arm64\ayame-diff.exe` を使います。バイナリは `CGO_ENABLED=0` でビルドしています。コード署名証明書は付いていないため、環境によっては初回実行時にWindowsの警告が表示されることがあります。

## 差分出力

出力は常にTSVです。先頭に `_diff` と `_side` を追加し、その後ろに左入力の列順で元の全列を出力します。

```text
_diff	_side	id	name	amount
LEFT_ONLY	left	10	Alice	100
RIGHT_ONLY	right	20	Bob	200
CHANGED	left	30	Carol	300
CHANGED	right	30	Carol	350
```

| `_diff` | `_side` | 意味 |
|---|---|---|
| `LEFT_ONLY` | `left` | そのキーが左側にだけ存在する |
| `RIGHT_ONLY` | `right` | そのキーが右側にだけ存在する |
| `CHANGED` | `left` | 両側にキーはあるが、相殺できなかった左側の行 |
| `CHANGED` | `right` | 両側にキーはあるが、相殺できなかった右側の行 |

同一キーの完全一致行は1行ずつ相殺します。このため、同じキー・同じ行が左に3件、右に2件ある場合、2件は一致、残りの左1件は `CHANGED` です。

全列をキーにする既定モードでは、同じキーは完全に同じ行です。そのため通常は `CHANGED` ではなく、差分行が `LEFT_ONLY` / `RIGHT_ONLY` として出ます。変更前後を同じキーで組にしたい場合は、識別列を `--key` で指定するか、変更しうる列を `--exclude-key` で除外してください。

出力順はハッシュ分割順とキー順であり、入力順でも全体のグローバルキー順でもありません。差分集合として利用してください。

## CSV / TSV パーサー

### `auto`

- CSV: `rfc4180`
- TSV: `simple`

### `simple`

引用符を解釈しない高速パーサーです。非圧縮の通常ファイルなら、ファイル範囲を分割して並列に読みます。

次の条件を満たすデータに向いています。

- フィールド内に区切り文字がない
- フィールド内に改行がない
- 引用符によるエスケープが不要

引用符を使わないCSVも高速化できます。

```bash
ayame-diff \
  --left old.csv --right new.csv \
  --left-parser simple --right-parser simple \
  --key id --out diff.tsv
```

### `rfc4180`

引用符、区切り文字を含むフィールド、フィールド内改行を扱います。TSVでも引用されたタブや改行を扱う必要がある場合は明示します。

```bash
ayame-diff \
  --left old.tsv --right new.tsv \
  --left-parser rfc4180 --right-parser rfc4180 \
  --key id --out diff.tsv
```

不正な引用符を許容する必要がある場合のみ `--lazy-quotes` を使ってください。

## 左右の列順が異なる場合

既定の `--align-columns-by-name=true` では、左右のヘッダー名が一意かつ同じ集合なら、右側を左側の列順に並べ替えて比較します。

```text
left:  id,name,amount
right: amount,id,name
```

この組み合わせも比較できます。ヘッダー名が重複している場合は曖昧になるためエラーにします。

列順も完全一致させたい場合は次を指定します。

```bash
--align-columns-by-name=false
```

## 5,000,000,000 行級の推奨設定

実際の最適値は、平均行長、キー偏り、CPU数、NVMe帯域、空きディスク、ファイルディスクリプタ上限で変わります。最初に数千万行の実データ断片で計測してください。

```bash
ayame-diff \
  --left /data/old.tsv \
  --right /data/new.tsv \
  --key tenant_id \
  --key record_id \
  --out /data/diff.tsv.gz \
  --temp-dir /local-nvme/ayame-diff \
  --memory 32GiB \
  --partitions 512 \
  --parse-workers 24 \
  --workers 16 \
  --merge-fan-in 32
```

重要な調整項目:

- `--temp-dir`: ネットワークストレージではなく、高速なローカルNVMeを推奨
- `--memory`: ソート用メモリ総量。ワーカー数だけ分割される
- `--partitions`: 2の累乗で `2..1024`。キーの偏りが少ないほど均等になる
- `--parse-workers`: 非圧縮 `simple` 入力の並列読み取り数
- `--workers`: ソート・比較する分割数の並列度
- `--partition-buffer`: 分割ファイル1個当たりの書き込みバッファ
- `--merge-fan-in`: 外部マージで同時に開くソート済みrun数

注意事項:

- 一時領域には、少なくとも左右の非圧縮入力合計の数倍を見込んでください。行幅、フィールド数、差分量によって増減します。
- `.gz` 入力は展開ストリームを逐次処理するため、入力パース自体は並列化されません。その後の分割比較は並列です。
- `--partitions 1024` は多数の一時ファイルを開くため、OSのファイルディスクリプタ上限を確認してください。
- 極端に同じキーへ集中するデータでは、そのキーを含む分割がボトルネックになります。
- 標準入力は使えません。形式判定と分割処理に再読可能なファイルが必要です。
- 中断時は既定で作業ディレクトリを削除します。調査したい場合は `--keep-temp` を指定します。

## 主要オプション

```text
--left PATH
--right PATH
--out PATH
--key NAME                 repeatable
--key-index N              repeatable
--exclude-key NAME         repeatable
--exclude-key-index N      repeatable
--index-base 0|1
--header=true|false
--align-columns-by-name=true|false
--left-format auto|csv|tsv
--right-format auto|csv|tsv
--left-delimiter VALUE
--right-delimiter VALUE
--left-parser auto|simple|rfc4180
--right-parser auto|simple|rfc4180
--partitions N
--parse-workers N
--workers N
--memory SIZE
--partition-buffer SIZE
--merge-fan-in N
--max-record-bytes SIZE
--temp-dir PATH
--work-dir PATH
--keep-temp
--progress=true|false
--summary-json PATH
--diff-exit-code
--output-header=true|false
```

全オプションは次で確認できます。

```bash
ayame-diff --help
```

## 終了コード

通常:

- `0`: 正常終了
- `2`: 入力、設定、I/Oなどのエラー
- `130`: 割り込み、または明示的なキャンセル（例: `remove` の確認で中止）

`--diff-exit-code` 指定時:

- `0`: 差分なし
- `1`: 差分あり
- `2`: エラー

## ビルド

Go 1.23 以降を使います。外部依存はありません。

現在のOS向け:

```bash
go build -trimpath -o ayame-diff ./cmd/ayame-diff
```

Linux、macOS、Windows向けをまとめて作る場合:

```bash
./scripts/build-all.sh
```

Windows PowerShellで全OS向けを作る場合:

```powershell
./scripts/build-all.ps1
```

Windows x64 / ARM64だけを作る場合:

```powershell
.\scripts\build-windows.ps1
```

成果物は `dist/` に作られます。

## テスト

```bash
go test ./...
go test -race ./...
go vet ./...
```

## 処理方式

1. 左右の形式、ヘッダー、列数、キー選択を検査
2. 選択された複数キーを長さ付きバイナリで厳密に符号化
3. キーの xxHash64 で分割先だけを決定
4. 元の完全なキーと完全な行を分割ファイルへ保存
5. 各分割をメモリ上限付き外部マージソート
6. ソート済み左右をストリーミング比較
7. 分割ごとのTSVを1つのTSVまたはTSV.GZへ結合

ハッシュ値は分割先の選択だけに使い、同一性判定には完全なキーを使います。ハッシュ衝突で異なるキーを同じキーとして扱うことはありません。

全列キーではキーが完全な行そのものになるため、行データを別に複製せず、ディスクI/Oと一時領域を抑えます。

## 制約

- 左右の列数は同じ必要があります。
- ヘッダー整列を使う場合、左右のヘッダー名集合は同じ必要があります。
- `--key` / `--key-index` と `--exclude-key` / `--exclude-key-index` は同時指定できません。
- 全列を除外してキーを0列にする指定はできません。
- `simple` パーサーでは引用符、フィールド内区切り文字、フィールド内改行を解釈しません。
- 1レコードの符号化後サイズは `--max-record-bytes` 以下である必要があります。
- 1フィールドは4GiB未満です。
- 差分出力自体が非常に大きい場合、最終結合と圧縮が処理時間を占めます。

## ライセンス

MIT License。xxHash64実装に関する通知は `THIRD_PARTY_NOTICES.md` を参照してください。
