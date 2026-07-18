package server

import (
	"strings"
	"testing"
)

// TestLongOperationsAreMutuallyExclusive is the #128 regression. Each flow used
// to disable only its own button, so a comparison and an export — or two
// comparisons — could run at once and race their results into the same DOM and
// status line.
func TestLongOperationsAreMutuallyExclusive(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")

	if !strings.Contains(app, "async function runExclusive(") {
		t.Fatal("app.js has no mutual-exclusion helper")
	}
	// Wrapping the flows rather than the buttons is the point: drag and drop,
	// folder-entry clicks, sync-point edits, and the Enter key all reach
	// compare() without touching #compare.
	for _, entry := range []string{
		`async function compare() { return runExclusive("compare", runCompare); }`,
		`async function exportPatch() { return runExclusive("exportPatch", runExportPatch); }`,
		`async function saveMergeResult() { return runExclusive("saveMerge", runSaveMergeResult); }`,
	} {
		if !strings.Contains(app, entry) {
			t.Errorf("missing exclusive entry point: %s", entry)
		}
	}
	// Cancel must stay reachable; locking it would strand a running operation.
	start := strings.Index(app, "const EXCLUSIVE_CONTROLS")
	if start < 0 {
		t.Fatal("no EXCLUSIVE_CONTROLS list")
	}
	list := app[start : start+strings.Index(app[start:], "];")]
	if strings.Contains(list, `"cancel"`) {
		t.Error("cancel is in the exclusive list, so a running operation could not be stopped")
	}
	for _, id := range []string{`"compare"`, `"exportPatch"`, `"saveMerge"`, `"saveProject"`, `"addSync"`} {
		if !strings.Contains(list, id) {
			t.Errorf("EXCLUSIVE_CONTROLS omits %s", id)
		}
	}
}

// TestLockRestoresPreviouslyDisabledControls keeps the lock from enabling a
// control that was disabled for its own reason, such as Add sync point with no
// selection.
func TestLockRestoresPreviouslyDisabledControls(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "controlsDisabledBeforeRun") {
		t.Fatal("the lock does not remember pre-existing disabled state")
	}
	unlock := renderFunctionBody(t, app, "function unlockExclusiveControls()")
	if !strings.Contains(unlock, "controlsDisabledBeforeRun") {
		t.Error("unlocking ignores which controls were already disabled")
	}
}

// TestStaleResponsesAreDiscarded covers the race the lock alone cannot close:
// a response that was already in flight must not paint over a newer result.
func TestStaleResponsesAreDiscarded(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	for _, want := range []string{"function beginRequest()", "function isCurrentRequest("} {
		if !strings.Contains(app, want) {
			t.Fatalf("missing %s", want)
		}
	}
	// All four comparison paths must take a generation and check it.
	if got := strings.Count(app, "beginRequest()"); got < 5 {
		t.Errorf("beginRequest used %d times; the helper plus four compare paths are expected", got)
	}
	if got := strings.Count(app, "if (!isCurrentRequest(generation)) return;"); got < 4 {
		t.Errorf("only %d paths discard a stale response; text, CSV, folder, and three-way all need it", got)
	}
}

// TestAllComparePathsShareTheBusyContract is the consistency half of #128:
// three-way had no AbortController, no elapsed counter, and no Cancel, and the
// folder path had no counter.
func TestAllComparePathsShareTheBusyContract(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	for _, fn := range []string{
		"async function compareThreeWay(",
		"async function compareDirectory(",
		"async function runCompare(",
		"async function compareCSV(",
	} {
		body := renderFunctionBody(t, app, fn)
		if !strings.Contains(body, "new AbortController()") {
			t.Errorf("%s has no AbortController, so it cannot be cancelled", fn)
		}
		if !strings.Contains(body, "signal: ac.signal") {
			t.Errorf("%s never passes its abort signal to the request", fn)
		}
		if !strings.Contains(body, `$("cancel").hidden = false`) {
			t.Errorf("%s does not offer Cancel", fn)
		}
		if !strings.Contains(body, "setInterval(tick") {
			t.Errorf("%s shows no elapsed time", fn)
		}
		if !strings.Contains(body, `err.name === "AbortError"`) {
			t.Errorf("%s reports a cancellation as an error", fn)
		}
	}
}
