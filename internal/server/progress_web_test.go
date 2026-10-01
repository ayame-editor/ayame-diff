package server

import (
	"strings"
	"testing"
)

// TestProgressModelIsWiredAndPure guards the #297 browser integration around the
// node-tested progress model: the page must load the module before app.js, and
// app.js must drive the status lane from it instead of an elapsed timer.
func TestProgressModelIsWiredAndPure(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "progress.js")

	if !strings.Contains(index, `<script src="progress.js"></script>`) {
		t.Error("index.html does not load progress.js")
	}
	if strings.Index(index, `src="progress.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("progress.js must load before app.js, which consumes it")
	}
	if !strings.Contains(module, "module.exports = api") {
		t.Error("progress.js has no CommonJS export, so node --test cannot require it")
	}
	// It is a model: no DOM, no app state. The translation and painting live in
	// app.js, which is what keeps the logic testable without a browser (#139).
	for _, leaked := range []string{"document.", "$(", "localStorage", "addEventListener", "apiFetch("} {
		if strings.Contains(module, leaked) {
			t.Errorf("progress.js contains DOM or application wiring (%q); it must stay pure", leaked)
		}
	}

	for _, want := range []string{
		"globalThis.AyameProgress",
		"createProgressTracker({ onChange: renderProgress })",
		"function renderProgress(",
		"degradationNotice(event)",
		"readDiffStream(",
		`apiFetch("/api/diff/stream"`,
		"function finishStreamRender(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing progress wiring %q", want)
		}
	}
	// A degradation forecast is an outcome, so it joins the persistent message
	// lane rather than silently replacing the running progress (#97, #297).
	if !strings.Contains(app, `messageLog.post(t(notice.key, notice.params), "warning")`) {
		t.Error("app.js does not post the truncation forecast to the message lane")
	}

	// The text-diff path no longer fakes progress with an elapsed ticker.
	body := renderFunctionBody(t, app, "async function runCompare(")
	if strings.Contains(body, "setInterval(tick") {
		t.Error("runCompare still runs the elapsed-time ticker for the text diff")
	}
	if !strings.Contains(body, `createProgressTracker`) {
		t.Error("runCompare does not build a progress tracker")
	}
}

// TestProgressStageKeysAreTranslated keeps the stage vocabulary in step with the
// catalog: a stage the model can report without a label would render its raw
// key into the status lane.
func TestProgressStageKeysAreTranslated(t *testing.T) {
	t.Parallel()

	module := readWebAsset(t, "progress.js")
	i18n := readWebAsset(t, "i18n.js")
	for _, key := range []string{"stageRead", "stageCompare", "stageMoves", "stageResult", "stageRender", "progressStage", "progressDeterminate", "degradedHunks", "degradedLines"} {
		if !strings.Contains(module, key) && !strings.Contains(i18n, key) {
			t.Errorf("no module or catalog entry for %q", key)
		}
		count := strings.Count(i18n, key+":")
		if count < 2 {
			t.Errorf("i18n key %q is defined %d time(s); ja and en must both have it", key, count)
		}
	}
}
