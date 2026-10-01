"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  ABSENT,
  positionalMapping,
  initialMapping,
  validateMapping,
  mappingToRequest,
  mappingFromRequest,
  reorderColumns,
} = require("../columnmap.js");

test("name alignment wins when every left header has a right counterpart", () => {
  const { mapping, source } = initialMapping(["id", "name", "value"], ["value", "id", "name"]);
  assert.equal(source, "name");
  assert.deepEqual(mapping, [
    { left: 0, right: 1, ignore: false },
    { left: 1, right: 2, ignore: false },
    { left: 2, right: 0, ignore: false },
  ]);
});

test("a missing or renamed header falls back to position and says so", () => {
  const { mapping, source } = initialMapping(["id", "label"], ["key", "tag"]);
  assert.equal(source, "position");
  assert.deepEqual(mapping, [
    { left: 0, right: 0, ignore: false },
    { left: 1, right: 1, ignore: false },
  ]);
});

test("name alignment can be turned off to start from position", () => {
  const { mapping, source } = initialMapping(["id", "name"], ["id", "name"], { alignByName: false });
  assert.equal(source, "position");
  assert.deepEqual(mapping.map((p) => [p.left, p.right]), [[0, 0], [1, 1]]);
});

test("a positional mapping keeps one-sided columns", () => {
  assert.deepEqual(positionalMapping(3, 2), [
    { left: 0, right: 0, ignore: false },
    { left: 1, right: 1, ignore: false },
    { left: 2, right: ABSENT, ignore: false },
  ]);
  assert.deepEqual(positionalMapping(1, 3), [
    { left: 0, right: 0, ignore: false },
    { left: ABSENT, right: 1, ignore: false },
    { left: ABSENT, right: 2, ignore: false },
  ]);
});

test("validation reports out-of-range, empty, and duplicate rows", () => {
  assert.deepEqual(validateMapping([{ left: 0, right: 0 }], 2, 2), { valid: true, errors: [] });

  const bad = validateMapping(
    [
      { left: 0, right: 0 },
      { left: 0, right: 1 },
      { left: -1, right: -1 },
      { left: 9, right: 9 },
    ],
    2,
    2
  );
  assert.equal(bad.valid, false);
  assert.deepEqual(bad.errors.map((entry) => entry.error), [
    "leftDuplicate",
    "empty",
    "leftRange",
    "rightRange",
  ]);
});

test("the request shape drops empty rows and keeps ignore", () => {
  const request = mappingToRequest([
    { left: 0, right: 1, ignore: false },
    { left: ABSENT, right: ABSENT, ignore: true },
    { left: 2, right: ABSENT, ignore: true },
  ]);
  assert.deepEqual(request, [
    { left: 0, right: 1, ignore: false },
    { left: 2, right: ABSENT, ignore: true },
  ]);
});

test("a saved mapping round-trips back into the editor", () => {
  const restored = mappingFromRequest([{ left: 0, right: 2, ignore: true }, { right: 1 }]);
  assert.deepEqual(restored, [
    { left: 0, right: 2, ignore: true },
    { left: ABSENT, right: 1, ignore: false },
  ]);
});

test("dragging a column reorders the display without touching the data", () => {
  assert.deepEqual(reorderColumns([0, 1, 2, 3], 0, 2), [1, 2, 0, 3]);
  assert.deepEqual(reorderColumns([0, 1, 2, 3], 3, 1), [0, 3, 1, 2]);
  assert.deepEqual(reorderColumns([0, 1, 2], 1, 1), [0, 1, 2]);
  assert.deepEqual(reorderColumns([0, 1, 2], 9, 0), [0, 1, 2], "an out-of-range drag is ignored");
  assert.deepEqual(reorderColumns([0, 1, 2], 0, 99), [1, 2, 0], "a drop past the end lands last");
});
