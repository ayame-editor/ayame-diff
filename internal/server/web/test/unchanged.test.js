"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  buildUnchangedRegions,
  initialContextRanges,
  missingContextSpans,
  batchContextRanges,
} = require("../unchanged.js");

test("buildUnchangedRegions maps the runs around hunks", () => {
  const regions = buildUnchangedRegions([
    { old_start: 3, old_len: 1, new_start: 3, new_len: 2 },
    { old_start: 8, old_len: 2, new_start: 9, new_len: 1 },
  ], 14, 14);
  assert.deepEqual(regions, [
    { index: 0, oldStart: 0, newStart: 0, oldCount: 3, newCount: 3, count: 3 },
    { index: 1, oldStart: 4, newStart: 5, oldCount: 4, newCount: 4, count: 4 },
    { index: 2, oldStart: 10, newStart: 10, oldCount: 4, newCount: 4, count: 4 },
  ]);
  const truncated = buildUnchangedRegions([
    { old_start: 3, old_len: 1, new_start: 3, new_len: 1 },
  ], 100, 100, false);
  assert.equal(truncated[1].count, 0, "an unclassified truncated tail must not be shown as unchanged");
});

test("initialContextRanges shows the hunk-facing edges without overlap", () => {
  const regions = [
    { index: 0, count: 10 },
    { index: 1, count: 20 },
    { index: 2, count: 4 },
  ];
  assert.deepEqual(initialContextRanges(regions, 3), [
    { region: 0, offset: 7, count: 3 },
    { region: 1, offset: 0, count: 3 },
    { region: 1, offset: 17, count: 3 },
    { region: 2, offset: 0, count: 3 },
  ]);
  assert.deepEqual(initialContextRanges([
    { index: 0, count: 2 },
    { index: 1, count: 5 },
    { index: 2, count: 2 },
  ], 3), [
    { region: 0, offset: 0, count: 2 },
    { region: 1, offset: 0, count: 5 },
    { region: 2, offset: 0, count: 2 },
  ]);
});

test("missingContextSpans tolerates adjacent and overlapping segments", () => {
  assert.deepEqual(missingContextSpans(20, [
    { offset: 0, count: 4 },
    { offset: 4, count: 3 },
    { offset: 6, count: 4 },
    { offset: 17, count: 3 },
  ]), [
    { offset: 10, count: 7 },
  ]);
  assert.deepEqual(missingContextSpans(6, []), [{ offset: 0, count: 6 }]);
});

test("batchContextRanges respects both server request budgets", () => {
  const ranges = Array.from({ length: 9 }, (_, index) => ({ index, count: index % 2 ? 3 : 2 }));
  const batches = batchContextRanges(ranges, 4, 9);
  assert.deepEqual(batches.map((batch) => batch.length), [3, 3, 3]);
  for (const batch of batches) {
    assert.ok(batch.length <= 4);
    assert.ok(batch.reduce((total, range) => total + range.count, 0) <= 9);
  }
});
