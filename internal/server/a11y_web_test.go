package server

import (
	"strings"
	"testing"
)

// TestDiffKindIsNotColourOnly is the #298 regression for the core complaint:
// a diff tool distinguishes add/delete/change with a colour wash alone. The kind
// must survive a colour-blind theme, a monochrome display and forced colours, so
// both views print a gutter glyph and every row carries its kind in data.
func TestDiffKindIsNotColourOnly(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")
	mark := readWebAsset(t, "diffmark.js")

	// The mapping itself lives in a pure module so it can be executed by
	// node --test; app.js must actually load and use it, not re-derive it.
	if !strings.Contains(index, `src="diffmark.js"`) {
		t.Fatal("index.html does not load diffmark.js")
	}
	if strings.Index(index, `src="diffmark.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("diffmark.js must load before the code that consumes it")
	}
	if !strings.Contains(app, "globalThis.AyameDiffMark") || !strings.Contains(app, "cellMarker,") {
		t.Error("app.js does not wire the shared marker module")
	}
	for _, want := range []string{"cellMarker", "cellKindKey", "hunkMarker", "hunkKindKey", "CONFLICT_MARK"} {
		if !strings.Contains(mark, "function "+want+"(") && !strings.Contains(mark, "const "+want+" ") {
			t.Errorf("diffmark.js does not export %s", want)
		}
	}

	cell := renderFunctionBody(t, app, "function cell(")
	for _, want := range []string{
		"cellMarker(cls, side)",     // the glyph, not a literal per call site
		"c.dataset.marker = marker", // readable by CSS and by tests
		"ln.dataset.marker = marker",
		"sr-only diff-kind", // the screen-reader kind name
		"c.dataset.kind = kindKey",
	} {
		if !strings.Contains(cell, want) {
			t.Errorf("cell() missing %q; the kind is colour-only", want)
		}
	}

	// Side-by-side gets the glyph inside the existing line-number gutter, so no
	// DOM node is added and the two-column grid is untouched (#127).
	side := sectionBetween(t, style, ".result:not(.unified) .cell > .ln[data-marker]::after {", "}")
	if !strings.Contains(side, "attr(data-marker)") {
		t.Error("side-by-side does not print the marker from data")
	}
	if strings.Contains(side, "grid-template-columns") {
		t.Error("the side-by-side marker must not re-lay-out the cell")
	}
	if !strings.Contains(style, "forced-colors: active") {
		t.Error("forced-colors mode has no rule keeping the glyph legible")
	}

	// The hunk header names its kind as text and carries a glyph; a replace is
	// "~" so it is not read as an unrelated delete plus insert.
	hunk := renderFunctionBody(t, app, "function renderHunk(")
	if !strings.Contains(hunk, "hunkMarker(h.kind)") || !strings.Contains(hunk, `box.dataset.kind`) {
		t.Error("a hunk header does not carry its kind without colour")
	}
	if !strings.Contains(hunk, `box.setAttribute("aria-label"`) {
		t.Error("a hunk does not expose its kind to assistive technology")
	}
	if !strings.Contains(app, "CONFLICT_MARK") || !strings.Contains(app, `box.dataset.kind = event.kind`) {
		t.Error("a three-way conflict is still colour-only")
	}
}

// TestLiveRegionsAreConsolidated is the #298 fix for the noisy live regions.
// #status is rewritten every 100ms while comparing, and four separate polite
// regions each announced themselves. Now one polite region carries navigation
// feedback and failures stay assertive through the message lane.
func TestLiveRegionsAreConsolidated(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	announcer := sectionBetween(t, index, `id="a11yAnnouncer"`, ">")
	for _, want := range []string{`role="status"`, `aria-live="polite"`, `aria-atomic="true"`} {
		if !strings.Contains(announcer, want) {
			t.Errorf("the announcer is missing %q", want)
		}
	}
	if !strings.Contains(index, `class="sr-only" id="a11yAnnouncer"`) {
		t.Error("the announcer is not visually hidden")
	}

	// The counters are readable on focus but must not be live regions of their
	// own, or every keypress talks over the others.
	for _, id := range []string{"status", "diffCounter", "searchCounter", "mergeUnresolved"} {
		element := sectionBetween(t, index, `id="`+id+`"`, ">")
		if strings.Contains(element, "aria-live") {
			t.Errorf("#%s is still its own live region: %s", id, element)
		}
	}
	// The control that changes a counter describes itself with it instead.
	for _, want := range []string{`aria-describedby="diffCounter"`, `aria-describedby="searchCounter"`, `aria-describedby="mergeUnresolved"`} {
		if !strings.Contains(index, want) {
			t.Errorf("no control is described by its counter: missing %q", want)
		}
	}

	// One helper writes the announcer, and the three keyboard-driven flows use
	// it for the position they changed.
	if !strings.Contains(app, "function announce(message)") || !strings.Contains(app, `$("a11yAnnouncer")`) {
		t.Fatal("app.js has no central announcer")
	}
	for _, call := range []string{
		`announce($("diffCounter").textContent)`,
		`announce($("searchCounter").textContent)`,
		`announce($("mergeUnresolved").textContent)`,
	} {
		if !strings.Contains(app, call) {
			t.Errorf("navigation does not announce through the central region: missing %q", call)
		}
	}
	// Failures keep the assertive lane; nothing new was made assertive.
	if !strings.Contains(app, `error ? "assertive" : "polite"`) {
		t.Error("the message lane no longer distinguishes failures")
	}
}

// TestFocusRingCoversRoleWidgets guards the keyboard half of #298: every
// control, including role-based widgets that are not native elements, shows the
// focus ring. The diff rows are role="button" cells, and the minimap is a
// role="scrollbar" div.
func TestFocusRingCoversRoleWidgets(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")

	rule := sectionBetween(t, style, ":where(button, input, select, textarea, summary, a[href], [tabindex],", "}")
	for _, want := range []string{`[role="button"]`, `[role="treeitem"]`, `[role="menuitem"]`, `[role="scrollbar"]`, "outline: var(--focus-ring)"} {
		if !strings.Contains(rule, want) {
			t.Errorf("the focus-visible rule is missing %q", want)
		}
	}
}

// TestStaticPagePassesTheChecksAutomatedToolsCoverMostOften approximates the
// rules an axe or Lighthouse run checks first, without adding a Node dependency
// or a headless browser: the document declares its language, the stylesheet
// never removes the focus outline, no live region asserts before a failure
// exists, and every dialog has an accessible name. It does not replace a real
// run against a rendered page — see docs/gui.md, "Accessibility and keyboard
// use", for what that would add.
func TestStaticPagePassesTheChecksAutomatedToolsCoverMostOften(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	style := readWebAsset(t, "style.css")

	if !strings.Contains(index, `<html lang="`) {
		t.Error("the document does not declare a language")
	}
	if strings.Contains(index, `aria-live="assertive"`) {
		t.Error("the static page asserts at load; only a runtime failure should")
	}
	for _, forbidden := range []string{"outline: none", "outline:none"} {
		if strings.Contains(style, forbidden) {
			t.Errorf("style.css removes the focus outline: %q", forbidden)
		}
	}

	// Every dialog is opened modally, so it must carry a name or a screen
	// reader announces an unnamed dialog.
	rest := index
	for {
		at := strings.Index(rest, "<dialog")
		if at < 0 {
			break
		}
		tag := sectionBetween(t, rest[at:], "<dialog", ">")
		if !strings.Contains(tag, "aria-labelledby=") {
			t.Errorf("a dialog has no accessible name: <dialog%s>", tag)
		}
		rest = rest[at+len("<dialog"):]
	}
}
