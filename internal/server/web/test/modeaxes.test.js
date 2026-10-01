"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  SHAPES,
  INTERPRETATIONS,
  interpretationsFor,
  supportsInterpretation,
  composeMode,
  decomposeMode,
} = require("../modeaxes.js");

test("the axes are a closed set", () => {
  assert.deepEqual(SHAPES, ["two", "three", "folder"]);
  assert.deepEqual(INTERPRETATIONS, ["text", "sorted", "csv"]);
});

test("not every reading applies to every shape", () => {
  assert.deepEqual(interpretationsFor("two"), ["text", "sorted", "csv"]);
  // The server has no sorted three-way mode, so it is not offered.
  assert.deepEqual(interpretationsFor("three"), ["text", "csv"]);
  // A folder comparison compares names and bytes, not a reading of a file.
  assert.deepEqual(interpretationsFor("folder"), []);
  assert.deepEqual(interpretationsFor("???"), []);
});

test("supportsInterpretation follows the shape", () => {
  assert.equal(supportsInterpretation("two", "sorted"), true);
  assert.equal(supportsInterpretation("three", "sorted"), false);
  assert.equal(supportsInterpretation("three", "csv"), true);
  assert.equal(supportsInterpretation("folder", "text"), false);
});

test("composeMode yields every server mode the GUI can ask for", () => {
  assert.equal(composeMode("two", "text"), "text");
  assert.equal(composeMode("two", "sorted"), "sorted");
  assert.equal(composeMode("two", "csv"), "csv");
  assert.equal(composeMode("three", "text"), "threeway");
  assert.equal(composeMode("three", "csv"), "threeway-csv");
  assert.equal(composeMode("folder", ""), "dir");
});

test("composeMode never invents a mode for an unsupported pair", () => {
  // A stale toolbar value (for example the old "sorted" after switching to a
  // three-file shape) falls back to the shape's first reading.
  assert.equal(composeMode("three", "sorted"), "threeway");
  assert.equal(composeMode("two", "???"), "text");
  assert.equal(composeMode("folder", "csv"), "dir");
});

test("decomposeMode round-trips every mode", () => {
  const modes = ["text", "sorted", "csv", "threeway", "threeway-csv", "dir"];
  for (const mode of modes) {
    const { shape, interpretation } = decomposeMode(mode);
    assert.equal(composeMode(shape, interpretation), mode, mode);
  }
});

test("decomposeMode reads an unknown mode as plain two-file text", () => {
  assert.deepEqual(decomposeMode("???"), { shape: "two", interpretation: "text" });
});
