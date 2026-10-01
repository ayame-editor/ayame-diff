"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  STAGE_KEYS,
  stagePlan,
  isStageKey,
  formatElapsed,
  createProgressTracker,
  degradationNotice,
} = require("../progress.js");

function fakeClock(start = 1000) {
  let current = start;
  return {
    now: () => current,
    advance(ms) {
      current += ms;
    },
  };
}

function newTracker(clock, overrides) {
  const seen = [];
  const tracker = createProgressTracker({
    now: clock.now,
    onChange: (snapshot) => seen.push(snapshot),
    ...overrides,
  });
  return { tracker, seen };
}

test("the stage plan follows the comparison that actually runs", () => {
  assert.deepEqual(stagePlan(), ["read", "compare", "result", "render"]);
  assert.deepEqual(stagePlan({ detectMoves: true }), ["read", "compare", "moves", "result", "render"]);
  for (const key of stagePlan({ detectMoves: true })) {
    assert.ok(isStageKey(key), `${key} is not a known stage`);
  }
  assert.ok(!isStageKey("skipping"));
  assert.deepEqual(STAGE_KEYS, ["read", "compare", "moves", "result", "render"]);
});

test("elapsed time reads well at every scale", () => {
  assert.equal(formatElapsed(0), "0.0s");
  assert.equal(formatElapsed(3400), "3.4s");
  assert.equal(formatElapsed(12500), "13s");
  assert.equal(formatElapsed(59000), "59s");
  assert.equal(formatElapsed(65000), "1m 05s");
  assert.equal(formatElapsed(754000), "12m 34s");
  assert.equal(formatElapsed(-5), "0.0s");
  assert.equal(formatElapsed(undefined), "0.0s");
});

test("a stage stays indeterminate until it reports real counts", () => {
  const clock = fakeClock();
  const { tracker } = newTracker(clock);
  tracker.start(stagePlan());
  clock.advance(1500);
  tracker.begin("read");

  let snapshot = tracker.snapshot();
  assert.equal(snapshot.stage, "read");
  assert.equal(snapshot.determinate, false);
  assert.equal(snapshot.percent, null);
  assert.equal(snapshot.stageElapsedMs, 0);

  clock.advance(2500);
  snapshot = tracker.snapshot();
  assert.equal(snapshot.stageElapsedMs, 2500);
  assert.equal(snapshot.elapsedMs, 4000);
});

test("finishing a stage records its elapsed and leaves no active stage", () => {
  const clock = fakeClock();
  const { tracker } = newTracker(clock);
  tracker.start(stagePlan());
  tracker.begin("compare");
  clock.advance(9000);
  tracker.finish("compare");

  const snapshot = tracker.snapshot();
  assert.equal(snapshot.stage, null);
  assert.equal(snapshot.stages.find((stage) => stage.key === "compare").elapsedMs, 9000);
  assert.equal(snapshot.stages.find((stage) => stage.key === "compare").state, "done");
});

test("the server's reported duration wins over the local clock", () => {
  const clock = fakeClock();
  const { tracker } = newTracker(clock);
  tracker.start(stagePlan());
  tracker.begin("read");
  clock.advance(1000);
  tracker.finish("read", 12345);
  assert.equal(tracker.snapshot().stages.find((stage) => stage.key === "read").elapsedMs, 12345);
});

test("a heartbeat does not reset the stage clock", () => {
  const clock = fakeClock();
  const { tracker } = newTracker(clock);
  tracker.start(stagePlan());
  tracker.begin("compare");
  clock.advance(5000);
  tracker.begin("compare"); // heartbeat
  clock.advance(2000);
  const snapshot = tracker.snapshot();
  assert.equal(snapshot.stage, "compare");
  assert.equal(snapshot.stageElapsedMs, 7000);
});

test("a determinate stage reports a real percentage", () => {
  const clock = fakeClock();
  const { tracker } = newTracker(clock);
  tracker.start(stagePlan());
  tracker.begin("result");
  tracker.progress(1, 4);

  const snapshot = tracker.snapshot();
  assert.equal(snapshot.determinate, true);
  assert.equal(snapshot.percent, 25);

  tracker.progress(4, 4);
  assert.equal(tracker.snapshot().percent, 100);

  tracker.begin("render");
  assert.equal(tracker.snapshot().determinate, false, "the next stage starts indeterminate again");
});

test("starting a new stage finishes the previous one", () => {
  const clock = fakeClock();
  const { tracker } = newTracker(clock);
  tracker.start(stagePlan());
  tracker.begin("read");
  clock.advance(3000);
  tracker.begin("compare");
  const snapshot = tracker.snapshot();
  assert.equal(snapshot.stages.find((stage) => stage.key === "read").state, "done");
  assert.equal(snapshot.stages.find((stage) => stage.key === "read").elapsedMs, 3000);
  assert.equal(snapshot.stage, "compare");
});

test("the truncation forecast becomes a localized notice", () => {
  assert.equal(degradationNotice(null), null);
  assert.equal(degradationNotice({ degraded: false }), null);

  assert.deepEqual(degradationNotice({
    degraded: true, reason: "hunk_limit", omitted_hunks: 5, hunk_count: 205, max_hunks: 200,
  }), { key: "degradedHunks", params: { omitted: 5, total: 205, max: 200 } });

  assert.deepEqual(degradationNotice({
    degraded: true, reason: "line_limit", max_lines: 200, truncated_hunks: 3,
  }), { key: "degradedLines", params: { max: 200, count: 3 } });
});

test("a new comparison resets the previous run's stages and forecast", () => {
  const clock = fakeClock();
  const { tracker } = newTracker(clock);
  tracker.start(stagePlan());
  tracker.begin("compare");
  tracker.setForecast({ degraded: true, reason: "hunk_limit", omitted_hunks: 1 });
  tracker.start(stagePlan());
  const snapshot = tracker.snapshot();
  assert.equal(snapshot.stage, null);
  assert.equal(snapshot.forecast, null);
  assert.ok(snapshot.stages.every((stage) => stage.state === "pending" && stage.elapsedMs === 0));
});
