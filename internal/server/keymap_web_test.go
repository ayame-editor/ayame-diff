package server

import (
	"strings"
	"testing"
)

// TestKeymapAssetsAreWired guards the browser half of #285. The chord syntax,
// the presets and the conflict rules live in a pure module exercised by
// node --test; what this checks is that the page loads it, that the help and the
// global handlers both read the resolved bindings rather than repeating key
// literals, and that the preferences are persisted under the documented key.
func TestKeymapAssetsAreWired(t *testing.T) {
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

	// app.js must consume the module rather than keep a second copy of the
	// bindings, and the dialog must be built from the resolved table.
	for _, want := range []string{
		"globalThis.AyameKeymap",
		"const SHORTCUT_ACTIONS = KEYMAP_ACTION_IDS.map(",
		"function matchesShortcut(",
		"function showShortcuts(",
		"function renderKeymapEditor(",
		`const KEYMAP_STORAGE = "ayame-keybindings"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing keymap wiring %q", want)
		}
	}
	if strings.Contains(app, "const SHORTCUTS = [") {
		t.Error("app.js still carries a second shortcut list, so the tested table is not the used one")
	}
	if strings.Count(app, `"ayame-keybindings"`) != 1 {
		t.Error("the storage key literal is not declared exactly once in app.js")
	}

	// The handlers must ask for an action by name; a literal chord comparison
	// would silently stop tracking a remap.
	for _, want := range []string{
		`matchesShortcut(event, "navigateNext")`,
		`matchesShortcut(event, "navigatePrev")`,
		`matchesShortcut(event, "chooseLeft")`,
		`matchesShortcut(event, "chooseBase")`,
		`matchesShortcut(event, "search")`,
		`matchesShortcut(event, "close")`,
		`matchesShortcut(event, "compare")`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js does not dispatch %q through the binding table", want)
		}
	}

	// The remap dialog has to be reachable from the help dialog and carry the
	// controls the feature promises: preset, reset and export.
	for _, want := range []string{
		`id="keymapDialog"`, `id="keymapPreset"`, `id="keymapReset"`,
		`id="keymapExport"`, `id="keymapList"`, `id="keymapStatus"`,
		`id="shortcutCustomize"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html is missing %s", want)
		}
	}

	// Both languages must name the dialog and its controls; the Go i18n test
	// already enforces parity, this catches a key that was dropped from both.
	for _, want := range []string{
		`shortcutCustomize:`, `shortcutEditorTitle:`, `shortcutPresetDefault:`,
		`shortcutResetPreset:`, `shortcutExportJSON:`, `shortcutConflict:`,
		`shortcutReserved:`, `shortcutNavigateNext:`, `shortcutChooseLeft:`,
	} {
		if !strings.Contains(catalog, want) {
			t.Errorf("i18n.js is missing %s", want)
		}
	}
}
