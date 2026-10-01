package server

import (
	"strings"
	"testing"
)

// TestTruncationComputeMoreIsWired covers the browser half of #261. The
// arithmetic that picks the next cap runs under node --test; this keeps the
// page loading it, the notice offering the action, and the action re-running
// through compare() (which owns the scroll anchor) from drifting apart.
func TestTruncationComputeMoreIsWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "truncation.js")
	catalog := readWebCatalog(t)

	if !strings.Contains(index, `<script src="truncation.js"></script>`) {
		t.Error("index.html does not load truncation.js")
	}
	if strings.Index(index, `src="truncation.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("truncation.js must load before app.js")
	}
	// The manual control stays reachable (completion condition 4): the action
	// writes to it and a person can still set it directly.
	if !strings.Contains(index, `id="maxHunks"`) {
		t.Error("index.html no longer offers the max hunks control")
	}

	if !strings.Contains(module, "module.exports = api") {
		t.Error("truncation.js is not a testable UMD module")
	}
	if !strings.Contains(module, "function nextMaxHunks(") {
		t.Error("truncation.js does not expose nextMaxHunks")
	}
	for _, forbidden := range []string{"document.", "fetch(", "localStorage"} {
		if strings.Contains(module, forbidden) {
			t.Errorf("truncation.js contains browser state %q; node must exercise the same arithmetic", forbidden)
		}
	}

	for _, want := range []string{
		"globalThis.AyameTruncation",
		"nextMaxHunks(used, res.omitted_hunks)",
		`$("maxHunks").value = String(next)`,
		"void compare()",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing truncation wiring %q", want)
		}
	}
	if strings.Contains(app, "function nextMaxHunks(") {
		t.Error("app.js duplicates the cap arithmetic instead of using the node-tested module")
	}

	button := renderFunctionBody(t, app, "function computeMoreButton(")
	for _, want := range []string{
		`t("computeMoreHunks")`,
		"nextMaxHunks(used, res.omitted_hunks)",
		`$("maxHunks").value = String(next)`,
		"void compare()",
	} {
		if !strings.Contains(button, want) {
			t.Errorf("computeMoreButton missing %q", want)
		}
	}
	summary := renderFunctionBody(t, app, "function renderSummary(res) {")
	if !strings.Contains(summary, "computeMoreButton(res)") {
		t.Error("the truncation notice does not offer the compute-more action")
	}
	if !strings.Contains(summary, `t("omitted"`) {
		t.Error("the truncation notice no longer states what happened")
	}

	// Both languages must name the action, and the notice must stop asking the
	// reader for a number of their own.
	if got := strings.Count(catalog, "computeMoreHunks:"); got < 2 {
		t.Errorf("computeMoreHunks is defined %d time(s); ja and en both need it", got)
	}
	for _, gone := range []string{"raise max hunks", "最大ハンク数を上げてください"} {
		if strings.Contains(catalog, gone) {
			t.Errorf("the catalog still tells the reader to %q", gone)
		}
	}
}
