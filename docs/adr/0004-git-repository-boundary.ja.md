<!-- i18n: language-switcher -->
[English](0004-git-repository-boundary.md) | [日本語](0004-git-repository-boundary.ja.md)

# ADR 0004: Git リポジトリ管理を ayame-diff の外に保つ

- ステータス: Accepted（2026-07-30）
- 関連 Issue: hjosugi/ayame-diff#290
- 関連作業: #280（ローカルアーキテクチャ）、#295（外部ツール呼び出し）

## 背景

ayame-diff の主な対象は、同じ version control system で管理されていないファイルの
比較です。repository を認識すると、object と revision の検索、index 状態、branch、
remote、認証、ignore rule、stage、commit という第2の製品スコープを抱えます。
この領域には成熟した editor と Git client が既にあります。

一方、それらの操作 pattern には Git の外でも有用なものがあります。VS Code Source
Control は機能境界ではなく、明示的な設計参考です。folder 比較には変更一覧と連続
multi-file view（#291）、直接編集には gutter の変更 marker（#292）と hunk 付近の
操作（#293）、一時入力には意味のある論理 label（#295）が役立ちます。

Git はファイルを実体化した後、外部 diff / merge tool を呼べます。この向きでは Git が
repository の意味を所有し、ayame-diff は通常の path だけを受け取ります。repository
管理を追加せず、製品と安全に合成できます。

## 決定

ayame-diff は Git repository を検査・管理しません。

- `.git` を読まず、`HEAD~1` のような revision の解決、履歴、branch、remote、
  staged / unstaged 状態の表示、stage、commit、fetch、pull、push、認証操作を
  実装しません。
- `.gitignore` を特別扱いしません。folder 比較では引き続き独自の明示 filter と
  project を使います。
- CLI と GUI の比較は、明示 path、貼り付け内容、ayame-diff project file から
  開始します。
- custom `git difftool` / `git mergetool` として呼ばれることは許可します。Git が
  `$LOCAL`、`$REMOTE`、`$BASE`、`$MERGED` を渡し、ayame-diff は repository 状態を
  探索しません。対応する端末設定は
  [ファイルマネージャーとクイック起動](../shell-integration.ja.md)に記載します。
- 一般的な比較作業を改善する Git 非依存の UX pattern は採用できます。multi-file
  result、gutter の変更 marker、hunk 付近の操作、論理 pane label が該当します。

## 影響

- comparison engine は repository 固有の状態や認証なしで、任意ファイルと自動化に
  利用できます。
- repository 操作は Git、editor、専用 Git client が担当します。
- Git object model を必要とする要求は却下するか、明示 path 入力へ置き換えます。
  一般的な比較操作の要求では、引き続き Git client を設計参考にできます。
- 外部ツール改善は依存方向を維持する必要があります。Git が ayame-diff を呼び、
  ayame-diff が Git client になることはありません。

## 却下した案

- **read-only Git browser を組み込む:** 書き込みがなくても revision 解決、
  worktree / index 状態、submodule、認証が大きな継続的互換範囲になります。
- **直接編集後に stage / commit 操作を追加する:** 比較の安全性を repository 変更へ
  結合し、既存 client と機能が重複します。
- **Git 関連 workflow をすべて拒否する:** custom difftool / mergetool 呼び出しでは
  repository の所有権が Git に残るため、この境界と両立します。
