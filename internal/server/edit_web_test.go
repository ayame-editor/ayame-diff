package server

import (
	"strings"
	"testing"
)

// TestEditablePaneAssetsAreWired guards the browser half of #255. The buffer
// itself is executed under node --test; what this checks is that the page still
// loads it, that editing still reaches the comparison, and that the parts a
// mistake would quietly remove — the IME guard, the read-only refusal, the
// unsaved-changes flag the watcher reads — are all still there.
func TestEditablePaneAssetsAreWired(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "editbuffer.js")
	style := readWebAsset(t, "style.css")

	if !strings.Contains(index, `<script src="editbuffer.js"></script>`) {
		t.Error("index.html does not load editbuffer.js")
	}
	if strings.Index(index, `src="editbuffer.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("editbuffer.js must load before app.js")
	}
	if !strings.Contains(index, `id="editMode"`) {
		t.Error("index.html has no edit toggle")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "localStorage") {
		t.Error("editbuffer.js touches the DOM; it must stay runnable without one")
	}

	for _, want := range []string{
		"globalThis.AyameEditBuffer",
		"function openLineEditor(",
		"function saveEditedPane(",
		`"/api/file/read"`,
		`"/api/file/save"`,
		"overwrite: true",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing editing wiring %q", want)
		}
	}

	// An IME composition is a run of intermediate values. Acting on them is the
	// bug the issue calls out by name, so the guards are load-bearing.
	for _, want := range []string{
		`editor.addEventListener("compositionstart"`,
		`editor.addEventListener("compositionend"`,
		"if (editComposing) return;",
		"event.isComposing",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing IME handling %q", want)
		}
	}
	// The editor lives inside the cell that opens it; without this the cell
	// re-opens it on every keystroke and click.
	if !strings.Contains(app, "editor.addEventListener(\"click\", (event) => event.stopPropagation());") {
		t.Error("app.js does not stop clicks inside the editor from re-opening it")
	}

	// The file watcher refuses to auto-reload over unsaved work by reading this
	// flag, and it is the editor that owns it.
	if !strings.Contains(app, `document.body.dataset.unsavedChanges = editedSides().length ? "true" : "false";`) {
		t.Error("the editor does not report unsaved changes to the file watcher")
	}
	if !strings.Contains(app, `document.addEventListener("ayame:discard-unsaved-changes"`) {
		t.Error("the editor does not honour an explicit discard from the external-change bar")
	}
	if !strings.Contains(app, `window.addEventListener("beforeunload"`) {
		t.Error("leaving the page with unsaved edits is not guarded")
	}

	for _, want := range []string{".line-editor", ".pane-head-save", ".pane-head-dirty", ".pane-head-readonly"} {
		if !strings.Contains(style, want) {
			t.Errorf("style.css missing %q", want)
		}
	}
}

// Editing is offered only where a line maps back to a file line that can be
// written. Offering it elsewhere would produce a save with nowhere to go.
func TestEditingIsOfferedOnlyWhereItCanWork(t *testing.T) {
	t.Parallel()

	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "editbuffer.js")

	if !strings.Contains(module, `return !scratch && mode === "text";`) {
		t.Error("editableComparison no longer restricts editing to two-file text comparisons")
	}
	if !strings.Contains(app, `editableComparison($("mode").value, $("scratch").checked)`) {
		t.Error("app.js does not gate the edit toggle on the comparison kind")
	}
	if !strings.Contains(app, "buffer.readOnly()") {
		t.Error("app.js does not refuse to edit a read-only pane")
	}
}
