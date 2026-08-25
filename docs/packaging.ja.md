<!-- i18n: language-switcher -->
[English](packaging.md) | [日本語](packaging.ja.md)

# パッケージングと Windows の信頼

## 配布チャネルの役割

| チャネル | 対象と責務 |
|---|---|
| GitHub Release ZIP / `.app` / tar | チェックサムで署名相当の確認を行う標準成果物とポータブル GUI ランチャー。パッケージマネージャーは不要。 |
| Scoop | GitHub Release の自動更新と SHA-256 検証を持つ、開発者向け Windows bucket マニフェスト。 |
| WinGet | x64 / ARM64 向けの標準的な Windows 検索・ユーザー単位ポータブルインストール。`ayame-diff` コマンド別名は WinGet が管理。 |
| Homebrew | 管理された macOS CLI のインストールと更新。`.app` は Releases からも入手可能。 |
| `install.ps1` / `install.sh` | パッケージマネージャーがない場合の直接インストール。 |

各リリースは、ビルド直後の `SHA256SUMS` に対して `cmd/packaging-gen` を実行します。
3 ファイル構成の WinGet 1.12 マニフェストツリーと、正確な Scoop / Homebrew
マニフェストを生成してリリースへ添付します。WinGet アーカイブの `manifests/`
ツリーは `microsoft/winget-pkgs` へそのままコピーできます。ファイルは Microsoft の
[マニフェスト仕様](https://github.com/microsoft/winget-pkgs/tree/master/doc/manifest)
に従い、公式 1.12 JSON Schema で検査します。

GitHub Release の公開前に、Windows runner がパッケージジョブの正確なリリース候補を
取得します。Windows / WinGet アーカイブを展開し、同梱 x64 バイナリの引数なし、
`--help`、`--version`、実際のテキスト比較を実行します。ARM64 ペイロードも確認し、
両マニフェストの SHA-256 がリリース ZIP と一致することを検証します。このゲートが
成功しない限り公開ジョブは実行されません。

```bash
go run ./cmd/packaging-gen \
  -version v1.2.3 -checksums release/SHA256SUMS -out dist/packaging
```

コミュニティマニフェストの承認後は次のようにインストールします。

```powershell
winget install ayame-diff
# 一意な形式:
winget install --id Hjosugi.AyameDiff --exact
```

## Inno Setup の判断

現時点で Inno インストーラーは**不要**です。WinGet と Scoop はポータブル実行ファイルを
トランザクションとして導入し、`ayame-diff shell-install` は昇格なしで明示的に
現在ユーザーだけの Explorer 登録を行います。対応する `shell-uninstall` があり、
リリース ZIP には両方のクリック可能なラッパーを同梱します。パッケージ導入時に
シェル統合を暗黙で有効にはしません。

Explorer 統合を選んだユーザーは、ポータブルバイナリの削除・移動前に
`ayame-diff shell-uninstall` を実行してください。シェル統合を自動化する場合、
マシン全体登録を追加する場合、スタートメニュー/関連付けにトランザクション的な
ロールバックが必要な場合、または明示的な解除手順が不十分というサポート実績が
現れた場合に Inno/MSIX を再検討します。

## リリース署名と自己更新の信頼

`ayame-diff update`は実行中のプラットフォーム向けアセットをダウンロードし、
同じリリースで公開された`SHA256SUMS`のSHA-256と照合します。ただしこれだけでは
「転送中に壊れていないこと」しか保証できません。アセットを公開できる者は、
それに一致するチェックサム一覧も公開できるからです。`SHA256SUMS`に対する
ed25519の分離署名を、バイナリに埋め込んだ公開鍵で検証することで、改竄された
リリースはインストールされずに失敗します。

リリースワークフローは、リポジトリシークレット`RELEASE_SIGNING_KEY`が設定されて
いる場合に`cmd/release-sign`で`SHA256SUMS`に署名し、`SHA256SUMS.sig`を並べて
公開します。シークレットが無い場合は明示的にスキップを記録し、従来どおり未署名で
リリースします。秘密鍵はコマンドラインに現れません（`AYAME_RELEASE_SIGNING_KEY`
から読み取ります）。

鍵が空のままビルドされた更新機能は検証できないため、「未署名のリリースである」と
表示したうえでチェックサムのみで続行します。鍵を埋め込んでビルドされた更新機能は、
署名が無い・解析できない・別の鍵の署名であるリリースを拒否します。

署名を有効にする手順:

```bash
go run ./cmd/release-sign keygen
```

出力された秘密鍵をリポジトリシークレット`RELEASE_SIGNING_KEY`に登録し、公開鍵を
`internal/selfupdate/verify.go`の`releasePublicKey`に設定してリリースします。
鍵を埋め込んだバイナリは未署名の更新を拒否するため、**その鍵を含むバイナリを配布する
前に、署名済みリリースを少なくとも1回公開してください。**

ダウンロードと展開には上限があります。リリースアセット、展開後の実行ファイル、
アーカイブ内のエントリ数のいずれにも上限を設けているため、展開爆弾や過大な応答は
確保せずに拒否します。

## 署名とマルウェアスキャン

現在の Windows 実行ファイルは未署名です。リリースゲート内で SHA-256 一覧を生成し、
成果物はプロジェクトの GitHub Release から配布します。これは完全性を検証しますが、
発行者の身元保証や SmartScreen 評判警告の除去にはなりません。Windows のダウンロード
量と警告対応コストが証明書・秘密管理コストを上回る段階で署名を導入します。その場合、
署名はチェックサム、パッケージマニフェスト、マルウェアスキャンの生成前に行います。

リリースワークフローは公開前に `scripts/virustotal-scan.sh` を実行します。リポジトリの
secret `VT_API_KEY` があれば VirusTotal API v3 を使い、解析完了を待って、既定で
malicious / suspicious 判定をブロックします。secret がなければ明示的な skip を記録し、
より厳格な環境では `REQUIRE_VT=1` で資格情報を必須にできます。API キーは出力しません。
アップロードとポーリングには公式の
[VirusTotal API v3 file endpoint](https://docs.virustotal.com/reference/files-scan)
を使います。リリースアーカイブは公開物であり、秘密を含めてはいけません。

VirusTotal の結果は早期警告であり、ソースレビュー、再現可能なチェックサム、将来の
コード署名の代替ではありません。
