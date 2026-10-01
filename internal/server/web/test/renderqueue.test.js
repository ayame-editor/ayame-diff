"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { createSliceBudget, createRenderGate, DEFAULT_BUDGET_MS } = require("../renderqueue.js");

// A clock the test moves by hand, so slice boundaries are exact rather than
// dependent on how long the machine takes.
function fakeClock(initial = 0) {
  let value = initial;
  return {
    now: () => value,
    set: (next) => { value = next; },
    advance: (by) => { value += by; },
  };
}

test("a slice yields once its budget is spent", () => {
  const clock = fakeClock(0);
  const budget = createSliceBudget({ budgetMs: DEFAULT_BUDGET_MS, now: clock.now });
  // The caller builds the first item before asking, so this is that item.
  assert.equal(budget.doneOne(), false, "one item is well inside the budget");
  clock.set(3);
  assert.equal(budget.doneOne(), false, "3ms is still inside the budget");
  clock.set(9);
  assert.equal(budget.doneOne(), true, "9ms is past the 8ms budget");
});

test("a slice always builds at least one item", () => {
  // Move the clock past the budget only after the slice starts: the first item
  // still counts, and only the second can trigger a yield.
  const clock = fakeClock(0);
  const budget = createSliceBudget({ budgetMs: 8, now: clock.now });
  clock.set(1000);
  assert.equal(budget.built, 0);
  assert.equal(budget.doneOne(), true);
  assert.equal(budget.built, 1);
});

test("reset starts a fresh slice", () => {
  const clock = fakeClock(0);
  const budget = createSliceBudget({ budgetMs: 8, now: clock.now });
  assert.equal(budget.doneOne(), false);
  assert.equal(budget.built, 1);
  clock.set(9);
  assert.equal(budget.doneOne(), true, "9ms of clock movement is past the budget");
  budget.reset();
  assert.equal(budget.built, 0, "reset clears the slice's count");
  assert.equal(budget.doneOne(), false, "and restarts its clock");
});

test("a nonsensical budget falls back to the default", () => {
  const clock = fakeClock(0);
  for (const budgetMs of [0, -1, NaN, undefined, "eight"]) {
    clock.set(0);
    const budget = createSliceBudget({ budgetMs, now: clock.now });
    clock.set(100);
    assert.equal(budget.doneOne(), true, `budget ${String(budgetMs)} should use the default`);
  }
});

test("a new render supersedes the previous one without cancelling it", () => {
  const gate = createRenderGate();
  const first = gate.begin();
  assert.equal(gate.isCurrent(first), true);
  const second = gate.begin();
  assert.equal(gate.isCurrent(first), false, "the superseded render stops");
  assert.equal(gate.isCurrent(second), true);
  assert.equal(gate.cancelled, false, "superseding is not a cancellation");
});

test("cancel stops the render in progress and reports it as cancelled", () => {
  const gate = createRenderGate();
  const token = gate.begin();
  gate.cancel();
  assert.equal(gate.isCurrent(token), false, "a cancelled render must stop");
  assert.equal(gate.cancelled, true);
});

test("starting the next render clears a previous cancellation", () => {
  const gate = createRenderGate();
  gate.begin();
  gate.cancel();
  assert.equal(gate.cancelled, true);
  const next = gate.begin();
  assert.equal(gate.cancelled, false);
  assert.equal(gate.isCurrent(next), true);
});
