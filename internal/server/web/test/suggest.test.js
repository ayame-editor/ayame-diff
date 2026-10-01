"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  MAX_PROPOSALS,
  classifyCell,
  differenceCount,
  estimateCauses,
  buildProposals,
  applyProposal,
  applyProposals,
  isApplied,
} = require("../suggest.js");

function changed(index, name, oldValue, newValue) {
  return { index, name, old: oldValue, new: newValue };
}

test("classifyCell attributes whitespace before case and numbers", () => {
  assert.equal(classifyCell("a", "a"), null, "equal values are not a difference");
  assert.deepEqual(classifyCell("a ", "a"), { cause: "whitespace", magnitude: 0 });
  assert.deepEqual(classifyCell(" 1", "1"), { cause: "whitespace", magnitude: 0 });
  assert.deepEqual(classifyCell("A", "a"), { cause: "case", magnitude: 0 });
  assert.deepEqual(classifyCell("10.0", "10.00"), { cause: "numeric", magnitude: 0 }, "formatting-only numbers still need a tolerance");
  assert.deepEqual(classifyCell("10", "10.5"), { cause: "numeric", magnitude: 0.5 });
  assert.deepEqual(classifyCell("1.10", "1.115"), { cause: "numeric", magnitude: 0.015 });
  assert.deepEqual(classifyCell("a", "b"), { cause: "content", magnitude: 0 });
});

test("differenceCount matches the server's logical difference count", () => {
  assert.equal(differenceCount({ changed_left: 5, changed_right: 5, left_only: 1, right_only: 2 }), 8);
  assert.equal(differenceCount({ changed_left: 0, changed_right: 0 }), 0);
  assert.equal(differenceCount(undefined), 0);
});

test("estimateCauses names the structural and cell-level causes", () => {
  const result = {
    summary: {
      changed_left: 3, changed_right: 3, left_only: 2, right_only: 2,
      column_changes: [{ index: 1, name: "name", count: 3 }, { index: 2, name: "price", count: 2 }],
    },
    differences: [
      { changed_columns: [changed(1, "name", "JAPAN", "japan"), changed(2, "price", "10", "10.01")] },
      { changed_columns: [changed(1, "name", "Alice ", "Alice")] },
    ],
  };
  // count/total: changed = 3, plus 2+2 = 7.
  const causes = estimateCauses(result, { alignColumnsByName: false });
  const names = causes.map((cause) => cause.cause);
  assert.ok(names.includes("columnOrder"), "column order is on offer while align-by-name is off");
  assert.ok(names.includes("whitespace"));
  assert.ok(names.includes("case"));
  assert.ok(names.includes("numeric"));
  assert.ok(names.includes("rowOrder"), "balanced left-only/right-only rows suggest moved rows");
  const numeric = causes.find((cause) => cause.cause === "numeric");
  assert.equal(numeric.magnitude, 0.01);
  assert.equal(numeric.columns[0].name, "price");
});

test("estimateCauses falls back to content when nothing normalizes", () => {
  const result = {
    summary: { changed_left: 1, changed_right: 1, column_changes: [] },
    differences: [{ changed_columns: [changed(0, "note", "alpha", "beta")] }],
  };
  assert.deepEqual(estimateCauses(result, { alignColumnsByName: true }).map((cause) => cause.cause), ["content"]);
  const total = differenceCount(result.summary);
  assert.equal(estimateCauses(result, { alignColumnsByName: true })[0].count, total);
});

test("estimateCauses flags a column most changes concentrate in", () => {
  const result = {
    summary: { changed_left: 4, changed_right: 4, column_changes: [{ index: 2, name: "updated_at", count: 8 }, { index: 1, name: "name", count: 1 }] },
    differences: [],
  };
  const causes = estimateCauses(result, {});
  const concentration = causes.find((cause) => cause.cause === "columnConcentration");
  assert.ok(concentration, "the dominant column is a candidate to ignore");
  assert.equal(concentration.columns[0].name, "updated_at");
});

test("buildProposals emits stackable patches with the observed tolerance", () => {
  const result = {
    summary: { changed_left: 3, changed_right: 3, left_only: 0, right_only: 0, column_changes: [{ index: 1, name: "name", count: 2 }, { index: 2, name: "price", count: 2 }] },
    differences: [
      { changed_columns: [changed(1, "name", "Alice ", "Alice"), changed(2, "price", "1.10", "1.115")] },
      { changed_columns: [changed(2, "price", "2", "2.02")] },
    ],
  };
  const proposals = buildProposals(result, { hasHeader: true, alignColumnsByName: false, whitespace: "none" });
  const ids = proposals.map((proposal) => proposal.id);
  assert.ok(ids.includes("alignColumnsByName"));
  assert.ok(ids.includes("whitespace"));
  const tolerance = proposals.find((proposal) => proposal.id === "tolerance:price");
  assert.ok(tolerance, "a per-column tolerance uses the header name");
  assert.deepEqual(tolerance.patch.columnTolerances, [{ name: "price", value: 0.02 }]);
  assert.equal(applyProposal({}, tolerance).columnTolerances.length, 1);
});

test("buildProposals offers a global tolerance only across several numeric columns", () => {
  const oneColumn = {
    summary: { changed_left: 1, changed_right: 1, column_changes: [{ index: 1, name: "price", count: 1 }] },
    differences: [{ changed_columns: [changed(1, "price", "1", "1.5")] }],
  };
  assert.equal(buildProposals(oneColumn, { hasHeader: true, alignColumnsByName: true }).some((proposal) => proposal.id === "tolerance:*"), false);

  const twoColumns = {
    summary: { changed_left: 1, changed_right: 1, column_changes: [{ index: 1, name: "price", count: 1 }, { index: 2, name: "amount", count: 1 }] },
    differences: [{ changed_columns: [changed(1, "price", "1", "1.5"), changed(2, "amount", "2", "2.25")] }],
  };
  const global = buildProposals(twoColumns, { hasHeader: true, alignColumnsByName: true, keyMode: "include", keyNames: ["id"] }).find((proposal) => proposal.id === "tolerance:*");
  assert.ok(global);
  assert.equal(global.patch.tolerance, 0.5);
  assert.equal(
    buildProposals(twoColumns, { hasHeader: true, alignColumnsByName: true, keyMode: "all" }).some((proposal) => proposal.id === "tolerance:*"),
    false,
    "a global tolerance cannot apply when the numeric columns are part of the key",
  );
});

test("buildProposals leaves key columns and already-set options alone", () => {
  const result = {
    summary: { changed_left: 1, changed_right: 1, column_changes: [{ index: 1, name: "price", count: 1 }] },
    differences: [{ changed_columns: [changed(1, "price", "1", "1.5")] }],
  };
  assert.equal(buildProposals(result, { hasHeader: true, keyMode: "include", keyNames: ["price"] }).length, 0);
  assert.equal(buildProposals(result, { hasHeader: true, whitespace: "change", ignoreCase: true }).length, 1, "only the numeric proposal remains");
});

test("proposals stack: applying twice unions columns without duplicates", () => {
  const base = { hasHeader: true, alignColumnsByName: false, columnTolerances: [] };
  const first = { id: "a", cause: "numeric", patch: { columnTolerances: [{ name: "price", value: 0.01 }] } };
  const second = { id: "b", cause: "numeric", patch: { columnTolerances: [{ name: "price", value: 0.02 }, { name: "amount", value: 0.1 }] } };
  const applied = applyProposals(base, [first, second]);
  assert.equal(applied.columnTolerances.length, 2, "the same column is not added twice");
  assert.equal(applied.columnTolerances[0].value, 0.01, "the earlier value wins for a shared column");
  assert.ok(isApplied(applied, first));
  assert.ok(isApplied(applied, second));
  assert.equal(isApplied(base, first), false);
});

test("applyProposal does not mutate the options it was given", () => {
  const base = { whitespace: "none", ignoreColumnNames: ["a"] };
  const next = applyProposal(base, { id: "i", cause: "columnConcentration", patch: { ignoreColumnNames: ["b"] } });
  assert.deepEqual(base.ignoreColumnNames, ["a"]);
  assert.deepEqual(next.ignoreColumnNames, ["a", "b"]);
});

test("the proposal list is capped so a suggest request stays bounded", () => {
  const columns = Array.from({ length: 12 }, (_, index) => changed(index + 1, `c${index}`, "1", String(1 + index / 100)));
  const result = {
    summary: { changed_left: 1, changed_right: 1, column_changes: columns.map((column) => ({ index: column.index, name: column.name, count: 1 })) },
    differences: [{ changed_columns: columns }],
  };
  const proposals = buildProposals(result, { hasHeader: true, alignColumnsByName: true, ignoreCase: true, whitespace: "change" });
  assert.ok(proposals.length <= MAX_PROPOSALS);
});
