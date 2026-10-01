"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { isDowngraded, navigableIndexes, countDowngraded, essentialIndex, essentialIndexes } = require("../downgrade.js");

test("treats a whitespace-only hunk as downgraded", () => {
  assert.equal(isDowngraded({ downgraded: true }), true);
  assert.equal(isDowngraded({ kind: "replace" }), false);
  assert.equal(isDowngraded(null), false);
});

test("difference navigation skips downgraded and manually ignored hunks", () => {
  const hunks = [
    { kind: "replace" },
    { kind: "replace", downgraded: true },
    { kind: "insert" },
    { kind: "replace" },
  ];
  assert.deepEqual(navigableIndexes(hunks, new Set([3])), [0, 2]);
  // An array of ignored indexes works as well as a Set.
  assert.deepEqual(navigableIndexes(hunks, [3]), [0, 2]);
});

test("difference navigation counts only essential differences", () => {
  const hunks = Array.from({ length: 12 }, (_, index) => ({
    kind: "replace",
    downgraded: index % 4 === 3,
  }));
  const indexes = navigableIndexes(hunks, []);
  assert.equal(indexes.length, 9);
  assert.ok(indexes.every((index) => !hunks[index].downgraded));
});

test("a three-way result navigates conflicts only", () => {
  const hunks = [{}, {}, {}, {}];
  const kinds = ["left", "conflict", "right", "conflict"];
  assert.deepEqual(navigableIndexes(hunks, [], kinds), [1, 3]);
});

test("counts the dismissed differences in a result", () => {
  assert.equal(countDowngraded([{}, { downgraded: true }, { downgraded: true }]), 2);
  assert.equal(countDowngraded(undefined), 0);
});

test("maps rendered ignore indexes onto the real-only server list", () => {
  const hunks = [
    { kind: "replace" },
    { kind: "replace", downgraded: true },
    { kind: "insert" },
    { kind: "replace", downgraded: true },
    { kind: "delete" },
  ];
  // Rendered indexes 2, 4 are the second and third real differences.
  assert.deepEqual(essentialIndexes(hunks, [2, 4]), [1, 2]);
  // A dismissed index is dropped, and an already-real index is unchanged.
  assert.deepEqual(essentialIndexes(hunks, [0, 1]), [0]);
  // The single-index form backs the merge-choice remap.
  assert.equal(essentialIndex(hunks, 4), 2);
  assert.equal(essentialIndex(hunks, 1), -1);
});
