"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  nextMaxHunks,
  DEFAULT_MAX_HUNKS,
  MAX_MAX_HUNKS,
} = require("../truncation.js");

test("nextMaxHunks derives the cap from what was omitted", () => {
  // 200 shown + 50 omitted = 250 total; a quarter headroom rounds up.
  assert.equal(nextMaxHunks(200, 50), 313);
  // The whole diff plus headroom is strictly more than current + omitted, so a
  // single click lifts the truncation of an unchanged input.
  const next = nextMaxHunks(200, 50);
  assert.ok(next > 200 + 50);
});

test("nextMaxHunks keeps growing across repeated truncated runs", () => {
  let cap = 200;
  const grown = [cap];
  for (const omitted of [1234, 5678, 9999]) {
    cap = nextMaxHunks(cap, omitted);
    grown.push(cap);
  }
  for (let i = 1; i < grown.length; i++) {
    assert.ok(grown[i] > grown[i - 1], `cap ${grown[i]} did not grow past ${grown[i - 1]}`);
  }
});

test("nextMaxHunks always raises the cap, never lowers it", () => {
  // Even a run with nothing omitted (or no reported cap) has to move forward,
  // otherwise the action would loop on an unchanged value.
  assert.ok(nextMaxHunks(5, 0) > 5);
  assert.ok(nextMaxHunks(0, 0) > 0);
  assert.equal(nextMaxHunks(0, 10), Math.ceil((DEFAULT_MAX_HUNKS + 10) * 1.25));
});

test("nextMaxHunks stays within the sanity bound and tolerates junk", () => {
  assert.equal(nextMaxHunks(MAX_MAX_HUNKS, 10), MAX_MAX_HUNKS);
  assert.equal(nextMaxHunks(MAX_MAX_HUNKS - 1, 10), MAX_MAX_HUNKS);
  assert.equal(nextMaxHunks("nonsense", -5), nextMaxHunks(DEFAULT_MAX_HUNKS, 0));
  assert.equal(nextMaxHunks(undefined, undefined), nextMaxHunks(DEFAULT_MAX_HUNKS, 0));
  assert.equal(nextMaxHunks(200, -100), nextMaxHunks(200, 0));
});
