<!-- i18n: language-switcher -->
[English](README_WINDOWS.md) | [日本語](README_WINDOWS.ja.md)

# ayame-diff for Windows

`ayame-diff.exe`はネイティブなWindowsバイナリです。Go、Python、WSL、Java、追加のDLLは必要ありません。

- x64: ZIPのルートに`ayame-diff.exe`
- ARM64: `arm64\ayame-diff.exe`
- GUI: `start-gui.cmd`をダブルクリックするか、`ayame-diff.exe gui`を実行
- CLI確認: `ayame-diff.exe --version`および`ayame-diff.exe --help`

インストール、比較、エンコーディング、大容量ファイルの調整に関する完全なガイドは一箇所にまとめられています。

- ドキュメント: <https://ayame-editor.github.io/ayame-diff/>
- 日本語README: <https://github.com/ayame-editor/ayame-diff/blob/main/README.ja.md>

配布されている実行ファイルはコード署名されていないため、WindowsのSmartScreenや組織のポリシーによって初回起動時に警告が表示される場合があります。必要に応じてリリースの`SHA256SUMS`ファイルでアーカイブの整合性を確認してください。