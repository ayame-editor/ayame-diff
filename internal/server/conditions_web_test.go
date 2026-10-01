package server

import (
	"strings"
	"testing"
)

// TestConditionToolbarIsPermanentAndReflectsPolicy guards the browser half of
// #264. The label derivation itself runs under node --test; what this checks is
// that the result toolbar actually carries the controls, that they mirror the
// settings dialog instead of holding a second copy of the state, that the
// numeric tuning stays behind the dialog, and that changing a row re-runs.
func TestConditionToolbarIsPermanentAndReflectsPolicy(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "conditions.js")

	if !strings.Contains(index, `<script src="conditions.js"></script>`) {
		t.Error("index.html does not load conditions.js")
	}
	if strings.Index(index, `src="conditions.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("conditions.js must load before app.js")
	}

	// The controls are permanent in the result navigation, not hidden in a
	// collapsed group. Each shows its current value rather than just its name.
	nav := sectionBetween(t, index, `<nav class="diff-nav"`, `</nav>`)
	for _, id := range []string{`id="tbWhitespace"`, `id="tbCase"`, `id="tbEol"`, `id="tbFilters"`, `id="tbMoves"`} {
		if !strings.Contains(nav, id) {
			t.Errorf("the result toolbar has no condition control %s", id)
		}
	}
	if !strings.Contains(nav, `id="conditionBar"`) {
		t.Error("the condition controls are loose in the toolbar rather than one bounded group")
	}

	// The toggles move out; the values that tune how much is computed — and the
	// settings controls they mirror — stay in the dialog. Losing one of these
	// would either grow the toolbar or leave it without a source of truth.
	dialog := sectionBetween(t, index, `<dialog id="settingsDialog"`, `</dialog>`)
	for _, id := range []string{
		`id="lineFilters"`, `id="detectMoves"`, `id="whitespaceScale"`, `id="ignoreCase"`,
		`id="moveMinLines"`, `id="contextLines"`,
	} {
		if !strings.Contains(dialog, id) {
			t.Errorf("%s left the settings dialog; it is a tuning value or the source of truth", id)
		}
	}

	// The module is geometry for pure tests: no DOM, no storage, no network.
	for _, forbidden := range []string{"document.", "fetch(", "localStorage", "addEventListener"} {
		if strings.Contains(module, forbidden) {
			t.Errorf("conditions.js touches browser state %q; node must exercise the same derivation", forbidden)
		}
	}
	if !strings.Contains(module, "TOOLBAR_CONTROLS") {
		t.Error("conditions.js has no fixed toolbar-control list, so the row count is not bounded")
	}

	for _, want := range []string{
		"globalThis.AyameConditions",
		"function currentConditionPolicy(",
		"function syncConditionToolbar(",
		"function applyConditionToolbar(",
		"readConditionPolicy({",
		"writeConditionPolicy(control, value, currentConditionPolicy())",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing condition-toolbar wiring %q", want)
		}
	}

	// Changing a condition on screen re-runs the comparison, replacing the
	// history entry rather than pushing the same inputs again (#254).
	apply := renderFunctionBody(t, app, "function applyConditionToolbar(")
	if !strings.Contains(apply, `compare({ urlHistory: "replace" })`) {
		t.Error("changing a toolbar condition does not re-run the comparison")
	}
	if !strings.Contains(apply, "updateDetailsBadges()") {
		t.Error("changing a toolbar condition does not refresh the toolbar labels")
	}
	// The dialog edits the same controls, so a change there has to re-run too or
	// the toolbar status and the shown result could disagree.
	if !strings.Contains(app, `["ignoreCase", "whitespaceScale", "lineFilters", "detectMoves"]`) {
		t.Error("a comparison-condition change in Settings does not re-run the comparison")
	}
}

// TestConditionToolbarHidesDeadControls keeps the visible set equal to what the
// active mode actually reads (#124): the toolbar must not offer a control whose
// change the mode ignores.
func TestConditionToolbarHidesDeadControls(t *testing.T) {
	t.Parallel()

	app := readWebAsset(t, "app.js")
	body := renderFunctionBody(t, app, "function conditionSettingHidden(")
	if !strings.Contains(body, "CONDITION_SETTING[id]") {
		t.Error("the toolbar does not follow the settings form's live/dead policy")
	}
	if !strings.Contains(body, "holder?.hidden") {
		t.Error("the toolbar decides liveness on its own instead of the control the form hid")
	}
	sync := renderFunctionBody(t, app, "function syncConditionToolbar(")
	if !strings.Contains(sync, "conditionSettingHidden(row.id)") {
		t.Error("the toolbar never hides the conditions the active mode ignores")
	}
}
