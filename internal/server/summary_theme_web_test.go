package server

import (
	"strings"
	"testing"
)

// TestSummaryCountsJumpToTheirDifferences covers #110: the counts named the
// differences but could not reach them, so finding "the deleted lines" meant
// scrolling.
func TestSummaryCountsJumpToTheirDifferences(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	if !strings.Contains(app, "function jumpToKind(") {
		t.Fatal("app.js has no jumpToKind")
	}
	summary := renderFunctionBody(t, app, "function renderSummary(")
	for _, kind := range []string{`"insert"`, `"delete"`, `"replace"`} {
		if !strings.Contains(summary, kind) {
			t.Errorf("the summary does not make %s jumpable", kind)
		}
	}
	// A zero count must stay inert rather than becoming a dead button.
	if !strings.Contains(summary, "kind && n > 0") {
		t.Error("a zero count still renders as a button")
	}

	jump := renderFunctionBody(t, app, "function jumpToKind(")
	// Repeated clicks must walk the group, not stick on the first match.
	if !strings.Contains(jump, "lastJumpIndex") {
		t.Error("jumpToKind does not remember where it left off, so it cannot advance")
	}
	if !strings.Contains(jump, "ignoredHunks.has(i)") {
		t.Error("jumpToKind would jump to a hunk the user chose to ignore")
	}
	if !strings.Contains(jump, "matches[0]") {
		t.Error("jumpToKind does not wrap around at the end of the group")
	}
}

// TestThemeCanBeChosenIndependentlyOfTheOS covers #106: dark mode followed the
// system only, so the choice could not be made per app.
func TestThemeCanBeChosenIndependentlyOfTheOS(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	if !strings.Contains(index, `id="theme"`) {
		t.Fatal("index.html has no theme selector")
	}
	for _, value := range []string{`value="system"`, `value="light"`, `value="dark"`} {
		if !strings.Contains(index, value) {
			t.Errorf("the theme selector is missing %s", value)
		}
	}

	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "function applyTheme(") {
		t.Fatal("app.js has no applyTheme")
	}
	body := renderFunctionBody(t, app, "function applyTheme(")
	// "system" must remove the attribute, or the OS rules can never apply again.
	if !strings.Contains(body, "removeAttribute") {
		t.Error("applyTheme cannot return to following the system")
	}
	if !strings.Contains(app, `localStorage.getItem("ayame-theme")`) {
		t.Error("the theme choice is not restored on load")
	}

	tokens := readWebAsset(t, "tokens.css")
	if !strings.Contains(tokens, `:root[data-theme="dark"]`) {
		t.Error("tokens.css has no explicit dark theme, so choosing dark on a light desktop would do nothing")
	}
	// The crucial part: an explicit choice has to beat the media query, or
	// "light" would be overridden on a dark desktop.
	if !strings.Contains(tokens, `:root:not([data-theme])`) {
		t.Error("the OS preference is not scoped to the follow-the-system case, so an explicit light theme loses to a dark desktop")
	}
}
