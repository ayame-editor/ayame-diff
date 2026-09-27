package server

import (
	"strings"
	"testing"
)

// TestHunkToolbarAssetsAreWired guards the browser half of #293. The action set
// is executed under node --test; what this checks is that the page loads the
// module, that the hunk's actions moved into a toolbar anchored to the hunk,
// and that the toolbar is reachable by more than a pointer hover — keyboard
// focus and touch both reach it. It also pins the controls that must not be
// lost while the always-visible ones are reduced.
func TestHunkToolbarAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "hunkactions.js")
	style := readWebAsset(t, "style.css")

	if !strings.Contains(index, `<script src="hunkactions.js"></script>`) {
		t.Error("index.html does not load hunkactions.js")
	}
	if strings.Index(index, `src="hunkactions.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("hunkactions.js must load before app.js")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "addEventListener") {
		t.Error("hunkactions.js touches the DOM; it must stay runnable without one")
	}
	if !strings.Contains(module, "root.AyameHunkActions = api") || !strings.Contains(module, "module.exports = api") {
		t.Error("hunkactions.js does not publish AyameHunkActions")
	}

	for _, want := range []string{
		"globalThis.AyameHunkActions",
		"function setHunkToolbarOpen(",
		".hunk-toolbar-toggle",
		`"hunk-toolbar"`,
		"hunkActions(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing hunk-toolbar wiring %q", want)
		}
	}

	// The actions belong to the hunk that owns them, not to its head alone.
	body := renderFunctionBody(t, app, "function renderHunk(")
	if !strings.Contains(body, "hunkActions(") {
		t.Error("renderHunk does not build its action toolbar through the tested module")
	}
	if !strings.Contains(body, `toolbar.className = "hunk-toolbar"`) {
		t.Error("renderHunk does not anchor a toolbar to the hunk")
	}
	if !strings.Contains(body, "setHunkToolbarOpen(box") {
		t.Error("the hunk actions handle does not open the toolbar")
	}
	if strings.Contains(body, `mergeActions.className = "hunk-merge"`) {
		t.Error("renderHunk still renders the old always-visible merge actions inline")
	}

	// Hover is one trigger among several. A toolbar that only appeared on
	// hover would be unusable with a keyboard or a finger.
	if !strings.Contains(style, ".hunk:focus-within .hunk-toolbar") {
		t.Error("keyboard focus does not reveal the hunk toolbar")
	}
	if !strings.Contains(style, ".hunk.toolbar-open .hunk-toolbar") {
		t.Error("an explicitly opened hunk toolbar is not shown")
	}
	if !strings.Contains(style, "@media (hover: none)") {
		t.Error("a device without hover has no fallback for the hunk toolbar")
	}
	for _, want := range []string{".hunk-toolbar-toggle", ".hunk-toolbar {", ".hunk-toolbar .hunk-adopt"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}
	// The handle is the visible affordance that keeps the toolbar discoverable.
	if !strings.Contains(app, `toggle.className = "hunk-toolbar-toggle"`) {
		t.Error("the toolbar has no always-visible handle")
	}
	if !strings.Contains(app, `toggle.setAttribute("aria-expanded"`) {
		t.Error("the toolbar handle does not report its expanded state")
	}
}

// The bulk controls are the other half of the operation model: per-hunk actions
// moved to the hunk, but the session-wide ones must stay where they are.
func TestHunkToolbarKeepsBulkAndNavigationControls(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	for _, want := range []string{
		`id="mergePanel"`, `id="allLeft"`, `id="allRight"`, `id="allBase"`,
		`id="mergeUndo"`, `id="mergeRedo"`, `id="diffNav"`, `id="addSync"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html no longer has %q", want)
		}
	}
	for _, want := range []string{
		`$("allLeft").addEventListener("click"`,
		`$("allRight").addEventListener("click"`,
		`$("allBase").addEventListener("click"`,
		`$("mergeUndo").addEventListener("click"`,
		`$("mergeRedo").addEventListener("click"`,
		`$("addSync").addEventListener("click"`,
		`$("nextDiff").addEventListener`, `$("prevDiff").addEventListener`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js no longer wires %q", want)
		}
	}
	// Adopt buttons still appear only in merge mode.
	if !strings.Contains(app, "function chooseMerge(index, side)") {
		t.Error("the hunk toolbar no longer reaches the merge choice")
	}
}
