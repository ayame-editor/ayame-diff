"use strict";

// Tests for the multi-line-aware highlighter added in #287. syntax.js keeps the
// line-local highlightSpans as the degradation path; createHighlighter adds
// state carried across consecutive lines for block comments, multi-line
// strings and shell heredocs.
const test = require("node:test");
const assert = require("node:assert/strict");

const {
  highlightSpans,
  languageForPath,
  createHighlighter,
  highlightLines,
  sliceSpans,
  lineRuns,
} = require("../syntax.js");

function parts(spans) {
  return spans.map((span) => `${span.kind}:${span.text}`);
}
function has(spans, kind, text) {
  return spans.some((span) => span.kind === kind && span.text === text);
}
function highlight(highlighter, lines) {
  return lines.map((line) => highlighter.highlightLine(line));
}

test("only languages with a known multi-line construct get a highlighter", () => {
  for (const path of ["a.js", "main.go", "app.py", "run.sh", "q.sql", "x.css", "lib.rs"]) {
    assert.ok(createHighlighter(path), `${path} should support accurate highlighting`);
  }
  // These keep the line-local fallback: no multi-line construct is modeled.
  for (const path of ["data.json", "notes.md", "app.log", "conf.yaml", "noextension"]) {
    assert.equal(createHighlighter(path), null, `${path} should fall back`);
  }
});

test("the line-local fallback still cannot see a block comment's next line", () => {
  // This is the behavior the accurate path improves on, and it must stay
  // available unchanged: a continuation line is tokenized as ordinary code.
  const spans = highlightSpans("still inside the comment", "app.js");
  assert.ok(spans.some((span) => span.kind !== "comment"));
  assert.ok(!spans.some((span) => span.kind === "comment"));
});

test("a JavaScript block comment colors every line through the closing marker", () => {
  const highlighter = createHighlighter("app.js");
  const [first, second, third] = highlight(highlighter, [
    "const a = 1; /* start",
    "still inside",
    "end */ const b = 2;",
  ]);
  assert.ok(has(first, "keyword", "const"));
  assert.ok(has(first, "comment", "/* start"));
  assert.deepEqual(parts(second), ["comment:still inside"]);
  assert.ok(has(third, "comment", "end */"));
  assert.ok(has(third, "keyword", "const"));
  assert.deepEqual(highlighter.state(), { open: null, heredoc: null });
});

test("a JavaScript template literal spans lines and treats interpolation as string", () => {
  const highlighter = createHighlighter("app.js");
  const [open, body, close] = highlight(highlighter, [
    "const t = `first",
    "second ${value}",
    "third`;",
  ]);
  assert.ok(has(open, "string", "`first"));
  assert.deepEqual(parts(body), ["string:second ${value}"]);
  assert.deepEqual(parts(close), ["string:third`", "op:;"]);
});

test("a Go raw string spans lines and a backslash does not close it", () => {
  const highlighter = createHighlighter("main.go");
  const [open, body, close] = highlight(highlighter, [
    "q := `raw",
    "path C:\\temp\\",
    "done`",
  ]);
  assert.ok(has(open, "string", "`raw"));
  assert.deepEqual(parts(body), ["string:path C:\\temp\\"]);
  assert.deepEqual(parts(close), ["string:done`"]);
});

test("Python triple-quoted strings span lines in both quote styles", () => {
  const highlighter = createHighlighter("app.py");
  const lines = highlight(highlighter, [
    'doc = """first',
    "second",
    'third"""',
    "other = '''a",
    "b''' + 1",
  ]);
  assert.ok(has(lines[0], "string", '"""first'));
  assert.deepEqual(parts(lines[1]), ["string:second"]);
  assert.deepEqual(parts(lines[2]), ["string:third\"\"\""]);
  assert.ok(has(lines[3], "string", "'''a"));
  assert.deepEqual(parts(lines[4]), ["string:b'''", "plain: ", "op:+", "plain: ", "number:1"]);
});

test("a shell heredoc colors its body until the delimiter, with <<- stripping tabs", () => {
  const highlighter = createHighlighter("run.sh");
  const lines = highlight(highlighter, [
    "cat <<EOF",
    "body one",
    "$var is literal",
    "EOF",
    "echo done",
  ]);
  assert.ok(has(lines[0], "op", "<<EOF"));
  assert.deepEqual(parts(lines[1]), ["string:body one"]);
  assert.deepEqual(parts(lines[2]), ["string:$var is literal"]);
  assert.deepEqual(parts(lines[3]), ["string:EOF"]);
  assert.ok(has(lines[4], "keyword", "done"));

  const tabbed = createHighlighter("run.sh");
  highlight(tabbed, ["cat <<-END", "\t\tbody", "\tEND", "echo x"]);
  assert.deepEqual(tabbed.state(), { open: null, heredoc: null });
});

test("a quoted heredoc delimiter is recognized and the marker is not the terminator", () => {
  const highlighter = createHighlighter("run.sh");
  const lines = highlight(highlighter, ["cat <<'EOF'", "END", "EOF"]);
  assert.ok(has(lines[0], "op", "<<'EOF'"));
  assert.deepEqual(parts(lines[1]), ["string:END"]);
  assert.deepEqual(parts(lines[2]), ["string:EOF"]);
  assert.deepEqual(highlighter.state(), { open: null, heredoc: null });
});

test("shell arithmetic shift is not mistaken for a heredoc", () => {
  const highlighter = createHighlighter("run.sh");
  const spans = highlighter.highlightLine("echo $((1 << 2))");
  assert.deepEqual(highlighter.state(), { open: null, heredoc: null });
  assert.ok(!spans.some((span) => span.kind === "string"));
});

test("a SQL block comment spans lines and resumes after */", () => {
  const highlighter = createHighlighter("query.sql");
  const lines = highlight(highlighter, ["/* note", "more", "*/ SELECT 1;"]);
  assert.ok(has(lines[0], "comment", "/* note"));
  assert.deepEqual(parts(lines[1]), ["comment:more"]);
  assert.ok(has(lines[2], "comment", "*/"));
  assert.ok(has(lines[2], "keyword", "SELECT"));
});

test("a line can close one construct and open the next", () => {
  const highlighter = createHighlighter("app.js");
  const lines = highlight(highlighter, ["/* c */ const s = `a", "b`;"]);
  assert.ok(has(lines[0], "comment", "/* c */"));
  assert.ok(has(lines[0], "keyword", "const"));
  assert.ok(has(lines[0], "string", "`a"));
  assert.deepEqual(parts(lines[1]), ["string:b`", "op:;"]);
});

test("single-line constructs are unchanged and leave no state behind", () => {
  const highlighter = createHighlighter("app.js");
  const spans = highlighter.highlightLine('let x = "hi"; /* c */ f(3.5)');
  assert.ok(has(spans, "string", '"hi"'));
  assert.ok(has(spans, "comment", "/* c */"));
  assert.ok(has(spans, "number", "3.5"));
  assert.deepEqual(highlighter.state(), { open: null, heredoc: null });
});

test("reset drops state so the next line is highlighted fresh", () => {
  const highlighter = createHighlighter("app.js");
  highlighter.highlightLine("/* open");
  assert.equal(highlighter.state().open.kind, "comment");
  highlighter.reset();
  assert.deepEqual(highlighter.state(), { open: null, heredoc: null });
  assert.ok(has(highlighter.highlightLine("const a = 1"), "keyword", "const"));
});

test("highlightLines returns null for unsupported languages and spans otherwise", () => {
  assert.equal(highlightLines(["a", "b"], "data.json"), null);
  const spans = highlightLines(["/* a", "b */"], "app.css");
  assert.equal(spans.length, 2);
  assert.ok(has(spans[0], "comment", "/* a"));
  assert.deepEqual(parts(spans[1]), ["comment:b */"]);
  // Each call starts clean, so no state leaks between invocations.
  assert.deepEqual(parts(highlightLines(["plain"], "app.css")[0]), ["plain:plain"]);
});

test("empty lines inside a construct keep the carried state", () => {
  const highlighter = createHighlighter("app.js");
  const lines = highlight(highlighter, ["/* a", "", "b */"]);
  assert.deepEqual(parts(lines[1]), []);
  assert.deepEqual(parts(lines[2]), ["comment:b */"]);
});

test("sliceSpans clips a line's spans to a word-diff part", () => {
  const spans = highlightSpans("ab /* cd */ ef", "app.js");
  assert.deepEqual(parts(sliceSpans(spans, 3, 11)), ["comment:/* cd */"]);
  assert.deepEqual(parts(sliceSpans(spans, 0, 2)), ["plain:ab"]);
  assert.deepEqual(sliceSpans(null, 0, 2), []);
});

test("lineRuns merges consecutive hunks into one run", () => {
  const runs = lineRuns([
    { old_start: 0, new_start: 0, old: ["a"], new: ["a"] },
    { old_start: 1, new_start: 1, old: ["b"], new: ["b"] },
  ], "old");
  assert.equal(runs.length, 1);
  assert.deepEqual(runs[0], {
    start: 0,
    end: 2,
    lines: [{ line: 0, text: "a" }, { line: 1, text: "b" }],
  });
});

test("lineRuns starts a new run at an omitted-line gap", () => {
  const runs = lineRuns([
    { old_start: 0, new_start: 0, old: ["a"], new: ["a"] },
    { old_start: 4, new_start: 4, old: ["e"], new: ["e"] },
  ], "new");
  assert.equal(runs.length, 2);
  assert.deepEqual(runs.map((run) => [run.start, run.end]), [[0, 1], [4, 5]]);
});

test("lineRuns ignores hunks that contribute no lines to a side", () => {
  const hunks = [
    { old_start: 3, new_start: 3, old: ["del1", "del2"], new: [] },
    { old_start: 5, new_start: 3, old: ["x"], new: ["x"] },
  ];
  // The delete hunk adds no new lines, so the new side sees one run starting at
  // line 3. On the old side the delete's lines are contiguous with the next
  // hunk, so they merge into a single run 3..5.
  assert.deepEqual(lineRuns(hunks, "new").map((run) => [run.start, run.end]), [[3, 4]]);
  assert.deepEqual(lineRuns(hunks, "old").map((run) => [run.start, run.end]), [[3, 6]]);
});

test("lineRuns tolerates a missing side array or start", () => {
  assert.deepEqual(lineRuns([{ old_start: 0, old: ["a"] }], "new"), []);
  assert.deepEqual(lineRuns([{ old: ["a"] }], "old"), []);
  assert.deepEqual(lineRuns(null, "old"), []);
});

// Mirrors what app.js does: a fresh highlighter per run, feeding lines in order.
function spansByLine(hunks, side, path) {
  const spans = new Map();
  for (const run of lineRuns(hunks, side)) {
    const highlighter = createHighlighter(path);
    for (const entry of run.lines) {
      spans.set(`${side}:${entry.line}`, highlighter.highlightLine(entry.text));
    }
  }
  return spans;
}

test("contiguous hunks carry multi-line state across the boundary", () => {
  const hunks = [
    { old_start: 0, new_start: 0, old: ["/* open"], new: ["/* open"] },
    { old_start: 1, new_start: 1, old: ["close */"], new: ["close */"] },
  ];
  const spans = spansByLine(hunks, "old", "app.js");
  assert.deepEqual(parts(spans.get("old:1")), ["comment:close */"]);
});

test("an omitted-line gap resets the state machine", () => {
  const hunks = [
    { old_start: 0, new_start: 0, old: ["/* open"], new: ["/* open"] },
    { old_start: 10, new_start: 10, old: ["far away"], new: ["far away"] },
  ];
  const spans = spansByLine(hunks, "new", "app.js");
  assert.ok(has(spans.get("new:0"), "comment", "/* open"));
  // The documented limit: the construct that began before the gap is not
  // carried, so the far line keeps the line-local (non-comment) result.
  assert.ok(!spans.get("new:10").some((span) => span.kind === "comment"));
});

test("documented limits: nested block comments close at the first marker", () => {
  // Rust nests block comments, but the machine is deliberately shallow (#287).
  const highlighter = createHighlighter("lib.rs");
  const lines = highlight(highlighter, ["/* outer /* inner */ still?", "after */"]);
  const comment = lines[0].find((span) => span.kind === "comment");
  assert.ok(comment && comment.text.endsWith("*/"));
  // The machine considered the comment closed at the first marker, so the tail
  // is tokenized as code rather than comment. This codifies the limit.
  assert.ok(lines[0].some((span) => span.text.includes("still")));
  assert.deepEqual(highlighter.state(), { open: null, heredoc: null });
});

test("languageForPath is unchanged by the accurate path", () => {
  assert.equal(languageForPath("/tmp/app.ts"), "javascript");
  assert.equal(languageForPath("Makefile"), "shell");
});
