package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOptimisticRecomputeIsWiredUp guards the #258 pieces against drifting
// apart: the page must load the tested module before app.js, the module must
// stay a pure DOM-free algorithm that node --test can require, and app.js must
// consume it rather than keeping its own copy.
func TestOptimisticRecomputeIsWiredUp(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	if !strings.Contains(index, `<script src="optimistic.js"></script>`) {
		t.Error("index.html does not load optimistic.js, so the tested module is not the one the page uses")
	}
	if strings.Index(index, `src="optimistic.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("optimistic.js loads after app.js, which consumes it")
	}

	module := readWebAsset(t, "optimistic.js")
	if !strings.Contains(module, "module.exports = api") {
		t.Error("optimistic.js has no CommonJS export, so node --test cannot require it")
	}
	for _, name := range []string{
		"function isPlainComparison(",
		"function planLineApproximation(",
		"function createRecomputeCoordinator(",
	} {
		if !strings.Contains(module, name) {
			t.Errorf("optimistic.js exports a symbol it does not define: %s", name)
		}
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "$(") {
		t.Error("optimistic.js touches the DOM; it must stay runnable without one")
	}
	// The module holds an algorithm and a scheduler, not application state.
	for _, leaked := range []string{
		"currentAbort", "lastData", "threeWayData", "mergeChoices",
		"addEventListener", "localStorage", "apiFetch(",
	} {
		if strings.Contains(module, leaked) {
			t.Errorf("optimistic.js contains application state or wiring (%q)", leaked)
		}
	}

	app := readWebAsset(t, "app.js")
	for _, want := range []string{
		"globalThis.AyameOptimistic",
		"createRecomputeCoordinator({",
		"optimisticPlanLine(",
		"applyOptimisticLineEdit(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing optimistic-recompute wiring %q", want)
		}
	}
	if !strings.Contains(app, "editRecompute.request()") {
		t.Error("app.js no longer routes a committed edit through the coalescing coordinator")
	}

	if _, err := os.Stat(filepath.Join("web", "test", "optimistic.test.js")); err != nil {
		t.Fatalf("no node test for the optimistic module: %v", err)
	}
}
