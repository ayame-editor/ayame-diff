"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { formatBytes, memoryStatus } = require("../memorybudget.js");

test("memory is shown in the binary units --memory accepts", () => {
  assert.equal(formatBytes(0), "0B");
  assert.equal(formatBytes(512), "512B");
  assert.equal(formatBytes(1024), "1.0KiB");
  assert.equal(formatBytes(1536), "1.5KiB");
  assert.equal(formatBytes(512 << 20), "512.0MiB");
  assert.equal(formatBytes(8 * 1024 ** 3), "8.0GiB");
  assert.equal(formatBytes("not a number"), "0B");
});

test("a comparison reports its budget against the server cap", () => {
  assert.deepEqual(memoryStatus({ budget_bytes: 512 << 20, cap: "8GiB" }), {
    budget: "512.0MiB",
    cap: "8GiB",
    spilled: false,
    dir: "",
  });
});

test("the spill directory only appears when the comparison spilled", () => {
  assert.deepEqual(memoryStatus({ budget_bytes: 512 << 20, cap: "8GiB", spilled: true, spill_dir: "/var/tmp/ayame" }), {
    budget: "512.0MiB",
    cap: "8GiB",
    spilled: true,
    dir: "/var/tmp/ayame",
  });
  // A spilled run with no directory never surfaces one from a stale field.
  assert.equal(memoryStatus({ budget_bytes: 512 << 20, spilled: false, spill_dir: "/var/tmp/ayame" }).dir, "");
});

test("a missing report renders nothing instead of a bogus limit", () => {
  assert.equal(memoryStatus(null), null);
  assert.equal(memoryStatus(undefined), null);
  assert.equal(memoryStatus({}), null);
  assert.equal(memoryStatus({ budget_bytes: 0 }), null);
});
