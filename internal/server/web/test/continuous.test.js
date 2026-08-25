"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { continuousEntries, windowAround, unloadTargets, stepSection, sectionAt } = require("../continuous.js");

test("only differing files are worth a continuous pass", () => {
  const entries = [
    { path: "a", status: "changed" },
    { path: "b", status: "same" },
    { path: "c", status: "added" },
    { path: "d", status: "removed" },
  ];
  assert.deepEqual(continuousEntries(entries).map((e) => e.path), ["a", "c", "d"]);
  assert.deepEqual(continuousEntries([]), []);
  assert.deepEqual(continuousEntries(undefined), []);
});

test("the window keeps the file being read, some of what is next, and a little behind", () => {
  assert.deepEqual(windowAround(5, 20), [4, 5, 6, 7]);
  assert.deepEqual(windowAround(0, 20), [0, 1, 2]);
  assert.deepEqual(windowAround(19, 20), [18, 19]);
  assert.deepEqual(windowAround(5, 20, { ahead: 0, behind: 0 }), [5]);
});

test("the window survives nonsense input rather than throwing at scroll time", () => {
  assert.deepEqual(windowAround(0, 0), []);
  assert.deepEqual(windowAround(-3, 4), [0, 1, 2]);
  assert.deepEqual(windowAround(99, 3), [1, 2]);
  assert.deepEqual(windowAround(null, 2), [0, 1]);
});

test("what is furthest from the file being read is given back first", () => {
  assert.deepEqual(unloadTargets([0, 1, 2, 3, 4, 5], 5, 3), [0, 1, 2]);
  assert.deepEqual(unloadTargets([0, 1, 2], 1, 3), [], "inside the budget nothing is unloaded");
  assert.deepEqual(unloadTargets([10, 11, 12, 13], 12, 2), [10, 11]);
});

test("the file being read is never unloaded", () => {
  const targets = unloadTargets([4, 5, 6], 5, 1);
  assert.ok(!targets.includes(5));
  assert.equal(targets.length, 2);
});

test("a tie is broken towards the earlier file, so scrolling down releases what is behind", () => {
  // 3 and 7 are both two files away from 5; the one already read goes first.
  assert.deepEqual(unloadTargets([3, 5, 7], 5, 2), [3]);
});

test("stepping stops at both ends of the change set", () => {
  assert.equal(stepSection(0, 1, 3), 1);
  assert.equal(stepSection(2, 1, 3), -1);
  assert.equal(stepSection(0, -1, 3), -1);
  assert.equal(stepSection(2, -1, 3), 1);
  assert.equal(stepSection(0, 1, 0), -1);
});

test("the file a scroll is at is the last one whose top has passed", () => {
  const offsets = [0, 400, 900, 1500];
  assert.equal(sectionAt(offsets, 0), 0);
  assert.equal(sectionAt(offsets, 399), 0);
  assert.equal(sectionAt(offsets, 400), 1);
  assert.equal(sectionAt(offsets, 1400), 2);
  assert.equal(sectionAt(offsets, 99999), 3);
  assert.equal(sectionAt([], 10), -1);
});
