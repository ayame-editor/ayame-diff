"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { csvPageCount, clampPage, visibleColumns, pagerState, pageSlice } = require("../csvview.js");

test("an empty result still has one page", () => {
  assert.equal(csvPageCount(0, 100), 1);
  assert.equal(csvPageCount(1, 100), 1);
  assert.equal(csvPageCount(100, 100), 1);
  assert.equal(csvPageCount(101, 100), 2);
  assert.equal(csvPageCount(4000, 100), 40);
});

test("a page out of range lands inside it", () => {
  assert.equal(clampPage(-5, 4), 0);
  assert.equal(clampPage(0, 4), 0);
  assert.equal(clampPage(3, 4), 3);
  assert.equal(clampPage(9, 4), 3);
  assert.equal(clampPage(1.7, 4), 1);
  assert.equal(clampPage(NaN, 4), 0);
});

test("changed-columns-only keeps what the summary reported", () => {
  assert.deepEqual(visibleColumns(5, [1, 3], false), [0, 1, 2, 3, 4]);
  assert.deepEqual(visibleColumns(5, [1, 3], true), [1, 3]);
  assert.deepEqual(visibleColumns(5, [], true), [], "no changed column shows no column");
  assert.deepEqual(visibleColumns(0, [1], false), []);
});

test("the pager knows where it is without being rebuilt", () => {
  assert.deepEqual(pagerState(0, 5), { page: 0, pageCount: 5, value: "1", atFirst: true, atLast: false });
  assert.deepEqual(pagerState(4, 5), { page: 4, pageCount: 5, value: "5", atFirst: false, atLast: true });
  assert.deepEqual(pagerState(2, 5), { page: 2, pageCount: 5, value: "3", atFirst: false, atLast: false });
  assert.deepEqual(pagerState(0, 1), { page: 0, pageCount: 1, value: "1", atFirst: true, atLast: true });
  assert.deepEqual(pagerState(99, 3).page, 2, "an out-of-range page is clamped");
});

test("a page carries the offset its rows have in the whole result", () => {
  const differences = Array.from({ length: 250 }, (_, index) => index);
  assert.deepEqual(pageSlice(differences, 0, 100).rows.slice(0, 2), [0, 1]);
  assert.equal(pageSlice(differences, 0, 100).start, 0);
  assert.equal(pageSlice(differences, 2, 100).start, 200);
  assert.equal(pageSlice(differences, 2, 100).rows.length, 50);
  assert.equal(pageSlice(differences, 99, 100).start, 200, "an out-of-range page is clamped");
  assert.deepEqual(pageSlice([], 0, 100), { start: 0, rows: [] });
});
