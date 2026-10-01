"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  ratio,
  formatPercent,
  barPercent,
  csvColumnRows,
  textSummary,
  buildStats,
  toCSV,
  toJSON,
} = require("../statsview.js");

test("formatPercent keeps a small share from reading as zero", () => {
  assert.equal(formatPercent(0), "0%");
  assert.equal(formatPercent(0.5), "50%");
  assert.equal(formatPercent(0.004), "<1%");
  assert.equal(formatPercent(0.125), "13%");
  assert.equal(formatPercent(0.125, 1), "12.5%");
  assert.equal(formatPercent(1), "100%");
});

test("barPercent clamps to the largest count", () => {
  assert.equal(barPercent(5, 10), 50);
  assert.equal(barPercent(10, 10), 100);
  assert.equal(barPercent(20, 10), 100);
  assert.equal(barPercent(1, 0), 0);
  assert.equal(ratio(1, 4), 0.25);
  assert.equal(ratio(1, 0), 0);
});

test("csvColumnRows keeps every column and scales the bar to the largest", () => {
  const rows = csvColumnRows({
    column_changes: [
      { index: 2, name: "price", count: 4, share: 0.5, numeric: { count: 4, sum: 7.5, mean: 1.875, min: 0, max: 5, increased: 2, decreased: 1, unchanged: 1 } },
      { index: 0, name: "id", count: 1, share: 0.125 },
      { index: 1, name: "a,b", count: 2, share: 0.25 },
      { index: 3, name: "qty", count: 3, share: 0.375 },
    ],
  });
  assert.equal(rows.length, 4, "the rows must not be capped");
  assert.deepEqual(rows.map((row) => row.name), ["price", "id", "a,b", "qty"]);
  assert.equal(rows[0].bar, 100);
  assert.equal(rows[1].bar, 25);
  assert.equal(rows[0].numeric.sum, 7.5);
  assert.equal(rows[1].numeric, undefined);
});

test("textSummary derives the whole-file share when the server omits it", () => {
  const summary = textSummary({ old_lines: 10, new_lines: 12, added: 2, deleted: 0, modified: 1, hunk_count: 3 });
  assert.equal(summary.changed_lines, 3);
  assert.equal(summary.total_lines, 12);
  assert.equal(summary.changed_share, 0.25);
  const reported = textSummary({ old_lines: 10, new_lines: 10, changed_lines: 4, changed_share: 0.4, largest_hunk: 7 });
  assert.equal(reported.changed_share, 0.4);
  assert.equal(reported.largest_hunk, 7);
});

test("toCSV writes a long-format machine-readable table", () => {
  const document = buildStats("csv", {
    header: ["id", "price"],
    summary: {
      left_rows: 3, right_rows: 3, changed_rows: 2,
      column_changes: [
        { index: 1, name: "price", count: 2, share: 1, numeric: { count: 2, sum: 3, mean: 1.5, min: 1, max: 2, increased: 2, decreased: 0, unchanged: 0 } },
      ],
    },
  });
  const csv = toCSV(document);
  const lines = csv.trim().split("\n");
  assert.equal(lines[0], "section,column,metric,value");
  assert.ok(lines.includes("summary,,changed_rows,2"));
  assert.ok(lines.includes("column,price,changed,2"));
  assert.ok(lines.includes("column,price,delta_mean,1.5"));
  assert.ok(lines.includes("column,price,increased,2"));
});

test("toCSV quotes a column name that contains a comma", () => {
  const document = buildStats("csv", { summary: { column_changes: [{ index: 0, name: "a,b", count: 1, share: 1 }] } });
  assert.ok(toCSV(document).includes('column,"a,b",changed,1'));
});

test("toJSON emits a parseable artifact", () => {
  const document = buildStats("text", { added: 1, deleted: 2, old_lines: 4, new_lines: 4 });
  const parsed = JSON.parse(toJSON(document));
  assert.equal(parsed.kind, "text");
  assert.equal(parsed.summary.changed_lines, 3);
});
