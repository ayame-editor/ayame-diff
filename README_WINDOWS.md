# ayame-diff for Windows

`ayame-diff.exe` は、行順が異なる巨大な CSV / TSV を比較する Windows ネイティブのコンソールアプリです。

- Windows x64: ZIP直下の `ayame-diff.exe`
- Windows ARM64: `arm64\ayame-diff.exe`
- Go / Python / WSL / Java / 外部データベースは不要
- CSV / TSV / CSV.GZ / TSV.GZ 入力
- TSV / TSV.GZ 差分出力
- 日本語のファイルパスとヘッダーに対応

## 0. GUI で使う（ターミナル不要）

ZIP 内の `start-gui.cmd` をダブルクリックすると、ローカル Web GUI が起動し既定のブラウザで開きます。パスや比較オプションを入力して差分を表示できます。コマンドラインからは `ayame-diff.exe gui` でも同じです。

詳しい使い方はドキュメント（<https://hjosugi.github.io/ayame-diff/>）も参照してください。

## 1. 起動確認

```powershell
.\ayame-diff.exe --version
.\ayame-diff.exe --help
```

`ayame-diff.exe` はWindowsのUnicode Console APIを使うネイティブコンソールEXEです。日本語ヘッダーと日本語パスを扱うための追加DLLは不要です。引数なしで起動した場合は使い方を表示して終了します。比較には `--left` / `--right` / `--out` を指定してください。

## 2. 全列をキーにする

キーオプションを指定しない場合、全列をキーとして行の多重集合差分を取ります。

```powershell
.\ayame-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.csv" `
  --out "D:\data\diff.tsv"
```

1列でも値が違う行は、変更前が `LEFT_ONLY`、変更後が `RIGHT_ONLY` になります。

## 3. キーから除外する列だけを指定

`updated_at` と `checksum` 以外の全列をキーにする例です。

```powershell
.\ayame-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --exclude-key updated_at `
  --exclude-key checksum `
  --out "D:\data\diff.tsv"
```

列番号で除外する例です。既定は0始まりです。

```powershell
.\ayame-diff.exe `
  --left "D:\data\old.csv" `
  --right "D:\data\new.csv" `
  --exclude-key-index 3 `
  --exclude-key-index 7 `
  --out "D:\data\diff.tsv"
```

1始まりで指定する場合は `--index-base 1` を追加します。

## 4. キーに含める列を明示指定

```powershell
.\ayame-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --key customer_id `
  --key event_date `
  --out "D:\data\diff.tsv"
```

`--key` 系と `--exclude-key` 系は同時に使えません。

## 5. ヘッダーなし

```powershell
.\ayame-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --header=false `
  --exclude-key-index 3 `
  --out "D:\data\diff.tsv"
```

ヘッダーなしの入力では、列は `column_0`、`column_1` … のような0始まりの合成名で扱われます。`--key column_0` のように名前で指定するか、`--key-index` / `--exclude-key-index` で列番号を指定できます。

## 6. 50億行向けの例

一時領域には高速なローカルNVMeを指定してください。

```powershell
.\ayame-diff.exe `
  --left "D:\data\old.tsv" `
  --right "D:\data\new.tsv" `
  --exclude-key updated_at `
  --exclude-key checksum `
  --out "D:\data\diff.tsv.gz" `
  --temp-dir "E:\ayame-diff-temp" `
  --memory 32GiB `
  --partitions 512 `
  --parse-workers 24 `
  --workers 16 `
  --merge-fan-in 32
```

入力サイズ、平均行長、キー偏り、差分率により、一時領域は左右の非圧縮入力合計の数倍必要になることがあります。

## 7. ソースからWindows版をビルド

Goをインストールしてから、ソースのルートで実行します。

```powershell
.\scripts\build-windows.ps1
```

成果物:

```text
dist\ayame-diff-windows-amd64.exe
dist\ayame-diff-windows-arm64.exe
dist\SHA256SUMS-WINDOWS.txt
```

## 注意

- 配布バイナリはコード署名されていません。組織のポリシーやSmartScreenにより警告される場合があります。
- Windows Terminal、PowerShell、コマンドプロンプトで利用できます。
- 処理中に作る一時ファイルは非常に大きくなる可能性があります。
