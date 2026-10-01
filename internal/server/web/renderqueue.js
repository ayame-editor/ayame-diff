// Slice budgeting and render gating for incremental rendering (#127, #128).
//
// A large result cannot be built in one task: 20,000 rows is roughly 800,000
// elements, which blocks the main thread for seconds. app.js therefore builds
// in slices — work for a few milliseconds, hand the frame back, continue. Two
// decisions drive that and live here, away from the DOM so node checks them:
// how much work fits in one slice, and whether the render is still the current
// one. Building the nodes themselves stays in app.js.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameRenderQueue = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // A slice is sized by elapsed time, not item count: one hunk can be a single
  // line or a hundred, so counting items would either yield far too often or
  // sail past the frame. Eight milliseconds is comfortably inside a frame.
  const DEFAULT_BUDGET_MS = 8;

  // createSliceBudget tracks how long the current slice has run. doneOne()
  // records one built item and reports whether the budget is spent; the caller
  // then yields and reset()s. The caller builds at least one item before its
  // first doneOne(), so a single very expensive item cannot stall the loop
  // forever.
  function createSliceBudget(options) {
    const budgetMs = Math.max(1, Number(options && options.budgetMs) || DEFAULT_BUDGET_MS);
    const now = options && typeof options.now === "function" ? options.now : () => Date.now();
    let started = now();
    let built = 0;
    return {
      doneOne() {
        built += 1;
        return now() - started >= budgetMs;
      },
      reset() {
        started = now();
        built = 0;
      },
      get built() {
        return built;
      },
    };
  }

  // createRenderGate coalesces renders. begin() starts a new render and makes
  // every earlier one stale, so a superseded render stops appending instead of
  // interleaving with its successor. cancel() does the same to the render in
  // progress but records that it was cancelled rather than replaced, so the
  // caller can tell the two apart and report a cancellation instead of a
  // finished comparison.
  function createRenderGate() {
    let token = 0;
    let cancelled = false;
    return {
      begin() {
        cancelled = false;
        token += 1;
        return token;
      },
      isCurrent(candidate) {
        return candidate === token;
      },
      cancel() {
        cancelled = true;
        token += 1;
      },
      get cancelled() {
        return cancelled;
      },
    };
  }

  return { createSliceBudget, createRenderGate, DEFAULT_BUDGET_MS };
});
