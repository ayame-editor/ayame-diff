"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { createMergeSelection, orderFor, ORDERS } = require("../mergeselect.js");

test("two-way hunks start unresolved and toggle symmetrically", () => {
  const selection = createMergeSelection("text");
  assert.deepEqual(selection.sides(0), []);
  assert.equal(selection.selected(0), false);

  assert.deepEqual(selection.toggle(0, "left"), ["left"]);
  assert.equal(selection.selected(0), true);
  // Clicking the chosen side again clears it: select and deselect are one gesture.
  assert.deepEqual(selection.toggle(0, "left"), []);
  assert.equal(selection.selected(0), false);
  assert.equal(selection.size(), 0);
});

test("toggling both sides adopts both in canonical order regardless of click order", () => {
  const first = createMergeSelection("text");
  first.toggle(0, "left");
  first.toggle(0, "right");
  const second = createMergeSelection("text");
  second.toggle(0, "right");
  second.toggle(0, "left");
  assert.deepEqual(first.sides(0), ["left", "right"]);
  assert.deepEqual(second.sides(0), ["left", "right"]);
  assert.equal(first.isBoth(0), true);
});

test("deselecting one side leaves the other adopted", () => {
  const selection = createMergeSelection("text");
  selection.toggle(0, "left");
  selection.toggle(0, "right");
  assert.deepEqual(selection.toggle(0, "right"), ["left"]);
  assert.equal(selection.isBoth(0), false);
});

test("three-way choices order base before left before right", () => {
  const selection = createMergeSelection("threeway");
  selection.toggle(7, "right");
  selection.toggle(7, "base");
  selection.toggle(7, "left");
  assert.deepEqual(selection.sides(7), ["base", "left", "right"]);
});

test("unresolved counts units without any adopted contribution", () => {
  const selection = createMergeSelection("text");
  assert.equal(selection.unresolved(3), 3);
  selection.toggle(0, "left");
  selection.toggle(2, "right");
  assert.equal(selection.unresolved(3), 1);
  selection.toggle(1, "left");
  assert.equal(selection.unresolved(3), 0);
  // Both sides on one hunk still resolve exactly that one hunk.
  selection.toggle(0, "right");
  assert.equal(selection.unresolved(3), 0);
  assert.equal(selection.size(), 3);
});

test("toWire emits comma-joined canonical sides and omits empty hunks", () => {
  const selection = createMergeSelection("text");
  selection.toggle(0, "right");
  selection.toggle(0, "left");
  selection.toggle(1, "left");
  selection.toggle(1, "left"); // cleared again
  assert.deepEqual(selection.toWire(), { 0: "left,right" });
});

test("clone is independent and replace restores a snapshot", () => {
  const selection = createMergeSelection("threeway-csv");
  selection.set("abc", ["right", "left"]);
  const copy = selection.clone();
  copy.toggle("abc", "base");
  assert.deepEqual(selection.sides("abc"), ["left", "right"]);
  assert.deepEqual(copy.sides("abc"), ["base", "left", "right"]);
  selection.replace(copy.entries());
  assert.deepEqual(selection.sides("abc"), ["base", "left", "right"]);
});

test("set keeps only known sides and choose replaces", () => {
  const selection = createMergeSelection("threeway");
  selection.set(0, ["left", "bogus", "left"]);
  assert.deepEqual(selection.sides(0), ["left"]);
  selection.choose(0, "right");
  assert.deepEqual(selection.sides(0), ["right"]);
  assert.throws(() => selection.toggle(0, "bogus"), /unknown merge side/);
});

test("the canonical order is exposed per kind", () => {
  assert.deepEqual(orderFor("text"), ["left", "right"]);
  assert.deepEqual(orderFor("threeway"), ["base", "left", "right"]);
  assert.deepEqual(ORDERS["threeway-csv"], ["base", "left", "right"]);
  // Unknown kinds fall back to the two-way order rather than an empty one.
  assert.deepEqual(orderFor("nope"), ["left", "right"]);
});
