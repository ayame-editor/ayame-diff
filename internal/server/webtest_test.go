package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWebUnitTestsAreWiredUp guards the #139 harness against quietly falling
// out of use. The JS tests run under node --test in CI, which this package
// cannot execute, so what it checks is that the pieces still line up: the
// module under test is loaded by the page, a test file exists for it, and CI
// still runs them.
//
// Without this, deleting the CI step or the test file would leave the client
// logic silently uncovered again, which is exactly the state #139 described.
func TestWebUnitTestsAreWiredUp(t *testing.T) {
	t.Parallel()

	index := readWebAsset(t, "index.html")
	if !strings.Contains(index, `<script src="worddiff.js"></script>`) {
		t.Error("index.html does not load worddiff.js, so the tested module is not the one the page uses")
	}
	// The module must load before app.js, which destructures it at parse time.
	if strings.Index(index, `src="worddiff.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("worddiff.js loads after app.js, which consumes it")
	}

	module := readWebAsset(t, "worddiff.js")
	if !strings.Contains(module, "module.exports = api") {
		t.Error("worddiff.js has no CommonJS export, so node --test cannot require it")
	}
	if strings.Contains(module, "document.") || strings.Contains(module, "$(") {
		t.Error("worddiff.js touches the DOM; it must stay runnable without one")
	}
	// Extracting the algorithm accidentally carried application state with it,
	// which the string-matching tests happily accepted while the page threw
	// ReferenceError on first use. The module holds an algorithm and nothing
	// else, so anything resembling app state here is a mistake.
	for _, leaked := range []string{
		"currentAbort", "lastData", "csvData", "threeWayData", "mergeChoices",
		"addEventListener", "localStorage",
	} {
		if strings.Contains(module, leaked) {
			t.Errorf("worddiff.js contains application state or wiring (%q); it must stay a pure algorithm", leaked)
		}
	}
	// Every export must actually exist in the module.
	for _, name := range []string{"function inlineWordDiff(", "function inlineTokens(", "function pushPart("} {
		if !strings.Contains(module, name) {
			t.Errorf("worddiff.js exports a symbol it does not define: %s", name)
		}
	}

	app := readWebAsset(t, "app.js")
	if !strings.Contains(app, "globalThis.AyameWordDiff") {
		t.Error("app.js does not consume the extracted module")
	}
	if strings.Contains(app, "function inlineWordDiff(") {
		t.Error("app.js still defines its own inlineWordDiff, so the tested copy is not the used one")
	}

	testDir := filepath.Join("web", "test")
	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("no web test directory: %v", err)
	}
	var tests []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".test.js") {
			tests = append(tests, entry.Name())
		}
	}
	if len(tests) == 0 {
		t.Fatalf("%s holds no .test.js files", testDir)
	}

	workflow, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "lint.yml"))
	if err != nil {
		t.Fatalf("read lint workflow: %v", err)
	}
	if !strings.Contains(string(workflow), "node --test") {
		t.Error("CI no longer runs the web unit tests")
	}
}
