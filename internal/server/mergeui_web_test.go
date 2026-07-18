package server

import (
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
