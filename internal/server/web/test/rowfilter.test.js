"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  OPERATORS,
  newCondition,
  newFilter,
  newGroup,
  normalizeFilter,
  pruneFilter,
  filterIsEmpty,
  validateFilter,
  compileFilter,
  summarizeFilter,
  matchesRow,
  countConditions,
} = require("../rowfilter.js");

test("an untouched builder compiles to no filter at all", () => {
  assert.equal(filterIsEmpty(newFilter()), true);
  assert.equal(compileFilter(newFilter(), { hasHeader: true }), null);
  const incomplete = newFilter();
  incomplete.conditions.push(newCondition("eq")); // no column, no value
  assert.equal(pruneFilter(incomplete), null);
});

test("conditions compile to the engine wire shape by header name", () => {
  const filter = newFilter();
  filter.conditions.push({ ...newCondition("eq"), column: "status", value: "active" });
  filter.groups.push(newGroup());
  filter.groups[0].match = "any";
  filter.groups[0].conditions.push({ ...newCondition("between"), column: "amount", value: "10", value2: "20" });
  const compiled = compileFilter(filter, { hasHeader: true, columns: ["status", "amount"] });
  assert.deepEqual(compiled, {
    match: "all",
    conditions: [{ op: "eq", column: "status", value: "active" }],
    groups: [{ match: "any", conditions: [{ op: "between", column: "amount", value: "10", value2: "20" }] }],
  });
});

test("headerless conditions compile to indices and carry negation", () => {
  const filter = newFilter();
  filter.ignoreCase = true;
  filter.not = true;
  filter.conditions.push({ ...newCondition("starts"), byIndex: true, index: 2, value: "tmp", not: true });
  const compiled = compileFilter(filter, { hasHeader: false });
  assert.deepEqual(compiled, {
    match: "all",
    ignore_case: true,
    not: true,
    conditions: [{ op: "starts", by_index: true, index: 2, value: "tmp", not: true }],
  });
});

test("validation reports each incomplete or malformed condition", () => {
  const filter = newFilter();
  filter.conditions.push({ ...newCondition("eq") }); // missing column and value
  filter.conditions.push({ ...newCondition("eq"), column: "missing", value: "x" });
  filter.conditions.push({ ...newCondition("regex"), column: "name", value: "(" });
  filter.conditions.push({ ...newCondition("between"), column: "amount", value: "1" }); // missing value2
  const codes = validateFilter(filter, { hasHeader: true, columns: ["name", "amount"] }).map((error) => error.code);
  assert.deepEqual(codes, ["missing_column", "missing_value", "unknown_column", "invalid_regex", "missing_value"]);
});

test("normalizeFilter tolerates untrusted project JSON", () => {
  const normalized = normalizeFilter({
    match: "bogus",
    conditions: [{ op: "nope", column: 5, index: -3, value: null }, null, "x"],
    groups: [null, { conditions: [{ op: "eq", column: "id", value: 1 }] }],
  });
  assert.equal(normalized.match, "all");
  assert.equal(normalized.conditions.length, 1);
  assert.equal(normalized.conditions[0].op, "eq", "unknown operators fall back to eq");
  assert.equal(normalized.conditions[0].column, "5");
  assert.equal(normalized.conditions[0].index, 0);
  assert.equal(normalized.conditions[0].value, "");
  assert.equal(normalized.groups.length, 1);
  assert.equal(normalized.groups[0].conditions[0].value, "1");
});

test("matchesRow mirrors the engine operators and boolean groups", () => {
  const columns = ["status", "region", "amount", "name"];
  const rows = [
    ["active", "east", "1500", "TEST_a"],
    ["off", "west", "50", "prod_b"],
    ["active", "north", "1200", "TEST_c"],
  ];
  const andOr = newFilter();
  andOr.match = "all";
  andOr.conditions.push({ ...newCondition("eq"), column: "status", value: "active" });
  andOr.groups.push(newGroup());
  andOr.groups[0].match = "any";
  andOr.groups[0].conditions.push({ ...newCondition("eq"), column: "region", value: "east" });
  andOr.groups[0].conditions.push({ ...newCondition("gt"), column: "amount", value: "1000" });
  const kept = rows.filter((row) => matchesRow(andOr, row, columns, true));
  assert.deepEqual(kept, [rows[0], rows[2]]);

  const regexNot = newFilter();
  regexNot.conditions.push({ ...newCondition("regex"), column: "name", value: "^TEST", not: true });
  assert.deepEqual(rows.filter((row) => matchesRow(regexNot, row, columns, true)), [rows[1]]);

  const between = newFilter();
  between.conditions.push({ ...newCondition("between"), column: "amount", value: "1000", value2: "2000" });
  assert.deepEqual(rows.filter((row) => matchesRow(between, row, columns, true)), [rows[0], rows[2]]);

  const empty = newFilter();
  empty.conditions.push({ ...newCondition("empty"), column: "name", value: "" });
  assert.deepEqual(rows.filter((row) => matchesRow(empty, row, columns, true)), []);
});

test("ignore case folds string operators but not regex classes", () => {
  const columns = ["name"];
  const filter = newFilter();
  filter.ignoreCase = true;
  filter.conditions.push({ ...newCondition("starts"), column: "name", value: "test" });
  assert.equal(matchesRow(filter, ["TEST_1"], columns, true), true);
});

test("headerless rows match by index and summarise by position", () => {
  const filter = newFilter();
  filter.conditions.push({ ...newCondition("eq"), byIndex: true, index: 1, value: "east" });
  assert.equal(matchesRow(filter, ["1", "east"], [], false), true);
  assert.equal(matchesRow(filter, ["1", "west"], [], false), false);
  assert.equal(summarizeFilter(filter, { hasHeader: false }), "#1 eq east");
});

test("the operator catalog is internally consistent", () => {
  assert.equal(OPERATORS.length, 13);
  for (const operator of OPERATORS) {
    assert.ok(operator.values === 0 || operator.values === 1 || operator.values === 2);
    assert.ok(operator.kind === "text" || operator.kind === "compare");
  }
  const filter = newFilter();
  filter.conditions.push({ ...newCondition("eq"), column: "a", value: "b" });
  filter.groups.push(newGroup());
  filter.groups[0].conditions.push({ ...newCondition("eq"), column: "c", value: "d" });
  assert.equal(countConditions(filter), 2);
  assert.match(summarizeFilter(filter, { hasHeader: true }), /a eq b/);
});
