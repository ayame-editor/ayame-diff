package server

import (
	"strings"
	"testing"
)

// TestIncrementalRenderAssetsAreWired guards the browser half of the sliced
// rendering work (#127, #128). The slice budget and the render gate are pure
// and run under node --test; this checks that the page still loads them and
// that app.js actually goes through them.
func TestIncrementalRenderAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "renderqueue.js")

	if !strings.Contains(index, `<script src="renderqueue.js"></script>`) {
		t.Error("index.html does not load renderqueue.js")
	}
	if strings.Index(index, `src="renderqueue.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("renderqueue.js must load before app.js")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "fetch(") {
		t.Error("renderqueue.js touches the browser; it must stay runnable without one")
	}
	if !strings.Contains(app, "globalThis.AyameRenderQueue") {
		t.Error("app.js does not use the render queue module")
	}
}

// TestCancelAbortsInProgressRender is the #128 regression. Cancel aborted the
// request, but by render time the response had already arrived, so a large
// render ran to completion and the comparison was reported as finished. Cancel
// must also stop the render and report the cancellation.
func TestCancelAbortsInProgressRender(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	for _, want := range []string{"function cancelRendering(", "function cancelCurrentOperation("} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing %s", want)
		}
	}
	// The Cancel button goes through the operation cancel, not a bare abort that
	// no longer reaches a render whose request has resolved.
	if !strings.Contains(app, `$("cancel").addEventListener("click", cancelCurrentOperation)`) {
		t.Error("the Cancel button does not cancel an in-progress render")
	}
	// The sliced builder yields between slices and stops when the gate says the
	// render is no longer the current one.
	slices := renderFunctionBody(t, app, "async function renderInSlices(")
	for _, want := range []string{"renderGate.begin()", "renderGate.isCurrent(token)", "budget.reset()"} {
		if !strings.Contains(slices, want) {
			t.Errorf("renderInSlices does not use the render gate: missing %q", want)
		}
	}
	// Every comparison path that renders in slices must notice a cancellation
	// and report it instead of a completed comparison.
	if got := strings.Count(app, "renderGate.cancelled"); got < 3 {
		t.Errorf("renderGate.cancelled appears %d times; the text, folder, and three-way paths each need it", got)
	}
}
