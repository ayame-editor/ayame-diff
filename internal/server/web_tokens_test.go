package server

import (
	"regexp"
	"strings"
	"testing"
)

func readWebAsset(t *testing.T, name string) string {
	t.Helper()
	b, err := webFS.ReadFile("web/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWebStylesUseAyameTokens(t *testing.T) {
	t.Parallel()
	tokens := readWebAsset(t, "tokens.css")
	style := readWebAsset(t, "style.css")
	index := readWebAsset(t, "index.html")

	for _, token := range []string{
		"--fs-ui: 13px", "--fs-data: 13px", "--fs-label: 12px", "--fs-caption: 11px",
		"--radius-control: 6px", "--on-accent: #fff", "--modal-backdrop:",
		"--bg-elevated:", "--fg-dim:", "--focus-ring:", "--focus-ring-offset:",
	} {
		if !strings.Contains(tokens, token) {
			t.Errorf("tokens.css missing %q", token)
		}
	}
	if !strings.Contains(tokens, `"DejaVu Sans Mono", "Noto Sans Mono CJK JP", "MS Gothic"`) {
		t.Error("canonical CJK mono fallback stack is missing")
	}
	if strings.Index(index, `href="tokens.css"`) > strings.Index(index, `href="style.css"`) {
		t.Error("tokens.css must load before the rules that consume it")
	}

	legacyType := regexp.MustCompile(`font(?:-size|):\s*0\.(?:72|75|78|8|82)rem`)
	legacyRadius := regexp.MustCompile(`border-radius:\s*(?:5|6|7|8|10)px`)
	if legacyType.MatchString(style) {
		t.Errorf("style.css bypasses the shared type scale: %q", legacyType.FindString(style))
	}
	if legacyRadius.MatchString(style) {
		t.Errorf("style.css bypasses radius tokens: %q", legacyRadius.FindString(style))
	}
	for _, bypass := range []string{`font-family: ui-monospace`, `font: 0.`, `color: #fff`, `font-weight: 450`, `font-weight: 650`} {
		if strings.Contains(style, bypass) {
			t.Errorf("style.css contains token bypass %q", bypass)
		}
	}
}

func TestSelectionBusyAndResultEmptyStatesAreConsistent(t *testing.T) {
	t.Parallel()
	tokens := readWebAsset(t, "tokens.css")
	style := readWebAsset(t, "style.css")
	app := readWebAsset(t, "app.js")

	if !strings.Contains(tokens, "--selection-ring-width: 2px") {
		t.Error("selection ring width token is missing")
	}
	for _, selector := range []string{
		".hunk.current { outline: var(--selection-ring-width)",
		".minimap-marker.current { outline: var(--selection-ring-width)",
		".cell.sync-selected { outline: var(--selection-ring-width)",
		"border: var(--selection-ring-width) solid var(--accent)",
	} {
		if !strings.Contains(style, selector) {
			t.Errorf("selection style missing %q", selector)
		}
	}
	for _, want := range []string{".status.busy::before", "@keyframes status-spin", "prefers-reduced-motion: reduce"} {
		if !strings.Contains(style, want) {
			t.Errorf("busy status style missing %q", want)
		}
	}
	if !strings.Contains(style, ".result-empty {") || strings.Count(app, "resultStateCard(") < 4 {
		t.Error("text, CSV, and three-way empty results must use the result card")
	}
}

func TestCompleteMatchCardsIncludeScopeAndDistinguishTruncation(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")
	for _, want := range []string{
		`completeMatch: "✔ 完全一致"`, `completeMatch: "✔ Complete match"`,
		"textMatchScope", "csvMatchScope", "threeWayTextMatchScope", "threeWayCSVMatchScope",
		`if (data.truncated) result.append(resultStateCard(t("matchNotVerified")`,
		`comparisonUsesRules(true) ? "filteredMatch" : "completeMatch"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("match result handling missing %q", want)
		}
	}
	for _, want := range []string{".result-match {", ".result-partial {"} {
		if !strings.Contains(style, want) {
			t.Errorf("match result style missing %q", want)
		}
	}
}

func TestSyntaxHighlightAssetsAreWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")

	for _, want := range []string{`id="syntax"`, `src="syntax.js"`} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	for _, want := range []string{"AyameSyntax", `localStorage.setItem("ayame-syntax"`} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	for _, want := range []string{".syn-comment", ".syn-keyword", ".syn-string", ".syn-level-error"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}
}

func TestDisplayTogglesDoNotRebuildDiffDOM(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")

	for _, want := range []string{
		`original.className = "ws-original"`,
		`visible.className = "ws-visible"`,
		`const spans = globalThis.AyameSyntax?.highlightSpans(text, path)`,
		`const wd = inlineWordDiff(old[k], neu[k])`,
		`result.classList.toggle("show-whitespace"`,
		`result.classList.toggle("syntax-highlight"`,
		`result.classList.toggle("word-highlight"`,
		`$("word").addEventListener("change", applyDisplayPreferences)`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("class-driven display toggle missing %q", want)
		}
	}
	for _, want := range []string{
		".result.show-whitespace .ws-original",
		".result.show-whitespace .ws-visible",
		".result:not(.syntax-highlight) .syn",
		".result:not(.word-highlight) .w-add",
	} {
		if !strings.Contains(style, want) {
			t.Errorf("display preference style missing %q", want)
		}
	}

	for _, id := range []string{"showWs", "syntax"} {
		start := strings.Index(app, `$("`+id+`").addEventListener("change"`)
		if start < 0 {
			t.Fatalf("%s change handler is missing", id)
		}
		end := strings.Index(app[start:], "\n});")
		if end < 0 {
			t.Fatalf("%s change handler is malformed", id)
		}
		if handler := app[start : start+end]; strings.Contains(handler, "renderResult(") {
			t.Errorf("%s change handler rebuilds the diff DOM", id)
		}
	}
}

func TestInteractiveControlsHaveHoverActiveAndTransitions(t *testing.T) {
	t.Parallel()
	// Normalize CRLF: git may check style.css out with CRLF on Windows
	// (autocrlf), but the multi-line matchers below are written with LF.
	style := strings.ReplaceAll(readWebAsset(t, "style.css"), "\r\n", "\n")

	// Hover feedback, an active press, and a selectable-line hover preview so no
	// control feels dead (#149). All clickables are <button>s, so the base rule
	// covers them; the primary/cancel keep their own identity on hover.
	for _, want := range []string{
		"button:hover:not(:disabled)",
		"button:active:not(:disabled)",
		"transform: translateY(1px)",
		".cell.selectable-line:hover",
		"#compare:hover:not(:disabled)",
		".cancel:hover:not(:disabled)",
	} {
		if !strings.Contains(style, want) {
			t.Errorf("interaction state missing %q", want)
		}
	}
	// State changes ease rather than snap: buttons/lines and hunk/status carry a
	// transition.
	for _, want := range []string{
		"button, .cell.selectable-line {\n  transition:",
		".hunk, .status {\n  transition:",
	} {
		if !strings.Contains(style, want) {
			t.Errorf("transition rule missing %q", want)
		}
	}
	// Motion must be respectful of prefers-reduced-motion: transitions and the
	// active-press transform are disabled there.
	rm := style[strings.Index(style, "@media (prefers-reduced-motion: reduce)")+1:]
	if next := strings.Index(rm, "@media"); next >= 0 {
		rm = rm[:next]
	}
	for _, want := range []string{"transition: none", "transform: none"} {
		if !strings.Contains(rm, want) {
			t.Errorf("reduced-motion block must include %q", want)
		}
	}
}

func TestChangeCellSyntaxAndWordHighlightRemainReadable(t *testing.T) {
	t.Parallel()
	style := readWebAsset(t, "style.css")

	// Every syntax token that shares a change cell's wash hue must be pulled
	// toward the body text colour on that cell so it stays legible (#150).
	for _, rule := range []string{
		".cell.add .syn-string { color: color-mix(in srgb, var(--add-fg)",
		".cell.chg .syn-literal { color: color-mix(in srgb, var(--chg-fg)",
		".cell.chg .syn-function,",
		".cell.chg .syn-level-warn { color: color-mix(in srgb, var(--gold)",
		".cell.del .syn-level-error { color: color-mix(in srgb, var(--del-fg)",
	} {
		if !strings.Contains(style, rule) {
			t.Errorf("change-cell syntax contrast rule missing %q", rule)
		}
	}
	// Each override must mix toward --fg (theme-adaptive contrast), not a fixed
	// colour, so it holds in light, dark, and colourblind schemes.
	for _, token := range []string{"--add-fg", "--chg-fg", "--gold", "--del-fg"} {
		if !regexp.MustCompile(`\.cell\.[a-z]+ \.syn-[a-z-]+[^}]*var\(` + token + `\) \d+%, var\(--fg\)\)`).MatchString(style) {
			t.Errorf("change-cell override for %s must mix toward var(--fg)", token)
		}
	}
	// Word highlights need horizontal breathing room and a defining edge so the
	// coloured pills are not cramped against the glyphs (#150).
	for _, want := range []string{
		".w-add, .w-del {",
		"padding-inline: 0.2em;",
		"box-decoration-break: clone;",
		".w-add { background: var(--word-add); box-shadow: inset 0 0 0 1px",
		".w-del { background: var(--word-del); box-shadow: inset 0 0 0 1px",
	} {
		if !strings.Contains(style, want) {
			t.Errorf("word-highlight readability style missing %q", want)
		}
	}
}

func TestLaunchParametersSupportThreeWayComparison(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	for _, want := range []string{`launch.has("base")`, `"threeway-csv"`, `launchReady`} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js launch handling missing %q", want)
		}
	}
}

func TestDirectoryFilterEditorIsWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	for _, want := range []string{`id="dirFilter"`, `id="dirFilterFile"`, `id="dirFilterSet"`, `id="dirCompareBy"`, `id="dirPreview"`, `id="dirProjectPath"`} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	for _, want := range []string{`/api/dir/preview`, `compareBy: $("dirCompareBy").value`, `saveDirectoryProject`, `applyDirectoryProject`} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}

func TestQuickKeyboardAndLocalizedNavigationWiring(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	for _, want := range []string{
		`data-i18n-aria-label="langSwitchLabel"`,
		`data-i18n-aria-label="firstDiff"`,
		`data-i18n-title="nextDiff"`,
		`aria-hidden="true"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	for _, want := range []string{
		`event.isComposing`,
		`event.keyCode === 229`,
		`["base", "old", "new", "oldText", "newText"]`,
		`data-i18n-aria-label`,
		`langButton: "日本語 → EN"`,
		`langButton: "English → 日本語"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}

func TestPrimaryCompareAndInitialEmptyState(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")

	for _, want := range []string{
		`id="old" type="text" placeholder="/path/to/old.txt" spellcheck="false" autofocus`,
		`class="empty-state initial-empty"`,
		`data-i18n="emptyDrop"`,
		`id="exportPatch" type="button" data-i18n="exportPatch" hidden`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	navStart := strings.Index(index, `id="diffNav"`)
	navEnd := strings.Index(index[navStart:], `</nav>`)
	exportAt := strings.Index(index, `id="exportPatch"`)
	if navStart < 0 || navEnd < 0 || exportAt < navStart || exportAt > navStart+navEnd {
		t.Error("Export patch must live in the result navigation toolbar")
	}
	if !strings.Contains(style, "#compare {") || !strings.Contains(style, "background: var(--accent)") {
		t.Error("Compare must be the explicitly accented primary action")
	}
	for _, want := range []string{`lastComparedRequest`, `syncExportPatchVisibility`, `$("setup").addEventListener("input"`, `res.move_detection_skipped`, `moveDetectionSkipped`} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}

func TestCSVPaginationIsDirectAndAccessible(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")

	for _, want := range []string{
		`pageInput.type = "number"`,
		`pageInput.min = "1"`,
		`pageInput.max = String(pageCount)`,
		`event.key === "Enter"`,
		`setAttribute("aria-label", t("previousPage"))`,
		`setAttribute("aria-label", t("nextPage"))`,
		`pageInput.setAttribute("aria-label", t("pageInput"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	if !strings.Contains(style, ".csv-page-input") {
		t.Error("CSV page input styling is missing")
	}
}

func TestLocalizedWebAttributesCoverStaticLabels(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	for _, tag := range regexp.MustCompile(`<[^>]+>`).FindAllString(index, -1) {
		if strings.Contains(tag, ` title="`) && !strings.Contains(tag, `data-i18n-title=`) {
			t.Errorf("static title is not localized: %s", tag)
		}
		if strings.Contains(tag, ` aria-label="`) && !strings.Contains(tag, `data-i18n-aria-label=`) {
			t.Errorf("static aria-label is not localized: %s", tag)
		}
	}
	for _, want := range []string{
		`data-i18n-placeholder="lineFiltersPlaceholder"`,
		`data-i18n-title="keyboardShortcuts"`,
		`data-i18n-aria-label="differenceMap"`,
		`data-i18n-aria-label="browseFile"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	for _, want := range []string{`[data-i18n-title]`, `[data-i18n-aria-label]`, `lineFiltersPlaceholder: "1行に1つの正規表現"`} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}

func TestClientValidationMessagesAreFullyLocalized(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	for _, line := range strings.Split(app, "\n") {
		if strings.Contains(line, "setStatus(") && (strings.Contains(line, " required") || strings.Contains(line, "invalid index")) {
			t.Errorf("status message contains an English literal: %s", line)
		}
	}
	for _, want := range []string{
		"requiredField: (v) => `${v.field}を指定してください。`",
		"invalidIndex: (v) => `${v.field}のインデックスが不正です。`",
		`t("requiredFields", { fields: missing })`,
		`t("invalidIndex", { field: t("ignoreColumns") })`,
		`if (!validateInputs(body)) return;`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}

func TestKeyboardFocusSyncSelectionAndStatusAnnouncements(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")

	for _, want := range []string{
		`:focus-visible`,
		`outline: var(--focus-ring)`,
		`.minimap-marker:focus-visible`,
		`outline-offset: -2px`,
		`.cell.selectable-line:focus-visible`,
	} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}
	for _, want := range []string{
		`c.tabIndex = 0`,
		`c.setAttribute("role", "button")`,
		`event.key !== "Enter" && event.key !== " "`,
		`cell.setAttribute("aria-pressed", "true")`,
		`error ? "assertive" : "polite"`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	for _, want := range []string{`role="status"`, `aria-live="polite"`, `aria-atomic="true"`} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
}

func TestColorblindSchemeKeepsSemanticDiffTokens(t *testing.T) {
	t.Parallel()
	tokens := readWebAsset(t, "tokens.css")
	start := strings.Index(tokens, `:root[data-scheme="colorblind"]`)
	if start < 0 {
		t.Fatal("colorblind scheme is missing")
	}
	block := tokens[start:]
	for _, token := range []string{"--add-bg", "--add-fg", "--del-bg", "--del-fg", "--chg-bg", "--word-add", "--word-del"} {
		if !strings.Contains(block, token+":") {
			t.Errorf("colorblind scheme missing %s", token)
		}
	}
	if !strings.Contains(block, "color-mix(") {
		t.Error("colorblind scheme must retain Ayame translucent washes")
	}
}
