"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  isPlainComparison,
  planLineApproximation,
  createRecomputeCoordinator,
} = require("../optimistic.js");

// A deterministic clock: timers fire only when advance() crosses their due
// time, and microtasks are drained so awaited runs settle inside the test.
function drainMicrotasks(times = 6) {
  let chain = Promise.resolve();
  for (let i = 0; i < times; i++) chain = chain.then(() => {});
  return chain;
}

function fakeClock() {
  let now = 0;
  let nextID = 1;
  const timers = new Map();
  return {
    now: () => now,
    setTimer(fn, ms) {
      const id = nextID++;
      timers.set(id, { fn, at: now + ms });
      return id;
    },
    clearTimer(id) { timers.delete(id); },
    async advance(ms) {
      now += ms;
      const due = [...timers.entries()]
        .filter(([, timer]) => timer.at <= now)
        .sort((a, b) => a[1].at - b[1].at);
      for (const [id, timer] of due) {
        if (!timers.has(id)) continue;
        timers.delete(id);
        timer.fn();
      }
      await Promise.resolve();
      await Promise.resolve();
    },
    pending: () => timers.size,
  };
}

test("a plain exact comparison is the only one judged locally", () => {
  assert.equal(isPlainComparison({}), true);
  assert.equal(isPlainComparison({ whitespace: "none", lineFilters: [] }), true);
  assert.equal(isPlainComparison({ whitespace: "all" }), false, "whitespace rule changes equality");
  assert.equal(isPlainComparison({ ignoreCase: true }), false);
  assert.equal(isPlainComparison({ ignoreEOL: true }), false);
  assert.equal(isPlainComparison({ ignoreTrailingEOL: true }), false);
  assert.equal(isPlainComparison({ numeric: true }), false);
  assert.equal(isPlainComparison({ detectMoves: true }), false);
  assert.equal(isPlainComparison({ lineFilters: ["^#"] }), false);
  assert.equal(isPlainComparison({ lineFilters: ["", "  "] }), true, "blank filters are no rule");
  assert.equal(isPlainComparison({ syncPoints: [{ old: 0, new: 0 }] }), false);
});

test("an edited line with a counterpart is re-judged exactly under a plain comparison", () => {
  const same = planLineApproximation({ editedValue: "beta", counterpartValue: "beta", plain: true });
  assert.equal(same.render, true);
  assert.equal(same.classify, true);
  assert.equal(same.same, true);
  assert.equal(same.reason, "ok");

  const changed = planLineApproximation({ editedValue: "BETA", counterpartValue: "beta", plain: true });
  assert.equal(changed.classify, true);
  assert.equal(changed.same, false);
});

test("comparison rules keep the text but defer the verdict to the server", () => {
  const plan = planLineApproximation({ editedValue: "BETA", counterpartValue: "beta", plain: false });
  assert.equal(plan.render, true, "what the user typed is still shown");
  assert.equal(plan.classify, false, "ignore rules mean the local verdict can be wrong");
  assert.equal(plan.same, false);
  assert.equal(plan.reason, "conditions");
});

test("a line with no counterpart on screen only shows the text", () => {
  const plan = planLineApproximation({ editedValue: "x", counterpartValue: null, plain: true });
  assert.equal(plan.render, true);
  assert.equal(plan.classify, false);
  assert.equal(plan.reason, "no-counterpart");
});

test("a burst of idle requests collapses into one run", async () => {
  const clock = fakeClock();
  let runs = 0;
  const coordinator = createRecomputeCoordinator({
    delay: 150,
    setTimer: clock.setTimer,
    clearTimer: clock.clearTimer,
    run: () => { runs += 1; },
  });

  coordinator.request();
  coordinator.request();
  coordinator.request();
  assert.equal(clock.pending(), 1, "only one timer is armed");
  await clock.advance(149);
  assert.equal(runs, 0);
  await clock.advance(1);
  assert.equal(runs, 1);
  assert.equal(clock.pending(), 0);
});

test("requests during a run become one trailing run, not one each", async () => {
  const clock = fakeClock();
  let runs = 0;
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  const coordinator = createRecomputeCoordinator({
    delay: 150,
    setTimer: clock.setTimer,
    clearTimer: clock.clearTimer,
    run: async () => { runs += 1; await gate; },
  });

  coordinator.request();
  await clock.advance(150);
  assert.equal(runs, 1);
  assert.equal(coordinator.state().running, true);

  // Three edits land while the first comparison is still in flight.
  coordinator.request();
  coordinator.request();
  coordinator.request();
  assert.equal(coordinator.state().dirty, true);
  assert.equal(runs, 1, "nothing new starts while one is running");

  release();
  await drainMicrotasks();
  assert.equal(clock.pending(), 1, "the catch-up run is armed");
  await clock.advance(150);
  await drainMicrotasks();
  assert.equal(runs, 2, "one catch-up run covers the whole burst");
  assert.equal(coordinator.state().pending, false, "no third run once the catch-up finishes");
});

test("cancel drops an armed run before it fires", async () => {
  const clock = fakeClock();
  let runs = 0;
  const coordinator = createRecomputeCoordinator({
    delay: 150,
    setTimer: clock.setTimer,
    clearTimer: clock.clearTimer,
    run: () => { runs += 1; },
  });
  coordinator.request();
  coordinator.cancel();
  assert.equal(clock.pending(), 0);
  await clock.advance(500);
  assert.equal(runs, 0);
});

test("a busy run is retried rather than counted as done", async () => {
  const clock = fakeClock();
  let attempts = 0;
  const coordinator = createRecomputeCoordinator({
    delay: 100,
    setTimer: clock.setTimer,
    clearTimer: clock.clearTimer,
    run: () => { attempts += 1; return attempts === 1 ? "busy" : true; },
  });
  coordinator.request();
  await clock.advance(100);
  assert.equal(attempts, 1);
  assert.equal(coordinator.state().pending, true, "a busy run leaves work outstanding");
  await clock.advance(100);
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(attempts, 2);
  assert.equal(coordinator.state().pending, false);
});

test("flush runs immediately, skipping the debounce", async () => {
  const clock = fakeClock();
  let runs = 0;
  const coordinator = createRecomputeCoordinator({
    delay: 150,
    setTimer: clock.setTimer,
    clearTimer: clock.clearTimer,
    run: () => { runs += 1; },
  });
  coordinator.request();
  await coordinator.flush();
  assert.equal(runs, 1);
  assert.equal(clock.pending(), 0);
});

test("dispose stops a pending run", async () => {
  const clock = fakeClock();
  let runs = 0;
  const coordinator = createRecomputeCoordinator({
    delay: 150,
    setTimer: clock.setTimer,
    clearTimer: clock.clearTimer,
    run: () => { runs += 1; },
  });
  coordinator.request();
  coordinator.dispose();
  await clock.advance(500);
  assert.equal(runs, 0);
  assert.equal(coordinator.state().pending, false);
});
