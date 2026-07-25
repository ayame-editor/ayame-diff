package server

import (
	"strings"
	"testing"
)

// TestFileWatchAssetsAreWired keeps the execution-tested long-poll controller,
// authenticated API, and comparison entry point connected. The scroll anchor is
// intentionally reused through compare(): an external save must follow the same
// restoration path as a manual re-comparison (#249, #251).
func TestFileWatchAssetsAreWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "filewatch.js")

	if !strings.Contains(index, `<script src="filewatch.js"></script>`) {
		t.Error("index.html does not load filewatch.js")
	}
	if strings.Index(index, `src="filewatch.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("filewatch.js must load before app.js")
	}
	for _, want := range []string{
		`id="autoReload"`,
		`id="externalChangeBar"`,
		`id="externalReload"`,
		`id="externalKeep"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("file watch UI is missing %q", want)
		}
	}
	for _, want := range []string{
		"globalThis.AyameFileWatch",
		`apiFetch("/api/watch"`,
		"createLongPollWatcher(",
		"EXTERNAL_CHANGE_DEBOUNCE_MS",
		"hasUnsavedResultChanges()",
		`compare({ watch: prepared, external: true })`,
		"captureResultScrollAnchor()",
		"restoreResultScrollAnchor(scrollAnchor",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing file watch wiring %q", want)
		}
	}
	for _, want := range []string{
		"function watchPathsForMode(",
		"function createLongPollWatcher(",
		"module.exports = api",
	} {
		if !strings.Contains(module, want) {
			t.Errorf("filewatch.js is missing %q", want)
		}
	}
}
