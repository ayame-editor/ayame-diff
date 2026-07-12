<!-- i18n: language-switcher -->
[English](projects.md) | [日本語](projects.ja.md)

# 比較プロジェクト

`.ayamediff.json` プロジェクトは、繰り返し可能な CSV/TSV 比較を1つ保存します。GUI と CLI は同じバージョン管理された JSON を使用し、相対パスはプロジェクトファイルのディレクトリから解決されます。これにより、プロジェクトはリポジトリのテストデータと並べてコミットするのに適しています。

## スキーマバージョン 1

```json
{
  "version": 1,
  "mode": "csv",
  "csv": {
    "LeftPath": "../fixtures/old.csv",
    "RightPath": "../fixtures/new.csv",
    "OutputPath": "../reports/diff.tsv",
    "KeyNames": ["id"],
    "IgnoreColumnNames": ["updated_at"],
    "Tolerance": 0.0001,
    "ToleranceSet": true,
    "HasHeader": true,
    "AlignColumnsByName": true,
    "LeftFormat": "auto",
    "RightFormat": "auto",
    "LeftParser": "auto",
    "RightParser": "auto",
    "Partitions": 256,
    "ParseWorkers": 8,
    "Workers": 8,
    "MemoryText": "2GiB",
    "PartitionBufferText": "256KiB",
    "MergeFanIn": 32,
    "MaxRecordText": "256MiB",
    "OutputHeader": true
  },
  "report": {
    "cell_diff": true,
    "output_format": "tsv"
  }
}
```

`csv` はシリアライズ可能な `engine.Config` です：パス、キー、パーサ/リソース設定、宣言的な無視ルール、数値の許容差、セルレポート設定、および出力設定が保持されます。実行時のライターやコールバックは意図的に除外されています。未知のフィールドやバージョン `1` 以外はクローズドに失敗します。

## CLI

```bash
# 効果的なフラグを保存し、その後比較を実行
ayame-diff csv --left data/a.csv --right data/b.csv --key id \
  --cell-diff --out reports/diff.tsv --save-project jobs/daily.ayamediff.json

# 同じプロジェクトを再実行（パスは jobs/ に相対解決）
ayame-diff csv --project jobs/daily.ayamediff.json --diff-exit-code
```

読み込み時には、プロジェクトの比較設定が優先されます。`--diff-exit-code` や `--summary-json` などの処理動作は呼び出し側によって制御されます。

## GUI と最近の履歴

CSV設定のレビューには、プロジェクトパス、**プロジェクトを開く**、および **プロジェクトを保存** のコントロールが含まれます。保存には出力パスが必要で、結果はヘッドレスでも実行できるようになります。ブラウザは、最近使用した CSV 設定をローカルストレージに10件保持し、選択時にヘッダーを再検査してキー選択を復元します。

## Cron / CI の例

```cron
15 2 * * * /usr/local/bin/ayame-diff csv --project /srv/audit/jobs/daily.ayamediff.json --diff-exit-code --summary-json /srv/audit/reports/summary.json
```

CI では、終了コード `0` は等しいことを意味し、`1` は差分が書き込まれたこと、`2` は設定または処理の失敗を示します。設定された TSV/JSONL 出力とサマリーをジョブのアーティファクトとしてアーカイブしてください。