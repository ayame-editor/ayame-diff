"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  TOOLBAR_CONTROLS,
  lineFilterCount,
  readPolicy,
  writePolicy,
  isDefault,
  activeConditionIds,
  describeConditions,
} = require("../conditions.js");

test("the toolbar vocabulary is derived from the settings controls", () => {
  assert.deepEqual(
    readPolicy({
      whitespace: "change",
      ignoreCase: true,
      ignoreEOL: true,
      ignoreTrailingEOL: true,
      lineFilters: "alpha\n\n  beta  \n",
      detectMoves: true,
    }),
    { whitespace: "change", case: "ignore", eol: "both", filters: 2, moves: "detect" },
  );
});

test("a missing or nonsensical policy reads as the all-defaults state", () => {
  const defaults = { whitespace: "none", case: "distinguish", eol: "as-is", filters: 0, moves: "off" };
  assert.deepEqual(readPolicy(), defaults);
  assert.deepEqual(readPolicy({ whitespace: "bogus", ignoreCase: false }), defaults);
});

test("the two EOL toggles collapse into the four states of one control", () => {
  assert.equal(readPolicy({ ignoreEOL: true }).eol, "eol");
  assert.equal(readPolicy({ ignoreTrailingEOL: true }).eol, "trailing");
  assert.equal(readPolicy({ ignoreEOL: true, ignoreTrailingEOL: true }).eol, "both");
});

test("the line-filter count accepts the textarea, an array, or a number", () => {
  assert.equal(lineFilterCount("a\nb"), 2);
  assert.equal(lineFilterCount(["a", "", " b "]), 2);
  assert.equal(lineFilterCount(4), 4);
  assert.equal(lineFilterCount(undefined), 0);
});

test("a toolbar change writes only the settings that control owns", () => {
  assert.deepEqual(writePolicy("whitespace", "all"), { whitespace: "all" });
  assert.deepEqual(writePolicy("case", "ignore"), { ignoreCase: true });
  assert.deepEqual(writePolicy("eol", "trailing"), { ignoreEOL: false, ignoreTrailingEOL: true });
  assert.deepEqual(writePolicy("eol", "both"), { ignoreEOL: true, ignoreTrailingEOL: true });
  assert.deepEqual(writePolicy("moves", "off"), { detectMoves: false });
});

test("an unrecognized selection changes nothing", () => {
  assert.deepEqual(writePolicy("case", "bogus"), {});
  assert.deepEqual(writePolicy("nonsense", "x"), {});
  // Filter definitions are edited in settings, so the toolbar cannot write them.
  assert.deepEqual(writePolicy("filters", "3"), {});
});

test("every toolbar row shows its current value, and filters are one row not N", () => {
  const seen = [];
  const t = (key, arg) => {
    seen.push(key);
    return arg ? `${key}:${arg.count}` : key;
  };
  const rows = describeConditions(
    readPolicy({ whitespace: "change", ignoreCase: true, ignoreEOL: true, lineFilters: "a\nb", detectMoves: true }),
    t,
  );
  assert.deepEqual(rows.map((row) => row.id), ["whitespace", "case", "eol", "filters", "moves"]);
  const byID = Object.fromEntries(rows.map((row) => [row.id, row]));
  assert.equal(byID.whitespace.text, "whitespaceChange");
  assert.equal(byID.case.text, "caseIgnore");
  assert.equal(byID.eol.text, "ignoreEOL");
  assert.equal(byID.filters.text, "conditionFilterCount:2");
  assert.equal(byID.moves.text, "conditionMovesOn");
  // The row count is the control count, independent of how many filters exist.
  assert.equal(describeConditions(readPolicy({ lineFilters: "a\nb\nc\nd\ne" }), t).length, TOOLBAR_CONTROLS.length);
});

test("a filter count of zero reads as off rather than an empty count", () => {
  const rows = describeConditions(readPolicy({ lineFilters: "" }), (key) => key);
  assert.equal(rows.find((row) => row.id === "filters").text, "conditionFilterOff");
});

test("the active set names exactly the non-default conditions", () => {
  assert.deepEqual(activeConditionIds(readPolicy()), []);
  assert.deepEqual(activeConditionIds(readPolicy({ whitespace: "all", ignoreCase: true })), ["whitespace", "case"]);
  assert.deepEqual(activeConditionIds(readPolicy({ ignoreTrailingEOL: true, lineFilters: "x", detectMoves: true })), ["eol", "filters", "moves"]);
  assert.equal(isDefault(readPolicy()), true);
  assert.equal(isDefault(readPolicy({ detectMoves: true })), false);
});
