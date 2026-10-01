package server

import (
	"os/exec"
	"strings"
	"testing"
)

// TestModeAxesAssetsAreWired guards the split from #263. The single `mode`
// dropdown collapsed two orthogonal questions — WHAT is compared (two files /
// three files / folder) and HOW it is read (text / sorted / CSV). The first is
// now an input-side control, the second a result-toolbar control, and the flat
// `mode` the API still takes is composed from them by web/modeaxes.js.
//
// Runs without node so it always executes in CI; the axis arithmetic itself is
// covered by web/test/modeaxes.test.js and by TestModeAxesPolicy.
func TestModeAxesAssetsAreWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	module := readWebAsset(t, "modeaxes.js")

	if !strings.Contains(index, `<script src="modeaxes.js"></script>`) {
		t.Error(`index.html missing <script src="modeaxes.js">`)
	}
	if strings.Index(index, `src="modeaxes.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("modeaxes.js must load before app.js")
	}
	// The axis arithmetic is DOM-free, so it stays testable without a browser.
	if strings.Contains(module, "document.") || strings.Contains(module, "localStorage") {
		t.Error("modeaxes.js touches the DOM; it must stay runnable without one")
	}
	for _, want := range []string{
		"globalThis.AyameModeAxes",
		"composeMode(",
		"decomposeMode(",
		"interpretationsFor(",
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}

	// The canonical flat value is still in the page, but it is no longer a
	// dropdown: it is a hidden input composed from the two axes.
	if !strings.Contains(index, `id="mode"`) || strings.Contains(index, `<select id="mode"`) {
		t.Error("the flat mode value is not a hidden input")
	}
	if !strings.Contains(index, `name="inputShape"`) {
		t.Error("index.html has no input-shape control")
	}
	if !strings.Contains(index, `id="interpretation"`) {
		t.Error("index.html has no interpretation control")
	}
	// The reading belongs on the result toolbar, where it can be switched after
	// a comparison.
	bar := sectionBetween(t, index, `<nav class="diff-nav" id="diffNav"`, `<div class="menubar"`)
	if !strings.Contains(bar, `id="interpretation"`) {
		t.Error("the interpretation axis is not on the result toolbar")
	}
	// The cross product must not be enumerated in the UI again.
	if strings.Contains(index, "threeway-csv") {
		t.Error("index.html still enumerates the mode cross product")
	}
}

// TestModeAxesPolicy pins the axis arithmetic in web/modeaxes.js. It is the
// only place the two axes meet the flat `mode`, so a change there is a change
// to every request the GUI can send.
func TestModeAxesPolicy(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	script := `
const axes = require('./web/modeaxes.js');
const eq = (a, b) => { const x=[...a], y=[...b]; return x.length===y.length && x.every((v,i)=>v===y[i]); };
if (!eq(axes.interpretationsFor('two'), ['text','sorted','csv'])) process.exit(30);
if (!eq(axes.interpretationsFor('three'), ['text','csv'])) process.exit(31);
if (axes.interpretationsFor('folder').length !== 0) process.exit(32);
if (axes.composeMode('two','sorted') !== 'sorted') process.exit(33);
if (axes.composeMode('three','csv') !== 'threeway-csv') process.exit(34);
if (axes.composeMode('three','sorted') !== 'threeway') process.exit(35);
if (axes.composeMode('folder','') !== 'dir') process.exit(36);
for (const mode of ['text','sorted','csv','threeway','threeway-csv','dir']) {
  const { shape, interpretation } = axes.decomposeMode(mode);
  if (axes.composeMode(shape, interpretation) !== mode) process.exit(37);
}
`
	cmd := exec.Command(node, "-e", script)
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mode axes policy test failed: %v\n%s", err, output)
	}
}
