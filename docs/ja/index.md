<div class="doc-family">
  <span>巨大なファイルを開いて編集したいですか？</span>
  <a href="https://hjosugi.github.io/ayame-editor/ja/">Ayame Editor へ移動 →</a>
</div>

<section class="doc-hero">
  <div class="doc-hero-copy">
    <p class="doc-eyebrow">ayame-diff Docs</p>
    <h1>ayame-diff</h1>
    <p>巨大なファイルでも、変更点がすぐ分かる。テキスト、CSV / TSV、フォルダ、アーカイブ、3-way を比較します。</p>
    <div class="doc-actions">
      <a class="doc-action doc-action-primary" href="../gui.ja/">GUI を使う</a>
      <a class="doc-action" href="../usage.ja/">CLI の使い方</a>
    </div>
  </div>
  <figure class="doc-preview">
    <img src="../assets/screenshot-folder.png" alt="ayame-diff のフォルダ比較結果" loading="eager">
    <figcaption>追加・削除・変更されたファイルを、フォルダ比較で一覧できます。</figcaption>
  </figure>
</section>

## 比較方法を選ぶ

<div class="doc-card-grid">
  <a class="doc-card" href="../gui.ja/">
    <strong>画面でファイルを比較</strong>
    <span>ローカル Web UI で2つのファイルを選び、差分を移動してパッチを書き出します。</span>
  </a>
  <a class="doc-card" href="../usage.ja/">
    <strong>テキスト・構造化データ</strong>
    <span>行、ソート済みテキスト、CSV / TSV のキー付き行を CLI から比較します。</span>
  </a>
  <a class="doc-card" href="../shell-integration.ja/">
    <strong>フォルダ・アーカイブ</strong>
    <span>ディレクトリツリーや圧縮入力を比較し、ファイルマネージャーからも起動できます。</span>
  </a>
  <a class="doc-card" href="../three-way.ja/">
    <strong>3-way マージ</strong>
    <span>BASE / LEFT / RIGHT を確認し、競合を解決してマージ結果を保存します。</span>
  </a>
</div>

## クイックスタート

[最新リリース](https://github.com/hjosugi/ayame-diff/releases/latest)から OS に合うバイナリをインストールし、目的に合う最短のコマンドを実行します。

```bash
ayame-diff gui                              # ブラウザでファイルを選ぶ
ayame-diff text old.txt new.txt             # テキストを比較
ayame-diff csv --left old.csv --right new.csv --key id --out diff.tsv
ayame-diff dir old-folder new-folder        # フォルダを比較
```

空白・大文字小文字・行フィルター・単語ハイライト・再同期は[比較オプション](../comparison-options.ja.md)を参照してください。繰り返す処理は設定全体を[比較プロジェクト](../projects.ja.md)として保存できます。

<div class="doc-link-row">
  <a href="https://github.com/hjosugi/ayame-diff">GitHub</a>
  <a href="https://github.com/hjosugi/ayame-diff/releases/latest">最新リリース</a>
  <a href="https://github.com/hjosugi/ayame-diff/blob/main/CHANGELOG.md">変更履歴</a>
  <a href="https://github.com/hjosugi/ayame-diff/blob/main/CONTRIBUTING.md">コントリビューション</a>
</div>
