<!-- i18n: language-switcher -->
[English](shell-integration.md) | [日本語](shell-integration.ja.md)

# ファイルマネージャーとクイック起動

サブコマンドを指定せずに 2 項目を比較できます。

```bash
ayame-diff old.txt new.txt
ayame-diff old-folder new-folder
ayame-diff --gui old.txt new.txt
```

最初の 2 形式はテキストまたはフォルダの CLI 出力を自動選択します。`--gui` はパスを
入力済みのローカル GUI を開き、すぐ比較を開始します。GUI の任意の場所へ 2 ファイル
またはフォルダをドロップしても比較できます。1 項目だけなら最初の空欄を埋めます。

## ファイルマネージャー統合のインストール

```bash
ayame-diff shell-install
# 後で解除:
ayame-diff shell-uninstall
```

登録はユーザー単位で、管理者権限は不要です。

- Windows はファイルとフォルダに Explorer の **Compare with Ayame Diff** を追加します。最初の項目、次の項目の順に選びます。SendTo 項目も追加し、2 項目を選んで SendTo を使うと GUI を直接起動します。リリース ZIP には `install-shell.cmd` と `uninstall-shell.cmd` も含まれます。
- macOS は `~/Library/Services` に **Compare with Ayame Diff** という Finder Quick Action を導入します。2 項目を選び Quick Actions から実行します。
- Linux はファイル、CSV、JSON、ディレクトリの MIME type と Ayame の scalable icon を持つ desktop entry を `~/.local/share/applications` に導入します。`%F` 対応のファイルマネージャーでは、2 項目を選んで **Open With Ayame Diff** を使います。

登録は実行ファイルの絶対パスを保存するため、実行ファイルを移動した後は
`shell-install` を再実行してください。

## Git difftool

端末の diff を Git のカスタムツールとして登録します。

```bash
git config --global diff.tool ayame-diff
git config --global difftool.ayame-diff.cmd \
  'ayame-diff text "$LOCAL" "$REMOTE"'
git config --global difftool.prompt false
```

次のように実行します。

```bash
git difftool --tool=ayame-diff HEAD~1 HEAD -- path/to/file
```

Git は一時ファイルを `$LOCAL` と `$REMOTE` で渡します。この端末ワークフローは
比較完了まで待機し、通常の CLI と同じ text engine を使います。ブラウザ GUI には、
`git difftool` から安全に繰り返し使うための blocking lifetime と論理ラベルがまだ
ないため、[#295](https://github.com/hjosugi/ayame-diff/issues/295)で追跡します。

## Git mergetool

現在の非対話連携では、競合のない自動マージだけを Git が受け入れます。
ayame-diff で conflict が残る場合は失敗を返し、Git はそのパスを未解決のまま
保ちます。

```bash
git config --global merge.tool ayame-diff
git config --global mergetool.ayame-diff.cmd \
  'ayame-diff 3way text --allow-conflicts --merge-exit-code --output "$MERGED" "$BASE" "$LOCAL" "$REMOTE"'
git config --global mergetool.ayame-diff.trustExitCode true
```

競合した `git merge` の後に実行します。

```bash
git mergetool --tool=ayame-diff -- path/to/file
```

Git はカスタム merge tool 向けに `$BASE`、`$LOCAL`、`$REMOTE`、`$MERGED` を
定義します。`--merge-exit-code` は `--output` を必須とし、未解決の
ayame-diff conflict がない出力を書けた場合だけ 0、標準 conflict marker を
書いた場合は 1、不正な呼び出しは 2、実行時または書き込み失敗は 3 を返します。
`trustExitCode=true` により、Git は marker 付き出力を「保存済み」と「解決済み」で
混同せず、未解決のまま保ちます。非ゼロ終了後に Git がツール実行前の worktree
内容を復元する場合があります。未解決パスを手動または別の対話ツールで解決し、
その後 `git add` してください。

これは意図的に端末・自動処理の基準経路だけを提供します。GUI での対話的な競合解消、
ブラウザ終了までの待機、一時ファイルの論理ラベル、反復セッション再利用は今後の
external-tool 対応として #295 に残します。
