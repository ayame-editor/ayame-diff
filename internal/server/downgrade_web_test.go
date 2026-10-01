package server

import (
	"strings"
	"testing"
)

// TestDowngradedDifferencesAreWiredIntoTheUI covers the #269 wiring: the page
// loads the pure classifier, app.js consumes it for navigation and the summary,
// and the hunk renderer carries the dismissed state.
func TestDowngradedDifferencesAreWiredIntoTheUI(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	if !strings.Contains(index, `<script src="downgrade.js"></script>`) {
		t.Error("index.html does not load downgrade.js, so the classifier the node test covers is not the one the page uses")
	}
	if strings.Index(index, `src="downgrade.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("downgrade.js loads after app.js, which consumes it")
	}

	module := readWebAsset(t, "downgrade.js")
	if !strings.Contains(module, "module.exports = api") {
		t.Error("downgrade.js has no CommonJS export, so node --test cannot require it")
	}
	for _, leaked := range []string{"document.", "addEventListener", "localStorage", "lastData", "$("} {
		if strings.Contains(module, leaked) {
			t.Errorf("downgrade.js contains application state or wiring (%q); it must stay pure", leaked)
		}
	}

	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "globalThis.AyameDowngrade") {
		t.Error("app.js does not consume the extracted classifier")
	}
	nav := renderFunctionBody(t, app, "function activeHunkIndexes(")
	if !strings.Contains(nav, "navigableIndexes(") {
		t.Error("difference navigation does not route through the shared classifier, so it may count a dismissed difference")
	}
	summary := renderFunctionBody(t, app, "function renderSummary(")
	if !strings.Contains(summary, "res.downgraded_hunks") {
		t.Error("the result summary does not report the dismissed count separately")
	}
	hunk := renderFunctionBody(t, app, "function renderHunk(")
	if !strings.Contains(hunk, "downgraded") {
		t.Error("the hunk renderer does not carry the downgraded state")
	}
	// The patch request's ignore list indexes a result without dismissed
	// hunks, so the rendered indexes must be remapped before sending them.
	if !strings.Contains(app, "essentialIndexes(lastData?.hunks") {
		t.Error("patch export sends rendered hunk indexes without mapping them onto the real-only list")
	}
	// The text merge API has the same real-only index space.
	if !strings.Contains(app, "essentialChoices(mergeSelection.toWire()") {
		t.Error("text merge sends rendered hunk choices without mapping them onto the real-only list")
	}

	style := readWebAsset(t, "style.css")
	if !strings.Contains(style, ".hunk.downgraded") {
		t.Error("style.css has no downgraded hunk treatment, so a dismissed difference would look like a real one")
	}

	i18n := readWebAsset(t, "i18n.js")
	for _, key := range []string{"downgraded:", "downgradedCount:", "downgradedHint:"} {
		if strings.Count(i18n, key) < 2 {
			t.Errorf("i18n key %q is not defined in both the ja and en tables", key)
		}
	}
}
