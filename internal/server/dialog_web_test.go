package server

import (
	"strings"
	"testing"
)

// TestNativeDialogsAreReplaced covers #98 and #99. window.confirm and
// window.alert cannot be styled, cannot be read alongside the UI they describe,
// and the merge path used confirm to ask about overwriting a file — blocking
// the page on a native prompt at the moment the user most needs to see what
// they are confirming.
func TestNativeDialogsAreReplaced(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	// The shortcut list and every confirmation must go through the in-app
	// dialogs. Bare confirm(/alert( would reintroduce the native prompt.
	for _, banned := range []string{"alert(t(", "confirm(t(", "window.alert(", "window.confirm("} {
		if strings.Contains(app, banned) {
			t.Errorf("app.js still calls a native dialog: %s", banned)
		}
	}
	for _, fn := range []string{"function askConfirm(", "function showShortcuts("} {
		if !strings.Contains(app, fn) {
			t.Errorf("app.js is missing %s", fn)
		}
	}

	index := readWebAsset(t, "index.html")
	for _, id := range []string{`id="confirmDialog"`, `id="shortcutsDialog"`, `id="confirmOk"`, `id="confirmCancel"`} {
		if !strings.Contains(index, id) {
			t.Errorf("index.html is missing %s", id)
		}
	}
	// A <dialog> with method="dialog" gives Escape and the backdrop for free;
	// hand-rolling those is how in-app modals become keyboard traps.
	if strings.Count(index, `<dialog`) < 2 {
		t.Error("the modals are not native <dialog> elements")
	}
	if !strings.Contains(index, `<form method="dialog">`) {
		t.Error("the dialog forms do not use method=\"dialog\", so Escape and the buttons would need hand-wiring")
	}
}

// TestConfirmDialogRestoresFocus keeps the modal from stranding keyboard focus,
// which is the usual regression when a native prompt is replaced.
func TestConfirmDialogRestoresFocus(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	body := renderFunctionBody(t, app, "function askConfirm(")
	if !strings.Contains(body, "document.activeElement") {
		t.Error("askConfirm does not remember what opened it")
	}
	if !strings.Contains(body, "opener.focus()") {
		t.Error("askConfirm does not return focus, so Escape would strand it")
	}
	if !strings.Contains(body, "showModal()") {
		t.Error("askConfirm does not open modally, so the page stays interactive behind it")
	}
	if !strings.Contains(body, `{ once: true }`) {
		t.Error("the close listener is not one-shot, so repeated prompts would stack resolvers")
	}
	shortcuts := renderFunctionBody(t, app, "function showShortcuts(")
	if !strings.Contains(shortcuts, "opener.focus()") {
		t.Error("the shortcut dialog does not return focus")
	}
}

// TestShortcutListIsGeneratedFromOneSource keeps the help from drifting away
// from the handlers it documents.
func TestShortcutListIsGeneratedFromOneSource(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "const SHORTCUTS = [") {
		t.Fatal("the shortcut list is not defined in one place")
	}
	list := sectionBetween(t, app, "const SHORTCUTS = [", "];")
	// The bindings the app actually installs must appear in the help.
	for _, keys := range []string{"Alt+↓", "Ctrl+F", "Esc", "Alt+B"} {
		if !strings.Contains(list, keys) {
			t.Errorf("the shortcut help omits %s", keys)
		}
	}
	// Entries are i18n keys, not baked-in English.
	if strings.Contains(list, "Next / previous difference") {
		t.Error("the shortcut help hardcodes English instead of using translation keys")
	}
}
