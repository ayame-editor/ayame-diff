package server

import (
	"strings"
	"testing"
)

// TestComparisonURLStateIsWired keeps the execution-tested URL codec connected
// to the browser entry point. The security-sensitive distinction is explicit:
// history URLs retain the launch token for reloads, while copied URLs delete it.
func TestComparisonURLStateIsWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "urlstate.js")

	if !strings.Contains(index, `<script src="urlstate.js"></script>`) {
		t.Error("index.html does not load urlstate.js")
	}
	if strings.Index(index, `src="urlstate.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("urlstate.js must load before app.js")
	}
	if !strings.Contains(index, `id="copyComparisonURL"`) {
		t.Error("the result toolbar has no comparison-link action")
	}

	for _, want := range []string{
		"globalThis.AyameURLState",
		"function captureComparisonState(",
		"function applyComparisonState(",
		`history.pushState(metadata, "", next)`,
		`history.replaceState(metadata, "", next)`,
		`compare({ urlHistory: "none" })`,
		`compare({ urlHistory: "replace" })`,
		`window.addEventListener("popstate"`,
		`buildShareURL(location.href, state)`,
		`askConfirm(t("shareURLWarning"))`,
		`navigator.clipboard?.writeText`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing URL-state wiring %q", want)
		}
	}
	for _, want := range []string{
		`const HASH_KEY = "compare"`,
		`const MAX_ENCODED_LENGTH = 32 * 1024`,
		`url.searchParams.delete("token")`,
		"function encodeComparisonState(",
		"function decodeComparisonState(",
		"function buildShareURL(",
		"module.exports = api",
	} {
		if !strings.Contains(module, want) {
			t.Errorf("urlstate.js is missing %q", want)
		}
	}
}
