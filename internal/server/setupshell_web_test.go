package server

import (
	"strings"
	"testing"
)

// TestResultOwnsTheViewport pins the application shell from #250. Every
// ancestor between the viewport and #result must be allowed to shrink; losing
// one min-height/overflow rule silently returns the page to document scrolling
// and can make the bottom of a long diff unreachable.
func TestResultOwnsTheViewport(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")

	rules := []struct {
		selector string
		wants    []string
	}{
		{"body {", []string{"height: 100dvh", "display: grid", "grid-template-rows: auto minmax(0, 1fr) auto", "overflow: hidden"}},
		{"main {", []string{"display: flex", "min-height: 0", "overflow: hidden"}},
		{".diff-layout, main > #result {", []string{"flex: 1 1 auto", "min-height: 0"}},
		{".diff-layout > #result {", []string{"align-self: stretch"}},
		{"\n#result {", []string{"min-height: 0", "overflow: auto", "grid-auto-rows: min-content"}},
		{".setup {", []string{"max-height: 50vh", "overflow-y: auto"}},
	}
	for _, rule := range rules {
		body := sectionBetween(t, style, rule.selector, "}")
		for _, want := range rule.wants {
			if !strings.Contains(body, want) {
				t.Errorf("%s no longer contains %q", rule.selector, want)
			}
		}
	}
}

// TestComparisonOptionsAreNotOnTheSetupForm pins the split every surveyed tool
// draws: the screen you pick files on carries what to compare, and how to
// compare it lives behind one button. WinMerge's Select-Files screen has three
// path fields, a filter, and an Options button — not one ignore checkbox.
//
// This is the rule the setup form kept breaking. It is asserted structurally
// rather than by counting controls, because a count passes as soon as someone
// hides one group and fails for reasons that have nothing to do with the rule.
func TestComparisonOptionsAreNotOnTheSetupForm(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")

	setup := sectionBetween(t, index, `<section class="setup" id="setup">`, `</section>`)
	launchPaths := sectionBetween(t, index, `<div class="paths launch-paths" id="paths">`, `<section class="setup"`)
	dialog := sectionBetween(t, index, `<dialog id="settingsDialog"`, `</dialog>`)

	// What counts as a difference, and how much of one is computed.
	for _, id := range []string{
		"compareConditions", "engineTuning",
		"ignoreCase", "ignoreEOL", "ignoreTrailingEOL", "whitespace", "lineFilters", "detectMoves",
		"window", "maxHunks", "maxLines", "moveMinLines",
	} {
		marker := `id="` + id + `"`
		if strings.Contains(setup, marker) {
			t.Errorf("%s is on the setup form; it belongs behind the settings dialog", id)
		}
		if !strings.Contains(dialog, marker) {
			t.Errorf("%s is not in the settings dialog — did it go missing?", id)
		}
	}

	// Paths are an initial-only rail, not part of the setup section that remains
	// reachable while reading a result (#252).
	for _, id := range []string{"old", "new", "base"} {
		if strings.Contains(setup, `id="`+id+`"`) {
			t.Errorf("the %s path field is still duplicated inside setup", id)
		}
		if !strings.Contains(launchPaths, `id="`+id+`"`) {
			t.Errorf("the initial %s path field is missing from the launch rail", id)
		}
	}
	if !strings.Contains(setup, `id="openSettings"`) {
		t.Error("no way to reach the settings dialog from the form")
	}
}

// TestCompareRefusesBeforeItIsAsked covers the other half of that screen:
// WinMerge greys out Compare and explains itself on a status line rather than
// accepting the click and returning an error. The answer to "both sides are
// invalid paths" is always to fix the field, so saying it up front is strictly
// better than saying it after.
func TestCompareRefusesBeforeItIsAsked(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	if !strings.Contains(index, `id="setupNote"`) {
		t.Fatal("no status line under the setup form")
	}
	if !strings.Contains(sectionBetween(t, index, `id="setupNote"`, ">"), `role="status"`) {
		t.Error("the status line is not announced, so the reason is invisible to a screen reader")
	}

	body := renderFunctionBody(t, app, "function syncCompareReady(")
	if !strings.Contains(body, `$("compare").disabled`) {
		t.Error("Compare is never disabled")
	}
	// Three-way needs a base; two-way must not demand one.
	if !strings.Contains(body, "needsBase") {
		t.Error("the base path is not required for a three-way comparison")
	}
	// Pasted text is a valid source and has no paths at all.
	if !strings.Contains(body, `$("scratch").checked`) {
		t.Error("pasted text is treated as missing paths")
	}
	if !strings.Contains(app, "syncCompareReady()") {
		t.Error("the readiness check is never run")
	}
}

// TestPathFieldsRememberWhatWasCompared covers the MRU dropdown WinMerge puts on
// every path field. Re-running a comparison is the common case, and without
// history it means retyping a path that was typed an hour ago.
func TestPathFieldsRememberWhatWasCompared(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	for _, side := range []string{"old", "new", "base"} {
		field := sectionBetween(t, index, `<input id="`+side+`"`, `/>`)
		if !strings.Contains(field, `list="`+side+`History"`) {
			t.Errorf("the %s field has no history list", side)
		}
		if !strings.Contains(index, `<datalist id="`+side+`History">`) {
			t.Errorf("no datalist backing the %s field", side)
		}
	}

	body := renderFunctionBody(t, app, "function rememberPaths(")
	// Pasted text has no path; recording an empty one would poison the list.
	if !strings.Contains(body, `$("scratch").checked`) {
		t.Error("pasted text is recorded as a path")
	}
	if !strings.Contains(body, "filter((item) => item !== value)") {
		t.Error("a repeated path is duplicated instead of moving to the top")
	}
	if !strings.Contains(body, "PATH_HISTORY_MAX") {
		t.Error("the history is unbounded")
	}
	// localStorage throws when full or blocked, and losing history must never
	// take the comparison down with it.
	if !strings.Contains(body, "catch (_)") {
		t.Error("a failed write to localStorage is not contained")
	}
	if !strings.Contains(app, "rememberPaths();") {
		t.Error("history is never recorded")
	}
}

// TestOpeningAFileKeepsTheFolderResult covers the drill-in path. Opening a file
// overwrote the mode and both paths and re-ran, which threw the folder
// comparison away — checking a hundred files meant re-scanning the tree a
// hundred times. The result is already in memory, so returning is a re-render.
func TestOpeningAFileKeepsTheFolderResult(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	index := readWebAsset(t, "index.html")

	if !strings.Contains(index, `id="backToFolder"`) {
		t.Fatal("no way back to the folder list")
	}

	open := renderFunctionBody(t, app, "async function openFromFolder(")
	if !strings.Contains(open, "folderReturn = {") {
		t.Error("the folder result is not kept when opening a file")
	}
	// Losing your place in a long list is most of the cost of going back. Keep
	// a logical path anchor rather than a pixel offset, since rows can resize.
	if !strings.Contains(open, "captureFolderTreeState(entry.path)") {
		t.Error("the logical position in the folder list is not kept")
	}

	back := renderFunctionBody(t, app, "async function returnToFolder(")
	if !strings.Contains(back, "renderDirectory(data, body, { expanded, selectedPath })") {
		t.Error("returning re-runs the comparison instead of re-rendering what is held")
	}
	if strings.Contains(back, "compareDirectory(") || strings.Contains(back, "apiFetch(") {
		t.Error("returning to the folder list hits the server again")
	}
	if !strings.Contains(back, "restoreResultScrollAnchor(anchor") {
		t.Error("returning drops the reader at the top of the list")
	}
	if !strings.Contains(back, "expanded, selectedPath") {
		t.Error("returning drops the folder expansion or selected-row state")
	}
	// A fresh folder comparison must not offer a stale way back.
	if !strings.Contains(renderFunctionBody(t, app, "async function renderDirectory("), "folderReturn = null") {
		t.Error("a new folder comparison keeps the previous return target")
	}
}
