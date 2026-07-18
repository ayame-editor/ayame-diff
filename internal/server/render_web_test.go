package server

import (
	"strings"
	"testing"
)

// TestLargeDiffRenderIsSliced is the #127 regression. Every render path built
// its whole result in one synchronous loop, so a large diff blocked the main
// thread — measured at ~15.7s for 20,000 rows, which is reachable at the
// default caps. Each path must yield between slices instead.
func TestLargeDiffRenderIsSliced(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	if !strings.Contains(app, "async function renderInSlices(") {
		t.Fatal("app.js has no sliced render helper")
	}
	for _, fn := range []string{
		"async function renderResult(",
		"async function renderThreeWay(",
		"async function renderDirectory(",
	} {
		if !strings.Contains(app, fn) {
			t.Errorf("%s is not async, so it cannot yield mid-render", fn)
		}
	}
	// Each of the three must actually go through the helper, not just be async.
	if got := strings.Count(app, "renderInSlices("); got < 4 {
		t.Errorf("renderInSlices appears %d times; the helper plus three render paths are expected", got)
	}
}

// TestRenderYieldsWithoutRequestAnimationFrame guards a bug found while
// measuring: rAF does not fire in a hidden tab, so a user who switched away
// mid-render came back to a diff frozen part-way through. Yielding must use a
// scheduler that runs in the background.
func TestRenderYieldsWithoutRequestAnimationFrame(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "function yieldToBrowser()") {
		t.Fatal("no yieldToBrowser helper")
	}
	if !strings.Contains(app, "new MessageChannel()") {
		t.Error("yielding does not use MessageChannel, which is what keeps a hidden tab rendering")
	}
	body := renderFunctionBody(t, app, "function yieldToBrowser()")
	if strings.Contains(body, "requestAnimationFrame") || strings.Contains(body, "setTimeout") {
		t.Error("yieldToBrowser uses rAF or setTimeout; both stall or are clamped in a background tab")
	}
}

// TestWhitespaceMarkersBuiltOnlyWhenShown is the DOM-size half of #127. The
// three-element marker structure per whitespace run was built unconditionally
// and merely hidden by CSS, accounting for 82% of the DOM on a large diff
// (660,000 of 801,400 elements) for a display that is off by default.
func TestWhitespaceMarkersBuiltOnlyWhenShown(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	body := renderFunctionBody(t, app, "function appendText(")
	if !strings.Contains(body, "showWhitespace()") {
		t.Error("appendText builds whitespace markers regardless of whether they are shown")
	}
	if !strings.Contains(app, "$(\"word\").checked ? inlineWordDiff(") {
		t.Error("the word-diff DP still runs when word highlighting is off")
	}
	// Because those toggles now change which nodes exist, flipping one has to
	// redraw rather than only swap a class.
	if !strings.Contains(app, "function rerenderForDisplayChange()") {
		t.Fatal("no re-render on display-option change")
	}
	if !strings.Contains(app, `$("word").addEventListener("change", rerenderForDisplayChange)`) {
		t.Error("the word toggle does not re-render")
	}
	if !strings.Contains(app, "rerenderForDisplayChange();\n});") {
		t.Error("the whitespace toggle does not re-render")
	}
}

// TestOffscreenHunksSkipLayout keeps the browser from laying out the whole
// document at once. Without it, appending a large diff ended in a single
// multi-second layout of everything below the fold.
func TestOffscreenHunksSkipLayout(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")
	if !strings.Contains(style, "content-visibility: auto") {
		t.Error("style.css does not let off-screen hunks skip layout")
	}
	if !strings.Contains(style, "contain-intrinsic-size") {
		t.Error("content-visibility without contain-intrinsic-size makes the scrollbar jump")
	}
}

// TestSkeletonShownWhileComparing covers the blank-area complaint: the result
// area was emptied the moment a comparison started and stayed blank until the
// render finished.
func TestSkeletonShownWhileComparing(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "function showResultSkeleton()") {
		t.Fatal("no skeleton helper")
	}
	if !strings.Contains(app, "  showResultSkeleton();") {
		t.Error("the compare flow does not show the skeleton")
	}
	style := readWebAsset(t, "style.css")
	for _, cls := range []string{".skeleton-hunk", ".skeleton-row"} {
		if !strings.Contains(style, cls) {
			t.Errorf("style.css lacks %s", cls)
		}
	}
	if !strings.Contains(style, "prefers-reduced-motion") {
		t.Error("the skeleton animation ignores prefers-reduced-motion")
	}
}

// renderFunctionBody returns the source of a function up to its closing brace
// at column 0.
func renderFunctionBody(t *testing.T, source, header string) string {
	t.Helper()
	start := strings.Index(source, header)
	if start < 0 {
		t.Fatalf("%q not found", header)
	}
	rest := source[start:]
	if end := strings.Index(rest, "\n}"); end >= 0 {
		return rest[:end]
	}
	return rest
}
