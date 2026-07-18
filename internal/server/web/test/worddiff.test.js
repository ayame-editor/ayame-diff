// Execution-level tests for the word diff (#139).
//
// The repository had no way to run its client logic at all — the Go tests read
// app.js as text, which catches a renamed symbol but never a wrong result. This
// runs on node --test with no dependencies and no DOM, which is why the
// algorithm was moved into its own module.
"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { inlineWordDiff, inlineTokens, INLINE_MAX_TOKENS } = require("../worddiff.js");

// text reassembles one side, so a test can assert the diff preserved the input.
const text = (parts) => parts.map((part) => part.text).join("");
// changed returns only the marked-up runs.
const changed = (parts) => parts.filter((part) => part.changed).map((part) => part.text);

test("identical lines produce no diff", () => {
  assert.equal(inlineWordDiff("same text", "same text"), null);
});

test("a single changed word is isolated", () => {
  const diff = inlineWordDiff("foo bar baz", "foo qux baz");
  assert.deepEqual(changed(diff.oldParts), ["bar"]);
  assert.deepEqual(changed(diff.newParts), ["qux"]);
});

test("both sides round-trip to their original text", () => {
  const cases = [
    ["const value = compute(a, b);", "const result = compute(a, b, c);"],
    ["  indented", "\tindented"],
    ["", "added from nothing"],
    ["removed entirely", ""],
    ["trailing space ", "trailing space"],
  ];
  for (const [oldText, newText] of cases) {
    const diff = inlineWordDiff(oldText, newText);
    if (diff === null) {
      assert.equal(oldText, newText, `null diff for differing texts: ${oldText} / ${newText}`);
      continue;
    }
    assert.equal(text(diff.oldParts), oldText, `old side lost text for ${JSON.stringify(oldText)}`);
    assert.equal(text(diff.newParts), newText, `new side lost text for ${JSON.stringify(newText)}`);
  }
});

test("adjacent runs of the same kind are merged", () => {
  const diff = inlineWordDiff("aaa bbb ccc", "xxx yyy ccc");
  // "aaa bbb" and "xxx yyy" each changed; they must not come back as one part
  // per token, since that would produce a span per word in the DOM.
  for (const parts of [diff.oldParts, diff.newParts]) {
    for (let i = 1; i < parts.length; i++) {
      assert.notEqual(parts[i].changed, parts[i - 1].changed, "adjacent parts share a changed flag");
    }
  }
});

test("CJK is compared per character, not as one run", () => {
  const diff = inlineWordDiff("日本語のテキスト", "日本国のテキスト");
  assert.deepEqual(changed(diff.oldParts), ["語"]);
  assert.deepEqual(changed(diff.newParts), ["国"]);
});

test("a combining mark stays with its base letter", () => {
  const composed = "café latte";
  const tokens = inlineTokens(composed);
  assert.ok(tokens.includes("café"), `combining mark was split off: ${JSON.stringify(tokens)}`);
});

test("oversized input bails out instead of running the DP", () => {
  const huge = "word ".repeat(4000);
  assert.equal(inlineWordDiff(huge, huge + "tail"), null);
  // Just past the token limit, with different text on each side.
  const many = Array.from({ length: INLINE_MAX_TOKENS }, (_, i) => `t${i}`).join(" ");
  const other = Array.from({ length: INLINE_MAX_TOKENS }, (_, i) => `u${i}`).join(" ");
  assert.equal(inlineWordDiff(many, other), null);
});

test("the shared DP buffer does not leak between calls", () => {
  // The table is reused across calls, so a stale value would surface as a
  // different result for the same input depending on what ran before it.
  const first = inlineWordDiff("alpha beta gamma delta", "alpha BETA gamma DELTA");
  inlineWordDiff("a much longer line with many more tokens than the first one", "entirely different content here to fill the table");
  const again = inlineWordDiff("alpha beta gamma delta", "alpha BETA gamma DELTA");
  assert.deepEqual(again, first, "a previous call changed a later result");
});

test("a pure insertion marks only the inserted words", () => {
  const diff = inlineWordDiff("keep tail", "keep inserted tail");
  assert.deepEqual(changed(diff.oldParts), []);
  assert.deepEqual(changed(diff.newParts), ["inserted "]);
});

test("randomised inputs never lose or invent text", () => {
  let seed = 987654321;
  const random = () => (seed = (seed * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff;
  const words = ["alpha", "beta", " ", "(", ")", ",", "_", "42", "日本", "x"];
  const line = () => {
    let out = "";
    const count = 1 + Math.floor(random() * 20);
    for (let i = 0; i < count; i++) out += words[Math.floor(random() * words.length)];
    return out;
  };
  for (let i = 0; i < 500; i++) {
    const oldText = line();
    const newText = line();
    const diff = inlineWordDiff(oldText, newText);
    if (!diff) continue;
    assert.equal(text(diff.oldParts), oldText);
    assert.equal(text(diff.newParts), newText);
  }
});
