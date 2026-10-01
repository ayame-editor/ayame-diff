"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { resultLines, panes, unresolved } = require("../threewayview.js");

const event = (kind, extra = {}) => ({
  id: 0,
  kind,
  base: ["base"],
  left: ["left"],
  right: ["right"],
  ...extra,
});

test("independent changes resolve to their own side", () => {
  assert.deepEqual(resultLines(event("left_only"), "").lines, ["left"]);
  assert.deepEqual(resultLines(event("right_only"), "").lines, ["right"]);
});

test("a same change on both sides resolves to the shared text", () => {
  const both = event("same_change", { left: ["same"], right: ["same"] });
  assert.deepEqual(resultLines(both, "").lines, ["same"]);
  assert.equal(resultLines(both, "").unresolved, false);
});

test("a CSV merged group resolves to its combined rows", () => {
  const merged = event("merged", { combined: [["1", "both"]] });
  assert.deepEqual(resultLines(merged, "").lines, [["1", "both"]]);
});

test("a conflict follows the chosen side", () => {
  assert.deepEqual(resultLines(event("conflict"), "left").lines, ["left"]);
  assert.deepEqual(resultLines(event("conflict"), "right").lines, ["right"]);
  assert.deepEqual(resultLines(event("conflict"), "base").lines, ["base"]);
  for (const side of ["left", "right", "base"]) {
    assert.equal(resultLines(event("conflict"), side).unresolved, false);
  }
});

test("an unresolved conflict previews base but is flagged", () => {
  const pending = resultLines(event("conflict"), "");
  assert.deepEqual(pending.lines, ["base"]);
  assert.equal(pending.unresolved, true);
  assert.equal(unresolved(event("conflict"), ""), true);
  assert.equal(unresolved(event("left_only"), ""), false);
});

test("the returned lines are copies", () => {
  const source = event("left_only");
  const result = resultLines(source, "");
  result.lines[0] = "tampered";
  assert.deepEqual(source.left, ["left"]);
});

test("the common view is LEFT | RESULT | RIGHT", () => {
  const roles = panes(event("left_only"), "", false).map((pane) => pane.role);
  assert.deepEqual(roles, ["left", "result", "right"]);
});

test("asking for base adds it ahead of the common view", () => {
  const view = panes(event("conflict"), "right", true);
  assert.deepEqual(view.map((pane) => pane.role), ["base", "left", "result", "right"]);
  const result = view.find((pane) => pane.role === "result");
  assert.deepEqual(result.lines, ["right"]);
  assert.equal(result.unresolved, false);
});

test("a missing side never crashes the pane builder", () => {
  const sparse = { kind: "right_only", base: ["b"], right: undefined };
  assert.deepEqual(resultLines(sparse, "").lines, []);
  assert.deepEqual(panes(sparse, "", false).map((pane) => pane.role), ["left", "result", "right"]);
});
