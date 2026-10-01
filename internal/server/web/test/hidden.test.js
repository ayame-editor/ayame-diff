"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  lineRange,
  formatRange,
  excerpt,
  symbol,
  keyValue,
  keyRange,
  formatKeyRange,
  keyIndexes,
  describeGap,
} = require("../hidden.js");

// A stand-in for the UI translate function, echoing the key and its argument.
// The module under test must not reach into the catalog itself.
function t(key, arg) {
  if (arg === undefined) return key;
  return `${key}:${JSON.stringify(arg)}`;
}

test("a hidden run becomes a 1-based inclusive range", () => {
  assert.deepEqual(lineRange(0, 1), { first: 1, last: 1 });
  assert.deepEqual(lineRange(41, 120), { first: 42, last: 161 });
  assert.equal(lineRange(4, 0), null, "an empty run has no range");
  assert.equal(lineRange(undefined, undefined), null);
});

test("a range prints as one number or an en dash span", () => {
  assert.equal(formatRange({ first: 1204, last: 1218 }), "1204\u20131218");
  assert.equal(formatRange({ first: 5, last: 5 }), "5");
  assert.equal(formatRange(null), "");
});

test("an excerpt collapses whitespace and is bounded", () => {
  assert.equal(excerpt("  \tdef  process(x):  "), "def process(x):");
  assert.equal(excerpt(""), "");
  assert.equal(excerpt("a".repeat(200)).length, 80);
  assert.ok(excerpt("a".repeat(200)).endsWith("\u2026"));
});

test("a symbol is taken only from a line that declares one", () => {
  assert.equal(symbol("func processOrder() {"), "func processOrder");
  assert.equal(symbol("  def handle_event(self):"), "def handle_event");
  assert.equal(symbol("export class Customer {"), "class Customer");
  assert.equal(symbol("## Billing rules"), "Billing rules");
  assert.equal(symbol("  customer_id,price"), "", "data is not a symbol");
});

test("the bar names the line range and the first hidden line", () => {
  const oldStart = 1340;
  assert.equal(
    describeGap({ count: 300, oldStart, newStart: oldStart, preview: "  customer_id,price" }, t),
    'contextHiddenRange:{"count":"300","range":"1341\u20131640"} \u00b7 customer_id,price',
  );
});

test("the bar names both sides when the hidden range is shifted", () => {
  assert.equal(
    describeGap({ count: 4, oldStart: 9, newStart: 19, preview: "" }, t),
    'contextHiddenRangeSides:{"count":"4","old":"10\u201313","new":"20\u201323"}',
  );
});

test("source mode names a symbol and otherwise keeps the range", () => {
  const text = describeGap({ count: 2, oldStart: 0, newStart: 0, preview: "func main() {", source: true }, t);
  assert.equal(text, 'contextHiddenRange:{"count":"2","range":"1\u20132"} \u00b7 func main');
  const fallback = describeGap({ count: 2, oldStart: 0, newStart: 0, preview: "just a line", source: true }, t);
  assert.equal(fallback, 'contextHiddenRange:{"count":"2","range":"1\u20132"}');
});

test("a line with no range still reports the hidden count", () => {
  assert.equal(describeGap({ count: 0 }, t), 'contextHidden:{"count":"0"}');
});

test("a composite key joins its columns in key order", () => {
  assert.equal(keyValue(["1002", "A", "x"], [0, 1]), "1002 / A");
  assert.equal(keyValue(null, [0]), "");
});

test("a numeric key range is compared as numbers, not strings", () => {
  const range = keyRange([["1002"], ["9"], ["1340"], ["300"]], [0]);
  assert.deepEqual(range, { first: "9", last: "1340", single: false, count: 4 });
  assert.equal(formatKeyRange(range), "9\u20131340");
});

test("a one-value key range prints as a single value", () => {
  const range = keyRange([["cust-1"]], [0]);
  assert.deepEqual(range, { first: "cust-1", last: "cust-1", single: true, count: 1 });
  assert.equal(formatKeyRange(range), "cust-1");
  assert.equal(keyRange([], [0]), null);
});

test("key names and indexes map onto the inspection header", () => {
  assert.deepEqual(keyIndexes(["id", "name", "price"], ["id", "nope"], []), [0]);
  assert.deepEqual(keyIndexes(["id", "name", "price"], [], [2, 0, 2, 9]), [2, 0]);
  assert.deepEqual(keyIndexes([], ["id"], []), []);
});
