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
		"--bg-elevated:", "--fg-dim:",
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

func TestLaunchParametersSupportThreeWayComparison(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	for _, want := range []string{`launch.has("base")`, `"threeway-csv"`, `launchReady`} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js launch handling missing %q", want)
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
	for _, want := range []string{`lastComparedRequest`, `syncExportPatchVisibility`, `$("setup").addEventListener("input"`} {
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
