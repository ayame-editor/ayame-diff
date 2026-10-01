"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { lineRange, header } = require("../hunkheader.js");

// A stand-in for the UI translate function, echoing the key and its argument.
// The module under test must not reach into the catalog itself.
function t(key, arg) {
  if (arg === undefined) return key;
  return `${key}:${JSON.stringify(arg)}`;
}

test("a range is inclusive and 1-based", () => {
  assert.equal(lineRange(1203, 15), "1204\u20131218");
  assert.equal(lineRange(0, 1), "1");
  assert.equal(lineRange(4, 2), "5\u20136");
});

test("a side the hunk does not touch has no range", () => {
  assert.equal(lineRange(0, 0), "");
  assert.equal(lineRange(10, -3), "");
  assert.equal(lineRange(undefined, undefined), "");
});

test("a change reads as old and new line ranges plus the kind", () => {
  const { text, moved } = header({ kind: "change", old_start: 1203, old_len: 15, new_start: 1197, new_len: 17 }, t);
  assert.equal(text, "hunkOldLabel 1204\u20131218 hunkNewLabel 1198\u20131214 hunkChange");
  assert.equal(moved, null);
});

test("an insert names the empty old side instead of a bogus range", () => {
  const { text } = header({ kind: "insert", old_start: 41, old_len: 0, new_start: 41, new_len: 2 }, t);
  assert.equal(text, "hunkOldLabel hunkEmptyRange hunkNewLabel 42\u201343 hunkInsert");
});

test("a delete names the empty new side", () => {
  const { text } = header({ kind: "delete", old_start: 0, old_len: 3, new_start: 0, new_len: 0 }, t);
  assert.equal(text, "hunkOldLabel 1\u20133 hunkNewLabel hunkEmptyRange hunkDelete");
});

test("a moved hunk keeps its identity and peer, 1-based", () => {
  const { text, moved } = header(
    { kind: "change", old_start: 9, old_len: 1, new_start: 99, new_len: 1, move_id: 4, move_peer: 6 },
    t,
  );
  assert.deepEqual(moved, { id: 4, peer: 7 });
  assert.equal(text, "hunkOldLabel 10 hunkNewLabel 100 hunkChange hunkMoved:{\"id\":4,\"peer\":7}");
});

test("missing fields do not throw and fall back to a change", () => {
  const { text, moved } = header({}, t);
  assert.equal(text, "hunkOldLabel hunkEmptyRange hunkNewLabel hunkEmptyRange hunkChange");
  assert.equal(moved, null);
});
