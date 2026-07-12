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
