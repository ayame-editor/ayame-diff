package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMergeRowsAreIndexedNotSearched is the #154 regression. Both merge UIs
// found their rows with a document-wide attribute selector, once per event, on
// every merge click. A three-way result has no cap on its event count, so a
// file with thousands of conflicts cost thousands of full-document scans per
// click: measured at 661ms per click for 3,000 events, against 4.5ms once the
// rows are indexed at render time.
func TestMergeRowsAreIndexedNotSearched(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	if !strings.Contains(app, "let mergeRowIndex = new Map()") {
		t.Fatal("no merge row index")
	}
	for _, fn := range []string{"function updateThreeWayMergeUI()", "function updateCSVMergeUI()"} {
		body := renderFunctionBody(t, app, fn)
		if strings.Contains(body, "document.querySelector") {
			t.Errorf("%s still searches the document for its rows", fn)
		}
		if !strings.Contains(body, "mergeRowIndex") {
			t.Errorf("%s does not use the index", fn)
		}
	}
	// The index must be filled where the rows are built and dropped where they
	// are replaced, or it would point at detached nodes.
	if got := strings.Count(app, "indexMergeRow("); got < 3 {
		t.Errorf("indexMergeRow appears %d times; the helper plus both render paths are expected", got)
	}
	if got := strings.Count(app, "resetMergeRowIndex()"); got < 3 {
		t.Errorf("resetMergeRowIndex appears %d times; the helper plus both render paths are expected", got)
	}
}

// TestMergeToggleModelIsWired is the #271 wiring check: the merge UI must adopt
// the ordered multi-side selection module, so a hunk can hold more than one
// side and clicking a chosen side clears it.
func TestMergeToggleModelIsWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "mergeselect.js")

	if !strings.Contains(index, `<script src="mergeselect.js"></script>`) {
		t.Fatal("index.html does not load mergesect.js, so the tested module is not the one the page uses")
	}
	if strings.Index(index, `src="mergeselect.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("mergeselect.js loads after app.js, which destructures it at parse time")
	}
	if !strings.Contains(app, "globalThis.AyameMergeSelect") {
		t.Error("app.js does not consume the selection module")
	}
	// The single-side Map and its default choice are gone: both would make the
	// toggle model impossible.
	for _, gone := range []string{"mergeChoices", "mergeDefault", "new Map(mergeChoices)"} {
		if strings.Contains(app, gone) {
			t.Errorf("app.js still references %q; the single-side merge model should be gone", gone)
		}
	}
	// Clicking a chosen side must clear it, not overwrite.
	choose := renderFunctionBody(t, app, "function chooseMerge(")
	if !strings.Contains(choose, "mergeSelection.toggle(") {
		t.Error("chooseMerge does not toggle the side, so deselecting is not symmetric")
	}
	for _, want := range []string{
		`$("allLeft").addEventListener("click", chooseAllMerge("left"))`,
		`$("allRight").addEventListener("click", chooseAllMerge("right"))`,
		`$("allBase").addEventListener("click", chooseAllMerge("base"))`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing all-side wiring %q", want)
		}
	}
	if !strings.Contains(app, "mergeSelection.unresolved(") {
		t.Error("the unresolved counter is not derived from the selection module")
	}
	if !strings.Contains(app, "mergeSelection.toWire()") {
		t.Error("the save paths do not send the ordered selection")
	}
	// The module stays pure so node --test can require it.
	if !strings.Contains(module, "module.exports = api") {
		t.Error("mergeselect.js has no CommonJS export")
	}
	for _, leaked := range []string{"document.", "addEventListener", "localStorage", "mergeSelection"} {
		if strings.Contains(module, leaked) {
			t.Errorf("mergeselect.js contains application wiring (%q); it must stay pure", leaked)
		}
	}
	if _, err := os.Stat(filepath.Join("web", "test", "mergeselect.test.js")); err != nil {
		t.Errorf("no node test for the selection module: %v", err)
	}
}

// TestColumnFilterIsCachedAndDebounced covers the key-column search, which ran
// a full pass per keystroke, re-reading and re-lowercasing every label's text.
func TestColumnFilterIsCachedAndDebounced(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	if !strings.Contains(app, "let columnFilterIndex") {
		t.Fatal("the column filter has no cache")
	}
	filter := renderFunctionBody(t, app, "function filterColumns()")
	if !strings.Contains(filter, "setTimeout") || !strings.Contains(filter, "clearTimeout") {
		t.Error("filterColumns is not debounced, so a burst of typing costs one full pass per keystroke")
	}
	apply := renderFunctionBody(t, app, "function applyColumnFilter()")
	if strings.Contains(apply, "querySelectorAll") {
		t.Error("applyColumnFilter re-queries the labels instead of using the cache")
	}
	if !strings.Contains(apply, "entry.label.hidden !== hidden") {
		t.Error("applyColumnFilter writes hidden unconditionally, dirtying layout for unchanged labels")
	}
	// The cache has to be rebuilt with the list it describes.
	selection := renderFunctionBody(t, app, "function renderColumnSelection(")
	if !strings.Contains(selection, "buildColumnFilterIndex()") {
		t.Error("the cache is not rebuilt when the column list is")
	}
}
