package server

import (
	"strings"
	"testing"
)

// TestContinuousViewAssetsAreWired guards the browser half of #291. The
// windowing decisions are executed under node --test; what this checks is that
// the page still loads them, that a file's diff is still fetched only when it
// is near, and that the parts a mistake would quietly remove — the budget, the
// held height, the cross-file navigation — are still there.
func TestContinuousViewAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "continuous.js")
	style := readWebAsset(t, "style.css")

	if !strings.Contains(index, `<script src="continuous.js"></script>`) {
		t.Error("index.html does not load continuous.js")
	}
	if strings.Index(index, `src="continuous.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("continuous.js must load before app.js")
	}
	if !strings.Contains(index, `id="dirContinuous"`) {
		t.Error("index.html has no continuous-view toggle")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "fetch(") {
		t.Error("continuous.js touches the browser; it must stay runnable without one")
	}

	for _, want := range []string{
		"globalThis.AyameContinuous",
		"async function renderContinuous(",
		"async function loadContinuousSection(",
		"function unloadContinuousSection(",
		"function trimContinuousWindow(",
		"async function continuousStep(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing continuous-view wiring %q", want)
		}
	}

	// A change set is read by scrolling, so the work has to be bounded: a file
	// is fetched near the viewport, and files that drift away are released.
	if !strings.Contains(app, "const CONTINUOUS_LOADED_LIMIT") {
		t.Error("the number of files held in the DOM is not bounded")
	}
	if !strings.Contains(app, "unloadTargets([...view.loaded], view.focus, CONTINUOUS_LOADED_LIMIT)") {
		t.Error("the budget is not enforced against what is loaded")
	}
	// Releasing a file above the viewport would pull the page up under the
	// reader unless the space it took is held.
	if !strings.Contains(app, `bodyElement.style.minHeight = `) {
		t.Error("an unloaded file does not hold its height, so the scroll will jump")
	}
	// A hidden tab never runs an animation frame, which would wedge the tracker.
	if strings.Contains(app, "requestAnimationFrame(() => {\n    view.frame = 0;") {
		t.Error("the scroll tracker depends on an animation frame")
	}

	body := renderFunctionBody(t, app, "async function continuousStep(")
	if !strings.Contains(body, "stepSection(view.focus, direction, view.entries.length)") {
		t.Error("navigation does not cross file boundaries through the tested step")
	}
	if !strings.Contains(body, "view.sections[view.focus]") {
		t.Error("navigation walks the DOM rather than the change set, so unloaded files would be skipped")
	}

	for _, want := range []string{".file-diff-head", ".file-diff-body", ".file-diff.collapsed", "--pane-heads-height"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}
}

// The continuous view and the tree are two ways to read one folder result, so
// each has to leave the other in a usable state.
func TestContinuousViewAndTreeReplaceEachOther(t *testing.T) {
	t.Parallel()

	app := readWebAsset(t, "app.js")
	tree := renderFunctionBody(t, app, "async function renderDirectory(")
	if !strings.Contains(tree, "teardownContinuousView()") {
		t.Error("rendering the tree does not end a continuous view")
	}
	open := renderFunctionBody(t, app, "async function openFromFolder(")
	if !strings.Contains(open, "teardownContinuousView()") {
		t.Error("opening a single file does not end a continuous view")
	}
	if !strings.Contains(app, "if (continuousActive()) { void continuousStep(1); return; }") {
		t.Error("the difference navigation does not route through the continuous view")
	}
}
