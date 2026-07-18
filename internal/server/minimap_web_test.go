package server

import (
	"strings"
	"testing"
)

// TestMinimapHidesWhenTheResultDoesNotScroll is the #102 regression. The map
// was shown whenever a diff had hunks, so a one-line change rendered a tall
// empty bar beside it — a control that navigated nothing. Visibility must
// depend on the result actually overflowing the viewport.
func TestMinimapHidesWhenTheResultDoesNotScroll(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	// The decision has to live in updateMinimapViewport: setupNavigation runs
	// before the hunks are in the DOM, so nothing can be measured there.
	if strings.Contains(app, `$("minimap").hidden = !hasHunks`) {
		t.Error("visibility is still decided at build time, before the hunks exist")
	}
	if !strings.Contains(app, "minimapHasMarkers") {
		t.Fatal("app.js does not track whether the minimap has markers")
	}
	viewport := functionBody(t, app, "function updateMinimapViewport()")
	if !strings.Contains(viewport, "window.innerHeight") || !strings.Contains(viewport, "map.hidden =") {
		t.Error("updateMinimapViewport does not decide visibility from the viewport height")
	}
	if !strings.Contains(viewport, "minimapHasMarkers") {
		t.Error("updateMinimapViewport ignores whether any markers were built")
	}

	// Resetting a result must clear the marker state, or a later scroll frame
	// would bring the map back over stale markers.
	resets := strings.Count(app, "minimapHasMarkers = false")
	if resets < 2 {
		t.Errorf("minimapHasMarkers is cleared %d times; both result-reset paths must clear it", resets)
	}
}

// TestMinimapColumnCollapsesWhenHidden keeps a short diff from being indented
// by a gutter that leads to nothing.
func TestMinimapColumnCollapsesWhenHidden(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")
	if !strings.Contains(style, ".diff-layout:has(.minimap[hidden])") {
		t.Error("style.css keeps the 18px minimap column reserved when the map is hidden")
	}
}

// functionBody returns the source of the function starting at header, up to its
// closing brace at column 0, so an assertion can be scoped to one function.
func functionBody(t *testing.T, source, header string) string {
	t.Helper()
	start := strings.Index(source, header)
	if start < 0 {
		t.Fatalf("%q not found", header)
	}
	rest := source[start:]
	if end := strings.Index(rest, "\n}"); end >= 0 {
		return rest[:end]
	}
	return rest
}
