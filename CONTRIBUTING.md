# Contributing

IssueやPull Requestを歓迎します。差分結果の不具合では、機密情報を除いた最小のCSVまたはTSVを添えてください。巨大な実データは添付しないでください。

## Development

Go 1.23以降を使用します。外部Go moduleへの依存はありません。

```bash
go test ./...
go test -race ./...
go vet ./...
```

現在のOS向けバイナリは次で作成できます。

```bash
make build
./scripts/smoke-test.sh
```

## Pull requests

- 1つのPull Requestでは1つの目的に集中してください。
- 新しい動作にはテストを追加してください。
- CLI、出力、既定値を変更した場合はREADMEとCHANGELOGも更新してください。
- Windows固有コードを変更した場合は、可能ならWindows x64またはARM64で実機確認してください。
- 大規模性能の数値は、入力条件、CPU、RAM、ストレージ、コマンドを併記してください。
