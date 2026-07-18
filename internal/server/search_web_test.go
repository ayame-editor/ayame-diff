package server

import (
	"strings"
	"testing"
)

// TestResultSearchExists covers the #118 feature surface: a search bar over the
// rendered result, reachable with Ctrl+F, with a count and stepping.
func TestResultSearchExists(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	for _, id := range []string{
		`id="resultSearch"`, `id="searchInput"`, `id="searchCounter"`,
		`id="searchNext"`, `id="searchPrev"`, `id="searchClose"`,
		`id="searchCase"`, `id="searchRegex"`, `id="searchChangedOnly"`,
	} {
		if !strings.Contains(index, id) {
			t.Errorf("index.html is missing %s", id)
		}
	}
	app := readWebAsset(t, "app.js")
	for _, fn := range []string{"function runSearch()", "function stepSearch(", "function clearSearchHits()"} {
		if !strings.Contains(app, fn) {
			t.Errorf("app.js is missing %s", fn)
		}
	}
	// Ctrl+F must only be taken over when there is something to search, so the
	// browser's own find still works on an empty page.
	if !strings.Contains(app, `$("result").children.length`) {
		t.Error("Ctrl+F is intercepted unconditionally")
	}
	style := readWebAsset(t, "style.css")
	for _, cls := range []string{".search-hit", ".search-hit.current", ".result-search"} {
		if !strings.Contains(style, cls) {
			t.Errorf("style.css is missing %s", cls)
		}
	}
}

// TestSearchMatchesWholeLineNotTextNodes pins the bug found while verifying:
// syntax highlighting and word-diff markup split a line into many text nodes,
// so scanning them individually made anchors and cross-token patterns silently
// fail. Matching runs against the container's whole text and the ranges are
// mapped back onto the nodes.
func TestSearchMatchesWholeLineNotTextNodes(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	body := renderFunctionBody(t, app, "function markMatchesIn(")
	if !strings.Contains(body, "cell.textContent") {
		t.Error("matching does not run against the whole line, so anchors and cross-token patterns fail")
	}
	if !strings.Contains(body, "createTreeWalker") {
		t.Error("matches are not mapped back onto text nodes")
	}
	// A zero-length match would spin forever.
	if !strings.Contains(body, `match[0] === ""`) {
		t.Error("a zero-length match is not guarded against")
	}
}

// TestSearchSkipsTheLineNumberGutter pins the other bug found while verifying:
// searching the whole cell made a numeric query light up line numbers.
func TestSearchSkipsTheLineNumberGutter(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	body := renderFunctionBody(t, app, "function searchableCells()")
	if !strings.Contains(body, ".cell .tx") {
		t.Error("search targets the whole cell, so it would match the line-number gutter")
	}
	if strings.Contains(body, `"#result .cell,`) {
		t.Error("search still includes bare .cell, which contains the line number")
	}
}

// TestSearchIsBoundedAndDebounced keeps the feature inside the render budget
// #127 established: every match inserts a node into the diff.
func TestSearchIsBoundedAndDebounced(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "const SEARCH_MAX_HITS") {
		t.Error("the number of inserted match markers is unbounded")
	}
	schedule := renderFunctionBody(t, app, "function scheduleSearch()")
	if !strings.Contains(schedule, "setTimeout") || !strings.Contains(schedule, "clearTimeout") {
		t.Error("typing re-scans the whole result without a debounce")
	}
	// A re-render replaces the nodes the hits point at.
	slices := renderFunctionBody(t, app, "async function renderInSlices(")
	if !strings.Contains(slices, "clearSearchHits()") {
		t.Error("a re-render leaves stale search hits pointing at detached nodes")
	}
}
