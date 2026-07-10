# fcsv-diff for Windows

`fcsv-diff.exe` は、行順が異なる巨大な CSV / TSV を比較する Windows ネイティブのコンソールアプリです。

- Windows x64: ZIP直下の `fcsv-diff.exe`
- Windows ARM64: `arm64\fcsv-diff.exe`
- Go / Python / WSL / Java / 外部データベースは不要
- CSV / TSV / CSV.GZ / TSV.GZ 入力
- TSV / TSV.GZ 差分出力
- 日本語のファイルパスとヘッダーに対応

## 1. 対話モードで起動

ZIP内の `start-interactive.cmd` をダブルクリックします。

PowerShellからは、引数なしで起動できます。

```powershell
.\fcsv-diff.exe
```

明示的に指定する場合です。

```powershell
.\fcsv-diff.exe --interactive
```

ウィザードで次を設定できます。

1. 左側と右側のCSV / TSV
2. 差分出力TSV / TSV.GZ
3. ヘッダー有無と列名による整列
4. キー選択方法
5. CSV / TSV形式、区切り文字、パーサー
6. メモリ、一時ディレクトリ、分割数、並列度
7. 最終設定の確認と比較開始

最初の論理レコードだけを読み込んでヘッダー選択画面を表示するため、50億行の入力全体を事前走査しません。

### ヘッダーの複数選択

| キー | 動作 |
|---|---|
| `↑` / `↓` | カーソル移動 |
| `Space` | 現在の列を選択・解除 |
| `PageUp` / `PageDown` | 1画面単位で移動 |
| `Home` / `End` | 先頭・末尾へ移動 |
| `A` | 表示中の列をすべて選択 |
| `N` | 表示中の列をすべて解除 |
| `I` | 表示中の列を反転 |
| `/` | ヘッダー名を部分一致検索 |
| `C` | 検索条件を解除 |
| `Enter` | 確定 |
| `Esc` | キャンセル |
| `Ctrl+C` | 中断 |

キーの選び方は次の3種類です。

- 全列をキーにする
- Spaceで選んだ列だけをキーにする
- Spaceで選んだ列だけをキーから除外する

除外した列も行比較と差分出力には残ります。同じ残存キーで除外列だけが違う場合は、左右の行を `CHANGED` として出力します。

## 2. 起動確認

```powershell
.\fcsv-diff.exe --version
.\fcsv-diff.exe --help
```

`fcsv-diff.exe` はWindowsのUnicode Console APIを使うネイティブコンソールEXEです。日本語ヘッダー、日本語パス、矢印キー、Spaceキーを扱うための追加DLLは不要です。

## 3. 非対話モード: 全列をキーにする

キーオプションを指定しない場合、全列をキーとして行の多重集合差分を取ります。

```powershell
.\fcsv-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.csv" `
  --out "D:\data\diff.tsv"
```

1列でも値が違う行は、変更前が `LEFT_ONLY`、変更後が `RIGHT_ONLY` になります。

## 4. 非対話モード: キーから除外する列だけを指定

`updated_at` と `checksum` 以外の全列をキーにする例です。

```powershell
.\fcsv-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --exclude-key updated_at `
  --exclude-key checksum `
  --out "D:\data\diff.tsv"
```

列番号で除外する例です。既定は0始まりです。

```powershell
.\fcsv-diff.exe `
  --left "D:\data\old.csv" `
  --right "D:\data\new.csv" `
  --exclude-key-index 3 `
  --exclude-key-index 7 `
  --out "D:\data\diff.tsv"
```

1始まりで指定する場合は `--index-base 1` を追加します。

## 5. 非対話モード: キーに含める列を明示指定

```powershell
.\fcsv-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --key customer_id `
  --key event_date `
  --out "D:\data\diff.tsv"
```

`--key` 系と `--exclude-key` 系は同時に使えません。

## 6. ヘッダーなし

```powershell
.\fcsv-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --header=false `
  --exclude-key-index 3 `
  --out "D:\data\diff.tsv"
```

対話モードでは `column_0`、`column_1` のような名前で選択できます。

## 7. 50億行向けの例

一時領域には高速なローカルNVMeを指定してください。

```powershell
.\fcsv-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --exclude-key updated_at `
  --exclude-key checksum `
  --out "D:\data\diff.tsv.gz" `
  --temp-dir "E:\fcsv-diff-temp" `
  --memory 32GiB `
  --partitions 512 `
  --parse-workers 24 `
  --workers 16 `
  --merge-fan-in 32
```

入力サイズ、平均行長、キー偏り、差分率により、一時領域は左右の非圧縮入力合計の数倍必要になることがあります。

## 8. ソースからWindows版をビルド

Goをインストールしてから、ソースのルートで実行します。

```powershell
.\scripts\build-windows.ps1
```

成果物:

```text
dist\fcsv-diff-windows-amd64.exe
dist\fcsv-diff-windows-arm64.exe
dist\SHA256SUMS-WINDOWS.txt
```

## 注意

- 配布バイナリはコード署名されていません。組織のポリシーやSmartScreenにより警告される場合があります。
- 対話モードはGUIウィンドウではなく、キーボード操作のコンソールUIです。
- Windows Terminal、PowerShell、コマンドプロンプトで利用できます。
- 処理中に作る一時ファイルは非常に大きくなる可能性があります。
