package server

import (
	"os/exec"
	"strings"
	"testing"
)

// TestModePolicy pins the mode → comparison-condition policy in web/modes.js so
// it stays in lockstep with what each request body in app.js actually reads
// (#124). If a request body starts (or stops) honoring a condition, the policy
// here must move with it or a control goes dead/hidden inconsistently.
func TestModePolicy(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	script := `
const modes = require('./web/modes.js');
const eq = (a, b) => { const x=[...a].sort(), y=[...b].sort(); return x.length===y.length && x.every((v,i)=>v===y[i]); };
// text / sorted / 3-way text spread requestBody(): every condition is live.
for (const mode of ['text','sorted','threeway']) {
  if (!eq(modes.liveCompareConditions(mode), modes.COMPARE_CONDITIONS)) process.exit(10);
  if (modes.deadCompareConditions(mode).length !== 0) process.exit(11);
}
// csvRequestBody() reads ignoreCase / whitespace / lineFilters only.
for (const mode of ['csv','threeway-csv']) {
  if (!eq(modes.liveCompareConditions(mode), ['ignoreCase','whitespace','lineFilters'])) process.exit(12);
  if (!eq(modes.deadCompareConditions(mode), ['ignoreEOL','ignoreTrailingEOL'])) process.exit(13);
}
// dirRequestBody() reads none of the shared comparison conditions.
if (modes.liveCompareConditions('dir').length !== 0) process.exit(14);
if (!eq(modes.deadCompareConditions('dir'), modes.COMPARE_CONDITIONS)) process.exit(15);
// Unknown mode fails safe: show everything, hide nothing.
if (!eq(modes.liveCompareConditions('???'), modes.COMPARE_CONDITIONS)) process.exit(16);
if (modes.deadCompareConditions('???').length !== 0) process.exit(17);
`
	cmd := exec.Command(node, "-e", script)
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mode policy test failed: %v\n%s", err, output)
	}
}

// TestModePolicyAssetsAreWired guards the load order and consumption of
// modes.js: index.html must pull it in before app.js, and app.js must both read
// the policy (deadCompareConditions) and keep moveMinLines in sync with move
// detection (#124). Runs without node so it always executes in CI.
func TestModePolicyAssetsAreWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")

	if !strings.Contains(index, `src="modes.js"`) {
		t.Error(`index.html missing <script src="modes.js">`)
	}
	if strings.Index(index, `src="modes.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("modes.js must load before app.js")
	}
	for _, want := range []string{"AyameModes", "deadCompareConditions", "syncMoveMinLines"} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js missing %q", want)
		}
	}
}
