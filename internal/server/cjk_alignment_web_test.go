package server

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/ayame-editor/ayame-diff/internal/diffout"
	"github.com/ayame-editor/ayame-diff/internal/linediff"
	"github.com/ayame-editor/ayame-diff/internal/textwidth"
)

// The GUI keeps its display widths in sync with the CLI by mirroring
// internal/textwidth in web/textwidth.js (#289). These tests execute that
// module under node and compare it to the Go package, then compare both to the
// actual side-by-side output diffout prints. A browser-free pixel test is
// impossible; what is covered is the width/alignment decision, not the glyph
// advances a particular installed font chooses (see docs/gui.md).

const jsWidthScript = `
const fs = require('fs');
const tw = require('./web/textwidth.js');
let input = '';
process.stdin.on('data', (d) => { input += d; });
process.stdin.on('end', () => {
  const cases = JSON.parse(input);
  const out = cases.map((c) => ({
    width: tw.displayWidth(c.text, c.options),
    padded: tw.padRight(c.text, c.width, c.options),
    truncated: tw.truncate(c.text, c.width, c.options),
    left: tw.padRight(tw.truncate(c.text, c.width, c.options), c.width, c.options),
  }));
  process.stdout.write(JSON.stringify(out));
});
`

type widthOptions struct {
	EastAsianAmbiguousWide bool `json:"eastAsianAmbiguousWide"`
}

type widthCase struct {
	Text    string       `json:"text"`
	Width   int          `json:"width"`
	Options widthOptions `json:"options"`
}

type widthResult struct {
	Width     int    `json:"width"`
	Padded    string `json:"padded"`
	Truncated string `json:"truncated"`
	Left      string `json:"left"`
}

// cjkWidthCases are deliberately mixed: Latin/CJK, ambiguous symbols, emoji
// clusters, halfwidth katakana, and one line long enough to truncate.
func cjkWidthCases() []string {
	return []string{
		"A東京B",
		"○※α日本語",
		"日本語のテキスト",
		"ascii only",
		"全角ＡＢ１２３",
		"絵文字🙂と日本",
		"👨‍👩‍👧‍👦 family",
		"ﾊﾝｶｸ 半角",
		"節点 〇 記号",
		strings.Repeat("東京", 60),
	}
}

// runJSWidthModel feeds the cases to web/textwidth.js and returns its answers.
func runJSWidthModel(t *testing.T, cases []widthCase) []widthResult {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	payload, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "-e", jsWidthScript)
	cmd.Dir = "."
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node width model failed: %v\n%s", err, stderr.String())
	}
	var results []widthResult
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		t.Fatalf("decoding node output: %v\n%s", err, stdout.String())
	}
	if len(results) != len(cases) {
		t.Fatalf("node returned %d results for %d cases", len(results), len(cases))
	}
	return results
}

func TestGUIWidthModelMatchesGoTextwidth(t *testing.T) {
	for _, wide := range []bool{false, true} {
		wide := wide
		name := "narrow"
		if wide {
			name = "wide"
		}
		t.Run(name, func(t *testing.T) {
			opts := textwidth.Options{EastAsianAmbiguousWide: wide}
			var cases []widthCase
			for _, text := range cjkWidthCases() {
				cases = append(cases, widthCase{Text: text, Width: 76, Options: widthOptions{EastAsianAmbiguousWide: wide}})
			}
			results := runJSWidthModel(t, cases)
			for i, c := range cases {
				got := results[i]
				if want := textwidth.DisplayWidthWithOptions(c.Text, opts); got.Width != want {
					t.Errorf("displayWidth(%q) = %d from JS, %d from Go", c.Text, got.Width, want)
				}
				if want := textwidth.PadRightWithOptions(c.Text, c.Width, opts); got.Padded != want {
					t.Errorf("padRight(%q, %d) = %q from JS, %q from Go", c.Text, c.Width, got.Padded, want)
				}
				if want := textwidth.TruncateWithOptions(c.Text, c.Width, opts); got.Truncated != want {
					t.Errorf("truncate(%q, %d) = %q from JS, %q from Go", c.Text, c.Width, got.Truncated, want)
				}
			}
		})
	}
}

// TestGUIWidthModelAgreesWithSideBySideDiff runs the actual CLI renderer and
// checks that the left (padded) and right (truncated) columns equal what the
// GUI's model produces for the same setting. width 160 yields the same column
// the default `--side-by-side` uses.
func TestGUIWidthModelAgreesWithSideBySideDiff(t *testing.T) {
	const width = 160
	column := (width - 7) / 2 // mirrors writeSideBySide
	for _, wide := range []bool{false, true} {
		wide := wide
		name := "narrow"
		if wide {
			name = "wide"
		}
		t.Run(name, func(t *testing.T) {
			twOpts := textwidth.Options{EastAsianAmbiguousWide: wide}
			var cases []widthCase
			for _, text := range cjkWidthCases() {
				cases = append(cases, widthCase{Text: text, Width: column, Options: widthOptions{EastAsianAmbiguousWide: wide}})
			}
			results := runJSWidthModel(t, cases)
			const counterpart = "different counterpart"
			for i, c := range cases {
				old := linediff.SplitLines(c.Text + "\n")
				new := linediff.SplitLines(counterpart + "\n")
				res := linediff.Diff(old, new, 200, 128)
				var out, summary bytes.Buffer
				if err := diffout.Write(&out, &summary, old, new, res, diffout.Options{
					Format: diffout.SideBySide, Width: width, EastAsianAmbiguousWide: wide,
				}); err != nil {
					t.Fatalf("diffout.Write: %v", err)
				}
				var line string
				for _, candidate := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
					if strings.HasPrefix(candidate, "- ") {
						line = candidate
						break
					}
				}
				if line == "" {
					t.Fatalf("no side-by-side row for %q", c.Text)
				}
				parts := strings.SplitN(line, " | ", 2)
				if len(parts) != 2 {
					t.Fatalf("unexpected side-by-side row %q", line)
				}
				// diffout pads the already-truncated left column to exactly
				// `column` cells; that composition is what the GUI mirrors.
				if got, want := parts[0][2:], results[i].Left; got != want {
					t.Errorf("CLI left column = %q, GUI model = %q", got, want)
				}
				if got, want := parts[1][2:], counterpart; got != want {
					t.Errorf("CLI right column = %q, want %q", got, want)
				}
				// And the CLI must agree with Go's own width package, so the
				// GUI/CLI agreement is not two wrongs matching.
				if want := textwidth.PadRightWithOptions(textwidth.TruncateWithOptions(c.Text, column, twOpts), column, twOpts); parts[0][2:] != want {
					t.Errorf("CLI left column = %q, textwidth = %q", parts[0][2:], want)
				}
			}
		})
	}
}

// TestCJKWidthSettingsAreWired is the asset/text half: the page must load the
// model, expose both settings, and render tabs at the token width. It runs
// without node so it always executes in CI.
func TestCJKWidthSettingsAreWired(t *testing.T) {
	t.Parallel()
	index := readWebAsset(t, "index.html")
	app := readWebAsset(t, "app.js")
	style := readWebAsset(t, "style.css")
	tokens := readWebAsset(t, "tokens.css")
	module := readWebAsset(t, "textwidth.js")

	if strings.Index(index, `src="textwidth.js"`) > strings.Index(index, `src="app.js"`) {
		t.Error("index.html must load textwidth.js before app.js consumes globalThis.AyameTextWidth")
	}
	for _, want := range []string{
		`data-i18n="tabSize"`, `data-i18n="ambiguousWide"`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index.html is missing width control %q", want)
		}
	}
	for _, want := range []string{
		"globalThis.AyameTextWidth",
		"applyTabSize(", "applyAmbiguousWide(", `localStorage.getItem("ayame-tab-size")`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing width wiring %q", want)
		}
	}
	if !strings.Contains(style, "tab-size: var(--tab-size)") {
		t.Error("style.css does not render tabs at the --tab-size token")
	}
	if !strings.Contains(tokens, "--tab-size: 8") {
		t.Error("tokens.css does not define the default tab width")
	}
	if !strings.Contains(module, "module.exports = api") {
		t.Error("textwidth.js has no CommonJS export, so node --test cannot require it")
	}
}
