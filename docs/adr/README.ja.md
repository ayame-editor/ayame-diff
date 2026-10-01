<!-- i18n: language-switcher -->
[English](README.md) | [日本語](README.ja.md)

# アーキテクチャ決定記録

方針決定の記録。番号は連番、ステータスは Proposed / Accepted / Superseded。

| # | タイトル | ステータス | Issue |
| --- | --- | --- | --- |
| [0001](0001-naming-unification.ja.md) | プロジェクト名の統一（`fcsv-diff` → `ayame-diff`） | Accepted | #3 |
| [0002](0002-diff-acceptance-architecture.ja.md) | diff / sortdiff 受け入れアーキテクチャ | Accepted | #4 |
| [0003](0003-encoding-dependency.ja.md) | 文字コード対応と依存性の例外（x/text） | Accepted | #9 |
| [0004](0004-git-repository-boundary.ja.md) | Git リポジトリ管理を ayame-diff の外に保つ | Accepted | #290 |
| [0005](0005-no-embedded-sql-engine.ja.md) | 常時埋め込みの SQL エンジン無しで比較前整形を行う | Proposed | #126 |
| [0006](0006-syntax-highlighting-treesitter.ja.md) | シンタックスハイライト: 現状は複数行対応の近似、tree-sitter は保留 | Accepted | #287 |

姉妹プロジェクト ayame-editor 側の切り出し方針: hjosugi/ayame-editor#104
