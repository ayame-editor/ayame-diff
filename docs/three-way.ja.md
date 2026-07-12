<!-- i18n: language-switcher -->
[English](three-way.md) | [日本語](three-way.ja.md)

# 3-way 比較

3-way モードは共通のベースに対して 2 つの派生ファイルを比較し、自動マージ可能な変更と
真の競合を分離します。

## テキスト CLI

```bash
ayame-diff 3way text BASE LEFT RIGHT
ayame-diff 3way text --json BASE LEFT RIGHT
ayame-diff 3way text --format unified BASE LEFT RIGHT
ayame-diff 3way text --choice 2=right --output merged.txt BASE LEFT RIGHT
ayame-diff 3way text --allow-conflicts --output review.txt BASE LEFT RIGHT
```

実装は bounded-window の BASE→LEFT / BASE→RIGHT 行差分を実行し、ベース範囲が重なる
部分だけをクラスタ化します。イベントは左のみ、右のみ、両側の同一変更、競合に分類します。
独立した変更と同一変更は自動マージします。未解決のテキスト競合は
`--allow-conflicts` なしでは拒否し、指定時は標準的な LEFT/BASE/RIGHT マーカーを書きます。

## キー付き CSV / TSV CLI

```bash
ayame-diff 3way csv --base base.csv --left team-a.csv --right team-b.csv \
  --key id --json

ayame-diff 3way csv --base base.csv --left team-a.csv --right team-b.csv \
  --key id --choice 0123456789abcdef=left --output reconciled.csv
```

明示的なキー（または exclude-key セット）が必要です。既存の分割・外部ソート比較を
2 回実行し、変更されたキーグループだけをメモリ上で結合します。保存時は BASE を
ストリーミングし、影響するキーグループを置換するため、変更のない行を実体化しません。
重複行は多重集合として扱います。選択のない CSV 競合は拒否します。明示的に
`--allow-conflicts` を指定した場合は、競合マーカーが構造化レコードとして無効なため
BASE 行を保持します。

調整済み出力は UTF-8 です。`.csv` / `.csv.gz` はカンマ、`.tsv` / `.tsv.gz` はタブを
使います。gzip 入力、日本語文字コード判定、引用、複数行セル、列整列、lazy quotes、
trim-leading-space の設定は通常の CSV エンジンに従います。

## GUI

**3-way text** または **3-way csv** を選び、BASE、LEFT (OLD)、RIGHT (NEW) を指定します。
結果は 3 ペインで競合数を表示します。競合カードでは BASE / LEFT / RIGHT を選択でき、
全競合操作、元に戻す/やり直し、アトミック保存は 2-way マージの安全モデルを再利用します。
差分ナビゲーションは 3-way イベント間でも動作し、`Alt+Left` / `Alt+Right` で側を、
`Alt+B` で BASE を選びます。

上書きオプションと破壊的確認の両方がない限り、入力は上書きしません。新しい結果パスは
一時的な同階層ファイルを経由して rename します。
