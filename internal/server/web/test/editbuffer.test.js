"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { createEditBuffer, editableComparison } = require("../editbuffer.js");

function newBuffer(lines = ["alpha", "beta", "gamma"], extra = {}) {
  return createEditBuffer({
    path: "/tmp/left.txt",
    lines,
    profile: { encoding: "utf-8", bom: false, lineEnding: "\r\n", finalNewline: true },
    stamp: { size: "17", modified: "1000" },
    ...extra,
  });
}

test("a fresh buffer is clean and holds the file it was given", () => {
  const buffer = newBuffer();
  assert.equal(buffer.isDirty(), false);
  assert.deepEqual(buffer.lines(), ["alpha", "beta", "gamma"]);
  assert.equal(buffer.count(), 3);
  assert.equal(buffer.path(), "/tmp/left.txt");
});

test("editing a line marks it changed and reports what moved", () => {
  const buffer = newBuffer();
  assert.equal(buffer.setLine(1, "BETA"), true);
  assert.equal(buffer.isDirty(), true);
  assert.deepEqual(buffer.changedLines(), [1]);
  assert.deepEqual(buffer.lines(), ["alpha", "BETA", "gamma"]);
  assert.deepEqual(buffer.original(), ["alpha", "beta", "gamma"]);
});

test("typing a line back to its original value clears it again", () => {
  const buffer = newBuffer();
  buffer.setLine(1, "BETA");
  buffer.setLine(1, "beta");
  assert.equal(buffer.isDirty(), false);
  assert.deepEqual(buffer.changedLines(), []);
});

test("an unchanged keystroke is not a change", () => {
  const buffer = newBuffer();
  assert.equal(buffer.setLine(1, "beta"), false);
  assert.equal(buffer.isDirty(), false);
});

test("out-of-range and read-only edits are refused", () => {
  const buffer = newBuffer();
  assert.equal(buffer.setLine(9, "x"), false);
  assert.equal(buffer.setLine(-1, "x"), false);
  assert.equal(buffer.setLine(1.5, "x"), false);
  assert.equal(buffer.isDirty(), false);

  const locked = newBuffer(["alpha"], { readOnly: true });
  assert.equal(locked.readOnly(), true);
  assert.equal(locked.setLine(0, "changed"), false);
  assert.equal(locked.isDirty(), false);
});

test("the compared text joins with newlines, leaving the terminator to the save", () => {
  const buffer = newBuffer();
  buffer.setLine(2, "GAMMA");
  assert.equal(buffer.text(), "alpha\nbeta\nGAMMA");
  assert.equal(buffer.profile().lineEnding, "\r\n");
});

test("accepting a save makes the buffer clean at the new stamp", () => {
  const buffer = newBuffer();
  buffer.setLine(0, "ALPHA");
  buffer.accept({ stamp: { size: "18", modified: "2000" } });
  assert.equal(buffer.isDirty(), false);
  assert.deepEqual(buffer.original(), ["ALPHA", "beta", "gamma"]);
  assert.deepEqual(buffer.stamp(), { size: "18", modified: "2000" });
});

test("accepting a reload replaces the content entirely", () => {
  const buffer = newBuffer();
  buffer.setLine(0, "ALPHA");
  buffer.accept({ lines: ["one", "two"], stamp: { size: "8", modified: "3000" } });
  assert.deepEqual(buffer.lines(), ["one", "two"]);
  assert.equal(buffer.isDirty(), false);
});

test("revert throws the edits away", () => {
  const buffer = newBuffer();
  buffer.setLine(0, "ALPHA");
  buffer.setLine(1, "BETA");
  buffer.revert();
  assert.deepEqual(buffer.lines(), ["alpha", "beta", "gamma"]);
  assert.equal(buffer.isDirty(), false);
});

test("the returned arrays are copies", () => {
  const buffer = newBuffer();
  buffer.lines()[0] = "tampered";
  buffer.original()[0] = "tampered";
  buffer.profile().encoding = "tampered";
  assert.equal(buffer.line(0), "alpha");
  assert.equal(buffer.profile().encoding, "utf-8");
});

test("only a two-file text comparison can be edited", () => {
  assert.equal(editableComparison("text", false), true);
  assert.equal(editableComparison("text", true), false, "pasted text has nowhere to save");
  assert.equal(editableComparison("sorted", false), false, "sorted rows do not address file lines");
  assert.equal(editableComparison("csv", false), false);
  assert.equal(editableComparison("dir", false), false);
  assert.equal(editableComparison("threeway", false), false);
});
