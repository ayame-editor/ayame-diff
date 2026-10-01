"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  ORIGINS,
  normalizeOrigin,
  marker,
  originClass,
  isManual,
  labelKey,
  breakdown,
  countLines,
} = require("../mergeprovenance.js");

test("marker follows KDiff3: inputs keep their initial, manual is m", () => {
  assert.equal(marker("base"), "B");
  assert.equal(marker("left"), "L");
  assert.equal(marker("right"), "R");
  assert.equal(marker("manual"), "m");
  assert.equal(marker(""), "B", "an unknown origin must not leave an empty gutter");
  assert.equal(marker(undefined), "B");
});

test("origins normalize onto the four known sources", () => {
  assert.deepEqual(ORIGINS, ["base", "left", "right", "manual"]);
  for (const name of ORIGINS) {
    assert.equal(normalizeOrigin(name), name);
    assert.equal(originClass(name), "origin-" + name);
  }
  assert.equal(normalizeOrigin("bogus"), "base");
  assert.equal(isManual("manual"), true);
  assert.equal(isManual("left"), false);
});

test("labelKey names an i18n key for every origin", () => {
  assert.equal(labelKey("base"), "originBase");
  assert.equal(labelKey("left"), "originLeft");
  assert.equal(labelKey("right"), "originRight");
  assert.equal(labelKey("manual"), "originManual");
  assert.equal(labelKey("bogus"), "originBase");
});

test("breakdown partitions the total into adopted and manual", () => {
  const counts = breakdown({ base: 10, left: 3, right: 2, manual: 1, total: 99 });
  assert.deepEqual(counts, { base: 10, left: 3, right: 2, manual: 1, adopted: 15, total: 16 });
  assert.equal(counts.adopted + counts.manual, counts.total, "adopted and manual must add up");
  assert.deepEqual(breakdown(null), { base: 0, left: 0, right: 0, manual: 0, adopted: 0, total: 0 });
  assert.deepEqual(breakdown({ base: -4, manual: "x" }), { base: 0, left: 0, right: 0, manual: 0, adopted: 0, total: 0 });
});

test("countLines agrees with the server's breakdown", () => {
  const lines = [
    { origin: "base" }, { origin: "base" },
    { origin: "left" }, { origin: "right" }, { origin: "manual" },
  ];
  assert.deepEqual(countLines(lines), breakdown({ base: 2, left: 1, right: 1, manual: 1 }));
});

test("countLines treats a missing origin as base", () => {
  assert.deepEqual(countLines([{}, { origin: "nonsense" }]), { base: 2, left: 0, right: 0, manual: 0, adopted: 2, total: 2 });
});
