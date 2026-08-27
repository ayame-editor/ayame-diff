package server

import (
	"strings"
	"testing"
)

// TestUnchangedContextIsWiredIntoTheTextDiff covers the DOM/API boundary of
// #267. The range geometry itself runs under node --test and the API has handler
// tests; this keeps either half from being left unused by a later renderer
// rewrite.
func TestUnchangedContextIsWiredIntoTheTextDiff(t *testing.T) {
	index := readWebAsset(t, "index.html")
	for _, want := range []string{
		`id="contextToggle"`,
		`id="contextLines" type="number" value="3" min="0" max="50"`,
		`<script src="unchanged.js"></script>`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	if strings.Index(index, `src="unchanged.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("unchanged.js loads after app.js, which consumes it")
	}

	module := readWebAsset(t, "unchanged.js")
	for _, want := range []string{
		"module.exports = api",
		"function buildUnchangedRegions(",
		"function initialContextRanges(",
		"function missingContextSpans(",
		"function batchContextRanges(",
	} {
		if !strings.Contains(module, want) {
			t.Errorf("unchanged.js missing %q", want)
		}
	}
	for _, forbidden := range []string{"document.", "fetch(", "localStorage"} {
		if strings.Contains(module, forbidden) {
			t.Errorf("unchanged.js contains browser state %q; node must exercise the same pure geometry", forbidden)
		}
	}

	app := readWebAsset(t, "app.js")
	for _, want := range []string{
		"globalThis.AyameUnchanged",
		`apiFetch("/api/diff/context"`,
		"async function expandContextSpan(",
		"CONTEXT_EXPAND_CHUNK = 20",
		"buildUnchangedRegions(data?.hunks",
		"!data?.omitted_hunks",
		"renderContextRegionsSliced(",
		`node.closest(".context-region .cell.context-duplicate")`,
		`localStorage.setItem("ayame-context-lines"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	if strings.Contains(app, "function buildUnchangedRegions(") {
		t.Error("app.js duplicates the geometry instead of using the node-tested module")
	}
}
