<!-- i18n: language-switcher -->
[English](usage.md) | [日本語](usage.ja.md)

# 使用方法

`ayame-diff` はファイル、構造化データ、フォルダ、アーカイブ、バイナリ、
3-way の比較と、Web GUI、ファイルマネージャー統合を1つのコマンドで扱います。

```text
ayame-diff csv    [flags] --left A --right B --out D   # CSV/TSVキー比較（デフォルト）
ayame-diff text   [flags] OLD NEW                      # 行指向のテキスト差分
ayame-diff sorted [flags] OLD NEW                      # 両方のソート後に差分を比較
ayame-diff dir    [flags] OLD NEW                      # フォルダ/アーカイブ比較
ayame-diff bin    [flags] OLD NEW                      # バイナリ/16進比較
ayame-diff 3way   [text|csv] [flags]                   # BASE / LEFT / RIGHT 比較
ayame-diff serve  [--addr host:port]                   # ローカルWeb GUI
ayame-diff gui    [flags] [OLD [NEW]]                  # GUIを開き、必要なら入力を事前設定
ayame-diff shell-install                               # ファイルマネージャー統合
ayame-diff shell-uninstall                             # 統合を削除
```

`--left ... --right ...`を指定してサブコマンドなしで`ayame-diff`を呼び出すと、後方互換性のために`csv`として動作します。`serve`と`gui`のサブコマンドについては[GUI](gui.ja.md)を参照してください。

2つのパスだけを渡す短縮形では、ファイルなら `text`、ディレクトリなら `dir` が
選ばれます。`--gui` を加えるとブラウザ GUI を開いてすぐ比較します。詳しくは
[ファイルマネージャーとクイック起動](shell-integration.ja.md)を参照してください。

<div class="doc-jump-grid">
  <a class="doc-jump" href="#csv">CSV / TSV を比較</a>
  <a class="doc-jump" href="#text">テキストを比較</a>
  <a class="doc-jump" href="#sorted">ソートして比較</a>
  <a class="doc-jump" href="#dir">フォルダを比較</a>
  <a class="doc-jump" href="#exit-codes">スクリプト / CI で使う</a>
  <a class="doc-jump" href="../gui.ja/">GUI を使う</a>
</div>

---

## `csv` — CSV/TSVキー比較（デフォルト） { #csv }

2つのCSV/TSVファイル（`.csv.gz`や`.tsv.gz`も含む）をキーで比較し、行の順序が異なっていても差異のある行をTSV形式で出力します。左と右は異なるフォーマットを使用しても構いません。ヘッダー名が一致すれば、異なる列の順序は自動的に揃えられます。

```bash
ayame-diff csv --left old.tsv --right new.csv --key id --out diff.tsv
```

### キーの選択

キーのモードは3つあり、包含と除外は混在できません。

1. **キーオプションなし** — 全ての列をキーとみなす（多重集合の行差分）。
2. **`--key` / `--key-index`** — 比較に含める列名（または0から始まるインデックス）を指定。
3. **`--exclude-key` / `--exclude-key-index`** — 指定した列以外をすべてキーとみなす。

```bash
# 全列をキー（デフォルト）
ayame-diff csv --left old.tsv --right new.csv --out diff.tsv

# 名前で指定したキー列（複数指定可能）
ayame-diff csv --left old.tsv --right new.csv --key customer_id --key event_date --out diff.tsv

# 列インデックスで指定（デフォルトは0ベース; 1ベースにしたい場合は --index-base 1）
ayame-diff csv --left old.tsv --right new.tsv --key-index 0 --key-index 3 --out diff.tsv

# 更新日時とチェックサム以外をキーにする
ayame-diff csv --left old.tsv --right new.tsv \
  --exclude-key updated_at --exclude-key checksum --out diff.tsv
```

除外された列も完全な行比較や出力には引き続き含まれます。2つの行が残りのキーを共有し、除外列だけが異なる場合、それらは`CHANGED`ペアとして出力されます。

値の比較から除外したい列には`--ignore-column` / `--ignore-column-index`を使用します。数値の差異を許容する場合は`--tolerance FLOAT`を指定し、数値列の絶対差を比較します。列ごとの許容範囲は`--column-tolerance NAME=FLOAT`または`--column-tolerance-index N=FLOAT`で設定可能です。大文字・空白・正規表現の正規化は`--ignore-case`、`--ignore-whitespace`、および繰り返し指定可能な`--filter-line`で行えます。

### 出力

デフォルトはTSV形式です。先頭に`_diff`と`_side`の2列が追加され、その後に左入力の列順に元の列が続きます。

```text
_diff       _side   id  name    amount
LEFT_ONLY   left    10  Alice   100
RIGHT_ONLY  right   20  Bob     200
CHANGED     left    30  Carol   300
CHANGED     right   30  Carol   350
```

| `_diff` | `_side` | 意味 |
|---|---|---|
| `LEFT_ONLY` | `left` | 左側にのみキーが存在します。 |
| `RIGHT_ONLY` | `right` | 右側にのみキーが存在します。 |
| `CHANGED` | `left` | 両側にキーはありますが、この左側の行はキャンセルできません。 |
| `CHANGED` | `right` | 両側にキーはありますが、この右側の行はキャンセルできません。 |

同じキーの行は相殺され、出力はハッシュパーティションとキーの順序に従います。入力の順序ではなく差分の集合として扱います。

`--cell-diff`を追加すると、`_side`の後に`_changed_cols`列を挿入します。ヘッダーのカンマ区切りの名前は、行の一致と同じ除外・数値許容ルールに従います。標準エラーのサマリーや`--summary-json`は、ペアのカウントに基づいて変更された列をランク付けします。フラグがない場合、デフォルトのTSVスキーマは変更されません。

`--json`（`--output-format jsonl --cell-diff`のエイリアス）を指定すると、各差異について構造化されたJSONオブジェクトが`--out`に出力されます。ペアの変更は`old`/`new`の行と、インデックス・名前・古い値・新しい値を持つ`changed_columns`のエントリを含みます。JSON Lines形式は大量の結果でもストリーム可能です。

```bash
ayame-diff csv --left old.csv --right new.csv --key id \
  --cell-diff --out diff.tsv
ayame-diff csv --left old.csv --right new.csv --key id \
  --json --out diff.jsonl
```

!!! note "gzip出力"
    `.gz`拡張子を付けると自動的にgzip圧縮されます。例：`--out diff.tsv.gz`

### `csv`の選択可能なオプション

```text
--left PATH  --right PATH  --out PATH
--key NAME                 (繰り返し指定可能)
--key-index N              (繰り返し指定可能)
--exclude-key NAME         (繰り返し指定可能)
--exclude-key-index N      (繰り返し指定可能)
--ignore-column NAME       (繰り返し指定可能)
--ignore-column-index N    (繰り返し指定可能)
--ignore-case
--ignore-whitespace none|change|all
--filter-line REGEX        (繰り返し指定可能)
--tolerance FLOAT
--column-tolerance NAME=FLOAT       (繰り返し指定可能)
--column-tolerance-index N=FLOAT    (繰り返し指定可能)
--cell-diff
--json
--output-format tsv|jsonl
--index-base 0|1
--header=true|false
--align-columns-by-name=true|false
--left-format auto|csv|tsv     --right-format auto|csv|tsv
--left-parser auto|simple|rfc4180   --right-parser auto|simple|rfc4180
--partitions N   --parse-workers N   --workers N
--memory SIZE    --merge-fan-in N    --temp-dir PATH
--diff-exit-code
```

`ayame-diff csv --help`で詳細なヘルプと、大規模入力向けの調整オプション（`--memory`、`--partitions`、`--parse-workers`、`--workers`、`--merge-fan-in`、`--temp-dir`）も確認できます。

設定全体を保存・再利用するには`--save-project FILE`や`--project FILE`を使用し、[比較プロジェクト](projects.md)のバージョン管理されたJSONや相対パス、GUI履歴、定期実行/CIでの利用例も参照してください。

---

## `text` — 行指向のテキスト差分 { #text }

2つのテキストファイル（プレーンまたは`.gz`）を行単位で比較します。差分は**挿入**、**削除**、**置換**のハンクとして報告されます。比較範囲はバウンダリウムウィンドウによって制限され、大きな入力でもメモリ使用量を抑えつつ線形に動作します。

```bash
ayame-diff text old.txt new.txt                 # デフォルトのユニファイド形式
ayame-diff text --side-by-side old.txt new.txt  # 2列表示（旧 | 新）
ayame-diff text --json old.txt new.txt          # 機械可読のJSON
ayame-diff text --summary old.txt new.txt       # 1行のサマリーのみ
ayame-diff text --format unified -U 3 old.txt new.txt > change.patch
ayame-diff text --format context -C 3 old.txt new.txt > change.patch
ayame-diff text --format normal old.txt new.txt > change.patch
ayame-diff text --detect-moves --move-min-lines 2 old.txt new.txt
ayame-diff text --window 32 --sync 100:120 --sync 5000:5100 old.txt new.txt
```

### 出力フォーマット

| フラグ | 出力内容 |
|---|---|
| *(なし)* | ユニファイドハンク（デフォルト） |
| `--side-by-side`（エイリアス `--side`） | 2列の旧 / 新レイアウト。`--width`で列幅を設定可能。 |
| `--json` | ハンクの種類、行番号、行数を含む構造化JSON |
| `--summary` | 標準エラーに1行のサマリーを出力 |
| `--format unified` / `-U N` | N行のコンテキスト付きユニファイドパッチ（デフォルトは3） |
| `--format context` / `-C N` | N行のコンテキスト付きコンテキストパッチ（デフォルトは3） |
| `--format normal` / `--normal` | 従来の`NcN`、`NaN`、`NdN`パッチ |

### `text`のフラグ

```text
--json                       差分をJSONとして出力
--side-by-side, --side       2列（旧 | 新）出力
--summary                    1行のサマリーのみ出力
--format FORMAT              パッチ形式：normal、context、unified
--normal                     --format normalのエイリアス
-U N                         N行のユニファイドパッチ
-C N                         N行のコンテキストパッチ
--context-lines N            --format context/unifiedのコンテキスト行数（デフォルト3）
--word                       置換ハンク内の変更された単語をハイライト
--encoding VALUE             自動（デフォルト）、utf-8、utf-16le、utf-16be、shift_jis、euc-jp、iso-2022-jp
--ignore-case                行比較時に大文字小文字を無視
--ignore-whitespace MODE     none（デフォルト）、change（連続空白をまとめる）、all（空白をすべて無視）
--ignore-all-space          --ignore-whitespace allのエイリアス
--ignore-space-change       --ignore-whitespace changeのエイリアス
--ignore-eol                CRLF/LFの違いを無視
--ignore-trailing-eol       最後の行末の違いだけを無視
--filter-line REGEX         比較から正規表現にマッチする行を除外（繰り返し指定可能）
--detect-moves              削除と挿入のブロックを移動としてペアリング（デフォルトはオフ）
--move-min-lines N          最小移動ブロック長（デフォルトは2）
--move-max-candidates N     検出候補の上限（デフォルトは10000）
--sync OLD:NEW              対応する1ベース行を強制（繰り返し指定可能）
--max-hunks N               出力する最大ハンク数。残りはカウントされる（デフォルト200）
--max-lines N               1ハンクあたりの最大行数（デフォルト200）
--window N                  行の差異時にリシンクの先読みウィンドウサイズ（デフォルト128）
--width N                   --side-by-sideの総列幅（デフォルト160）
```

パッチ出力は`--max-hunks`や`--max-lines`で切り詰められません。LF/CRLFや最後の改行なしマーカーを保持し、デコード済みのバイナリやNUL入力を拒否します。ロケールに依存しないファイルヘッダのタイムスタンプを使用します。CIはGNU `patch`とともにこれらのフォーマットを適用し、ユニファイド出力は`git apply`で検証します。

[エンコーディング](encoding.md)や[比較オプション](comparison-options.md)の詳細も参照してください。

---

## `sorted` — ソートしてから差分比較 { #sorted }

順序が異なる同じ行を持つファイルに対して、`sorted`は両方の入力を行単位でソートし、その後`text`と同じ差分比較を行います。`text`と同じ表示フラグに加え、ソート制御も可能です。ソート済みのビューのパッチは元のファイルに安全に適用できないため、パッチ形式は拒否されます。

```bash
ayame-diff sorted old.txt new.txt
ayame-diff sorted --numeric metrics-a.txt metrics-b.txt
ayame-diff sorted --reverse a.txt b.txt
```

### 追加の`sorted`フラグ

```text
--numeric, -n    数値の先頭部分でソート
--reverse, -r    ソート順を逆に
```

!!! note
    v1では`sorted`はメモリ内でソートします。外部メモリを使った行ソートはプロジェクトの課題トラッカーで追跡中です。

---

## `dir` — 再帰的なフォルダ/アーカイブ比較 { #dir }

`dir OLD NEW`はスラッシュ区切りの相対パスを正規化してペアにします。まずサイズを比較し、同じサイズの候補は並列でストリーミングしながらバイト単位で比較します。`--quick`を指定すると、サイズとmtimeだけを信頼します。`.gz`ファイルは解凍内容を比較し、zip/tar/tar.gzアーカイブはフォルダソースとして比較します。

```bash
ayame-diff dir --include '*.csv' --exclude 'tmp/**' --workers 8 old/ new/
ayame-diff dir --tsv --all old/ new/ > folders.tsv
ayame-diff dir --json --diff-exit-code snapshot-a/ snapshot-b/
ayame-diff dir --html folder-report.html old/ new/
ayame-diff dir --csv folder-summary.csv --all old/ new/
```

ドットファイルやディレクトリは`--hidden`を指定しない限りスキップされます。シンボリックリンクも常にスキップされ、ループや不明瞭なツリー外の読み込みを防ぎます。TSVやJSONには状態、相対パス、サイズ、mtimeが含まれます。GUIでは「フォルダ」を選び、状態ツリーをフィルタし、変更された通常ファイルをクリックしてテキスト差分を確認できます。

`--html FILE` は状態別件数、パス、サイズ、更新時刻を含む、ライト/ダーク対応の
自己完結ツリーレポートを書き出します。`--csv FILE` は同じ項目を後続処理向けの
RFC 4180 CSV として書き出します。どちらもアトミックに保存し、`--all` がなければ
同一項目を省略します。これらのファイル出力は `--json` / `--tsv` と同時指定できません。

---

## 終了コード { #exit-codes }

通常：

- `0` — 正常終了
- `2` — 入力、設定、I/Oエラー
- `130` — 中断または明示的にキャンセル（例：`remove`を拒否）

`--diff-exit-code`を指定した場合（`csv`と`dir`）：

- `0` — 差異なし
- `1` — 差異あり
- `2` — エラー

---

## 範囲の境界

`ayame-diff`は大規模な構造化/テキストデータに特化しています。画像レンダリングやWebページのスクリーンショット比較は対象外です。これらには画像デコーダやブラウザエンジンが必要で、WinMergeや専用のビジュアル回帰ツールの方が適しています。

画像やその他の非テキストファイルは`dir`比較にバイナリコンテンツとして参加可能です。`ayame-diff bin LEFT RIGHT`を使って異なるバイトオフセットを調査できます。ただし、ピクセルレベルの画像ビューアやDOM/レンダリングページの比較はありません。
