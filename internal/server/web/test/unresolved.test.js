"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { unresolvedItems, unresolvedCount, targetsFor } = require("../unresolved.js");

const ITEMS = [
  { id: 0, label: "replace 3" },
  { id: 1, label: "insert 8" },
  { id: "abc", label: "conflict #abc" },
];

test("unresolvedItems lists the items with no choice in display order", () => {
  const choices = new Map([[1, "right"]]);
  const list = unresolvedItems(ITEMS, choices, null);
  assert.deepEqual(list, [
    { index: 0, id: 0, label: "replace 3" },
    { index: 2, id: "abc", label: "conflict #abc" },
  ]);
  assert.equal(unresolvedCount(ITEMS, choices, null), 2);
});

test("unresolvedItems accepts a plain-object choice map and treats empty as missing", () => {
  assert.deepEqual(unresolvedItems(ITEMS, { "0": "left", abc: "" }, null).map((item) => item.id), [1, "abc"]);
});

test("a fallback target covers every item, so none is unresolved", () => {
  assert.deepEqual(unresolvedItems(ITEMS, new Map(), "left"), []);
  assert.equal(unresolvedCount(ITEMS, new Map(), "left"), 0);
});

test("unresolvedItems tolerates malformed input", () => {
  assert.deepEqual(unresolvedItems(null, null, null), []);
  assert.deepEqual(unresolvedItems([null, { id: 7 }], null, null), [{ index: 1, id: 7, label: "" }]);
});

test("targetsFor offers only the targets each merge kind can honor", () => {
  assert.deepEqual(targetsFor("text"), ["left", "right", "markers"]);
  assert.deepEqual(targetsFor("csv"), ["left", "right"]);
  assert.deepEqual(targetsFor("threeway-text"), ["left", "right", "base", "markers"]);
  assert.deepEqual(targetsFor("threeway-csv"), ["left", "right", "base"]);
  assert.deepEqual(targetsFor("unknown"), ["left", "right", "markers"]);
});
