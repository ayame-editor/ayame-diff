package server

import (
	"regexp"
	"strings"
	"testing"
)

// TestSwapSidesExists covers #90: reversing a comparison meant retyping both
// paths, a control every comparable tool provides.
func TestSwapSidesExists(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	if !strings.Contains(index, `id="swapSides"`) {
		t.Fatal("index.html has no swap control")
	}
	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "function swapSides()") {
		t.Fatal("app.js has no swapSides")
	}
	body := renderFunctionBody(t, app, "function swapSides()")
	// Scratch mode holds its text in different fields; swapping the paths there
	// would silently do nothing.
	if !strings.Contains(body, `$("scratch").checked`) {
		t.Error("swapSides ignores scratch mode, where the text lives in other fields")
	}
	// The pairing that was inspected no longer applies once the sides move.
	if !strings.Contains(body, "csvInspection = null") {
		t.Error("swapSides leaves the CSV inspection describing the old pairing")
	}
	// It must not start a comparison that was never requested.
	if !strings.Contains(body, "if (lastData || csvData || threeWayData || directoryData) compare()") {
		t.Error("swapSides re-runs unconditionally, or never re-runs")
	}
	if !strings.Contains(app, `$("swapSides").addEventListener("click", swapSides)`) {
		t.Error("the swap button is not wired")
	}
}

// TestEngineTuningIsCollapsed covers #92: window, max hunks, max lines per hunk
// and move-min-lines sat in the main options row, where they read as ordinary
// comparison settings rather than output limits.
func TestEngineTuningIsCollapsed(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")

	group := sectionBetween(t, index, `<details class="csv-advanced" id="engineTuning">`, "</details>")
	for _, id := range []string{`id="window"`, `id="maxHunks"`, `id="maxLines"`, `id="moveMinLines"`} {
		if !strings.Contains(group, id) {
			t.Errorf("%s is not inside the engine-tuning group", id)
		}
	}
	if regexp.MustCompile(`<details[^>]*id="engineTuning"[^>]*\bopen\b`).MatchString(index) {
		t.Error("the engine-tuning group is open by default")
	}

	// The controls must not also remain in the main row, which ends where the
	// collapsed group begins.
	mainOptions := sectionBetween(t, index, `<div class="opts">`, "<details")
	for _, id := range []string{`id="maxHunks"`, `id="maxLines"`} {
		if strings.Contains(mainOptions, id) {
			t.Errorf("%s is still in the main options row", id)
		}
	}

	// Collapsing must not hide a change, same contract as the CSV groups.
	if !strings.Contains(index, `id="engineTuningBadge"`) {
		t.Error("the engine-tuning group has no changed-settings badge")
	}
	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, `["engineTuning", "engineTuningBadge"]`) {
		t.Error("the engine-tuning badge is not refreshed with the others")
	}
	if !strings.Contains(app, `captureControlDefaults($("engineTuning"))`) {
		t.Error("the engine-tuning defaults are never captured, so the badge cannot be accurate")
	}
}
