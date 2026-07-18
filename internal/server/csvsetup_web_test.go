package server

import (
	"regexp"
	"strings"
	"testing"
)

// TestCSVSetupIsProgressivelyDisclosed is the #93 acceptance check: opening the
// CSV mode used to present about 30 controls at once, seventeen of them engine
// tuning a first comparison never needs.
//
// The count is derived from the markup rather than a browser, so it tracks the
// file that would regress: any control added outside a <details> shows up here.
func TestCSVSetupIsProgressivelyDisclosed(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	setup := sectionBetween(t, index, `<fieldset id="csvOptions"`, "</fieldset>")

	// Strip the collapsed groups; what remains is what a first visit shows.
	collapsed := regexp.MustCompile(`(?s)<details.*?</details>`)
	visible := collapsed.ReplaceAllString(setup, "")
	controls := regexp.MustCompile(`<(input|select)\b`).FindAllString(visible, -1)
	if len(controls) > 10 {
		t.Errorf("%d controls are visible on first open; #93 requires 10 or fewer:\n%s", len(controls), visible)
	}

	// The performance group must not be open by default — that was the specific
	// complaint, seventeen tuning controls expanded on arrival.
	if regexp.MustCompile(`<details[^>]*\bopen\b`).MatchString(setup) {
		t.Error("a setup group is still open by default")
	}
	if !strings.Contains(setup, `id="csvAdvanced"`) {
		t.Error("the performance group is not a collapsible section")
	}
}

// TestCollapsedSetupGroupsReportChanges keeps collapsing from hiding a setting
// someone actually changed: each group's header carries a count of controls
// differing from their captured defaults.
func TestCollapsedSetupGroupsReportChanges(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	for _, id := range []string{`id="csvAdvancedBadge"`, `id="csvParsingBadge"`} {
		if !strings.Contains(index, id) {
			t.Errorf("index.html is missing %s", id)
		}
	}
	app := readWebAsset(t, "app.js")
	for _, fn := range []string{
		"function captureControlDefaults(",
		"function changedControlCount(",
		"function updateDetailsBadges(",
	} {
		if !strings.Contains(app, fn) {
			t.Errorf("app.js is missing %s", fn)
		}
	}
	// Defaults must be snapshotted rather than hardcoded per control, or a
	// changed default would silently start reporting every control as changed.
	if !strings.Contains(app, "defaultControlValues") {
		t.Error("defaults are not captured from the markup")
	}
	if !strings.Contains(app, `control.addEventListener("change", updateDetailsBadges)`) {
		t.Error("badges do not refresh when a control changes")
	}
	style := readWebAsset(t, "style.css")
	if !strings.Contains(style, ".details-badge") {
		t.Error("style.css has no badge style")
	}
}

// sectionBetween returns the text from the first occurrence of start up to the
// following end marker.
func sectionBetween(t *testing.T, source, start, end string) string {
	t.Helper()
	from := strings.Index(source, start)
	if from < 0 {
		t.Fatalf("%q not found", start)
	}
	rest := source[from:]
	to := strings.Index(rest, end)
	if to < 0 {
		t.Fatalf("%q not found after %q", end, start)
	}
	return rest[:to]
}
