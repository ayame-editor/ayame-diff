package server

import (
	"strings"
	"testing"
)

// TestComparisonScopeModuleIsWired keeps the classification that #260 extracted
// connected to the page. comparisonscope.js is pure data and helpers: node
// exercises it directly, so the browser side only needs to load it before
// app.js consumes it.
func TestComparisonScopeModuleIsWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "comparisonscope.js")

	if !strings.Contains(index, `<script src="comparisonscope.js"></script>`) {
		t.Error("index.html does not load comparisonscope.js")
	}
	if strings.Index(index, `src="comparisonscope.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("comparisonscope.js must load before app.js")
	}
	if !strings.Contains(app, "globalThis.AyameComparisonScope") {
		t.Error("app.js does not consume the extracted condition-scope module")
	}

	// The module holds a classification and pure helpers, never DOM or storage.
	for _, leaked := range []string{"document.", "localStorage", "addEventListener", "$("} {
		if strings.Contains(module, leaked) {
			t.Errorf("comparisonscope.js contains application wiring (%q); it must stay pure", leaked)
		}
	}
	if !strings.Contains(module, "module.exports = api") {
		t.Error("comparisonscope.js has no CommonJS export, so node --test cannot require it")
	}
	for _, want := range []string{
		"const GLOBAL_PREFERENCES",
		"const COMPARISON_CONTROLS",
		"function globalPreference(",
		"function comparisonKey(",
		"function conditionSnapshot(",
		"function parseStore(",
		"function serializeStore(",
		"function readConditions(",
		"function writeConditions(",
	} {
		if !strings.Contains(module, want) {
			t.Errorf("comparisonscope.js is missing %q", want)
		}
	}
}

// TestWordHighlightIsPersisted is the first half of #260: word highlight was
// the one display toggle that reset on reload while wrap, syntax and whitespace
// survived.
func TestWordHighlightIsPersisted(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "comparisonscope.js")

	if !strings.Contains(app, `localStorage.setItem("ayame-word"`) {
		t.Error("toggling word highlight does not persist it")
	}
	if !strings.Contains(app, `localStorage.getItem("ayame-word")`) {
		t.Error("word highlight is not restored on load")
	}
	// The key lives in the classification too, so the display set is the source
	// of truth rather than scattered across app.js.
	if !strings.Contains(module, `key: "ayame-word"`) {
		t.Error("comparisonscope.js does not classify word highlight as a display preference")
	}
}

// TestComparisonConditionsAreScopedPerComparison is the second half of #260:
// the per-comparison controls are resolved before a request is built, remembered
// under the comparison's identity, and restored from there instead of being
// inherited by the next comparison.
func TestComparisonConditionsAreScopedPerComparison(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "comparisonscope.js")

	// One list drives both the URL state (#254) and the local memory (#260).
	if !strings.Contains(app, "const URL_STATE_CONTROL_IDS = COMPARISON_CONTROLS;") {
		t.Error("app.js still declares its own comparison-control list instead of using the classification")
	}
	for _, want := range []string{
		`const CONDITION_STORE_KEY = "ayame-conditions"`,
		"function currentComparisonIdentity(",
		"return comparisonKey(mode, {",
		"function adoptCurrentComparison(",
		"function resetConditions(",
		"function storeComparisonState(",
		"function scopeConditionsForRun(",
		"await scopeConditionsForRun(Boolean(options.keepConditions))",
		"storeComparisonState(conditionKey, captureComparisonState())",
		"await applyComparisonState(remembered)",
		"activeComparisonKey",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing per-comparison scoping %q", want)
		}
	}
	// The default snapshot has to be taken before any restore can disturb the
	// controls, so a comparison with no memory resets rather than inherits.
	if !strings.Contains(app, "captureConditionDefaults();") {
		t.Error("app.js does not capture the per-comparison defaults")
	}
	// Swapping sides is the same comparison reversed (#90), not a new scope.
	if !strings.Contains(app, "compare({ keepConditions: true })") {
		t.Error("swapping sides resets the conditions as though it were a different comparison")
	}
	for _, want := range []string{"function comparisonKey(", "function conditionSnapshot("} {
		if !strings.Contains(module, want) {
			t.Errorf("comparisonscope.js is missing the pure helper %q", want)
		}
	}
}
