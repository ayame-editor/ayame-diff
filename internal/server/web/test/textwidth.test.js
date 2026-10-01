// Execution-level tests for the GUI's display-width model (#289).
//
// internal/textwidth already has Go tests for the same decisions; these run the
// JavaScript the browser actually uses so a divergence is caught here rather
// than only by eye. A separate Go test checks the two implementations against
// each other and against the CLI's side-by-side output.
"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const {
  displayWidth,
  expandTabs,
  nextTabStop,
  normalizeTabSize,
  truncate,
  padRight,
  DEFAULT_TAB_SIZE,
} = require("../textwidth.js");

const WIDE = { eastAsianAmbiguousWide: true };

test("mixed ASCII and CJK occupies the sum of its cells", () => {
  assert.equal(displayWidth("A東京"), 5); // 1 + 2 + 2
  assert.equal(displayWidth("東京"), 4);
  assert.equal(displayWidth("hello"), 5);
  // Halfwidth katakana is one cell each; fullwidth Latin is two.
  assert.equal(displayWidth("ﾊﾝｶｸ"), 4);
  assert.equal(displayWidth("ＡＢ"), 4);
});

test("East Asian Ambiguous characters follow the locale option", () => {
  const ambiguous = "○※α";
  assert.equal(displayWidth(ambiguous), 3);
  assert.equal(displayWidth(ambiguous, WIDE), 6);
});

test("clusters are counted once and never split", () => {
  assert.equal(displayWidth("👨‍👩‍👧‍👦"), 2); // ZWJ family
  assert.equal(displayWidth("👍🏽"), 2); // emoji + skin tone
  assert.equal(displayWidth("🇯🇵"), 2); // regional-indicator flag
  assert.equal(displayWidth("1️⃣"), 2); // keycap sequence
  assert.equal(displayWidth("e\u0301"), 1); // combining acute
  assert.equal(displayWidth("a\x00\x1fb"), 2); // controls contribute nothing
});

test("padding and truncating use display cells, not code units", () => {
  assert.equal(padRight("A東", 6), "A東   ");
  assert.equal(displayWidth(padRight("A東", 6)), 6);
  assert.equal(padRight("○", 4, WIDE), "○  ");
  const cut = truncate("東京都", 4);
  assert.ok(displayWidth(cut) <= 4, `truncate overflowed: ${JSON.stringify(cut)}`);
  assert.ok(cut.length <= "東京都".length, "truncate grew the string");
  // A cluster is kept whole even when it would not fit.
  assert.equal(truncate("👨‍👩‍👧‍👦abcd", 5), "👨‍👩‍👧‍👦...");
  assert.equal(truncate("日本語", 3), "...");
});

test("tab stops are a function of the column and the tab size", () => {
  assert.equal(DEFAULT_TAB_SIZE, 8);
  assert.equal(nextTabStop(0, 8), 8);
  assert.equal(nextTabStop(7, 8), 8);
  assert.equal(nextTabStop(8, 8), 16);
  assert.equal(nextTabStop(3, 4), 4);
  assert.equal(normalizeTabSize("4"), 4);
  assert.equal(normalizeTabSize(0), DEFAULT_TAB_SIZE);
});

test("expanding tabs uses display cells, so CJK shifts the stop", () => {
  // Two CJK cells before the tab push it two columns later than ASCII would.
  assert.equal(expandTabs("a\tb", 4), "a   b");
  assert.equal(expandTabs("東\tb", 4), "東  b");
  assert.equal(expandTabs("ab\tc", 4), "ab  c");
  assert.equal(expandTabs("a\tb", 4, WIDE), "a   b");
});

test("no tab expansion happens when there is no tab", () => {
  assert.equal(expandTabs("no tabs here", 4), "no tabs here");
});
