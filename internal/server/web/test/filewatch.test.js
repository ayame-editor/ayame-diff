"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { watchPathsForMode, createLongPollWatcher } = require("../filewatch.js");

test("selects only bounded file-backed comparison paths", () => {
  const fields = { base: " base.txt ", old: "old.txt", new: "new.txt" };
  assert.deepEqual(watchPathsForMode("text", fields, false), ["old.txt", "new.txt"]);
  assert.deepEqual(watchPathsForMode("csv", fields, false), ["old.txt", "new.txt"]);
  assert.deepEqual(watchPathsForMode("threeway", fields, false), ["base.txt", "old.txt", "new.txt"]);
  assert.deepEqual(watchPathsForMode("threeway-csv", fields, false), ["base.txt", "old.txt", "new.txt"]);
  assert.deepEqual(watchPathsForMode("dir", fields, false), []);
  assert.deepEqual(watchPathsForMode("future-mode", fields, false), []);
  assert.deepEqual(watchPathsForMode("text", fields, true), []);
  assert.deepEqual(watchPathsForMode("text", { old: "same", new: "same" }, false), ["same"]);
});

test("renews an unchanged long poll and emits the first change", async () => {
  const seen = [];
  let calls = 0;
  let resolveChanged;
  const changed = new Promise((resolve) => { resolveChanged = resolve; });
  const watcher = createLongPollWatcher({
    request: async (_paths, baseline) => {
      calls++;
      seen.push(baseline);
      if (calls === 1) return { changed: [], snapshot: [{ path: "old", size: "1" }] };
      return { changed: ["old"], snapshot: [{ path: "old", size: "2" }] };
    },
    onChange: (event) => resolveChanged(event),
    onError: (error) => assert.fail(error),
  });

  assert.equal(watcher.start(["old"], [{ path: "old", size: "1" }]), true);
  const event = await changed;
  assert.equal(calls, 2);
  assert.deepEqual(seen[1], [{ path: "old", size: "1" }]);
  assert.deepEqual(event, {
    paths: ["old"],
    changed: ["old"],
    snapshot: [{ path: "old", size: "2" }],
  });
});

test("stop aborts the in-flight request without reporting an error", async () => {
  let aborted = false;
  let reported = false;
  const watcher = createLongPollWatcher({
    request: (_paths, _baseline, signal) => new Promise((_resolve, reject) => {
      signal.addEventListener("abort", () => {
        aborted = true;
        const error = new Error("aborted");
        error.name = "AbortError";
        reject(error);
      }, { once: true });
    }),
    onChange: () => assert.fail("unexpected change"),
    onError: () => { reported = true; },
  });

  watcher.start(["old"], [{ path: "old", size: "1" }]);
  watcher.stop();
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(aborted, true);
  assert.equal(reported, false);
  assert.equal(watcher.isRunning(), false);
});
