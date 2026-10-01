"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { localChangeRegions, localChangeIndex, regionAt } = require("../quickdiff.js");

test("a buffer identical to its baseline has no regions", () => {
  assert.deepEqual(localChangeRegions(["a", "b"], ["a", "b"]), []);
  assert.equal(localChangeIndex(["a", "b"], ["a", "b"]).size, 0);
});

test("one substituted line is a modified region", () => {
  const regions = localChangeRegions(["a", "b", "c"], ["a", "B", "c"]);
  assert.deepEqual(regions, [{ kind: "modified", start: 1, end: 1 }]);
});

test("adjacent changed lines merge into one region", () => {
  const regions = localChangeRegions(["a", "b", "c", "d"], ["a", "B", "C", "d"]);
  assert.deepEqual(regions, [{ kind: "modified", start: 1, end: 2 }]);
});

test("a restored line splits two regions", () => {
  const regions = localChangeRegions(["a", "b", "c", "d", "e"], ["A", "b", "C", "d", "E"]);
  assert.deepEqual(regions, [
    { kind: "modified", start: 0, end: 0 },
    { kind: "modified", start: 2, end: 2 },
    { kind: "modified", start: 4, end: 4 },
  ]);
});

test("changes at the edges are found", () => {
  assert.deepEqual(localChangeRegions(["a", "b"], ["A", "b"]), [
    { kind: "modified", start: 0, end: 0 },
  ]);
  assert.deepEqual(localChangeRegions(["a", "b"], ["a", "B"]), [
    { kind: "modified", start: 1, end: 1 },
  ]);
});

test("a longer current buffer reports the tail as added", () => {
  assert.deepEqual(localChangeRegions(["a", "b"], ["a", "b", "c"]), [
    { kind: "added", start: 2, end: 2 },
  ]);
  assert.deepEqual(localChangeRegions(["a", "b"], ["a", "X", "c"]), [
    { kind: "modified", start: 1, end: 1 },
    { kind: "added", start: 2, end: 2 },
  ]);
});

test("a shorter current buffer reports the missing tail as removed", () => {
  assert.deepEqual(localChangeRegions(["a", "b", "c"], ["a", "b"]), [
    { kind: "removed", start: 2, end: 2 },
  ]);
});

test("blank and non-string lines compare as written", () => {
  assert.deepEqual(localChangeRegions(["", "x"], ["", "x"]), []);
  assert.deepEqual(localChangeRegions([""], [" "]), [{ kind: "modified", start: 0, end: 0 }]);
});

test("the line index maps every changed line to its kind", () => {
  const index = localChangeIndex(["a", "b", "c", "d"], ["A", "b", "C", "d"]);
  assert.deepEqual([...index.entries()], [[0, "modified"], [2, "modified"]]);
});

test("regionAt finds the run a line falls in and refuses the rest", () => {
  const regions = [
    { kind: "modified", start: 1, end: 3 },
    { kind: "modified", start: 7, end: 8 },
  ];
  assert.equal(regionAt(regions, 0), null);
  assert.equal(regionAt(regions, 1), regions[0]);
  assert.equal(regionAt(regions, 3), regions[0]);
  assert.equal(regionAt(regions, 5), null);
  assert.equal(regionAt(regions, 8), regions[1]);
  assert.equal(regionAt(regions, 1.5), null);
  assert.equal(regionAt(null, 1), null);
});

test("malformed input is treated as empty rather than throwing", () => {
  assert.deepEqual(localChangeRegions(null, undefined), []);
  assert.deepEqual(localChangeRegions("not an array", []), []);
  assert.deepEqual(localChangeRegions([], []), []);
});
