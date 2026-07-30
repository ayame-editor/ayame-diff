package server

import (
	"strings"
	"testing"
)

// TestMinimapHidesWhenTheResultDoesNotScroll is the #102 regression. The map
// was shown whenever a diff had hunks, so a one-line change rendered a tall
// empty bar beside it — a control that navigated nothing. Visibility must
// depend on the result pane actually overflowing.
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
	for _, dimension := range []string{"result.scrollTop", "result.scrollHeight", "result.clientHeight"} {
		if !strings.Contains(viewport, dimension) {
			t.Errorf("updateMinimapViewport does not use %s", dimension)
		}
	}
	if strings.Contains(viewport, "window.innerHeight") || strings.Contains(viewport, "getBoundingClientRect") {
		t.Error("updateMinimapViewport still measures the window instead of the result scroll container")
	}
	if !strings.Contains(viewport, "map.hidden =") {
		t.Error("updateMinimapViewport does not decide minimap visibility")
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

func TestMinimapViewportIsInteractive(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")

	for _, attribute := range []string{
		`<script src="minimap.js"></script>`,
		`role="scrollbar"`,
		`tabindex="0"`,
		`aria-controls="result"`,
		`data-i18n-aria-label="visibleRange"`,
	} {
		if !strings.Contains(index, attribute) {
			t.Errorf("minimap viewport is missing %s", attribute)
		}
	}
	for _, behavior := range []string{
		`$("result").addEventListener("scroll"`,
		`$("minimap").addEventListener("pointerdown"`,
		`$("minimap").addEventListener("pointermove"`,
		`$("minimapViewport").addEventListener("keydown"`,
		"scrollTopForMinimapPointer",
	} {
		if !strings.Contains(app, behavior) {
			t.Errorf("app.js is missing minimap interaction %q", behavior)
		}
	}
	if !strings.Contains(style, "touch-action: none") || !strings.Contains(style, "cursor: grab") {
		t.Error("style.css does not expose the minimap drag affordance")
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
