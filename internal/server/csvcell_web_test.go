package server

import (
	"strings"
	"testing"
)

// TestLongCSVCellsCanBeOpened covers #105: a value past the column's max-width
// was readable only as a hover tooltip, so on a touch screen it could not be
// read at all, and a change hidden past the ellipsis was invisible.
func TestLongCSVCellsCanBeOpened(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	if !strings.Contains(app, "function makeCellExpandable(") {
		t.Fatal("app.js cannot expand a truncated cell")
	}
	body := renderFunctionBody(t, app, "function makeCellExpandable(")
	if !strings.Contains(body, `td.classList.toggle("expanded")`) {
		t.Error("clicking a cell does not open it")
	}
	// The copy button sits inside the cell, so its click must not also toggle.
	if !strings.Contains(body, "event.stopPropagation()") {
		t.Error("the copy button also toggles the cell it lives in")
	}
	// Clipboard access can be refused; the value must still be obtainable.
	if !strings.Contains(body, "selectNodeContents") {
		t.Error("a refused clipboard leaves no way to get the value")
	}

	style := readWebAsset(t, "style.css")
	for _, rule := range []string{".csv-table td.expanded", ".csv-table.wrap-cells td", ".csv-cell-copy"} {
		if !strings.Contains(style, rule) {
			t.Errorf("style.css is missing %s", rule)
		}
	}
	// Expanding must undo the truncation, not merely widen the cell.
	expanded := sectionBetween(t, style, ".csv-table td.expanded, .csv-table.wrap-cells td {", "}")
	if !strings.Contains(expanded, "pre-wrap") {
		t.Error("an expanded cell still uses nowrap, so the value stays cut off")
	}

	// The existing wrap toggle should reach the table too, not just the diff.
	wrap := renderFunctionBody(t, app, "function applyWrap(")
	if !strings.Contains(wrap, "wrap-cells") {
		t.Error("the wrap toggle does not apply to CSV cells")
	}
}

// TestMinimapIsWideEnoughToHit covers the reachable half of #102. The strip was
// 18px, below the ~24px a pointer can reliably target.
func TestMinimapIsWideEnoughToHit(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")
	layout := sectionBetween(t, style, ".diff-layout {", "}")
	if strings.Contains(layout, "18px") {
		t.Error("the minimap column is still 18px wide")
	}
	if !strings.Contains(layout, "28px") {
		t.Errorf("unexpected minimap column width: %s", layout)
	}
	narrowStart := strings.Index(style, "@media (max-width: 720px) {")
	if narrowStart < 0 {
		t.Fatal("style.css has no narrow-layout breakpoint")
	}
	narrow := style[narrowStart:]
	if strings.Contains(narrow, "12px") || !strings.Contains(narrow, "28px") {
		t.Errorf("narrow layout shrinks the minimap hit target: %s", narrow)
	}
}
