package server

import (
	"strings"
	"testing"
)

// TestCSVPagingReplacesRowsOnly guards the last part of #154. A page turn
// changes a hundred rows; it used to discard the pane headers, the summary, the
// column headers and the pager to do it, and then hunt the focused button back
// out of the document it had just rebuilt.
func TestCSVPagingReplacesRowsOnly(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "csvview.js")

	if !strings.Contains(index, `<script src="csvview.js"></script>`) {
		t.Error("index.html does not load csvview.js")
	}
	if strings.Index(index, `src="csvview.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("csvview.js must load before app.js")
	}
	if strings.Contains(module, "document.") {
		t.Error("csvview.js touches the DOM; it must stay runnable without one")
	}

	for _, want := range []string{
		"globalThis.AyameCSVView",
		"function showCSVPage(",
		"function renderCSVRows(",
		"function renderCSVColumns(",
		"function syncCSVPager(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing paging wiring %q", want)
		}
	}

	// Paging replaces the rows. Anything that rebuilds the result from scratch
	// here is the regression this test exists for.
	rows := renderFunctionBody(t, app, "function renderCSVRows(")
	for _, forbidden := range []string{"renderCSVSummary(", "paneHeads(", `result.innerHTML = ""`, "csv-pages"} {
		if strings.Contains(rows, forbidden) {
			t.Errorf("a page turn rebuilds %q", forbidden)
		}
	}
	if !strings.Contains(rows, "tbody.textContent = \"\"") {
		t.Error("a page turn does not replace the rows in place")
	}
	if !strings.Contains(rows, "resetMergeRowIndex()") || !strings.Contains(rows, "indexMergeRow(") {
		t.Error("the merge row index does not follow the rows it points at")
	}

	// The pager keeps its nodes, so the button that was clicked keeps focus.
	pager := renderFunctionBody(t, app, "function syncCSVPager(")
	for _, want := range []string{"prev.disabled = state.atFirst", "next.disabled = state.atLast"} {
		if !strings.Contains(pager, want) {
			t.Errorf("the pager state is not applied in place: missing %q", want)
		}
	}
	if strings.Contains(app, `rerenderAndFocus`) {
		t.Error("paging still re-renders and then hunts the focus back")
	}

	// Toggling the columns changes the columns, not the whole result.
	if !strings.Contains(app, "renderCSVColumns();\n  restoreResultScrollAnchor(scrollAnchor, true);") {
		t.Error("the changed-columns toggle no longer rebuilds only the columns")
	}
}

// The three defects fixed earlier under the same issue must stay fixed: neither
// merge UI may go back to searching the whole document, and the column filter
// keeps its debounce and its cached nodes.
func TestCSVAndMergeUIStayOffTheDocument(t *testing.T) {
	t.Parallel()

	app := readWebAsset(t, "app.js")
	for _, function := range []string{"function updateCSVMergeUI(", "function updateThreeWayMergeUI("} {
		body := renderFunctionBody(t, app, function)
		if strings.Contains(body, "document.querySelector") {
			t.Errorf("%s searches the whole document again", function)
		}
		if !strings.Contains(body, "mergeRowIndex") {
			t.Errorf("%s no longer uses the row index", function)
		}
	}
	filter := renderFunctionBody(t, app, "function filterColumns(")
	if !strings.Contains(filter, "clearTimeout(columnFilterTimer)") || !strings.Contains(filter, "setTimeout(applyColumnFilter") {
		t.Error("the column search lost its debounce")
	}
	apply := renderFunctionBody(t, app, "function applyColumnFilter(")
	if strings.Contains(apply, "querySelectorAll") {
		t.Error("the column search scans the document on every application again")
	}
}
