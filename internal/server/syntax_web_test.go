package server

import (
	"os/exec"
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
`
	cmd := exec.Command(node, "-e", script)
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("syntax helper test failed: %v\n%s", err, output)
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
