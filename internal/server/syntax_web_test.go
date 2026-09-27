package server

import (
	"os/exec"
	"strings"
	"testing"
)

func TestSyntaxJavaScriptHelpers(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	script := `
const syntax = require('./web/syntax.js');
if (syntax.languageForPath('/tmp/app.ts') !== 'javascript') process.exit(10);
const json = syntax.highlightSpans('{"ok": true, "n": 42}', 'data.json');
if (!json.some((span) => span.kind === 'key' && span.text === '"ok"')) process.exit(11);
if (!json.some((span) => span.kind === 'number' && span.text === '42')) process.exit(12);
const code = syntax.highlightSpans('export function run() { return true } // done', 'app.ts');
if (!code.some((span) => span.kind === 'keyword' && span.text === 'export')) process.exit(13);
if (!code.some((span) => span.kind === 'function' && span.text === 'run')) process.exit(14);
if (code.at(-1).kind !== 'comment') process.exit(15);
// Cover the number / string / hex / block-comment tokenizer paths (#152).
const code2 = syntax.highlightSpans('let x = 0xFF; const s = "hi"; /* c */ f(3.5)', 'app.js');
if (!code2.some((span) => span.kind === 'number' && span.text === '0xFF')) process.exit(16);
if (!code2.some((span) => span.kind === 'string' && span.text === '"hi"')) process.exit(17);
if (!code2.some((span) => span.kind === 'comment' && span.text === '/* c */')) process.exit(18);
if (!code2.some((span) => span.kind === 'number' && span.text === '3.5')) process.exit(19);
// Multi-line state machine (#287). createHighlighter carries state across
// consecutive lines; unsupported languages return null for the fallback.
const hl = syntax.createHighlighter('app.js');
if (!hl) process.exit(20);
const block = [hl.highlightLine('const a = 1; /* start'), hl.highlightLine('still'), hl.highlightLine('end */ const b = 2;')];
if (!block[1].every((span) => span.kind === 'comment')) process.exit(21);
if (!block[2].some((span) => span.kind === 'comment' && span.text === 'end */')) process.exit(22);
if (!block[2].some((span) => span.kind === 'keyword' && span.text === 'const')) process.exit(23);
if (syntax.createHighlighter('data.json') !== null) process.exit(24);
if (syntax.highlightLines(['/* a', 'b */'], 'app.css').length !== 2) process.exit(25);
const sliced = syntax.sliceSpans(syntax.highlightSpans('ab /* cd */ ef', 'app.js'), 3, 11);
if (!sliced.some((span) => span.kind === 'comment' && span.text === '/* cd */')) process.exit(26);
// The line-local fallback must not gain multi-line state: a continuation line
// on its own is ordinary code, unchanged.
if (syntax.highlightSpans('still', 'app.js').some((span) => span.kind === 'comment')) process.exit(27);
`
	cmd := exec.Command(node, "-e", script)
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("syntax helper test failed: %v\n%s", err, output)
	}
}

// TestAccurateSyntaxHasDegradationPath is the #287 wiring check: app.js must use
// the stateful highlighter when it is available, and must still fall back to the
// unchanged line-local highlightSpans everywhere else. Without the fallback the
// accurate pass would be mandatory, which is exactly what the issue forbids.
func TestAccurateSyntaxHasDegradationPath(t *testing.T) {
	t.Parallel()
	app := readWebAsset(t, "app.js")
	for _, want := range []string{
		"api?.createHighlighter",
		"api.lineRuns",
		"function prepareAccurateSyntax(",
		"globalThis.AyameSyntax?.highlightSpans(text, path)",
		"globalThis.AyameSyntax?.sliceSpans?.",
		`prepareAccurateSyntax(data.hunks, syntaxPath("old"), syntaxPath("new"))`,
	} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js is missing the accurate-syntax wiring %q", want)
		}
	}
	// The accurate spans are a lookup, not a replacement: a miss must reach
	// highlightSpans. Guard the shape of appendSyntax so it cannot be rewritten
	// to always take the accurate branch.
	body := renderFunctionBody(t, app, "function appendSyntax(")
	if !strings.Contains(body, "highlightSpans") {
		t.Error("appendSyntax no longer falls back to highlightSpans when no accurate spans exist")
	}
}

func TestEmbeddedJavaScriptSyntax(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	for _, path := range []string{"web/syntax.js", "web/modes.js", "web/app.js"} {
		cmd := exec.Command(node, "--check", path)
		cmd.Dir = "."
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("node --check %s: %v\n%s", path, err, output)
		}
	}
}
