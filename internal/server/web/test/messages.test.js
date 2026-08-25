"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { createMessageLog, normalizeTone, selfClearing } = require("../messages.js");

function fakeClock() {
  let current = 1000;
  let nextHandle = 1;
  const pending = new Map();
  return {
    now: () => current,
    schedule: (fn, delay) => {
      const handle = nextHandle++;
      pending.set(handle, { fn, at: current + delay });
      return handle;
    },
    cancel: (handle) => { pending.delete(handle); },
    advance(ms) {
      current += ms;
      for (const [handle, timer] of [...pending.entries()]) {
        if (timer.at <= current) {
          pending.delete(handle);
          timer.fn();
        }
      }
    },
    pendingCount: () => pending.size,
  };
}

function newLog(overrides) {
  const clock = fakeClock();
  const seen = [];
  const log = createMessageLog({
    now: clock.now,
    schedule: clock.schedule,
    cancel: clock.cancel,
    onChange: (entries) => seen.push(entries),
    ...overrides,
  });
  return { clock, log, seen };
}

test("a failure and a success occupy the lane at the same time", () => {
  const { log } = newLog();
  log.post("compare failed", "error");
  log.post("patch written", "success");
  assert.deepEqual(log.entries().map((entry) => [entry.tone, entry.message]), [
    ["error", "compare failed"],
    ["success", "patch written"],
  ]);
});

test("success withdraws itself and an error stays until dismissed", () => {
  const { clock, log } = newLog({ autoDismissMs: 5000 });
  const errorID = log.post("compare failed", "error");
  log.post("patch written", "success");

  clock.advance(5000);
  assert.deepEqual(log.entries().map((entry) => entry.message), ["compare failed"]);

  clock.advance(60000);
  assert.deepEqual(log.entries().map((entry) => entry.message), ["compare failed"]);

  log.dismiss(errorID);
  assert.deepEqual(log.entries(), []);
  assert.equal(clock.pendingCount(), 0);
});

test("a repeated message refreshes one entry instead of stacking", () => {
  const { clock, log } = newLog({ autoDismissMs: 5000 });
  log.post("watch failed", "warning");
  clock.advance(1000);
  log.post("watch failed", "warning");

  const entries = log.entries();
  assert.equal(entries.length, 1);
  assert.equal(entries[0].count, 2);
  assert.equal(entries[0].at, 2000);
});

test("the cap drops self-clearing lines before a failure", () => {
  const { log } = newLog({ maxEntries: 3 });
  log.post("compare failed", "error");
  log.post("first", "success");
  log.post("second", "success");
  log.post("third", "success");

  assert.deepEqual(log.entries().map((entry) => entry.message), [
    "compare failed",
    "second",
    "third",
  ]);
});

test("empty text posts nothing and clear empties the lane", () => {
  const { log, seen } = newLog();
  assert.equal(log.post("", "error"), null);
  assert.equal(log.post(null, "error"), null);
  assert.deepEqual(log.entries(), []);
  assert.equal(seen.length, 0);

  log.post("compare failed", "error");
  log.clear();
  assert.deepEqual(log.entries(), []);
});

test("unknown tones fall back to a neutral, self-clearing note", () => {
  assert.equal(normalizeTone(""), "info");
  assert.equal(normalizeTone(undefined), "info");
  assert.equal(normalizeTone("busy"), "info");
  assert.equal(normalizeTone("error"), "error");
  assert.equal(selfClearing("info"), true);
  assert.equal(selfClearing("success"), true);
  assert.equal(selfClearing("warning"), false);
  assert.equal(selfClearing("error"), false);
});

test("entries are copies, so a renderer cannot mutate the log", () => {
  const { log } = newLog();
  const id = log.post("compare failed", "error");
  const entries = log.entries();
  entries[0].message = "tampered";
  assert.equal(log.entries()[0].message, "compare failed");
  log.dismiss(id);
  assert.deepEqual(log.entries(), []);
});
