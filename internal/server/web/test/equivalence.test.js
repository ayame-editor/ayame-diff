"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { equivalenceTitleKey, alignmentProposal } = require("../equivalence.js");

test("a plain match keeps the existing complete/filtered wording", () => {
  assert.equal(equivalenceTitleKey({ status: "equal", substantively_equal: true }, false), "completeMatch");
  assert.equal(equivalenceTitleKey({ status: "equal", substantively_equal: true }, true), "filteredMatch");
});

test("presentation-only verdicts name the difference", () => {
  assert.equal(
    equivalenceTitleKey({ status: "equal_columns_reordered", substantively_equal: true }, false),
    "dataEqualColumnsReordered",
  );
  assert.equal(
    equivalenceTitleKey({ status: "equal_row_order", substantively_equal: true }, false),
    "dataEqualRowOrder",
  );
  assert.equal(
    equivalenceTitleKey({ status: "equal_presentation", substantively_equal: true }, false),
    "dataEqualPresentation",
  );
});

test("a missing or unknown verdict falls back to the match wording", () => {
  assert.equal(equivalenceTitleKey(undefined, false), "completeMatch");
  assert.equal(equivalenceTitleKey({ status: "something_else", substantively_equal: true }, false), "completeMatch");
});

test("only an alignable reorder asks for alignment", () => {
  assert.equal(alignmentProposal({ status: "columns_reordered", alignable_columns: true }), true);
  assert.equal(alignmentProposal({ status: "columns_reordered", alignable_columns: false }), false);
  assert.equal(alignmentProposal({ status: "equal_columns_reordered", alignable_columns: true }), false);
  assert.equal(alignmentProposal(undefined), false);
});
