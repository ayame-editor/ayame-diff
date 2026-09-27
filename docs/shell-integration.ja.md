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

以下は ayame-diff を Git から呼ばれるツールにする設定であり、ayame-diff に
repository の検査・管理を追加するものではありません。この境界は
[ADR 0004](adr/0004-git-repository-boundary.ja.md)を参照してください。
`difftool`は Git の2ファイル形式、`mergetool`は後述の3-way形式を受け取ります。

端末の diff を登録します。

```bash
git config --global diff.tool ayame-diff
git config --global difftool.ayame-diff.cmd \
  'ayame-diff difftool "$LOCAL" "$REMOTE"'
git config --global difftool.prompt false

git difftool --tool=ayame-diff HEAD~1 HEAD -- path/to/file
```

ブラウザで比較する場合は`--wait`を付けます。タブが閉じるまでプロセスが待機するため、
`git difftool`は1ファイルずつ実行し、それぞれの完了を待ちます。

```bash
git config --global difftool.ayame-diff.cmd \
  'ayame-diff difftool --wait "$LOCAL" "$REMOTE"'
```

Git は一時ファイルを`$LOCAL`と`$REMOTE`で渡します。ペインにはその一時パスが表示されて
しまうため、`--label`で論理名に置き換えられます。

```bash
git config --global difftool.ayame-diff.cmd \
  'ayame-diff difftool --label "$LOCAL" --label "$REMOTE" "$LOCAL" "$REMOTE"'
```

Git は custom tool に revision 式を渡さないため、`HEAD~1:foo.txt`のような名前を
表示するには固定ラベルか、`$LOCAL`/`$REMOTE`を名前に割り当てる小さなラッパーを使います。
ラベルは位置引数の順に対応し、1つ目が LEFT、2つ目が RIGHT です。

## Git mergetool

3-way マージを登録し、終了コードを信頼させます。

```bash
git config --global merge.tool ayame-diff
git config --global mergetool.ayame-diff.cmd \
  'ayame-diff mergetool --output "$MERGED" "$BASE" "$LOCAL" "$REMOTE"'
git config --global mergetool.ayame-diff.trustExitCode true

git mergetool --tool=ayame-diff -- path/to/file
```

Git はカスタム merge tool 向けに`$BASE`、`$LOCAL`、`$REMOTE`、`$MERGED`を定義します。
位置引数の順は`BASE LOCAL REMOTE`（P4Merge/Git）で、Meld の`LOCAL BASE REMOTE`順は
`--order local-base-remote`で選べます。`--label`を3回指定すると BASE、LOCAL、REMOTE を
名付けられ、`--gui`でブラウザ解決（タブが閉じるまで待機）になります。

```bash
git config --global mergetool.ayame-diff.cmd \
  'ayame-diff mergetool --gui --order base-local-remote --label BASE --label LOCAL --label REMOTE --output "$MERGED" "$BASE" "$LOCAL" "$REMOTE"'
```

`mergetool`は`$MERGED`が未解決の競合なしで書けたときだけ0を返します。保存済みの出力に
標準の conflict marker が残る場合は1、GUI セッションが何も保存せず終了した場合（中断）は
130を返すため、Git は「保存済み」と「解決済み」を混同しません。端末経路は
`ayame-diff 3way text --merge-exit-code --output "$MERGED"`と同じエンジンを使い、GUI
経路は保存時の未解決数をサーバーから受け取ります。`trustExitCode=true`により、Git は
marker 付き出力を未解決のまま保ちます。非ゼロ終了後に Git がツール実行前の worktree 内容を
復元する場合があります。未解決パスを手動または別の対話ツールで解決し、その後`git add`
してください。

## SVN

SVN と TortoiseSVN も同じコマンドを外部 diff/merge ツールに指定できます。
TortoiseSVN は2ファイル diff に`%mine` / `%yours`、マージに
`%base` / `%mine` / `%theirs` / `%merged`を使います。

```text
Diff:  ayame-diff difftool "%mine" "%yours"
Merge: ayame-diff mergetool --output "%merged" "%base" "%mine" "%theirs"
```

コマンドラインの`--diff-cmd`は独自の`-u -L …`引数を渡し、`difftool`はそれを受け取り
ません。`svn diff --diff-cmd`で使う場合は2つのパスだけを転送するラッパーを挟んでください。

## IDE の外部ツール

外部 diff/merge プログラムを設定できる IDE では、プレースホルダーの記法（JetBrains の
`$1`、Visual Studio の`%1`など）が違っても契約は同じです。プログラムを`ayame-diff`にし、
引数を次のようにします。

```text
Diff:  difftool <left> <right>
Merge: mergetool --output <merged> <base> <local> <remote>
```

IDE の left/right、または base/local/remote/output のプレースホルダーをこの位置に
対応付け、ブラウザ表示にしたい場合は`--gui`を追加します。終了コードが解決状態を運ぶため、
「ツールの終了コードを信頼する」オプションを有効にすると、中断または競合が残るマージを
未解決のまま保てます。

## 繰り返し呼び出し

`git difftool`は1ファイルにつき1回ツールを起動し、各実行を待ちます。呼び出しごとに
短命のローカルサーバーを起動・停止し、セッションは直列で、呼び出しをまたぐサーバー再利用は
ありません。ファイルごとの確認を省くには`difftool.prompt false`を設定します。テキスト
出力だけでよい場合は端末形式を使ってください。
