package server

import (
	"os/exec"
	"strings"
	"testing"
)

// TestConditionScopePrecedence pins the session-vs-default precedence in
// web/conditionscope.js under node (#262). Changing a condition must stay a
// session override; only an explicit promote may touch the saved default, and
// a corrupt stored value must degrade to the factory default rather than
// blanking a control.
func TestConditionScopePrecedence(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	script := `
const scope = require('./web/conditionscope.js');
if (scope.STORAGE_KEY !== 'ayame-condition-defaults') process.exit(10);
const factory = {ignoreCase:false,ignoreEOL:false,ignoreTrailingEOL:false,whitespace:'none',lineFilters:'',detectMoves:false};
const saved = {...factory, whitespace:'change'};
const session = {ignoreCase:true};
const effective = scope.resolve(factory, saved, session);
if (effective.whitespace !== 'change' || effective.ignoreCase !== true || effective.detectMoves !== false) process.exit(11);
if (scope.isDefault(factory, factory) !== true) process.exit(12);
if (scope.isDefault(session, factory) !== false) process.exit(13);
if (scope.changes({...factory, lineFilters:'x'}, factory).join(',') !== 'lineFilters') process.exit(14);
if (scope.parse('{"whitespace":"bogus"}', factory).whitespace !== 'none') process.exit(15);
if (scope.parse('not json', factory) !== null) process.exit(16);
`
	cmd := exec.Command(node, "-e", script)
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("condition scope precedence test failed: %v\n%s", err, output)
	}
}

// TestConditionScopeAssetsAreWired guards the load order and consumption of
// conditionscope.js: index.html must pull it in before app.js, the dialog must
// carry the scope chip and both actions, and app.js must persist the default
// under its own key. Runs without node so it always executes in CI.
func TestConditionScopeAssetsAreWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "conditionscope.js")

	if !strings.Contains(index, `<script src="conditionscope.js"></script>`) {
		t.Error("index.html does not load conditionscope.js")
	}
	if strings.Index(index, `src="conditionscope.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("conditionscope.js must load before app.js")
	}
	// The scope chip and both explicit actions live with the conditions.
	for _, want := range []string{
		`id="conditionScopeState"`, `data-i18n="conditionScopeDefault"`,
		`id="makeConditionsDefault"`, `data-i18n="makeConditionsDefault"`,
		`id="resetConditionsDefault"`, `data-i18n="resetConditionsDefault"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	for _, want := range []string{
		"globalThis.AyameConditionScope",
		"CONDITION_DEFAULTS_KEY",
		"localStorage.getItem(CONDITION_DEFAULTS_KEY)",
		"localStorage.setItem(CONDITION_DEFAULTS_KEY",
		"makeConditionsDefault",
		"resetConditionsToDefault",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	for _, want := range []string{"module.exports = api", "STORAGE_KEY", "function resolve("} {
		if !strings.Contains(module, want) {
			t.Errorf("conditionscope.js is missing %q", want)
		}
	}
	// The module holds rules, not wiring: precedence has to be testable without
	// a page, a DOM, or browser storage.
	for _, leaked := range []string{"document.", "addEventListener", "localStorage", "$("} {
		if strings.Contains(module, leaked) {
			t.Errorf("conditionscope.js contains application wiring (%q); it must stay pure", leaked)
		}
	}
}
