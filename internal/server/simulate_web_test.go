package server

import (
	"strings"
	"testing"
)

// TestSimulateImpactAssetsAreWired guards the browser half of #273. The impact
// plan itself is executed under node --test; what this checks is that the page
// loads the tested module before app.js, that the Simulate affordance and the
// list container exist, and that the pure helper stays free of the DOM.
func TestSimulateImpactAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "simulate.js")
	style := readWebAsset(t, "style.css")

	if !strings.Contains(index, `<script src="simulate.js"></script>`) {
		t.Fatal("index.html does not load simulate.js")
	}
	if strings.Index(index, `src="simulate.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("simulate.js must load before app.js, which consumes it")
	}
	for _, id := range []string{`id="simulateMerge"`, `id="confirmDetails"`} {
		if !strings.Contains(index, id) {
			t.Errorf("index.html is missing %s", id)
		}
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "fetch(") {
		t.Error("simulate.js touches the browser; it must stay runnable without one")
	}
	for _, want := range []string{"module.exports = api", "function mergeImpact(", "root.AyameSimulate = api"} {
		if !strings.Contains(module, want) {
			t.Errorf("simulate.js is missing %q", want)
		}
	}
	for _, want := range []string{
		"globalThis.AyameSimulate.mergeImpact(",
		"function buildMergeImpact(",
		"function mergeImpactRows(",
		"function previewMergeImpact(",
		`$("simulateMerge").addEventListener("click"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing impact-preview wiring %q", want)
		}
	}
	for _, want := range []string{".confirm-details", ".impact-overwrite"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}

	// Both languages must carry the new strings (the parity test enforces the
	// same key set; this pins the exact keys the feature depends on).
	catalog := readWebAsset(t, "i18n.js")
	for _, want := range []string{"simulateMerge:", "impactLead:", "impactWrite:", "impactOverwrite:"} {
		if strings.Count(catalog, want) < 2 {
			t.Errorf("i18n.js defines %q in fewer than both languages", want)
		}
	}
}

// TestMergeSavesShowImpactBeforeOverwrite is the behavior guard: a save that
// may overwrite an input must ask through the impact list, not a bare warning.
func TestMergeSavesShowImpactBeforeOverwrite(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	for _, header := range []string{
		"async function saveTextMerge(",
		"async function saveCSVMerge(",
		"async function saveThreeWayMerge(",
	} {
		body := renderFunctionBody(t, app, header)
		if !strings.Contains(body, "mergeImpactRows(impact)") || !strings.Contains(body, "mergeImpactPrompt(impact)") {
			t.Errorf("%s does not preview the impact list before overwriting", header)
		}
		if strings.Contains(body, `askConfirm(t("overwriteWarning"))`) {
			t.Errorf("%s still overwrites on a bare warning without the list", header)
		}
		// Existing safety must survive: the request still carries both flags.
		if !strings.Contains(body, "confirmOverwrite") || !strings.Contains(body, "overwrite") {
			t.Errorf("%s dropped the overwrite confirmation fields", header)
		}
	}
}

// TestAskConfirmRendersImpactDetails keeps the two-step affordance honest: the
// list has to be rendered inside the existing confirm dialog, and a plain
// confirmation has to clear it so an earlier preview cannot leak into a later
// question.
func TestAskConfirmRendersImpactDetails(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	body := renderFunctionBody(t, app, "function askConfirm(")

	if !strings.Contains(body, `$("confirmDetails")`) {
		t.Error("askConfirm does not render the impact list")
	}
	if !strings.Contains(body, "list.textContent = \"\"") {
		t.Error("askConfirm does not clear a previous impact list")
	}
	if !strings.Contains(body, "details") {
		t.Error("askConfirm accepts no details argument")
	}
}
