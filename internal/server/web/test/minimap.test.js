"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const {
  calculateMinimapSegments,
  calculateMinimapViewport,
  minimapMarkerPriority,
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

test("uses visible hunk length instead of source line offsets", () => {
  const segments = calculateMinimapSegments([
    { index: 0, kind: "insert", displayLength: 9 },
    { index: 1, kind: "replace", displayLength: 89 },
  ], 100);

  assert.deepEqual(segments.map(({ index, top, height }) => ({ index, top, height })), [
    { index: 0, top: 0, height: 0.1 },
    { index: 1, top: 0.1, height: 0.9 },
  ]);
});

test("keeps conflict and moved markers above ordinary changes", () => {
  assert.ok(minimapMarkerPriority({ kind: "conflict" }) > minimapMarkerPriority({ kind: "replace" }));
  assert.ok(minimapMarkerPriority({ kind: "replace", moved: true }) > minimapMarkerPriority({ kind: "replace" }));

  const conflict = calculateMinimapSegments([
    { index: 0, kind: "insert", displayLength: 1 },
    { index: 1, kind: "conflict", displayLength: 1 },
    { index: 2, kind: "replace", displayLength: 1 },
  ], 1);
  assert.equal(conflict.length, 1);
  assert.equal(conflict[0].index, 1);

  const moved = calculateMinimapSegments([
    { index: 0, kind: "insert", displayLength: 1 },
    { index: 1, kind: "replace", moved: true, displayLength: 1 },
    { index: 2, kind: "delete", displayLength: 1 },
  ], 1);
  assert.equal(moved.length, 1);
  assert.equal(moved[0].index, 1);
});

test("puts ignored markers last and clamps every segment to the track", () => {
  const priority = calculateMinimapSegments([
    { index: 0, kind: "conflict", ignored: true, displayLength: 1 },
    { index: 1, kind: "insert", displayLength: 1 },
  ], 1);
  assert.equal(priority[0].index, 1);

  const segments = calculateMinimapSegments([
    { index: 0, kind: "insert", displayLength: 1 },
    { index: 2, kind: "delete", displayLength: 1000 },
  ], 7);

  for (const segment of segments) {
    assert.ok(segment.top >= 0);
    assert.ok(segment.height > 0);
    assert.ok(segment.top + segment.height <= 1);
  }
});

test("bounds dense minimaps by track pixels without losing a conflict", () => {
  const markers = Array.from({ length: 1000 }, (_, index) => ({
    index,
    kind: index === 501 ? "conflict" : "replace",
    displayLength: 1,
  }));
  const segments = calculateMinimapSegments(markers, 32);

  assert.ok(segments.length <= 32);
  assert.ok(segments.some((segment) => segment.index === 501 && segment.kind === "conflict"));
});
