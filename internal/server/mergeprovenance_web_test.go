package server

import (
	"strings"
	"testing"
)

// TestMergeProvenancePreviewIsWired covers #257 at the seam: the page loads the
// pure module before app.js, the preview is fetched from the server that writes
// the merge, and each origin has a gutter style.
func TestMergeProvenancePreviewIsWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")
	module := readWebAsset(t, "mergeprovenance.js")

	for _, script := range []string{`<script src="mergeprovenance.js"></script>`} {
		if !strings.Contains(index, script) {
			t.Errorf("index.html missing %q", script)
		}
	}
	if strings.Index(index, `src="mergeprovenance.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("mergeprovenance.js loads after app.js, which consumes it")
	}
	for _, id := range []string{`id="mergeProvenance"`, `id="mergePreviewLines"`, `id="mergeProvenanceCounts"`} {
		if !strings.Contains(index, id) {
			t.Errorf("index.html missing %q", id)
		}
	}
	if !strings.Contains(module, "module.exports = api") {
		t.Error("mergeprovenance.js has no CommonJS export, so node --test cannot require it")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "$(") {
		t.Error("mergeprovenance.js touches the DOM; it must stay runnable without one")
	}
	if !strings.Contains(app, "globalThis.AyameMergeProvenance") {
		t.Error("app.js does not consume the extracted module")
	}
	if !strings.Contains(app, `/api/three-way/text/preview`) {
		t.Error("app.js does not fetch the server preview, so the result is not the one the save writes")
	}
	for _, want := range []string{
		".merge-origin-marker",
		".merge-preview-line.origin-manual",
		".merge-preview-line.origin-left .merge-origin-marker",
		".merge-preview-line.origin-right .merge-origin-marker",
	} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing provenance style %q", want)
		}
	}
	// A choice change must refresh an open preview, or it would keep showing a
	// result the save no longer produces.
	body := renderFunctionBody(t, app, "function updateThreeWayMergeUI()")
	if !strings.Contains(body, "syncMergeProvenancePanel") || !strings.Contains(body, "scheduleMergePreview") {
		t.Error("updateThreeWayMergeUI does not keep the provenance preview in step with the choices")
	}
	// Manual edits have to reach the save, or the report and the file disagree.
	if !strings.Contains(app, "body.manual = Object.fromEntries(mergeManual)") {
		t.Error("saveThreeWayMerge does not send manually typed lines")
	}
}
