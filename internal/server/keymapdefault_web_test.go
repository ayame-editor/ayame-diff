package server

import (
	"strings"
	"testing"
)

// TestKeymapDefaultAssetsAreWired guards the browser half of #277. The chord
// table and the auto-advance walk live in a pure module exercised by
// node --test; what this checks is that the page loads it before app.js, that
// the merge handlers dispatch through the binding table instead of repeating key
// literals, and that difference and conflict navigation stay separate.
func TestKeymapDefaultAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "keymap.js")
	catalog := readWebAsset(t, "i18n.js")

	if !strings.Contains(index, `<script src="keymap.js"></script>`) {
		t.Error("index.html does not load keymap.js")
	}
	if strings.Index(index, `src="keymap.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("keymap.js must load before app.js, which destructures it at parse time")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "localStorage") ||
		strings.Contains(module, "addEventListener") || strings.Contains(module, "fetch(") {
		t.Error("keymap.js touches the browser; it must stay runnable without one")
	}
	if !strings.Contains(module, "module.exports = api") {
		t.Error("keymap.js has no CommonJS export, so node --test cannot require it")
	}

	// The default map is the documented one: the WinMerge navigation and side
	// keys survive, and the merge-flow keys have distinct bindings.
	for _, want := range []string{
		`navigateNext: "Alt+ArrowDown"`,
		`navigatePrev: "Alt+ArrowUp"`,
		`firstDiff: "Alt+Home"`,
		`lastDiff: "Alt+End"`,
		`nextConflict: "F8"`,
		`prevConflict: "Shift+F8"`,
		`chooseLeft: "Alt+ArrowLeft"`,
		`chooseRight: "Alt+ArrowRight"`,
		`chooseBase: "Alt+B"`,
		`chooseBoth: "Alt+A"`,
		`toggleAutoAdvance: "Alt+Shift+A"`,
		`saveMerge: "Ctrl+Shift+S"`,
	} {
		if !strings.Contains(module, want) {
			t.Errorf("the default keymap is missing %s", want)
		}
	}

	// app.js must consume the module rather than keep a second copy of the map.
	if !strings.Contains(app, "globalThis.AyameKeymap") {
		t.Error("app.js does not read the default keymap module")
	}
	if !strings.Contains(app, "mergeBindings(SHORTCUT_ACTIONS, presetBindings(") {
		t.Error("app.js does not resolve the tested default map through the preset layer")
	}
	if strings.Contains(app, "event.altKey || event.ctrlKey") {
		t.Error("app.js still compares raw modifiers instead of asking the binding table")
	}

	// The handlers must ask for an action by name; a literal chord comparison
	// would silently stop tracking the documented map.
	for _, want := range []string{
		`matchesShortcut(event, "navigateNext")`,
		`matchesShortcut(event, "navigatePrev")`,
		`matchesShortcut(event, "firstDiff")`,
		`matchesShortcut(event, "lastDiff")`,
		`matchesShortcut(event, "nextConflict")`,
		`matchesShortcut(event, "prevConflict")`,
		`matchesShortcut(event, "chooseLeft")`,
		`matchesShortcut(event, "chooseRight")`,
		`matchesShortcut(event, "chooseBase")`,
		`matchesShortcut(event, "chooseBoth")`,
		`matchesShortcut(event, "toggleAutoAdvance")`,
		`matchesShortcut(event, "saveMerge")`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js does not dispatch %q through the binding table", want)
		}
	}

	// Difference navigation and conflict navigation must be different lists.
	for _, want := range []string{
		"function activeConflictIndexes(",
		"function stepConflict(",
		"function activeHunkIndexes(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing the navigation split %q", want)
		}
	}
	differences := renderFunctionBody(t, app, "function activeHunkIndexes(")
	if strings.Contains(differences, `"conflict"`) {
		t.Error("difference navigation still filters to conflicts")
	}
	conflicts := renderFunctionBody(t, app, "function activeConflictIndexes(")
	if !strings.Contains(conflicts, `=== "conflict"`) {
		t.Error("conflict navigation does not select conflicts")
	}

	// Auto-advance is optional and reads the pure walker.
	for _, want := range []string{
		"let mergeAutoAdvance = false;",
		"function setMergeAutoAdvance(",
		"function advanceAfterAdopt(",
		"nextUnresolved(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing auto-advance wiring %q", want)
		}
	}
	if !strings.Contains(index, `id="mergeAutoAdvance"`) {
		t.Error("index.html has no auto-advance control")
	}
	if !strings.Contains(app, `$("mergeAutoAdvance")`) {
		t.Error("app.js does not read the auto-advance control")
	}

	// Both languages must name the new actions and controls; the parity test
	// enforces the two tables match, this catches a key dropped from both.
	for _, want := range []string{
		`chooseBoth:`, `autoAdvanceOption:`, `autoAdvanceOn:`, `autoAdvanceOff:`,
		`shortcutChooseBoth:`, `shortcutNavigateConflictNext:`,
		`shortcutNavigateConflictPrev:`, `shortcutAutoAdvance:`,
		`shortcutSaveMerge:`, `shortcutUnbound:`,
	} {
		if !strings.Contains(catalog, want) {
			t.Errorf("i18n.js is missing %s", want)
		}
	}
}
