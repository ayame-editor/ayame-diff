package server

import (
	"strings"
	"testing"
)

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

	// The paths are the one thing that must stay on the form.
	for _, id := range []string{"old", "new", "base"} {
		if !strings.Contains(setup, `id="`+id+`"`) {
			t.Errorf("the %s path field left the setup form", id)
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
