package server

import (
	"strings"
	"testing"
)

// TestScrollAnchorAssetsAreWired guards the browser integration around the
// execution-tested scroll anchor module. A pure algorithm is not useful if the
// page stops loading it or a renderer stops marking logical rows (#249).
func TestScrollAnchorAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "scrollanchor.js")

	if !strings.Contains(index, `<script src="scrollanchor.js"></script>`) {
		t.Error("index.html does not load scrollanchor.js")
	}
	if strings.Index(index, `src="scrollanchor.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("scrollanchor.js must load before app.js")
	}
	for _, want := range []string{
		"globalThis.AyameScrollAnchor",
		"captureResultScrollAnchor()",
		"restoreResultScrollAnchor(scrollAnchor)",
		"dataset.scrollAnchor",
		"dataset.scrollKey",
		"dataset.scrollOrder",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing scroll preservation wiring %q", want)
		}
	}
	for _, function := range []string{
		"async function compare(",
		"function applyWrap(",
		"function applyViewMode(",
		"async function rerenderForDisplayChange(",
	} {
		body := renderFunctionBody(t, app, function)
		if !strings.Contains(body, "captureResultScrollAnchor()") ||
			!strings.Contains(body, "restoreResultScrollAnchor(scrollAnchor") {
			t.Errorf("%s does not preserve the logical result position", function)
		}
	}
	restore := renderFunctionBody(t, app, "function restoreResultScrollAnchor(")
	for _, want := range []string{"announceFailure", "scrollRestoreUnavailable", "setStatus("} {
		if !strings.Contains(restore, want) {
			t.Errorf("failed restoration is not reported (%q missing)", want)
		}
	}
	for _, want := range []string{"captureScrollAnchor", "findScrollAnchor", "restoreScrollAnchor", "module.exports = api"} {
		if !strings.Contains(module, want) {
			t.Errorf("scrollanchor.js is missing %q", want)
		}
	}
}
