"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  LEVELS,
  LABEL_KEYS,
  requestFields,
  whitespaceMode,
  labelKey,
  levelForWhitespace,
  normalize,
} = require("../whitespace-scale.js");

// The values the old individual controls sent, keyed by the scale level that is
// equivalent to them. This is the mapping #259 promises keeps the API working.
const LEGACY = {
  strict: { whitespace: "none", ignoreEOL: false, ignoreTrailingEOL: false },
  eol: { whitespace: "none", ignoreEOL: true, ignoreTrailingEOL: false },
  "eol-change": { whitespace: "change", ignoreEOL: true, ignoreTrailingEOL: false },
  "eol-all": { whitespace: "all", ignoreEOL: true, ignoreTrailingEOL: false },
};

test("the scale is ordered from least to most ignored", () => {
  assert.deepEqual(LEVELS, ["strict", "eol", "eol-change", "eol-all"]);
});

test("each level maps to the legacy request fields", () => {
  for (const level of LEVELS) {
    assert.deepEqual(requestFields(level), LEGACY[level], `level ${level}`);
  }
});

test("the scale is monotonic: later levels never un-ignore an axis", () => {
  for (let i = 1; i < LEVELS.length; i++) {
    const previous = requestFields(LEVELS[i - 1]);
    const current = requestFields(LEVELS[i]);
    // Once line endings are ignored they stay ignored.
    assert.ok(previous.ignoreEOL <= current.ignoreEOL, `${LEVELS[i]} dropped ignoreEOL`);
    // Whitespace only ever moves none -> change -> all.
    const rank = { none: 0, change: 1, all: 2 };
    assert.ok(rank[previous.whitespace] <= rank[current.whitespace], `${LEVELS[i]} reduced whitespace`);
  }
  // Every non-strict level is built on ignoring line endings (P4Merge's axis).
  for (const level of ["eol", "eol-change", "eol-all"]) {
    assert.equal(requestFields(level).ignoreEOL, true, level);
  }
});

test("requestFields returns a copy, not the shared table", () => {
  const first = requestFields("strict");
  first.ignoreEOL = true;
  assert.equal(requestFields("strict").ignoreEOL, false);
});

test("an unknown or missing level falls back to strict", () => {
  assert.deepEqual(requestFields("nonsense"), LEGACY.strict);
  assert.deepEqual(requestFields(undefined), LEGACY.strict);
  assert.equal(normalize("nonsense"), "strict");
  assert.equal(normalize("eol-all"), "eol-all");
});

test("CSV consumes only the whitespace part of a level", () => {
  assert.equal(whitespaceMode("strict"), "none");
  assert.equal(whitespaceMode("eol"), "none");
  assert.equal(whitespaceMode("eol-change"), "change");
  assert.equal(whitespaceMode("eol-all"), "all");
  assert.equal(whitespaceMode("nonsense"), "none");
});

test("every level has its own label key", () => {
  const keys = LEVELS.map((level) => labelKey(level));
  assert.deepEqual(keys, ["wsScaleStrict", "wsScaleEOL", "wsScaleEOLChange", "wsScaleEOLAll"]);
  assert.equal(new Set(keys).size, LEVELS.length);
  assert.deepEqual(Object.keys(LABEL_KEYS), LEVELS);
});

test("a legacy whitespace value restores a level with that whitespace", () => {
  // CSV projects saved the whitespace value alone; CSV ignores line endings, so
  // the reverse mapping recovers the whitespace part and defaults the EOL part.
  for (const level of LEVELS) {
    const restored = levelForWhitespace(whitespaceMode(level));
    assert.equal(whitespaceMode(restored), whitespaceMode(level), `level ${level}`);
  }
  // "none" cannot tell strict from the EOL-only level, and CSV has no line
  // endings, so it restores to the safe default.
  assert.equal(levelForWhitespace("none"), "strict");
  assert.equal(levelForWhitespace("nonsense"), "strict");
  assert.equal(levelForWhitespace(undefined), "strict");
});
