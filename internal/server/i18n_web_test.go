package server

import (
	"os/exec"
	"strings"
	"testing"
)

// TestI18NParityAndRequiredKeys extracts the message catalog from i18n.js and asserts
// the ja and en tables define exactly the same keys (so no string is left
// untranslated in one language) and that the #125 keys exist. Runs via node
// because the table is a JS object literal with function-valued entries.
func TestI18NParityAndRequiredKeys(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	script := `
const fs = require('fs');
const src = fs.readFileSync('./web/i18n.js', 'utf8');
const s = src.indexOf('const CATALOG = {');
if (s < 0) process.exit(20);
const open = src.indexOf('{', s);
const e = src.indexOf('\n  };', open);     // the table's closing "};" inside the module
if (e < 0) process.exit(21);
const I18N = eval('(' + src.slice(open, e + 4) + ')');
const ja = Object.keys(I18N.ja), en = Object.keys(I18N.en);
const jaSet = new Set(ja), enSet = new Set(en);
const onlyJa = ja.filter((k) => !enSet.has(k));
const onlyEn = en.filter((k) => !jaSet.has(k));
if (onlyJa.length || onlyEn.length) {
  console.error('parity break -- ja-only:', onlyJa, 'en-only:', onlyEn);
  process.exit(22);
}
const required = ['modeText','modeSorted','modeCsv','modeFolder','modeThreeway','modeThreewayCsv',
  'statusDifferent','statusAll','changed','removed','same','left','right','leftOnly','rightOnly','equalRows','bytes'];
const missing = required.filter((k) => !jaSet.has(k));
if (missing.length) { console.error('missing required keys:', missing); process.exit(23); }
for (const [name, table] of [['ja', I18N.ja], ['en', I18N.en]]) {
  for (const k of Object.keys(table)) {
    const v = table[k];
    if (typeof v !== 'string' && typeof v !== 'function') { console.error(name, k, 'not a string/function'); process.exit(24); }
    if (typeof v === 'string' && v.length === 0) { console.error(name, k, 'empty'); process.exit(25); }
  }
}
`
	cmd := exec.Command(node, "-e", script)
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("i18n parity test failed: %v\n%s", err, output)
	}
}

// TestI18NControlsAreWired guards that the previously hardcoded strings now go
// through the i18n table (#125): the mode / folder-status <option>s carry
// data-i18n, and the CSV / folder / 3-way summary renderers call t() instead of
// literal English. Runs without node so it always executes in CI.
func TestI18NControlsAreWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	for _, want := range []string{
		`data-i18n="modeText"`, `data-i18n="modeThreewayCsv"`,
		`data-i18n="statusDifferent"`, `data-i18n="statusAll"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html missing %q", want)
		}
	}
	// Summary labels must be resolved through t(), not baked-in English.
	for _, want := range []string{`t("leftOnly")`, `t("rightOnly")`, `t("left")`, `t("right")`, `t("bytes")`, `t(name)`} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	for _, gone := range []string{`add("left only"`, `add("right only"`, `add("left"`, "` bytes\\n"} {
		if strings.Contains(app, gone) {
			t.Errorf("app.js still has hardcoded %q", gone)
		}
	}
}

// TestMessageCatalogLivesInItsOwnModule guards the extraction: the page must
// load the catalog before app.js consumes it, and app.js must not grow a second
// copy of the table it was split out of.
func TestMessageCatalogLivesInItsOwnModule(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "i18n.js")

	if !strings.Contains(index, `<script src="i18n.js"></script>`) {
		t.Error("index.html does not load i18n.js")
	}
	if strings.Index(index, `src="i18n.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("i18n.js must load before app.js")
	}
	if strings.Contains(app, "const I18N = {") {
		t.Error("app.js still defines its own catalog, so the tested copy is not the used one")
	}
	for _, want := range []string{
		"globalThis.AyameI18N",
		"const { CATALOG: I18N, translate, pickLanguage } = globalThis.AyameI18N;",
		"pickLanguage(localStorage.getItem(\"ayame-lang\"), navigator.language)",
		"return translate(I18N, lang, key, arg);",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing catalog wiring %q", want)
		}
	}
	for _, want := range []string{"function translate(", "function pickLanguage(", "module.exports = api"} {
		if !strings.Contains(module, want) {
			t.Errorf("i18n.js is missing %q", want)
		}
	}
	// The catalog is data. Application wiring belongs in app.js.
	for _, leaked := range []string{"document.", "localStorage", "addEventListener", "apiFetch("} {
		if strings.Contains(module, leaked) {
			t.Errorf("i18n.js contains application wiring (%q); it must stay a catalog", leaked)
		}
	}
}
