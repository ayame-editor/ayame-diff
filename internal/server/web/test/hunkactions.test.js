"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { hunkActions } = require("../hunkactions.js");

const ids = (state) => hunkActions(state).map((action) => action.id);

test("a plain hunk offers the ignore action", () => {
  const actions = hunkActions({ moved: false, ignored: false, mergeable: false });
  assert.deepEqual(actions.map((a) => a.id), ["ignore"]);
  assert.equal(actions[0].labelKey, "ignoreHunk");
  assert.equal(actions[0].className, "hunk-ignore");
});

test("an ignored hunk offers restore instead, and says so", () => {
  const actions = hunkActions({ ignored: true });
  assert.deepEqual(actions.map((a) => a.id), ["ignore"]);
  assert.equal(actions[0].labelKey, "restoreHunk");
});

test("a moved hunk gains its jump, ahead of the ignore action", () => {
  const actions = hunkActions({ moved: true });
  assert.deepEqual(actions.map((a) => a.id), ["move-jump", "ignore"]);
  assert.equal(actions[0].glyph, "↔");
  assert.equal(actions[0].labelKey, "moved");
});

test("a moved and ignored hunk keeps both actions", () => {
  assert.deepEqual(ids({ moved: true, ignored: true, mergeable: false }), ["move-jump", "ignore"]);
});

test("adopt buttons are offered only where a text diff can be merged", () => {
  assert.deepEqual(ids({ mergeable: false }), ["ignore"]);
  assert.deepEqual(ids({ mergeable: true }), ["ignore", "choose-left", "choose-right"]);
  const adopt = hunkActions({ mergeable: true }).filter((a) => a.side);
  assert.deepEqual(adopt.map((a) => a.side), ["left", "right"]);
  assert.equal(adopt[0].className, "choose-left hunk-adopt");
});

test("nonsense input still yields a usable toolbar rather than throwing", () => {
  assert.deepEqual(ids(), ["ignore"]);
  assert.deepEqual(ids(null), ["ignore"]);
  assert.equal(hunkActions(undefined)[0].className, "hunk-ignore");
});

test("every descriptor is complete enough to build a button", () => {
  for (const action of hunkActions({ moved: true, ignored: true, mergeable: true })) {
    assert.equal(typeof action.id, "string");
    assert.ok(action.className.length > 0);
    assert.ok(action.labelKey.length > 0);
    assert.ok(action.glyph === null || typeof action.glyph === "string");
  }
});
