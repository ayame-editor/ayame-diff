package server

import (
	"strings"
	"testing"
)

// TestMultipleComparisonTabsAreWired guards the browser half of #281. The tab
// list arithmetic is executed under node --test; what this checks is that the
// page still loads it, that switching a tab restores its own comparison and
// scroll position, and that the tab set rides along in the URL without
// displacing the single-comparison fragment the rest of the tool depends on.
func TestMultipleComparisonTabsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "tabs.js")
	urlstate := readWebAsset(t, "urlstate.js")
	style := readWebAsset(t, "style.css")

	if !strings.Contains(index, `<script src="tabs.js"></script>`) {
		t.Error("index.html does not load tabs.js")
	}
	if strings.Index(index, `src="tabs.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("tabs.js must load before app.js, which destructures it")
	}
	for _, want := range []string{`id="comparisonTabs"`, `id="tabList"`, `id="newTab"`, `role="tablist"`} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing tab markup %q", want)
		}
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "fetch(") ||
		strings.Contains(module, "localStorage") || strings.Contains(module, "addEventListener") {
		t.Error("tabs.js touches the browser; it must stay runnable without one")
	}
	for _, want := range []string{
		"function emptyDoc(",
		"function activeTab(",
		"function addTab(",
		"function removeTab(",
		"function activateTab(",
		"function updateTab(",
		"function labelFromState(",
		"function serializeDoc(",
		"function parseDoc(",
		"module.exports = api",
	} {
		if !strings.Contains(module, want) {
			t.Errorf("tabs.js is missing %q", want)
		}
	}

	for _, want := range []string{
		"globalThis.AyameTabs",
		"function renderTabs(",
		"function rememberActiveTab(",
		"function stashActiveTab(",
		"function loadActiveTab(",
		"function switchTab(",
		"function openTab(",
		"function closeTab(",
		"$(\"newTab\").addEventListener(\"click\", openTab)",
		"readTabState(location.href)",
		"serializeTabDoc(tabDoc)",
		"buildTabStateURL(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing tab wiring %q", want)
		}
	}

	// Switching a tab must apply that tab's saved state and restore its own
	// scroll, not the one left on screen.
	load := renderFunctionBody(t, app, "async function loadActiveTab(")
	if !strings.Contains(load, "applyComparisonState(active.state)") {
		t.Error("loading a tab does not restore its comparison state")
	}
	if !strings.Contains(load, "scrollAnchor: active.scroll || null") {
		t.Error("loading a tab does not restore its scroll position")
	}
	switchBody := renderFunctionBody(t, app, "async function switchTab(")
	if !strings.Contains(switchBody, "stashActiveTab()") {
		t.Error("switching away does not save the outgoing tab")
	}
	if !strings.Contains(switchBody, "activateTabInDoc(tabDoc, id)") {
		t.Error("switching does not activate the requested tab through the tested model")
	}
	// The comparison itself stays one click away for the active tab.
	if !strings.Contains(app, `$("setupRecompare").addEventListener("click", compare)`) {
		t.Error("Re-compare is no longer wired to the active comparison")
	}

	// The tab set is a sibling fragment key, so the existing comparison key and
	// its history behavior are untouched.
	if !strings.Contains(urlstate, `const TABS_HASH_KEY = "tabs"`) {
		t.Error("urlstate.js has no tab-set fragment key")
	}
	for _, want := range []string{"function readTabState(", "function buildTabStateURL("} {
		if !strings.Contains(urlstate, want) {
			t.Errorf("urlstate.js is missing %q", want)
		}
	}
	if !strings.Contains(urlstate, "params.delete(TABS_HASH_KEY)") {
		t.Error("a shared URL must drop the reader's open tab set")
	}

	for _, want := range []string{".comparison-tabs", ".tab.active", ".tab-close", ".tab-new"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing tab style %q", want)
		}
	}

	// i18n parity covers the new keys; pin both languages here so a rename on
	// one side is caught in the same run as the wiring.
	catalog := readWebCatalog(t)
	for _, want := range []string{
		`comparisonTabs: "比較タブ"`, `comparisonTabs: "Comparison tabs"`,
		`newTab: "新しいタブ"`, `newTab: "New tab"`,
		"closeTab: (v) => `タブを閉じる: ${v.label}`",
		"closeTab: (v) => `Close tab: ${v.label}`",
	} {
		if !strings.Contains(catalog, want) {
			t.Errorf("i18n.js missing tab key %q", want)
		}
	}
}
