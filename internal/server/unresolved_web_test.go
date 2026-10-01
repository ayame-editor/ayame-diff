package server

import (
	"strings"
	"testing"
)

// TestUnresolvedListAndTargetAreWired guards #272 in the page: a save must list
// the unresolved differences with a way to jump to one, and the implicit
// resolution target must be selectable and travel in every merge request body.
func TestUnresolvedListAndTargetAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "unresolved.js")

	if !strings.Contains(index, `<script src="unresolved.js"></script>`) {
		t.Error("index.html does not load unresolved.js")
	}
	if strings.Index(index, `src="unresolved.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("unresolved.js must load before app.js, which destructures it at parse time")
	}
	if !strings.Contains(module, "module.exports = api") {
		t.Error("unresolved.js has no CommonJS export, so node --test cannot require it")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "addEventListener") || strings.Contains(module, "apiFetch(") {
		t.Error("unresolved.js touches the DOM or network; it must stay runnable without either")
	}

	for _, want := range []string{
		`id="mergeUnresolvedBox"`, `id="mergeUnresolvedList"`,
		`id="mergeGoUnresolved"`, `id="mergeUnresolvedTarget"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html is missing the unresolved-list control %q", want)
		}
	}
	for _, want := range []string{
		"globalThis.AyameUnresolved",
		"function refreshMergeUnresolved()",
		"function jumpToMergeItem(",
		"function reportMergeSaved(",
		`t("mergeSavedWithMarkers"`,
		`t("mergeSavedWithImplicit"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing unresolved-list wiring %q", want)
		}
	}
	if !strings.Contains(index, `data-i18n="unresolvedListHeading"`) || !strings.Contains(index, `data-i18n="goToUnresolved"`) {
		t.Error("index.html does not translate the unresolved list")
	}
	// All three merge paths must forward the chosen target.
	if got := strings.Count(app, `unresolvedTarget: allowUnresolved ? target : ""`); got != 3 {
		t.Errorf("app.js sends unresolvedTarget in %d merge bodies; want text, csv, and three-way", got)
	}
	// The old wording claimed the left side; the confirmation now names the
	// selected target instead of silently assuming left.
	if strings.Contains(app, `t("unresolvedWarning", unresolved)`) {
		t.Error("app.js still asks the old left-only unresolved question")
	}

	for _, key := range []string{
		"unresolvedListHeading", "goToUnresolved", "unresolvedTarget", "targetMarkers",
		"mergeSavedWithImplicit", "mergeSavedWithMarkers",
	} {
		if !strings.Contains(readWebAsset(t, "i18n.js"), key+":") {
			t.Errorf("i18n.js is missing key %q", key)
		}
	}
}
