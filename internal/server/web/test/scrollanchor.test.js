"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const {
  captureScrollAnchor,
  findScrollAnchor,
  restoreScrollAnchor,
} = require("../scrollanchor.js");

function item(group, key, order, top, bottom = top + 20) {
  return {
    dataset: {
      scrollAnchor: group,
      scrollKey: String(key),
      scrollOrder: String(order),
    },
    getBoundingClientRect: () => ({ top, bottom }),
  };
}

function container(nodes, { top = 100, bottom = 500, scrollTop = 0 } = {}) {
  return {
    scrollTop,
    querySelectorAll: () => nodes,
    getBoundingClientRect: () => ({ top, bottom }),
  };
}

test("captures the first visible logical line below a sticky header", () => {
  const hidden = item("old", 40, 40, 80, 119);
  const partial = item("old", 41, 41, 110, 135);
  const later = item("new", 41, 41, 110, 135);
  const anchor = captureScrollAnchor(container([hidden, partial, later]), 20);
  assert.deepEqual(anchor, {
    group: "old",
    key: "41",
    order: 41,
    offset: -10,
  });
});

test("restores an exact logical line at the same viewport offset", () => {
  const result = container([item("old", 41, 41, 210)], { scrollTop: 300 });
  const restored = restoreScrollAnchor(result, {
    group: "old",
    key: "41",
    order: 41,
    offset: -10,
  }, 20);
  assert.equal(restored, true);
  assert.equal(result.scrollTop, 400);
});

test("a removed line falls back to the nearest surviving line deterministically", () => {
  const earlier = item("old", 38, 38, 180);
  const later = item("old", 44, 44, 240);
  const result = container([later, earlier]);
  const anchor = { group: "old", key: "41", order: 41, offset: 0 };
  assert.equal(findScrollAnchor(result, anchor), earlier, "an equal-distance tie must choose the earlier line");
});

test("fallback never jumps to the other side", () => {
  const otherSide = item("new", 41, 41, 200);
  const result = container([otherSide]);
  const anchor = { group: "old", key: "41", order: 41, offset: 0 };
  assert.equal(findScrollAnchor(result, anchor), null);
  assert.equal(restoreScrollAnchor(result, anchor), false);
  assert.equal(result.scrollTop, 0);
});

test("stable non-numeric keys restore CSV differences and folder paths", () => {
  for (const [group, key] of [["csv", "diff-a1"], ["directory", "src/main.go"]]) {
    const target = item(group, key, 7, 175);
    const result = container([target], { scrollTop: 20 });
    assert.equal(restoreScrollAnchor(result, { group, key, order: 7, offset: 25 }), true);
    assert.equal(result.scrollTop, 70);
  }
});
