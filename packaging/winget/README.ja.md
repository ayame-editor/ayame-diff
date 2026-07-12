<!-- i18n: language-switcher -->
[English](README.md) | [日本語](README.ja.md)

# WinGet マニフェスト

WinGet マニフェストはプレースホルダーのハッシュを手編集せず、リリースアーカイブから
生成します。

```bash
go run ./cmd/packaging-gen \
  -version v1.2.3 \
  -checksums release/SHA256SUMS \
  -out dist/packaging
```

結果は `dist/packaging/winget/manifests/h/Hjosugi/AyameDiff/<version>/` にあり、
現在の 3 ファイル構成 1.12 schema を使います。`scripts/package-release.sh` がこの
コマンドを実行し、リリース添付ファイルを自動作成します。

最初のコミュニティリポジトリ提出:
[microsoft/winget-pkgs#400883](https://github.com/microsoft/winget-pkgs/pull/400883)
