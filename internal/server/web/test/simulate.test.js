"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { normalizePath, samePath, mergeImpact } = require("../simulate.js");

test("normalizePath folds separators and cosmetic noise", () => {
  assert.equal(normalizePath("/a/b/"), "/a/b");
  assert.equal(normalizePath("/a//b/./c"), "/a/b/c");
  assert.equal(normalizePath("a/../b"), "b");
  assert.equal(normalizePath("C:\\old\\file.txt"), "C:/old/file.txt");
  assert.equal(normalizePath("c:/old/file.txt"), "C:/old/file.txt");
  assert.equal(normalizePath("   "), "");
  assert.equal(normalizePath(undefined), "");
});

test("samePath matches spellings and rejects blanks", () => {
  assert.equal(samePath("/a/b/", "/a/b"), true);
  assert.equal(samePath("C:\\x\\y", "c:/x/y"), true);
  assert.equal(samePath("/a/b", "/a/c"), false);
  assert.equal(samePath("", ""), false);
  assert.equal(samePath("/a", ""), false);
});

test("a distinct output writes and touches no input", () => {
  const impact = mergeImpact({
    mode: "text",
    output: "/work/merged.txt",
    old: "/work/left.txt",
    new: "/work/right.txt",
    overwrite: false,
    unresolved: 2,
  });

  assert.equal(impact.output, "/work/merged.txt");
  assert.deepEqual(impact.affected, []);
  assert.equal(impact.destructive, false);
  assert.equal(impact.unresolved, 2);
  assert.deepEqual(impact.actions, [{ type: "write", path: "/work/merged.txt", role: null, overwrite: false }]);
  assert.deepEqual(impact.summary, { writes: 1, overwrites: 0, deletes: 0 });
  assert.deepEqual(impact.deletes, []);
});

test("an output that aliases the left input lists the overwrite", () => {
  const impact = mergeImpact({
    mode: "text",
    output: "/work/left.txt",
    old: "/work/left.txt",
    new: "/work/right.txt",
    overwrite: true,
  });

  assert.deepEqual(impact.affected, [{ role: "old", path: "/work/left.txt" }]);
  assert.equal(impact.destructive, true);
  assert.deepEqual(impact.actions, [
    { type: "write", path: "/work/left.txt", role: null, overwrite: true },
    { type: "overwrite", path: "/work/left.txt", role: "old", overwrite: true },
  ]);
  assert.deepEqual(impact.summary, { writes: 1, overwrites: 1, deletes: 0 });
});

test("three-way merges consider the base as an affected input", () => {
  const impact = mergeImpact({
    mode: "threeway",
    output: "C:\\work\\base.txt",
    base: "c:/work/base.txt",
    old: "/work/left.txt",
    new: "/work/right.txt",
    overwrite: true,
  });

  assert.deepEqual(impact.affected, [{ role: "base", path: "C:/work/base.txt" }]);
  assert.equal(impact.actions[1].type, "overwrite");
  assert.equal(impact.actions[1].role, "base");
});

test("a missing output produces no write action", () => {
  const impact = mergeImpact({ mode: "csv", old: "/work/left.csv", new: "/work/right.csv" });
  assert.equal(impact.output, "");
  assert.deepEqual(impact.actions, []);
  assert.deepEqual(impact.summary, { writes: 0, overwrites: 0, deletes: 0 });
  assert.equal(impact.unresolved, 0);
});
