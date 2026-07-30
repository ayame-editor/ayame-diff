"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const {
  calculateMinimapViewport,
  scrollTopForMinimapPointer,
} = require("../minimap.js");

test("hides the viewport when the result pane does not overflow", () => {
  assert.deepEqual(
    calculateMinimapViewport({ scrollTop: 0, scrollHeight: 500, clientHeight: 500 }),
    { scrollable: false, top: 0, height: 1, maxScrollTop: 0 },
  );
});

test("tracks the result pane from its own scroll dimensions", () => {
  const metrics = calculateMinimapViewport({
    scrollTop: 750,
    scrollHeight: 2000,
    clientHeight: 500,
  });
  assert.equal(metrics.scrollable, true);
  assert.equal(metrics.height, 0.25);
  assert.equal(metrics.top, 0.375);
  assert.equal(metrics.maxScrollTop, 1500);
});

test("keeps a tiny viewport visible and aligned at both ends", () => {
  const top = calculateMinimapViewport({
    scrollTop: 0,
    scrollHeight: 100000,
    clientHeight: 500,
  });
  const bottom = calculateMinimapViewport({
    scrollTop: 99500,
    scrollHeight: 100000,
    clientHeight: 500,
  });
  assert.equal(top.height, 0.03);
  assert.equal(top.top, 0);
  assert.equal(bottom.height, 0.03);
  assert.equal(bottom.top, 0.97);
});

test("maps a grabbed viewport position back to result scrollTop", () => {
  const options = {
    trackTop: 100,
    trackHeight: 400,
    viewportHeight: 100,
    grabOffset: 25,
    scrollHeight: 2000,
    clientHeight: 500,
  };
  assert.equal(scrollTopForMinimapPointer({ ...options, pointerY: 125 }), 0);
  assert.equal(scrollTopForMinimapPointer({ ...options, pointerY: 275 }), 750);
  assert.equal(scrollTopForMinimapPointer({ ...options, pointerY: 425 }), 1500);
  assert.equal(scrollTopForMinimapPointer({ ...options, pointerY: 999 }), 1500);
});
