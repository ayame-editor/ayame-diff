package server

import (
	"strings"
	"testing"
)

// TestConfirmedMarksAssetsAreWired guards the browser half of #288. The pure
// key derivation runs under node --test; what this checks is that the page
// loads it, that the explicit confirmed state stays separate from the automatic
// read-on-scroll state, and that the controls a mistake would quietly remove —
// the per-hunk toggle, the unconfirmed-only navigation, the progress counter,
// and the persisted key — are still present.
func TestConfirmedMarksAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "confirmed.js")
	style := readWebAsset(t, "style.css")

	if !strings.Contains(index, `<script src="confirmed.js"></script>`) {
		t.Error("index.html does not load confirmed.js")
	}
	if strings.Index(index, `src="confirmed.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("confirmed.js must load before app.js")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "localStorage") || strings.Contains(module, "fetch(") {
		t.Error("confirmed.js touches the browser; it must stay runnable without one")
	}

	for _, want := range []string{
		"globalThis.AyameConfirmed",
		"function prepareConfirmations(",
		"function toggleConfirmedHunk(",
		"function stepUnconfirmed(",
		"hunkSignatures(",
		"restoreSignatures(",
		`localStorage.getItem(CONFIRMED_STORAGE_KEY)`,
		"ayame-confirmed",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing confirmed-mark wiring %q", want)
		}
	}

	// The two states must not collapse: a hunk read by scrolling is not verified,
	// so confirmation is derived from a content signature and nothing else.
	for _, want := range []string{
		"confirmedHunks.has(confirmedSignatures[index])",
		"readHunks.has(index)",
		"restoreSignatures(confirmedSignatures, stored)",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js does not keep confirmation distinct from read state: missing %q", want)
		}
	}

	// The first/prev/next/last buttons stay; unconfirmed-only stepping is added
	// beside them with its own progress count.
	for _, id := range []string{"prevUnconfirmed", "nextUnconfirmed", "confirmCounter"} {
		if !strings.Contains(index, `id="`+id+`"`) {
			t.Errorf("index.html has no %s control", id)
		}
	}
	for _, want := range []string{
		`$("prevUnconfirmed").addEventListener("click", () => stepUnconfirmed(-1))`,
		`$("nextUnconfirmed").addEventListener("click", () => stepUnconfirmed(1))`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
	for _, want := range []string{`id="firstDiff"`, `id="prevDiff"`, `id="nextDiff"`, `id="lastDiff"`} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html lost the existing difference navigation control %q", want)
		}
	}

	for _, want := range []string{".hunk-confirm", ".hunk.confirmed", ".minimap-marker.confirmed", "#confirmCounter"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}
}

// TestConfirmedMarksAreLocalized checks every new string goes through the
// catalog rather than being baked into the markup; the parity test then holds
// the ja and en tables to the same keys.
func TestConfirmedMarksAreLocalized(t *testing.T) {
	t.Parallel()

	catalog := readWebCatalog(t)
	for _, key := range []string{"confirmCounter", "confirmHunk", "unconfirmHunk", "confirmed", "prevUnconfirmed", "nextUnconfirmed"} {
		if !strings.Contains(catalog, key+":") {
			t.Errorf("i18n.js missing %q", key)
		}
	}
	if strings.Contains(readWebAsset(t, "index.html"), `title="Confirm`) {
		t.Error("the confirm controls carry a hardcoded English title")
	}
}
