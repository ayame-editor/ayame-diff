"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { VALID_WHITESPACE, ABSORB_ALIGNMENT, alignmentWhitespace } = require("../alignment.js");

test("the module exposes the supported comparison values", () => {
  assert.deepEqual(VALID_WHITESPACE, ["none", "change", "all"]);
  assert.equal(ABSORB_ALIGNMENT, "change");
});

test("the toggle is off unless re-indentation is absorbed", () => {
  for (const whitespace of ["none", "change", "all", ""]) {
    assert.equal(alignmentWhitespace(whitespace, false), "none");
  }
});

test("absorbing re-indentation asks for whitespace-collapsing alignment", () => {
  assert.equal(alignmentWhitespace("none", true), "change");
});

test("alignment is skipped when comparison already ignores as much", () => {
  // Detection with "change" or "all" already hides the re-indentation, so a
  // separate alignment policy would be indistinguishable and is not sent.
  assert.equal(alignmentWhitespace("change", true), "none");
  assert.equal(alignmentWhitespace("all", true), "none");
});

test("an unknown comparison value falls back to exact whitespace", () => {
  assert.equal(alignmentWhitespace("sometimes", true), "change");
  assert.equal(alignmentWhitespace(undefined, true), "change");
});

test("alignment never asks for a policy coarser than needed by comparison", () => {
  // Only "none" upgrades; the explanation stays at the absorb level.
  for (const whitespace of VALID_WHITESPACE) {
    const align = alignmentWhitespace(whitespace, true);
    assert.ok(align === "none" || align === "change", `unexpected ${align}`);
  }
});
