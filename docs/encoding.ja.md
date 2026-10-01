<!-- i18n: language-switcher -->
[English](encoding.md) | [日本語](encoding.ja.md)

# エンコーディング

`text` および `sorted` サブコマンドは、さまざまなエンコーディングのファイルを読み込み、差分比較の前にUnicodeにデコードします。デフォルトではエンコーディングは自動的に検出されますが、検出が誤っている場合は `--encoding` オプションで上書きできます。

関係する外部依存関係は `golang.org/x/text` のみで、これは日本語とUTF-16のデコーダを提供します。

## 自動検出

`--encoding auto`（デフォルト）を使用すると、検出は二段階で行われます。

1. **BOM（バイト順マーク）を最初に。** UTF-8、UTF-16LE、またはUTF-16BEのバイト順マークが即座に認識され、内容から除去されます。
2. **ヒューリスティック。** BOMがない場合、バイト列が有効なUTF-8かつ日本語のエンコーディング（Shift_JIS、EUC-JP、ISO-2022-JP）かどうかをバイトパターンのヒューリスティックでチェックします。

自動検出は一般的なケースをカバーしますが、ヒューリスティックは誤ることもあります。短いファイルや、複数のエンコーディングで有効な場合は誤分類されることがあります。その場合は、明示的にエンコーディングを指定してください。

## `--encoding` フラグ

```text
--encoding auto | utf-8 | utf-16le | utf-16be | shift_jis | euc-jp | iso-2022-jp
```

| 値 | エンコーディング |
|---|---|
| `auto` | BOMから検出し、その後ヒューリスティック（デフォルト）。 |
| `utf-8` | UTF-8。 |
| `utf-16le` | UTF-16、リトルエンディアン。 |
| `utf-16be` | UTF-16、ビッグエンディアン。 |
| `shift_jis` | Shift_JIS（日本語）。 |
| `euc-jp` | EUC-JP（日本語）。 |
| `iso-2022-jp` | ISO-2022-JP（日本語）。 |

このフラグは `text` と `sorted` の両方で利用可能で、[GUI](gui.md) `/api/diff` リクエストの `encoding` フィールドと連動しています。

## CP932 と Shift_JIS

ayame-diff の `shift_jis` は Windows コードページ **CP932**（Windows-31J、MS932 とも表記）です。Excel や Access など、日本の業務ツールが出力するのはこちらです。旧来の JIS X 0208 の Shift_JIS の上位集合で、NEC/IBM 拡張文字を追加し、一部のコードポイントの対応先が異なります。実務上の差は機種依存文字（`①`、`㈱`、`～` など）です。`shift_jis` なら正しくデコードできますが、厳密な JIS X 0208 のデコーダでは拒否されたり別のグリフになったりします。たとえばバイト列 `81 60` は CP932 では `～`（全角チルダ、U+FF5E）、厳密な JIS では `〜`（波ダッシュ、U+301C）です。

両者はバイト範囲のほとんどが重なるため、自動検出では区別できません。どちらも `shift_jis` と報告しますが、日本の業務ファイルを編集するうえではそれが有用な選択です。厳密な JIS の対応が必要な場合は、`--encoding` に頼らず、先に変換ツールで変換してください。テキスト比較では、[GUI](gui.md) が各ペインヘッダに検出した文字コードを表示し、そこから切り替えて、スクロール位置を保ったまま比較をやり直せます。

## 例

ファイルの誤検出時にShift_JISを強制指定：

```bash
ayame-diff text --encoding shift_jis a.txt b.txt
```

EUC-JPログを比較：

```bash
ayame-diff text --encoding euc-jp old.log new.log
```

UTF-16LEファイル（例：Windowsツールからエクスポートされたもの）を比較：

```bash
ayame-diff text --encoding utf-16le old.txt new.txt
```

Shift_JISファイルをソートして比較：

```bash
ayame-diff sorted --encoding shift_jis --numeric a.csv b.csv
```

!!! tip "日本語ファイル"
    Shift_JIS、EUC-JP、UTF-16、ISO-2022-JPはすべて自動検出されます（最初にBOM、その後ヒューリスティック）。日本語ファイルがゴミのようにデコードされる場合は、対応する `--encoding` の値を再実行してください。

!!! note "出力はUnicode"
    入力エンコーディングに関係なく、差分出力はUnicodeテキストとして書き出されます。`--encoding` フラグは入力の *デコード* 方法を制御しますが、出力の再エンコードには影響しません。