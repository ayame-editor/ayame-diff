package server

import (
	"regexp"
	"strings"
	"testing"
)

// TestUnifiedViewIsOfferedInTheResultToolbar covers #115: the diff was
// side-by-side only, even though patch output could already be written as
// unified. The switch belongs beside the result it changes (#91), not in the
// setup form.
func TestUnifiedViewIsOfferedInTheResultToolbar(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")

	// The view switch is one of the few controls that stays on the bar itself
	// rather than moving into a menu: it is flipped while reading a diff, which
	// is the line every surveyed tool draws.
	bar := sectionBetween(t, index, `<nav class="diff-nav" id="diffNav"`, `<div class="menubar"`)
	if !strings.Contains(bar, `id="viewMode"`) {
		t.Error("the view switch is not on the result toolbar")
	}
	// The rest are set once and left alone, so they belong behind a menu. Loose
	// on the bar they wrapped it to two ragged rows and buried the navigation
	// the bar exists for.
	for _, id := range []string{"theme", "scheme", "showWs", "syntax", "word", "wrap"} {
		if strings.Contains(bar, `id="`+id+`"`) {
			t.Errorf("%s is loose on the toolbar instead of in a menu", id)
		}
		if !strings.Contains(sectionBetween(t, index, `<div class="menu-panel">`, `</details>`), `id="`+id+`"`) &&
			!strings.Contains(index, `id="`+id+`"`) {
			t.Errorf("%s went missing entirely", id)
		}
	}
	for _, option := range []string{`value="side"`, `value="unified"`} {
		if !strings.Contains(bar, option) {
			t.Errorf("the view switch is missing %s", option)
		}
	}

	app := readWebAsset(t, "app.js")
	apply := renderFunctionBody(t, app, "function applyViewMode(")
	if !strings.Contains(apply, `classList.toggle("unified"`) {
		t.Error("choosing unified does not reach the result")
	}
	// A view preference that resets every visit is a per-visit chore; git users
	// want unified to stick.
	if !strings.Contains(apply, `localStorage.setItem("ayame-view"`) {
		t.Error("the chosen view is not remembered")
	}
	if !strings.Contains(app, `applyViewMode(localStorage.getItem("ayame-view")`) {
		t.Error("the remembered view is not restored on load")
	}
}

// TestUnifiedViewMarksRemovalsAndAdditions checks the -/+ prefixes. A changed
// pair is two cells sharing the "chg" class, so the side is what separates the
// removal from the addition.
func TestUnifiedViewMarksRemovalsAndAdditions(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	cell := renderFunctionBody(t, app, "function cell(")
	if !strings.Contains(cell, `dataset.marker = "+"`) || !strings.Contains(cell, `dataset.marker = "-"`) {
		t.Fatal("cells carry no unified marker")
	}
	if !strings.Contains(cell, `cls === "chg" && side === "new"`) {
		t.Error("a changed line's addition is not distinguished from its removal")
	}

	style := readWebAsset(t, "style.css")
	// The marker is a pseudo-element so that side-by-side pays no DOM cost for
	// it (#127); .cell is itself a grid, so it needs its own column or it drops
	// onto a second line.
	marker := sectionBetween(t, style, ".result.unified .cell[data-marker]::before {", "}")
	if !strings.Contains(marker, "attr(data-marker)") {
		t.Error("the -/+ marker is not printed from the cell")
	}
	if !strings.Contains(marker, "grid-column: 2") {
		t.Error("the marker is not placed in its own column and will wrap")
	}
	track := sectionBetween(t, style, ".result.unified .cell[data-marker] {", "}")
	if !strings.Contains(track, "var(--diff-marker)") {
		t.Errorf("the marker has no column of its own: %s", track)
	}
}

// TestDiffGridMetricsComeFromTokens: the base cell and the unified override
// both declare the same grid, so a literal in either one is a silent drift —
// change the gutter in one place and unified stops lining up, with nothing to
// catch it. Every rule that sets those tracks must read the shared variable.
func TestDiffGridMetricsComeFromTokens(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")

	for _, name := range []string{"--diff-gutter", "--diff-gap", "--diff-marker"} {
		if !strings.Contains(style, name+":") {
			t.Errorf("%s is not defined", name)
		}
	}

	// Every --diff-* that is read must also be declared. A misspelled custom
	// property is not a CSS error: it resolves to nothing, the track silently
	// collapses, and the layout is quietly wrong with no console warning.
	used := regexp.MustCompile(`var\((--diff-[a-z-]+)\)`)
	for _, match := range used.FindAllStringSubmatch(style, -1) {
		if !strings.Contains(style, match[1]+":") {
			t.Errorf("style.css reads %s but never defines it; the track will collapse silently", match[1])
		}
	}

	// "\n.cell {" anchors the base rule at column 0; a bare ".cell {" would
	// match the ".result.unified .cell {" override that appears earlier.
	for _, rule := range []string{"\n.cell {", ".result.unified .cell[data-marker] {"} {
		body := sectionBetween(t, style, rule, "}")
		columns := ""
		for _, line := range strings.Split(body, "\n") {
			if strings.Contains(line, "grid-template-columns") {
				columns = line
			}
		}
		if columns == "" {
			t.Errorf("%s no longer declares grid-template-columns", rule)
			continue
		}
		// "1fr" is a ratio rather than a metric, so it is not a drift risk.
		for _, unit := range []string{"rem", "ch", "px", "em"} {
			if strings.Contains(columns, unit) && !strings.Contains(columns, "var(--diff-") {
				t.Errorf("%s hard-codes a %s track instead of using the shared variable: %s",
					rule, unit, strings.TrimSpace(columns))
			}
		}
	}
}

// TestUnifiedViewCollapsesToOneColumn is the layout half of #115. Nothing is
// re-rendered when the view changes: a changed row already holds both cells, so
// one column stacks them old-above-new, the order a patch has.
func TestUnifiedViewCollapsesToOneColumn(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")

	row := sectionBetween(t, style, ".result.unified .row {", "}")
	if !strings.Contains(row, "grid-template-columns: 1fr") {
		t.Errorf("unified rows are still two columns: %s", row)
	}
	// The placeholder opposite an insert or a delete has nothing to say once
	// there is only one column.
	if !strings.Contains(style, ".result.unified .cell.empty { display: none; }") {
		t.Error("the empty placeholder still takes a line in unified view")
	}

	// Word highlight, syntax, whitespace and wrap must keep working, which they
	// do only because no separate rendering path exists.
	app := readWebAsset(t, "app.js")
	if strings.Contains(app, "function renderHunkUnified(") {
		t.Error("a second rendering path exists; the views will drift apart")
	}
}

// TestViewSwitchHidesWhereItDoesNothing: three-way is three panes and CSV is a
// table, neither built from .row, so the switch would be a dead control there
// (the #124 rule).
func TestViewSwitchHidesWhereItDoesNothing(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	body := renderFunctionBody(t, app, "function syncViewModeVisibility(")
	if !strings.Contains(body, `$("mode").value === "text"`) {
		t.Error("the view switch is offered outside the two-way text diff")
	}
	if !strings.Contains(body, "lastData?.hunks?.length") {
		t.Error("the view switch shows before there is a diff to switch")
	}
	// It has to run on every render, and the hunks arrive asynchronously, so a
	// DOM probe here would read an empty result.
	if strings.Contains(body, "querySelector") {
		t.Error("visibility is decided by probing a DOM that is not built yet")
	}
	if !strings.Contains(renderFunctionBody(t, app, "function syncExportPatchVisibility("), "syncViewModeVisibility()") {
		t.Error("the view switch visibility is never refreshed after a render")
	}
}
