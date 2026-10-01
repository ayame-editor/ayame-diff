package server

import (
	"strings"
	"testing"
)

// TestThemeCustomizationIsWired guards the browser half of #286. The token
// model, the merge rules and the WCAG maths run under node --test; what this
// checks is that the page loads them, that the colours menu gained a custom
// choice with somewhere to edit it, and that the parts a careless edit would
// quietly drop — live application to :root, import/export, named storage and
// the reset — are still there.
func TestThemeCustomizationIsWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "theme.js")

	if !strings.Contains(index, `<script src="theme.js"></script>`) {
		t.Error("index.html does not load theme.js")
	}
	if strings.Index(index, `src="theme.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("theme.js must load before app.js")
	}
	// The model has to stay runnable without a browser, like i18n.js and
	// continuous.js, or node --test cannot reach it.
	for _, leaked := range []string{"document.", "localStorage", "addEventListener", "showModal"} {
		if strings.Contains(module, leaked) {
			t.Errorf("theme.js contains browser wiring (%q); it must stay pure", leaked)
		}
	}

	// A custom theme is one more choice beside default and colorblind, and the
	// editor is reachable from the View menu.
	for _, want := range []string{
		`value="custom"`,
		`id="themeCustomize"`,
		`id="themeDialog"`,
		`id="themePreset"`,
		`id="themeBase"`,
		`id="themeName"`,
		`id="themeTokens"`,
		`id="themeContrast"`,
		`id="themeSavedList"`,
		`id="themeJSON"`,
		`id="themeReset"`,
		`id="themeSave"`,
		`id="themeExport"`,
		`id="themeImport"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}

	for _, want := range []string{
		"globalThis.AyameTheme",
		"function applyActiveTheme(",
		"function applyThemeTokens(",
		"function openThemeEditor(",
		"function renderThemeContrast(",
		"function saveDraftTheme(",
		"function importThemeDraft(",
		"function exportThemeDraft(",
		`$("themeCustomize").addEventListener("click", openThemeEditor)`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}

	// Switching to "custom" has to apply the saved theme, and switching away
	// has to clear the inline overrides instead of painting everything forever.
	scheme := renderFunctionBody(t, app, "function applyScheme(")
	if !strings.Contains(scheme, "applyActiveTheme(v === THEME_CUSTOM_SCHEME)") {
		t.Error("the colours menu does not apply or clear the custom theme")
	}
	active := renderFunctionBody(t, app, "function applyActiveTheme(")
	if !strings.Contains(active, "clearAppliedThemeTokens()") || !strings.Contains(active, "applyThemeTokens(activeTheme)") {
		t.Error("selecting or leaving custom does not clear and apply the overrides")
	}
	// Live preview: every token change commits, and commit writes the tokens on
	// to the document so the open diff is the sample.
	commit := renderFunctionBody(t, app, "function commitThemeDraft(")
	if !strings.Contains(commit, "applyThemeTokens(activeTheme)") {
		t.Error("a theme edit is not applied to the page, so there is no live preview")
	}
	tokens := renderFunctionBody(t, app, "function applyThemeTokens(")
	if !strings.Contains(tokens, "setProperty") {
		t.Error("applying a theme does not set the CSS custom properties")
	}
	if !strings.Contains(renderFunctionBody(t, app, "function clearAppliedThemeTokens("), "removeProperty") {
		t.Error("leaving a custom theme does not reset the inline custom properties")
	}
	if !strings.Contains(renderFunctionBody(t, app, "function renderThemeContrast("), "checkContrast(") {
		t.Error("the editor does not report contrast")
	}
	if !strings.Contains(renderFunctionBody(t, app, "function saveDraftTheme("), "saveNamedTheme(") {
		t.Error("the editor cannot save a named theme")
	}
	if !strings.Contains(renderFunctionBody(t, app, "function importThemeDraft("), "parseTheme(") {
		t.Error("the editor cannot import a theme")
	}
	if !strings.Contains(renderFunctionBody(t, app, "function exportThemeDraft("), "serializeTheme(") {
		t.Error("the editor cannot export a theme")
	}

	// The reset returns to a shipped preset, which is what "reset" means here.
	if !strings.Contains(renderFunctionBody(t, app, "function resetThemeToPreset("), "loadThemePreset(") {
		t.Error("the reset does not reload a preset")
	}

	style := readWebAsset(t, "style.css")
	for _, want := range []string{".theme-dialog", ".theme-token-grid", ".theme-swatch", ".theme-contrast-row.fail", ".theme-saved-row"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}
}

// TestThemeEditorIsFullyLocalized guards the #144 rule for the new surface: a
// dialog with hardcoded English is exactly the drift the table exists to stop.
func TestThemeEditorIsFullyLocalized(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	catalog := readWebCatalog(t, "app.js")
	dialog := sectionBetween(t, index, `id="themeDialog"`, `</dialog>`)
	for _, key := range []string{
		"themeDialogTitle", "themePreset", "themeBase", "themeName", "themeTokens",
		"themeContrast", "themeSavedThemes", "themeShare", "themeReset", "themeSave",
		"themeExport", "themeImport", "themeImportPlaceholder", "themeCustomize", "themeCustom",
	} {
		if !strings.Contains(dialog, key) && !strings.Contains(index, `data-i18n="`+key+`"`) && !strings.Contains(index, `data-i18n-placeholder="`+key+`"`) {
			t.Errorf("the theme editor does not use the %q key", key)
		}
		if !strings.Contains(catalog, key+":") {
			t.Errorf("i18n.js has no %q key", key)
		}
	}
}

// cssDeclarations parses the custom-property declarations in the block that
// starts at selector. It is deliberately small: the stylesheet blocks it reads
// have no nested braces, and values (font stacks, color-mix()) contain commas
// and parentheses but no semicolons.
func cssDeclarations(t *testing.T, css, selector string) map[string]string {
	t.Helper()
	start := strings.Index(css, selector)
	if start < 0 {
		t.Fatalf("%q not found in tokens.css", selector)
	}
	open := strings.Index(css[start:], "{")
	if open < 0 {
		t.Fatalf("%q has no block", selector)
	}
	rest := css[start+open+1:]
	close := strings.Index(rest, "}")
	if close < 0 {
		t.Fatalf("%q is not closed", selector)
	}
	declarations := map[string]string{}
	for _, part := range strings.Split(rest[:close], ";") {
		colon := strings.Index(part, ":")
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(part[:colon])
		if !strings.HasPrefix(key, "--") {
			continue
		}
		declarations[key] = strings.TrimSpace(part[colon+1:])
	}
	if len(declarations) == 0 {
		t.Fatalf("%q declared no custom properties", selector)
	}
	return declarations
}
