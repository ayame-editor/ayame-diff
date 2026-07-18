package server

import (
	"strings"
	"testing"
)

// TestInlineWordDiffUsesAFlatReusedTable is the #155 regression. The LCS table
// was an array of Uint16Arrays, so every call allocated one array per token and
// every read cost an array-of-arrays double dereference. One flat buffer,
// reused across calls, measured 1.64x faster on worst-case sized pairs.
func TestInlineWordDiffUsesAFlatReusedTable(t *testing.T) {
	t.Parallel()
	// The algorithm moved into its own module so it could be executed in tests
	// (#139); these structural checks follow it there.
	app := readWebAsset(t, "worddiff.js")

	body := renderFunctionBody(t, app, "function inlineWordDiff(")
	if strings.Contains(body, "Array.from({ length: m + 1 }") {
		t.Error("the DP table is still an array of typed arrays, one per token")
	}
	if !strings.Contains(body, "inlineDPBuffer(") {
		t.Error("inlineWordDiff does not use the shared buffer")
	}
	if !strings.Contains(app, "let inlineDPScratch") {
		t.Fatal("no reusable DP buffer")
	}
	// The algorithm reads cells it has not written (the last row and column must
	// be zero), so a reused buffer has to be cleared.
	buffer := renderFunctionBody(t, app, "function inlineDPBuffer(")
	if !strings.Contains(buffer, ".fill(0") {
		t.Error("the reused buffer is not cleared, so a previous call's values would leak into the next")
	}
	if !strings.Contains(buffer, "inlineDPScratch.length < size") {
		t.Error("the buffer is not grown on demand")
	}
}

// TestResizeIsThrottled covers the last #155 item: resize fires continuously
// while a window is dragged and each call forces a layout read, yet it was the
// one handler without the throttle its scroll counterpart already had.
func TestResizeIsThrottled(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	if strings.Contains(app, `window.addEventListener("resize", updateMinimapViewport)`) {
		t.Error("resize still calls the layout-reading handler directly on every event")
	}
	start := strings.Index(app, `window.addEventListener("resize"`)
	if start < 0 {
		t.Fatal("no resize handler")
	}
	handler := app[start : start+240]
	if !strings.Contains(handler, "requestAnimationFrame") || !strings.Contains(handler, "viewportFrame") {
		t.Error("the resize handler does not reuse the frame throttle the scroll handler uses")
	}
}
