package server

import (
	"regexp"
	"strings"
	"testing"
)

// setupSection returns the setup fieldset's markup up to the collapsed groups,
// i.e. what a first visit shows before opening anything.
func setupFirstTier(t *testing.T, index string) string {
	t.Helper()
	return sectionBetween(t, index, `<div class="opts">`, "<details")
}

// TestSetupFirstTierIsSmall is the #86 acceptance check: the setup was a single
// flex-wrap row of 24 controls mixing four unrelated kinds — comparison
// conditions, display settings, engine tuning and patch output.
func TestSetupFirstTierIsSmall(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	first := setupFirstTier(t, index)
	controls := regexp.MustCompile(`<(input|select|textarea)\b`).FindAllString(first, -1)
	if len(controls) > 8 {
		t.Errorf("%d controls in the first tier; #86 requires 8 or fewer:\n%s", len(controls), first)
	}
	// Each remaining kind must be a labelled group rather than loose controls.
	for _, group := range []string{`id="compareConditions"`, `id="engineTuning"`} {
		if !strings.Contains(index, group) {
			t.Errorf("index.html has no %s group", group)
		}
	}
	// The conditions group is the one that changes results, so it stays open.
	if !regexp.MustCompile(`id="compareConditions"[^>]*\bopen\b`).MatchString(index) {
		t.Error("the comparison-conditions group is collapsed; it changes what a comparison means and should stay visible")
	}
}

// TestDisplaySettingsLiveWithTheResult is the #91 acceptance check: wrap,
// syntax, whitespace, word highlight, theme and colours are decided while
// reading a result, but the controls were in the form above it — out of sight
// behind a long diff.
func TestDisplaySettingsLiveWithTheResult(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")

	nav := sectionBetween(t, index, `<nav class="diff-nav"`, "</nav>")
	for _, id := range []string{`id="wrap"`, `id="syntax"`, `id="showWs"`, `id="word"`, `id="scheme"`, `id="theme"`} {
		if !strings.Contains(nav, id) {
			t.Errorf("%s is not in the result navigation", id)
		}
	}
	// And they must be gone from the setup form, not duplicated into both.
	setup := sectionBetween(t, index, `<fieldset`, `<nav class="diff-nav"`)
	for _, id := range []string{`id="wrap"`, `id="syntax"`, `id="showWs"`, `id="word"`} {
		if strings.Contains(setup, id) {
			t.Errorf("%s is still in the setup form", id)
		}
	}

	style := readWebAsset(t, "style.css")
	// The nav is sticky, which is what makes "while reading" work at all.
	navStyle := sectionBetween(t, style, ".diff-nav {", "}")
	if !strings.Contains(navStyle, "sticky") {
		t.Error("the result navigation is not sticky, so its controls scroll away from the result")
	}
	if !strings.Contains(style, ".view-settings") {
		t.Error("style.css has no view-settings rule")
	}
}

// TestSetupGroupsUseAGridNotFlexWrap covers #86's spatial-memory complaint: in
// a flex-wrap row every control moves when the window resizes.
func TestSetupGroupsUseAGridNotFlexWrap(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")
	grid := sectionBetween(t, style, ".csv-advanced .opts {", "}")
	if !strings.Contains(grid, "display: grid") {
		t.Error("setup groups still lay out with flex-wrap, so controls migrate on resize")
	}
	// The line-filter box carries a min-width from the flex layout; in a grid
	// that overflows its cell and lands on the neighbouring control.
	if !strings.Contains(style, ".csv-advanced .opts .filter-input") {
		t.Error("the line-filter box has no grid rule, so its min-width overflows its cell")
	}
}

// TestPatchSettingsFollowTheirButton keeps the patch format controls with the
// Export patch action they configure, rather than in the setup form.
func TestPatchSettingsFollowTheirButton(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	nav := sectionBetween(t, index, `<nav class="diff-nav"`, "</nav>")
	for _, id := range []string{`id="patchFormat"`, `id="patchContext"`} {
		if !strings.Contains(nav, id) {
			t.Errorf("%s is not beside the Export patch button", id)
		}
	}
	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "function syncPatchSettingsVisibility()") {
		t.Fatal("the patch settings have no visibility rule")
	}
	export := renderFunctionBody(t, app, "function syncExportPatchVisibility()")
	if !strings.Contains(export, "syncPatchSettingsVisibility()") {
		t.Error("the patch settings do not follow the Export patch button's visibility")
	}
}
