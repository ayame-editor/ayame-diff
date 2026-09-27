// Front-end performance budgets for the pure web helpers (#279).
//
// Browser rendering — first paint, scrolling, re-rendering a result — cannot be
// measured in CI without a browser, so it is deliberately not asserted here;
// what runs instead is the DOM-free work app.js drives on large synthetic
// input: the word-diff DP, the minimap segment packing, and the unchanged-range
// arithmetic. Budgets are generous because shared runners are noisy, so only a
// dramatic regression (an accidental O(n^2), a lost bound) trips them.
//
// node --test needs no dependencies, and this file skips unless
// AYAME_WEB_PERF=1 so the always-on lint job stays fast. The e2e-benchmarks CI
// job sets it.
"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { inlineWordDiff, INLINE_MAX_CHARS, INLINE_MAX_TOKENS } = require("../worddiff.js");
const { calculateMinimapSegments } = require("../minimap.js");
const {
  buildUnchangedRegions,
  initialContextRanges,
  batchContextRanges,
} = require("../unchanged.js");

const enabled = process.env.AYAME_WEB_PERF === "1";

function budgeted(name, budgetMs, run) {
  test(
    `performance budget: ${name}`,
    { skip: enabled ? false : "set AYAME_WEB_PERF=1 to run the front-end performance budgets" },
    () => {
      const start = process.hrtime.bigint();
      const detail = run();
      const ms = Number(process.hrtime.bigint() - start) / 1e6;
      if (detail) process.stdout.write(`# ${name}: ${ms.toFixed(1)}ms\n`);
      assert.ok(ms <= budgetMs, `${name} took ${ms.toFixed(1)}ms, past the ${budgetMs}ms budget`);
    },
  );
}

// denseLine builds a line close to the inline-diff caps without crossing them,
// so the DP does real work instead of bailing out.
function denseLine(seed) {
  const words = [];
  for (let i = 0; i < 55; i++) words.push(`word${(i * 7 + seed) % 11}`);
  return words.join(" ");
}

budgeted("word diff over a full result", 10000, () => {
  const left = denseLine(0);
  const right = denseLine(1);
  assert.ok(left.length + right.length <= INLINE_MAX_CHARS);
  assert.ok(left.split(" ").length * 2 <= INLINE_MAX_TOKENS);
  const runs = 10000;
  let changed = 0;
  for (let i = 0; i < runs; i++) {
    const diff = inlineWordDiff(left, right);
    if (diff) changed++;
  }
  assert.ok(changed > 0, "the synthetic lines should differ");
  return `${runs} diffs`;
});

budgeted("minimap packing of a dense result", 10000, () => {
  const markers = Array.from({ length: 100000 }, (_, index) => ({
    index,
    kind: index % 1000 === 501 ? "conflict" : "replace",
    displayLength: 1 + (index % 3),
  }));
  const runs = 20;
  let segments = 0;
  for (let i = 0; i < runs; i++) {
    segments = calculateMinimapSegments(markers, 256).length;
  }
  assert.ok(segments > 0 && segments <= 256);
  return `${markers.length} markers x ${runs}`;
});

budgeted("unchanged-range arithmetic on a huge diff", 10000, () => {
  const hunks = Array.from({ length: 200000 }, (_, index) => ({
    old_start: index * 3,
    old_len: index % 2,
    new_start: index * 3,
    new_len: (index + 1) % 2,
  }));
  const runs = 10;
  let total = 0;
  for (let i = 0; i < runs; i++) {
    const regions = buildUnchangedRegions(hunks, 600000, 600000);
    const ranges = initialContextRanges(regions, 3);
    total = batchContextRanges(ranges, 50, 500).length;
  }
  assert.ok(total >= 0);
  return `${hunks.length} hunks x ${runs}`;
});
