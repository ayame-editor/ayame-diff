package server

import (
	"strings"
	"testing"
)

// TestMemoryBudgetIsLoadedAndRendered covers #138 at the GUI seam: the memory
// status module loads before app.js, stays DOM-free so node can check it, and the
// CSV summary actually renders the budget and spill state rather than dropping
// them.
func TestMemoryBudgetIsLoadedAndRendered(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "memorybudget.js")

	if !strings.Contains(index, `<script src="memorybudget.js"></script>`) {
		t.Error("index.html does not load memorybudget.js")
	}
	if strings.Index(index, `src="memorybudget.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("memorybudget.js must load before app.js")
	}
	for _, leaked := range []string{"document.", "localStorage", "addEventListener", "apiFetch("} {
		if strings.Contains(module, leaked) {
			t.Errorf("memorybudget.js contains application wiring (%q); it must stay pure", leaked)
		}
	}
	for _, want := range []string{"root.AyameMemoryBudget", "module.exports = api"} {
		if !strings.Contains(module, want) {
			t.Errorf("memorybudget.js is missing %q", want)
		}
	}

	if !strings.Contains(app, "globalThis.AyameMemoryBudget") {
		t.Error("app.js does not consume the memory status module")
	}
	summary := renderFunctionBody(t, app, "function renderCSVSummary(")
	for _, want := range []string{"memoryStatus(data.memory)", `t("memoryBudget"`, `t("memorySpilled"`} {
		if !strings.Contains(summary, want) {
			t.Errorf("renderCSVSummary does not render %q", want)
		}
	}
}
