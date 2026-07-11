<!-- i18n: language-switcher -->
[English](README_WINDOWS.md) | [日本語](README_WINDOWS.ja.md)

# ayame-diff for Windows

`ayame-diff.exe` is a native Windows binary; Go, Python, WSL, Java, and extra
DLLs are not required.

- x64: `ayame-diff.exe` in the ZIP root
- ARM64: `arm64\ayame-diff.exe`
- GUI: double-click `start-gui.cmd`, or run `ayame-diff.exe gui`
- CLI check: `ayame-diff.exe --version` and `ayame-diff.exe --help`

Complete installation, comparison, encoding, and large-file tuning guidance is
maintained in one place:

- Documentation: <https://hjosugi.github.io/ayame-diff/>
- Japanese README: <https://github.com/hjosugi/ayame-diff/blob/main/README.ja.md>

The distributed executables are not code-signed, so Windows SmartScreen or an
organization policy may show a warning on first launch. Verify the archive with
the release's `SHA256SUMS` file when required.
