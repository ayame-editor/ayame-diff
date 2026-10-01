// Optimistic re-computation for editable panes (#258).
//
// A committed line edit used to leave the old diff on screen until a full
// server comparison came back. This module holds the two pieces of that wait
// which can be decided without the DOM:
//
//   * planLineApproximation — the cheap local guess: show the line the user just
//     typed, and, only when the comparison is a plain exact diff, re-judge that
//     one line as same/changed. Anything else (ignore rules, move detection,
//     sync points, no counterpart on screen) is honestly not decidable here, so
//     the caller keeps its existing behaviour for that path.
//   * createRecomputeCoordinator — a single-flight, trailing-debounce scheduler
//     so a burst of edits collapses into at most one authoritative comparison
//     plus one that catches up with whatever arrived while it was running.
//
// DOM-free on purpose, so it runs under node --test (#139). Timers are injected
// so the scheduling policy can be tested without a real clock.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameOptimistic = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // isPlainComparison reports whether a local line judgement would match the
  // server's. Every ignore rule (case, EOL, whitespace, line filters), numeric
  // comparison, move detection and sync point can make two lines equal or
  // unequal in a way the buffers alone cannot see, so they all disqualify it.
  function isPlainComparison(options = {}) {
    const filters = Array.isArray(options.lineFilters)
      ? options.lineFilters.filter((value) => String(value ?? "").trim() !== "")
      : [];
    const syncPoints = Array.isArray(options.syncPoints) ? options.syncPoints : [];
    return !options.numeric &&
      !options.ignoreCase &&
      !options.ignoreEOL &&
      !options.ignoreTrailingEOL &&
      !options.detectMoves &&
      (options.whitespace || "none") === "none" &&
      filters.length === 0 &&
      syncPoints.length === 0;
  }

  // planLineApproximation decides what can be shown from one committed edit.
  //
  // render is always true: the new text is what the user typed, so displaying
  // it is a fact, not a guess. classify is true only when the counterpart line
  // is on screen and no comparison rule can change the verdict; same is then the
  // exact line equality. When classify is false the caller shows the text but
  // leaves the difference classification to the authoritative result.
  function planLineApproximation(spec = {}) {
    const editedValue = String(spec.editedValue ?? "");
    const counterpartValue = spec.counterpartValue == null
      ? null
      : String(spec.counterpartValue);
    let reason = "ok";
    if (counterpartValue === null) reason = "no-counterpart";
    else if (!spec.plain) reason = "conditions";
    const classify = reason === "ok";
    return {
      render: true,
      classify,
      same: classify ? editedValue === counterpartValue : false,
      reason,
    };
  }

  // createRecomputeCoordinator folds re-computation requests into a single
  // in-flight run plus at most one trailing run.
  //
  // request() coalesces: while idle it arms one debounce timer, and while a run
  // is in flight it only raises a dirty flag, so N burst edits never queue N
  // comparisons. When a run finishes dirty, one more run starts so the final
  // edit is not lost. run() may return the string "busy" to ask for a retry
  // (another exclusive operation held the lock) without counting as a failure.
  function createRecomputeCoordinator(spec = {}) {
    const delay = Math.max(0, Number(spec.delay) || 0);
    const run = typeof spec.run === "function" ? spec.run : null;
    const setTimer = typeof spec.setTimer === "function"
      ? spec.setTimer
      : ((fn, ms) => setTimeout(fn, ms));
    const clearTimer = typeof spec.clearTimer === "function"
      ? spec.clearTimer
      : ((id) => clearTimeout(id));
    const onChange = typeof spec.onChange === "function" ? spec.onChange : () => {};

    let timer = null;
    let running = false;
    let dirty = false;
    let disposed = false;
    let runCount = 0;

    function state() {
      return {
        armed: timer !== null,
        running,
        dirty,
        pending: timer !== null || running || dirty,
        runCount,
      };
    }
    function notify() { onChange(state()); }

    function arm() {
      if (disposed) return;
      if (timer !== null) clearTimer(timer);
      timer = setTimer(fire, delay);
      notify();
    }

    function fire() {
      timer = null;
      if (disposed) { notify(); return; }
      if (running) { dirty = true; notify(); return; }
      void start();
    }

    async function start() {
      if (disposed || running) return;
      running = true;
      notify();
      let outcome = true;
      try {
        outcome = run ? await run() : true;
      } catch (_) {
        outcome = false;
      } finally {
        running = false;
        runCount += 1;
        const retry = outcome === "busy";
        if (!disposed && (dirty || retry)) {
          dirty = false;
          arm();
        } else {
          notify();
        }
      }
    }

    return {
      request() {
        if (disposed) return;
        if (running) { dirty = true; notify(); return; }
        arm();
      },
      cancel() {
        if (timer !== null) { clearTimer(timer); timer = null; }
        dirty = false;
        notify();
      },
      async flush() {
        if (disposed) return false;
        if (timer !== null) { clearTimer(timer); timer = null; }
        if (running) { dirty = true; notify(); return false; }
        await start();
        return true;
      },
      state,
      dispose() {
        disposed = true;
        if (timer !== null) { clearTimer(timer); timer = null; }
        dirty = false;
        notify();
      },
    };
  }

  return {
    isPlainComparison,
    planLineApproximation,
    createRecomputeCoordinator,
  };
});
