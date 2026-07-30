## Summary

- Describe the user-visible change and the reason for it.

## Validation

- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go vet ./...`
- [ ] CLIや出力形式を変更した場合、READMEとCHANGELOGを更新した

## Compatibility

- [ ] Windows
- [ ] Linux
- [ ] macOS
- [ ] 該当なし

<!-- ui-regression-checklist -->
## GUI regression checklist

[GUI変更の退行防止チェックリスト](https://github.com/hjosugi/ayame-diff/blob/main/docs/ui-regression-checklist.ja.md)を
GUIに影響する変更で使用し、該当しない場合は最初の項目を選んでください。

- [ ] GUIの動作・配置に影響しない
- [ ] Compare/再比較（直接編集導入後は保存も）が1クリックで到達できる
- [ ] 影響を受ける常用作業の変更前後クリック数を記録し、増えていない
- [ ] 変更・追加した操作をキーボードだけで完了できる
- [ ] 削減したchromeが空白ではなく結果領域を広げている
- [ ] 100% / 200%スケールでアイコン、重なり、フォーカス表示を確認した
- [ ] 移動した操作の到達経路と、自動化できる不変条件のテストを更新した

Evidence:

- Click counts:
- Keyboard path:
- Viewport / scale:
- Screenshots or recording:
