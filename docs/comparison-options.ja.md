<!-- i18n: language-switcher -->
[English](comparison-options.md) | [日本語](comparison-options.ja.md)

# 比較オプション

`text`、`sorted`、および `csv` のサブコマンドは、WinMergeスタイルの比較オプションを共有します。これらの中には、行やフィールドの*一致方法*（正規化）を変更するものや、出力の量や差分エンジンの再同期方法を制御するものがあります。正規化は比較のみに影響し、出力には常に元の行が表示されます。

テキストオプションは、[GUI](gui.md) `/api/diff` APIを通じても利用可能です。

## 正規化（行の一致方法）

### `--ignore-case`

行を大文字・小文字を無視して比較します。文字ケースだけが異なる行は等しいとみなされます。

```bash
ayame-diff text --ignore-case a.txt b.txt
```

### `--ignore-whitespace`

行の一致時に空白文字の扱いを制御します。

```text
--ignore-whitespace なし | 変更 | すべて
```

| 値 | 動作 |
|---|---|
| `none` | 空白は重要（デフォルト） |
| `change` | 空白の連続を1つのスペースに縮約し、端をトリム |
| `all` | 比較前にすべての空白を除去 |

```bash
# 空白やタブの連続を1つのスペースとして扱い、先頭・末尾の空白を無視
ayame-diff text --ignore-whitespace change a.txt b.txt

# 空白を完全に無視
ayame-diff text --ignore-whitespace all a.txt b.txt
```

GNU互換のエイリアス `--ignore-space-change` と `--ignore-all-space` は、それぞれ対応するモードを選択します。

!!! note
    `change` と `all` は比較時のみ正規化します。出力される行は元のままです。

### 行末（改行コード）

`text`モードでは、デフォルトで行末（EOL）は重要です。`--ignore-eol` はLF/CRLFの違いを無視し、`--ignore-trailing-eol` は最後の行に終端文字があるかどうかだけを無視します。CSVの解析はレコード単位で行われるため、これらの違いは構造的に無視されます。

```bash
ayame-diff text --ignore-eol windows.txt unix.txt
ayame-diff text --ignore-trailing-eol generated.txt checked-in.txt
```

### `--filter-line`

Goの正規表現に一致する行を比較ビューから除外します。同じフラグを繰り返すことで複数のフィルターを作成可能です。行全体にマッチした場合、その行の内容は無視されます。部分一致の場合は、タイムスタンプやリクエストIDなどの変動しやすい部分だけを除外できます。出力には元のテキストが残ります。CSVモードでは、各フィールドがキーと値の比較前にフィルタリングされます。

```bash
ayame-diff text --filter-line 'timestamp=\S+' --filter-line 'request_id=\d+' a.log b.log
```

## CSV値の制御

`--ignore-column NAME` / `--ignore-column-index N` は、値の比較から特定の列を除外します。デフォルトのすべての列をキーに含める設定では、その列もキーから除外されます。明示的なキーが設定されている場合、その列の除外はキーの制御には影響しません。

数値値は絶対許容誤差を用いて比較可能です。グローバルな許容誤差を設定するには明示的なキーが必要です。列ごとの許容誤差は自動的にその列をデフォルトのキーから除外します。許容誤差列自体は明示的なキー列にはできません。非数値値は引き続き正確に比較されます。

```bash
ayame-diff csv --left a.csv --right b.csv --key id \
  --ignore-column updated_at --tolerance 0.0001 --out diff.tsv

ayame-diff csv --left a.csv --right b.csv \
  --column-tolerance price=0.01 --column-tolerance-index 4=0.1 --out diff.tsv
```

重複キーグループは最大一致を使用するため、許容誤差に対応したペアリングは、前の行に他の候補者がいても失われません。

## 単語レベルのハイライト

### `--word`

統一（デフォルト）出力では、`--word` は差分のある置換部分の単語だけをハイライトします。行全体をマークするのではなく、削除は `[-...-]` で囲み、挿入は `{+...+}` で囲みます。

```bash
ayame-diff text --word old.txt new.txt
```

`--word` は統一フォーマットに適用されます。[GUI](gui.md) では、サイドバイサイドのグリッド内で同等の単語レベルのハイライトが表示されます。

## 再同期と出力制限

巨大なファイルの場合、差分エンジンは行が分岐した後に再同期するために先読みできる距離を制限し、出力量も制限します。これらの設定は、その挙動を調整します。

### `--window`

行の差異があるときに使用される再同期の先読みウィンドウ（デフォルト `128`）。より大きなウィンドウは、大きな挿入・削除ブロック後の再整列を可能にしますが、その分作業量が増えます。

```bash
ayame-diff text --window 512 old.txt new.txt
```

### `--max-hunks`

出力する最大ハンク数（デフォルト `200`）。制限を超えたハンクもカウントされ、合計はサマリーやJSONレポートに反映されますが、出力には表示されません。

```bash
ayame-diff text --max-hunks 50 old.txt new.txt
```

### `--max-lines`

1つのハンクあたりに表示される最大行数（デフォルト `200`）。長いハンクは出力で切り詰められます。

```bash
ayame-diff text --max-lines 40 old.txt new.txt
```

### `--width`

`--side-by-side` 出力の総列幅（デフォルト `160`）。

```bash
ayame-diff text --side-by-side --width 200 old.txt new.txt
```

## オプションの組み合わせ

これらのオプションは自由に組み合わせ可能です。

```bash
ayame-diff text \
  --ignore-case \
  --ignore-whitespace change \
  --ignore-eol \
  --filter-line 'timestamp=\S+' \
  --word \
  --window 256 \
  --max-hunks 100 \
  old.txt new.txt
```

これらは `sorted` でも同様に動作し、`--numeric` / `-n` や `--reverse` / `-r` のソート制御とも併用できます。

```bash
ayame-diff sorted --numeric --ignore-whitespace all a.txt b.txt
```