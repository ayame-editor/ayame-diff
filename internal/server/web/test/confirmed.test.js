"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  hashText,
  hunkSignature,
  hunkSignatures,
  comparisonIdentity,
  restoreSignatures,
  unconfirmedIndexes,
  confirmProgress,
} = require("../confirmed.js");

test("the content hash is stable and eight hex digits", () => {
  assert.equal(hashText(""), "811c9dc5");
  assert.match(hashText("hello"), /^[0-9a-f]{8}$/);
  assert.equal(hashText("hello"), hashText("hello"));
  assert.notEqual(hashText("hello"), hashText("hellp"));
});

test("a hunk signature is derived from kind, range, and content, not the index", () => {
  const hunk = { kind: "replace", old_start: 4, old_len: 2, new_start: 6, new_len: 2, old: ["a", "b"], new: ["c", "d"] };
  assert.equal(hunkSignature(hunk), hunkSignature({ ...hunk }));
  assert.deepEqual(hunkSignatures([hunk, hunk]), [hunkSignature(hunk), hunkSignature(hunk)]);
});

test("changing the compared lines changes the signature even at the same position", () => {
  const before = { kind: "replace", old_start: 1, old_len: 1, new_start: 1, new_len: 1, old: ["a"], new: ["b"] };
  const after = { ...before, new: ["B"] };
  assert.notEqual(hunkSignature(before), hunkSignature(after));
});

test("insert and delete of the same text stay distinct", () => {
  const insert = { kind: "insert", old_start: 2, old_len: 0, new_start: 2, new_len: 1, old: [], new: ["x"] };
  const remove = { kind: "delete", old_start: 2, old_len: 1, new_start: 2, new_len: 0, old: ["x"], new: [] };
  assert.notEqual(hunkSignature(insert), hunkSignature(remove));
});

test("the same change at a different offset is a different signature", () => {
  const first = { kind: "replace", old_start: 1, old_len: 1, new_start: 1, new_len: 1, old: ["a"], new: ["b"] };
  const second = { ...first, old_start: 9, new_start: 9 };
  assert.notEqual(hunkSignature(first), hunkSignature(second));
});

test("malformed hunks still produce a signature instead of throwing", () => {
  assert.match(hunkSignature(undefined), /^:0:0:0:0:[0-9a-f]{8}$/);
  assert.match(hunkSignature({ kind: "insert", old_len: "2" }), /^insert:0:2:0:0:[0-9a-f]{8}$/);
});

test("the comparison identity separates paths but ignores order-insensitive wording", () => {
  const base = { mode: "text", old: "a.txt", new: "b.txt" };
  assert.equal(comparisonIdentity(base), comparisonIdentity({ ...base }));
  assert.notEqual(comparisonIdentity(base), comparisonIdentity({ ...base, new: "c.txt" }));
  assert.notEqual(comparisonIdentity(base), comparisonIdentity({ ...base, mode: "sorted" }));
  assert.equal(comparisonIdentity({ mode: "text", inline: true, oldText: "x", newText: "y" }),
    comparisonIdentity({ mode: "text", inline: true, oldText: "x", newText: "y" }));
  assert.notEqual(comparisonIdentity({ mode: "text", inline: true, oldText: "x", newText: "y" }),
    comparisonIdentity({ mode: "text", inline: true, oldText: "x", newText: "z" }));
});

test("restoring keeps current signatures and drops the ones whose hunk changed", () => {
  const signatures = ["a", "b", "c"];
  assert.deepEqual(restoreSignatures(signatures, ["a", "c", "stale"]), ["a", "c"]);
  assert.deepEqual(restoreSignatures(signatures, []), []);
  assert.deepEqual(restoreSignatures(signatures, undefined), []);
});

test("unconfirmed indexes list every hunk not in the confirmed set", () => {
  const signatures = ["a", "b", "c", "d"];
  assert.deepEqual(unconfirmedIndexes(signatures, new Set(["b", "d"])), [0, 2]);
  assert.deepEqual(unconfirmedIndexes(signatures, new Set(signatures)), []);
  assert.deepEqual(unconfirmedIndexes([], new Set(["a"])), []);
});

test("progress counts confirmed hunks inside the given scope", () => {
  const signatures = ["a", "b", "c", "d"];
  const confirmed = new Set(["a", "d"]);
  assert.deepEqual(confirmProgress(signatures, confirmed), { confirmed: 2, total: 4, unconfirmed: 2 });
  assert.deepEqual(confirmProgress(signatures, confirmed, [0, 1, 2]), { confirmed: 1, total: 3, unconfirmed: 2 });
  assert.deepEqual(confirmProgress(signatures, confirmed, []), { confirmed: 0, total: 0, unconfirmed: 0 });
});
