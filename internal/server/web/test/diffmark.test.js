// Execution-level tests for the colour-independent diff signalling (#298).
//
// The point of the module is that the kind of a line is decidable from data
// alone, so this pins the mapping itself: a green wash may vanish under a
// colour-blind theme or a monochrome screen, the glyph and the kind key may not.
"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { cellMarker, cellKindKey, hunkMarker, hunkKindKey, CONFLICT_MARK } = require("../diffmark.js");

test("added and deleted lines carry the patch glyphs", () => {
  assert.equal(cellMarker("add", "new"), "+");
  assert.equal(cellMarker("del", "old"), "-");
});

test("a changed pair reads as a removal then an addition", () => {
  assert.equal(cellMarker("chg", "old"), "-");
  assert.equal(cellMarker("chg", "new"), "+");
});

test("only diff classes get a marker", () => {
  assert.equal(cellMarker("same", "old"), "");
  assert.equal(cellMarker("empty", null), "");
});

test("each diff class names a kind that exists in the catalogue", () => {
  assert.equal(cellKindKey("add"), "added");
  assert.equal(cellKindKey("del"), "deleted");
  assert.equal(cellKindKey("chg"), "modified");
  assert.equal(cellKindKey("same"), "");
});

test("hunk kinds carry a glyph and a kind key", () => {
  assert.equal(hunkMarker("insert"), "+");
  assert.equal(hunkMarker("delete"), "\u2212");
  assert.equal(hunkMarker("replace"), "~");
  assert.equal(hunkMarker("unknown"), "");
  assert.equal(hunkKindKey("insert"), "added");
  assert.equal(hunkKindKey("delete"), "deleted");
  assert.equal(hunkKindKey("replace"), "modified");
  assert.equal(hunkKindKey("unknown"), "modified");
});

test("a conflict has a glyph of its own", () => {
  assert.equal(CONFLICT_MARK, "\u2260");
  // It must not be one of the +/-/~ already used for a line or a hunk kind, or
  // a screen reader or a monochrome reader could not tell them apart.
  for (const glyph of ["+", "-", "\u2212", "~"]) {
    assert.notEqual(CONFLICT_MARK, glyph);
  }
});
