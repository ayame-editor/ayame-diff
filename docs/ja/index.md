# ayame-diff

**巨大な** CSV / TSV とテキストファイルを比較するネイティブ CLI ツールです。

`ayame-diff` は 2 つのファイルを比較し、差分を出力します。数十億行規模の入力
を想定しており、全行をメモリに載せません。入力をキーのハッシュで分割し、各
パーティションをメモリ上限付きの外部マージソートで整列してから、複数ワーカーで
比較します。依存は `golang.org/x/text` のみで、データベースも CGO も追加ランタイム
も不要な、単一のスタティックバイナリとして配布します。

!!! note "English documentation"
    このページは日本語のホームです。詳細な英語ドキュメントは
    [English site](../index.md) を参照してください。

## 主な特長

- **CSV/TSV キー比較**（`csv`、既定）に加え、行単位のテキスト diff（`text`）と、
  ソートしてから diff する `sorted`。
- **エンコーディング自動判定** — UTF-8、UTF-16（LE/BE）、Shift_JIS、EUC-JP、
  ISO-2022-JP — と `--encoding` による明示指定。
- **WinMerge 風の比較オプション** — `--ignore-case`、`--ignore-whitespace`、
  `--word` によるワード単位ハイライト、`--window` / `--max-hunks` によるリシンク
  制御。
- **ローカル Web GUI** — `serve` と `gui` で埋め込みのシングルページアプリを起動し、
  ブラウザ上でファイルを比較。
- **単一バイナリ** — Linux、macOS、Windows 向けにクロスコンパイル。

## インストール

### GitHub Releases から

Go を入れたくない場合は、[最新リリース](https://github.com/hjosugi/ayame-diff/releases/latest)
から OS と CPU に合うアーカイブをダウンロードしてください。

- Windows x64 / ARM64: `ayame-diff-<version>-windows.zip`
- Linux x64 / ARM64: `ayame-diff-<version>-linux-<arch>.tar.gz`
- macOS Intel / Apple Silicon: `ayame-diff-<version>-darwin-<arch>.tar.gz`

各リリースには `SHA256SUMS` が同梱されており、ダウンロードの完全性を検証できます。

### `go install` で

Go 1.23 以降なら、ソースから直接ビルドできます。

```bash
go install github.com/hjosugi/ayame-diff/cmd/ayame-diff@latest
```

## クイックスタート

2 つのテキストファイルを既定の unified フォーマットで行 diff します。

```bash
ayame-diff text old.txt new.txt
```

2 つの CSV/TSV ファイルをキーで比較し、差分行を TSV に書き出します。

```bash
ayame-diff csv --left old.tsv --right new.csv --key id --out diff.tsv
```

空きポートを自動で選んでブラウザ GUI を開きます。

```bash
ayame-diff gui
```

!!! tip "サブコマンドなしは `csv`"
    サブコマンドを付けずに `ayame-diff --left A --right B --out D` と実行すると、
    後方互換のため `ayame-diff csv ...` と同じ動作になります。

## サブコマンド一覧

```text
ayame-diff csv    [flags] --left A --right B --out D   # CSV/TSV キー比較（既定）
ayame-diff text   [flags] OLD NEW                      # 行単位のテキスト diff
ayame-diff sorted [flags] OLD NEW                      # 両側をソートしてから diff
ayame-diff serve  [--addr host:port]                   # ローカル Web GUI
ayame-diff gui    [--addr host:port] [--no-open]       # 空きポートで起動しブラウザを開く
```

各サブコマンドは自身のヘルプも表示します。

```bash
ayame-diff --help
ayame-diff text --help
```

## 差分の種類

- `csv`: `LEFT_ONLY` / `RIGHT_ONLY` / `CHANGED`。
- `text` / `sorted`: Insert / Delete / Replace のハンク。

## 対象範囲

本プロジェクトは巨大な CSV / TSV / テキストなど、構造化データの比較に集中します。
画像のピクセル比較と Web ページのレンダリング比較は対象外です。これらには画像
デコーダやブラウザエンジンが必要となり、単一バイナリ・小さな依存関係という方針と
合わないため、WinMerge や専用のビジュアルリグレッションツールを推奨します。

画像などの非テキストファイルも `dir` ではバイナリ内容の同一性を比較でき、差異の
バイト位置は `ayame-diff bin LEFT RIGHT` で確認できます。ただし画像ビューアや
DOM / スクリーンショット比較は提供しません。

## リンク

- [GitHub repository](https://github.com/hjosugi/ayame-diff)
- [Latest release](https://github.com/hjosugi/ayame-diff/releases/latest)
- [Changelog](https://github.com/hjosugi/ayame-diff/blob/main/CHANGELOG.md)
- [Contributing](https://github.com/hjosugi/ayame-diff/blob/main/CONTRIBUTING.md)
