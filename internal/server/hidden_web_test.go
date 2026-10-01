package server

import (
	"strings"
	"testing"
)

// TestHiddenLabelsAreWiredIntoTheResult covers the DOM/API boundary of #268:
// the collapsed unchanged bar must name what it hides, and the pure module that
// decides the text has to be the one the page actually loads. The range/key
// arithmetic runs under node --test and the context API has handler tests.
func TestHiddenLabelsAreWiredIntoTheResult(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	if !strings.Contains(index, `<script src="hidden.js"></script>`) {
		t.Fatal("index.html does not load hidden.js")
	}
	if strings.Index(index, `src="hidden.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("hidden.js loads after app.js, which consumes it")
	}

	module := readWebAsset(t, "hidden.js")
	for _, want := range []string{
		"module.exports = api",
		"function lineRange(",
		"function describeGap(",
		"function keyRange(",
		"function keyIndexes(",
	} {
		if !strings.Contains(module, want) {
			t.Errorf("hidden.js missing %q", want)
		}
	}
	for _, forbidden := range []string{"document.", "fetch(", "localStorage"} {
		if strings.Contains(module, forbidden) {
			t.Errorf("hidden.js contains browser state %q; node must exercise the same pure summary", forbidden)
		}
	}

	app := readWebAsset(t, "app.js")
	for _, want := range []string{
		"globalThis.AyameHidden",
		"describeGap(",
		"async function loadHiddenPreviews(",
		"region.previews",
		"function csvShownKeyRange(",
		"function csvRowsKeyRange(",
		`t("csvShownKeyRange"`,
		`t("csvPageKeyRange"`,
		"loadHiddenPreviews()",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	if strings.Contains(app, "function describeGap(") {
		t.Error("app.js duplicates the summary instead of using the node-tested module")
	}
}

// TestHiddenLabelKeysExistInBothLanguages guards the two new strings against a
// one-language edit; the parity test would catch a missing key too, but this
// names the feature so the failure is readable.
func TestHiddenLabelKeysExistInBothLanguages(t *testing.T) {
	t.Parallel()

	catalog := readWebAsset(t, "i18n.js")
	for _, key := range []string{"contextHiddenRange", "contextHiddenRangeSides", "csvShownKeyRange", "csvPageKeyRange"} {
		if got := strings.Count(catalog, key+":"); got < 2 {
			t.Errorf("i18n.js defines %s %d time(s); both ja and en tables need it", key, got)
		}
	}
}
