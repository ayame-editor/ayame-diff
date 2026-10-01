<!-- i18n: language-switcher -->
[English](ui-design-research-followup.md) | [日本語](ui-design-research-followup.ja.md)

# 競合 UI 調査：未検証項目（#284）

2026-07 の UI 設計調査では、実機で触れない商用・未公開ツールに属する挙動が未検証のまま残りました。このページは issue [#284](https://github.com/ayame-editor/ayame-diff/issues/284) の**文書ベースの続き**です。公式ドキュメント・公開ソース・公開トラッカーで確認できたこと、できなかったこと、その理由を記録します。

!!! warning "ここに実機確認はありません"
    以下はいずれもアプリの実行による確認ではありません。Araxis Merge と Kaleidoscope はライセンスが必要で、WinXMerge のデスクトップ版・WASM 版はこの環境で起動できませんでした。各項目は **文書で確認**（公式／ソースに記述あり）か **未検証**（確認できず）で示します。実機で行うべき確認は末尾のチェックリストにまとめます。

## 手法と制約

- 情報源はベンダー自身のユーザーガイド、リリースノート、ヘルプ、リポジトリの README／ソース、公開 issue トラッカーです（URL は本文と各節に記載）。
- Araxis Merge は **Merge for Windows 2026.1**（2026-09-23 リリース）のドキュメントを読みました。macOS 版は UI が異なり（リボンではなくツールバー）、同じ深さでは確認していません。
- WinXMerge は **v0.51.0** の README とソース（`src/lib.rs`、`src/wasm.rs`、`Cargo.toml`）に基づきます。公開中の Web 版 <https://winxmerge.app> は WASM の `<canvas>` アプリですが、内蔵ブラウザが未接続のため読み込めず、WASM の記述は実測ではなくソースからの読み取りです。
- Meld の undo はヘルプに無いため、Meld のソース（`meld/filediff.py`、`meld/undo.py`）から読み取りました。

## Araxis Merge

- 製品／ドキュメント：<https://www.araxis.com/merge/windows/index.en>
- テキスト比較：<https://www.araxis.com/merge/windows/comparing-text-files.en>
- フォルダ比較：<https://www.araxis.com/merge/windows/comparing-folders.en>
- フォルダ概要：<https://www.araxis.com/merge/windows/folder-comparison-overview.en>
- ファイル概要：<https://www.araxis.com/merge/windows/file-comparison-overview.en>
- リボン：<https://www.araxis.com/merge/windows/merge-ribbon-interface.en>
- 3-way：<https://www.araxis.com/merge/windows/three-way-file-comparison-and-merging.en>
- FAQ：<https://www.araxis.com/merge/windows/frequently-asked-questions.en>
- リリースノート：<https://www.araxis.com/merge/release-notes-2026.en>

| #284 の問い | 公開情報で確認できたこと | 状態 |
|---|---|---|
| setup の形（開始画面はあるか） | **独立した開始画面は無い。**「By default, an empty new text comparison tab is automatically opened when you start Merge.」比較はリボン左端のドロップダウン（New text comparison = `Ctrl+L`、New folder comparison = `Ctrl+D`）や相当するリボンボタンから新規作成します。フォルダ比較の初期レイアウトは **Two-way with file comparison** の分割表示です。 | 文書で確認 |
| オプションの置き場所 | **二層構造。** (1) リボンの **Options** メニューの簡易トグル。例：フォルダ比較の `Show changes column`、`Show timestamps and sizes`、`Show executable file and product versions`、`Show Unicode code points`、テキスト比較の `Add vertical padding to align changes`、行折り返し、`Show line-detail panel`。(2) 本格的な **Options…** ダイアログ。Application／File Comparisons／Text Comparisons（Display、Expressions、Line expressions、Line pairing、Editing、Syntax highlighting）／Folder Comparisons（Method、Launch behaviour、Filters、Method…）の各ページがあります。一部はレジストリ専用（`MRUTabOrder`、`IOThrottleTime`）。 | 文書で確認 |
| 編集可否と再計算 | 左右どちらのファイルも**その場で編集**でき、「The file comparison dynamically updates as you make changes.」。変更済みインジケータがペインに出ます。読み取り専用ファイルの編集は既定で無効ですが有効化できます。FAQ は大ファイル性能を「as I edit files」の文脈で説明しており、編集→再計算のループが確認できます。 | 文書で確認 |
| 3-way の配置 | 3 ペイン横並びで、「**common ancestor file is used as the centre file**」、両脇に 2 つの変更版。リボンで 2-way／3-way を切替でき、**Three-way with file comparison** の分割表示もあります。 | 文書で確認 |
| ペインごとのパス表示 | 各ペインの上にパスの入力欄があり、Browse と Show history ボタン、他リビジョンを列挙する **Versions** ボタンが付きます。 | 文書で確認 |
| フォルダ結果はツリーかフラットリストか | **フラットな結果リスト**で、状態は背景色で示します（ツリーではない）。フォルダ行は展開／折りたたみでき、ダブルクリックで別ウィンドウが開きます。分割表示では選択行のファイルが下に出ます。 | 文書で確認 |
| Windows のリボン vs 旧メニュー | Merge 2020 以降はリボン。旧メニュー／ツールバー向けに「Merge 2019 and earlier」のトピックが別途あります。 | 文書で確認 |
| 大ファイルのハング | ドキュメントは比較性能のオプションを多数説明しますが、特定のハングを再現・言及していません。利用者報告はドキュメントだけでは解消できません。 | 未検証 |

**Araxis の未検証項目：** レイアウトの実寸（ペイン比率、センターマーカーの挙動）、現在の UI で全オプションページに到達できるか、読み取り専用編集トグルの正確な位置、インライン編集とマージボタン操作が単一の undo スタックを共有するか（undo は文書化されていない）、大ファイルハングの再現。**理由：** 商用製品で、ライセンスと Windows／macOS へのインストールが必要です。

## WinXMerge

- リポジトリ／README（v0.51.0）：<https://github.com/masak1yu/winxmerge>
- ドキュメント：<https://winxmerge-site.pages.dev/en/guides/introduction/>
- クイックスタート：<https://winxmerge-site.pages.dev/en/guides/quickstart/>
- インライン編集：<https://winxmerge-site.pages.dev/en/features/inline-editing/>
- キーボードショートカット：<https://winxmerge-site.pages.dev/en/reference/keyboard-shortcuts/>
- Web 版：<https://winxmerge.app>
- 参照ソース：[`src/wasm.rs`](https://github.com/masak1yu/winxmerge/blob/main/src/wasm.rs)、[`src/lib.rs`](https://github.com/masak1yu/winxmerge/blob/main/src/lib.rs)、[`Cargo.toml`](https://github.com/masak1yu/winxmerge/blob/main/Cargo.toml)、[`index.html`](https://github.com/masak1yu/winxmerge/blob/main/index.html)

| #284 の問い | 公開情報で確認できたこと | 状態 |
|---|---|---|
| UI の形状（ツールバー、setup） | ネイティブメニューバー、**WinMerge 風の 1 段アイコンツールバー**（New、Open、Save、Undo/Redo、Rescan、Options、Navigation、Copy、Copy & Advance、Copy All、Ignore WS/Case）、タブバー、差分バー付きステータスバー、下部の diff-detail ペイン、**location pane（diff 位置のミニマップ）**。起動時に **WinMerge 風のファイル選択ダイアログ（最近使ったファイル一覧付き）** が出ます（`File → New → Text / Table / 3-way` もあり）。 | 文書で確認 |
| WASM の制約／File System Access API | Web 版は「text input, file upload, clipboard paste, diff navigation with stats」に対応。デスクトップ専用機能（ネイティブファイルダイアログ、シンタックスハイライト、フォルダ比較、インライン編集、アーカイブ、Excel、画像）は `cfg(not(target_arch = "wasm32"))` で除外。ファイルを開く際は隠し `<input type="file" accept="…">` を生成し `Blob.text()` で読む（`src/wasm.rs` の `open_file_picker`）。`Cargo.toml` の `web-sys` は `HtmlInputElement`、`File`、`FileList`、`Blob`、`Document`、`Element`、`EventTarget`、`Event`、`Window`、`console` のみ。**`showOpenFilePicker` / `showSaveFilePicker` / `FileSystemHandle` は使われておらず**、Web 版は File System Access API を使わず、ローカルパスを直接読み書きできません。 | 文書で確認（ソース） |
| 大入力での挙動 | README はデスクトップで「Performance optimizations for large files」と主張しますが、サイズや時間の記載はありません。WASM 版の `on_compare` は `compute_diff_with_options()` を**メインスレッドで同期実行**しており、Web Worker はありません。巨大な貼り付けは UI をブロックすると推測されますが、実測していません。 | 未検証 |
| WinMerge のキーバインド互換度 | README は「Same keyboard shortcuts」と主張し、CLI は WinMerge 互換のスラッシュ構文（`/ignorews`、`/m`、`/t`、`/dl` など）。文書化された既定は `Alt+↑`/`Alt+↓`（前／次の差分）、`Alt+Home`/`Alt+End`、`F2` ブックマーク、`F5` 再スキャン、`Cmd/Ctrl+S` 保存、`Ctrl+Z`/`Ctrl+Shift+Z`、`Cmd+N`/`Cmd+T`/`Cmd+W`、`Cmd+F`、`Cmd+G`、`Cmd+M`。WinMerge 本体のマニュアルは前／次の差分の主キーを `F7`/`F8`、コンフリクトを `Shift+F7` としており、こちらでは使われていません。互換は**部分的**（`Alt+↑/↓`、保存、undo、検索は共通）で、`F7`/`F8` とコンフリクト移動は異なります。 | 一部文書で確認 |
| Undo/Redo | デスクトップの README はマージ操作とインライン編集の undo/redo を挙げています。`src/wasm.rs` は `on_undo`/`on_redo` を登録しておらず、Web 版には undo が無いように見えます。 | 文書で確認（ソース）／ブラウザ確認が必要 |

**WinXMerge の未検証項目：** デスクトップ UI と setup ダイアログの実物、WASM UI／undo の実際の状態、両版の大入力時間、Web 版へのドラッグ＆ドロップ、WinMerge 既定ショートカットとの逐一比較。**理由：** デスクトップ版が未インストールで、内蔵ブラウザが未接続のため <https://winxmerge.app> を操作できませんでした。実際に起動する必要があります。

## Meld

- ヘルプ：<https://help.gnome.org/meld/>
- ファイル比較の開始：<https://help.gnome.org/meld/file-mode.html>
- 変更の扱い：<https://help.gnome.org/meld/file-changes.html>
- Meld がしないこと：<https://help.gnome.org/meld/missing-functionality.html>
- ソース：<https://gitlab.gnome.org/GNOME/meld>（`meld/filediff.py`、`meld/undo.py`）

| #284 の問い | 公式情報の記述 | 状態 |
|---|---|---|
| setup ページは結果に置き換わるか | 「Once you've selected your files, Meld will show them side-by-side.」選択画面は常設パネルではなく比較エディタに置き換わります。 | 文書で確認 |
| undo のセマンティクス（変更バー操作と打鍵編集は同一スタックか） | **ヘルプには undo/redo の記述が一切ありません。** ソースを読むと答えが出ます。`Filediff` は全ペイン共有の `UndoSequence(self.textbuffer)` を 1 つ作ります（`meld/filediff.py`）。`on_text_insert_text`/`on_text_delete_range` が**すべての**バッファ変更に対して `BufferInsertionAction`/`BufferDeletionAction` を追加し、変更バーのマージ操作（`copy_chunk`、pull-all、merge-all）も同じバッファを `begin_user_action()`/`end_user_action()` 内で変更します。`UndoSequence.begin_group()`/`end_group()` は複数バッファの編集を 1 つの `GroupAction` にまとめます。つまり変更バー操作と打鍵編集は**比較ごとに同一の undo スタックを共有**し、バッファ単位のチェックポイントと複数ファイルのグループ化を持ちます。 | 文書は未確認／ソースで確認 |
| ライブ再計算 | 「Editing the files will cause the comparison to update on-the-fly.」 | 文書で確認 |
| 変更バーの操作 | 中央の変更バーの矢印／×印で置換（既定）、削除（Shift）、挿入（Ctrl）。 | 文書で確認 |

**Meld の未検証項目：** 利用者が体感する undo の粒度（1 回の打鍵が次の変更バー操作とまとまるか等）、undo から除外される変更バー操作の有無。**理由：** ソースは仕組みを示しますが、体感上のグルーピングや GTK／GtkSourceView の干渉は動作中の Meld で確認する必要があります。

## KDiff3

- ハンドブック（SourceForge）：<https://kdiff3.sourceforge.net/doc/index.html>
- マージと出力ウィンドウ：<https://kdiff3.sourceforge.net/doc/merging.html>
- 入力ウィンドウの解釈：<https://kdiff3.sourceforge.net/doc/interpretinginformation.html>
- ナビゲーションと編集：<https://kdiff3.sourceforge.net/doc/navigation.html>
- FAQ：<https://kdiff3.sourceforge.net/doc/faq.html>
- 現行 KDE ドキュメント：<https://docs.kde.org/stable_kf6/en/kdiff3/kdiff3/documentation.html>

| #284 の問い | 公式情報の記述 | 状態 |
|---|---|---|
| A/B/C 入力ペインは編集可能か | 入力ウィンドウはナビゲーション、選択、コピー＆ペースト、手動 diff 整列、セクションの分割／結合のために説明されています。編集は常に**マージ出力エディタ**についてのみ記述されます：「It is often helpful directly edit the merge output.」「In the merge output editor you can also use the other keys for editing.」入力の情報行は「the editable filename」（パスであって本文ではない）と説明されます。入力本文の編集は**文書化されておらず**、コミュニティの回答も編集は出力ペインのみと述べています。 | 未検証（文書に記述なし） |
| 出力編集時にライブ再計算するか | 出力は差分の下にある別のマージ対象です。「immediately recalculate」と記述されるのは入力ペインの**手動 diff 整列**のみです。出力編集が diff を再実行するとは記述されていません。 | 未検証（文書に記述なし） |
| Undo | **明示的に無し。** FAQ 5.7「Why does the editor in the merge result window not have an 'undo'-function?」—「This was too much effort until now. You can always restore a version from one source (A, B or C) by clicking the respective button.」 | 文書で確認 |
| その他の制約 | すべてのコンフリクトを解決するまで保存できません。出力はセクション境界を記憶するため、保存して再開することはできません。 | 文書で確認 |

**KDiff3 の未検証項目：** 現行の KDE Frameworks 6 版でも入力ペインの編集を拒否し undo が無いか、出力編集で diff が再実行されないか。FAQ と SourceForge のハンドブックは古いため、現行リリースで確認が必要です。**理由：** この環境に KDiff3 のインストールがありません。

## P4Merge（Helix Core Visual Merge Tool）

- P4Merge ユーザーガイド（r18.4 PDF）：<https://ftp.perforce.com/perforce/r18.4/doc/manuals/p4merge.pdf>
- 製品ページ：<https://www.perforce.com/products/helix-core-apps/merge-diff-tool-p4merge>

r18.4 ガイドは diff・編集・画像・マージの流れを説明しますが、#284 のいくつかの問いには触れていません。

| #284 の問い | ガイドの記述 | 状態 |
|---|---|---|
| 編集モデル | その場編集では**ありません**。diff モードではツールバーのボタンが**ウィンドウ下部の別の編集ペイン**を開きます（書き込み可能なワークスペースファイル、二重ペイン表示時）。マージモードでは「To edit the text in the merge result file, click on the text in the **bottom pane**」。 | 文書で確認 |
| 3-way の配置 | 上半分：base を中央に、両脇に 2 つの変更版。下半分：マージ結果。 | 文書で確認 |
| ミニマップ／オーバービュー | ガイドにはミニマップ、オーバービュールーラー、スクロールバーの変更マーカーの記述が**一切ありません**。#284 の方針どおり、存在を仮定しません。 | 未検証（文書に記述なし） |
| ライブ再計算 | 記述なし。ガイドはチャンクの選択と結果の編集のみを説明します。 | 未検証（文書に記述なし） |
| undo／redo | ガイドのどこにも記述がありません。 | 未検証（文書に記述なし） |
| ペインごとのパス表示 | 「each file in its own pane」とあるだけで、ペインごとのパス／ファイル名ラベルは文書化されていません。 | 未検証（文書に記述なし） |
| 現行版の性能 | ガイドは r18.4（2018 年）。#284 で挙げたハング／クラッシュ報告は macOS Mavericks／Qt4 期のものです。現行の Qt5／現行版で残るかは確認できていません。 | 未検証 |

**P4Merge の未検証項目：** ミニマップ／オーバービュー、ライブ再計算、undo、ペインごとのパス表示、現行版の性能。**理由：** 公式ガイドに記述が無く、ツールも未インストールのため、現行版の実機とリリースノートでの確認が必要です。

## JetBrains IDE（IntelliJ IDEA 2026.2）

- Diff Viewer：<https://www.jetbrains.com/help/idea/differences-viewer.html>
- ファイル／フォルダのリビジョン比較：<https://www.jetbrains.com/help/idea/comparing-file-versions.html>
- Git コンフリクトの解決：<https://www.jetbrains.com/help/idea/resolve-conflicts.html>
- トラッカー：<https://youtrack.jetbrains.com/issue/IJPL-241701>（公開 REST の記録も確認）

| #284 の問い | 公開情報で確認できたこと | 状態 |
|---|---|---|
| diff ビューア内で比較リビジョンを変更する UI | **存在しません。** リビジョン選択はビューアの外で行います：Project ビューの `Git | Compare With Revision` / `Git | Compare With Branch or Tag`、または Commit ツールウィンドウからの diff 起動。ビューア内にリビジョンのドロップダウンを追加する**未解決の機能要望**があります — IJPL-241701「Allow to select different file revision directly from the diff viewer」（State：**Open**、修正未割当。要望本文は「there is no direct way to switch one of the compared versions … within the Diff viewer itself」と明記）。 | 文書で確認 |
| 確認済みのビューア内操作 | ドキュメントは **Compare with Clipboard**（ペインのコンテキストメニュー）と **Switch to Three-Side Viewer** を確認。先行調査の「Swap Sides」もビューア内操作です。コンフリクト解決のマージ操作には **Compare Contents** ボタンがあります。 | 文書で確認 |
| コンフリクトの数値カウンタ | ドキュメントはコンフリクトファイルを一覧し解決済／未解決の進捗状態を示す **Conflicts ダイアログ**と、コンフリクトごとの Accept／Ignore を伴う 3 ペインの解決ツールを説明します。**`N 個中 n 個`のような数値カウンタは文書化されていません**（VS Code のマージエディタとは異なる）。 | 未検証（文書に記述なし） |
| undo のセマンティクス（対比） | 「Ctrl+Z/Ctrl+Shift+Z — undo/redo a merge operation. Conflicts will be kept in sync with the text.」マージとテキストの undo が同じ操作で、コンフリクトが同期します。 | 文書で確認 |

**JetBrains の未検証項目：** コンフリクトカウンタ UI の正確な有無（おそらく無し）、隠れたビューア内リビジョン切替の有無。**理由：** 動作中の IDE が必要。ドキュメントと未解決トラッカーから、現時点では「無い」と見なします。

## Kaleidoscope 7

- ヘルプ：<https://kaleidoscope.app/help>
- テキスト差分と色：<https://kaleidoscope.app/help/docs/text-diffs-and-colors>
- テキスト比較ビュー：<https://kaleidoscope.app/help/docs/text-comparison-views>
- テキスト比較設定：<https://kaleidoscope.app/help/docs/text-compare-settings>
- ブログ「Navigating Changes and Conflicts」：<https://blog.kaleidoscope.app/2024/12/30/navigating-changes-and-conflicts/>

| #284 の問い | 公開情報で確認できたこと | 状態 |
|---|---|---|
| スクロールバーの変更マーカーはミニマップ相当か | #284 は目視でスクロールバーの変更マーカーを確認済みです。ヘルプとブログは**下部ツールバーの change stepper**と、「how many changes there are and where within those changes you are currently located」を示すラベル、すなわち**数値の変更カウンタ**を説明します。**ミニマップ／コードマップは文書化されておらず**、テキストビュー（Fluid、Blocks、Unified）はレイアウトモードであってマップではありません。スクロールバーのマークが最も近い文書化機能ですが、それが縮小テキストマップを描くかは記述がありません。 | 一部文書で確認 |
| 変更検出とナビゲーション | 追加（緑）、削除（赤）、変更（青、語は濃い青）。stepper、`View > Next/Previous Change`、`⌘▼`/`⌘▲`、3 本指スワイプで前後の変更へ移動。 | 文書で確認 |
| マージ／コンフリクトのナビゲーション | マージモードでは stepper が変更ではなくコンフリクトへ移動します。Option+`⌘▼/▲` で変更移動に切替。未解決コンフリクトには警告アイコン。 | 文書で確認 |

**Kaleidoscope の未検証項目：** スクロールバーのマークが単なる目盛りか真のミニマップか、カウンタの正確な表示形式。**理由：** Kaleidoscope は有償の macOS アプリで、この環境にライセンス／インストールがありません。

## 補足：DiffMerge（確認済み）

issue の補足にある SourceGear DiffMerge の事実は別途確認済みで、ここでは再調査しません：webhelp の TLS 問題（`curl -k` が必要）、2024-12 の GPL-3.0 オープンソース化がコミット 3 件で停止、Homebrew cask の `fails_gatekeeper_check` による非推奨化（2026-09-01 に無効化）、リリースノートが 4.2.0（2013-10）で停止。

## 実機確認チェックリスト

ライセンス済み／インストール済みのビルドで実行してください。各項目は 1 つの観察可能な確認です。結果にはビルド／バージョンを併記してください。

### Araxis Merge（ライセンスが必要。Windows 2026.1 と現行 macOS）

- [ ] 引数なしで起動し、空の比較タブが開くこと（開始／ウェルカム画面が無いこと）と、どの比較種別が出るかを確認する。
- [ ] 実寸を記録する：ペイン幅、センターポイントマーカー、ペインごとのパスラベル。
- [ ] 読み取り専用編集トグルの位置を特定し、有効化時に読み取り専用ファイルをその場で編集できるか確認する。
- [ ] 大ファイルでの編集→再計算の遅延を確認し、報告されているハングの再現を試みる。
- [ ] インライン編集とマージボタン操作が一緒に undo されるか（同一スタックか）を確認する。
- [ ] macOS のツールバー UI と Windows のリボンを比較する。

### WinXMerge（v0.51.0 デスクトップをビルド。ブラウザで <https://winxmerge.app> を開く）

- [ ] デスクトップ：ファイル選択ダイアログ、ツールバー、ステータスバー、detail ペイン、location pane（ミニマップ）を撮る。
- [ ] Web：undo/redo が存在するか、ファイルを開くのがアップロード選択のみかを確認する。
- [ ] Web：大きな貼り付け（例：数 MB）で UI が固まらないか計測し、Worker が無いことを確認する。
- [ ] 既定ショートカットを WinMerge のマニュアル（F7/F8、Shift+F7、Alt+↑/↓、保存、undo、検索）と比較する。
- [ ] Web アプリへのファイルのドラッグ＆ドロップを試す。

### Meld（デスクトップ）

- [ ] 変更バーのコピー／削除が `Ctrl+Z` で戻り、打鍵と変更バーの混在編集が妥当な単一シーケンスになることを確認する。
- [ ] undo から除外される変更バー操作が無いか確認する。

### KDiff3（現行 KDE ビルド）

- [ ] A/B/C 入力ペインが本文編集を拒否することを確認する。
- [ ] 現行リリースでマージ出力に undo が無いこと（FAQ 5.7）を確認する。
- [ ] 出力を編集して diff が再実行されないこと（想定：されない）を確認する。

### P4Merge（現行 Helix Core ビルド）

- [ ] ミニマップ／オーバービューの有無を確認する（既定の想定：無し）。
- [ ] undo/redo と、結果編集で diff が再実行されるかを確認する。
- [ ] ペインごとのパス／ファイル名ラベルを記録する。
- [ ] 現行ビルドで大ファイルを試す（Qt5 期の挙動）。

### JetBrains IDE（現行リリース）

- [ ] ビューア内リビジョンドロップダウンが無いこと（IJPL-241701 参照）を確認し、トラッカーの状態を再確認する。
- [ ] コンフリクト解決 UI に数値の `N 個中 n 個` カウンタが無いことを確認する。
- [ ] `Ctrl+Z`/`Ctrl+Shift+Z` でコンフリクトが同期することを確認する。

### Kaleidoscope 7（サブスクリプションが必要）

- [ ] スクロールバーのマークが目盛りかミニマップかを確認する。
- [ ] 変更カウンタの表示形式と、マップビューを有効化できるかを記録する。
