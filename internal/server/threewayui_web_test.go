package server

import (
	"strings"
	"testing"
)

// TestThreeWayLayoutAssetsAreWired guards the browser half of #282. The layout
// and resolution decisions are executed under node --test; what this checks is
// that the page still loads them and that the placement the issue asked for has
// not quietly regressed back to a permanent BASE column or a path input the
// result depends on.
func TestThreeWayLayoutAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "threewayview.js")
	style := readWebAsset(t, "style.css")

	if !strings.Contains(index, `<script src="threewayview.js"></script>`) {
		t.Error("index.html does not load threewayview.js")
	}
	if strings.Index(index, `src="threewayview.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("threewayview.js must load before app.js")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "$(") {
		t.Error("threewayview.js touches the DOM; it must stay runnable without one")
	}
	if !strings.Contains(module, "module.exports = api") {
		t.Error("threewayview.js has no CommonJS export, so node --test cannot require it")
	}

	for _, want := range []string{
		"globalThis.AyameThreeWayView",
		"threeWayPanes(",
		"threeWayResultLines(",
		"function renderThreeWayPane(",
		`role === "result"`,
		"function setThreeWayBase(",
		"function askMergeOutput(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing three-way layout wiring %q", want)
		}
	}

	// BASE must be on demand: the toggle lives beside the three-way controls,
	// and the ancestor column is only shown once it is pressed.
	for _, want := range []string{`id="toggleBase"`, `id="mergeSaveDialog"`, `id="mergeOutput"`} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html is missing %q", want)
		}
	}
	if !strings.Contains(app, `$("result").classList.toggle("show-base", threeWayShowBase)`) {
		t.Error("the BASE column is not toggled on the result")
	}
	for _, want := range []string{".result.show-base .three-grid", ".three-pane.result", ".three-pane.base"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css is missing %q", want)
		}
	}

	// The result pane must not depend on the save path input. The path moved
	// into a save dialog, so it is no longer part of the merge panel.
	panel := sectionBetween(t, index, `<section class="merge-panel"`, `</section>`)
	if strings.Contains(panel, `id="mergeOutput"`) {
		t.Error("the merge output path is still part of the merge panel")
	}
	if !strings.Contains(index, `id="mergeSaveDialog"`) {
		t.Error("there is no save dialog for the output path")
	}
}
