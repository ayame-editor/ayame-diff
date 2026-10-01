// Progress model for long comparisons (#297).
//
// The status lane used to show nothing but an elapsed-time counter, which says
// how long a comparison has run but not what it is doing, how far along it is,
// or whether the result is about to be cut short. The model here turns the
// stages a comparison genuinely passes through — reading the inputs, comparing,
// detecting moved blocks, streaming the result pages, rendering — plus a
// truncation forecast into one snapshot the lane can render.
//
// It is deliberately DOM-free and clock-injected so node --test exercises the
// exact logic the page runs (#139). Translation and painting stay in app.js.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameProgress = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // The phases the server reports for a text comparison, in order. "moves" is
  // present only when move detection is on, so it is not faked when it is off.
  // "render" is the browser's own sliced DOM pass, added after the result lands.
  const STAGE_KEYS = ["read", "compare", "moves", "result", "render"];

  function stagePlan(options) {
    const opts = options || {};
    const stages = ["read", "compare"];
    if (opts.detectMoves) stages.push("moves");
    stages.push("result", "render");
    return stages;
  }

  function isStageKey(key) {
    return STAGE_KEYS.indexOf(key) !== -1;
  }

  // formatElapsed renders milliseconds compactly enough for a status line:
  // "3.4s" under ten seconds, whole seconds under a minute, then "2m 05s".
  function formatElapsed(ms) {
    const value = Number(ms);
    if (!Number.isFinite(value) || value <= 0) return "0.0s";
    const seconds = value / 1000;
    if (seconds < 10) return seconds.toFixed(1) + "s";
    if (seconds < 60) return Math.round(seconds) + "s";
    const minutes = Math.floor(seconds / 60);
    const rest = Math.round(seconds - minutes * 60);
    return minutes + "m " + String(rest).padStart(2, "0") + "s";
  }

  // createProgressTracker follows one comparison. The caller drives it from the
  // server's stage events (begin/finish), reports determinate counts for the
  // stages that have them (page N of M while streaming, hunks while rendering),
  // and records any truncation forecast.
  function createProgressTracker(options) {
    const opts = options || {};
    const now = opts.now || (() => Date.now());
    const onChange = opts.onChange || (() => {});

    let stages = [];
    let startedAt = 0;
    let current = null;
    let currentStart = 0;
    let records = new Map();
    let forecast = null;
    let finished = false;

    function recordFor(key) {
      return records.get(key) || { state: "pending", elapsedMs: 0, done: 0, total: 0 };
    }

    function snapshot() {
      const elapsedMs = startedAt ? now() - startedAt : 0;
      const stageElapsedMs = currentStart ? now() - currentStart : 0;
      const active = current ? recordFor(current) : null;
      const determinate = Boolean(active && active.total > 0);
      return {
        elapsedMs,
        stage: current,
        stageElapsedMs,
        determinate,
        percent: determinate ? Math.max(0, Math.min(100, Math.round((active.done / active.total) * 100))) : null,
        done: determinate ? active.done : 0,
        total: determinate ? active.total : 0,
        stages: stages.map((key) => {
          const record = recordFor(key);
          return { key, state: record.state, elapsedMs: record.elapsedMs };
        }),
        forecast,
        finished,
      };
    }

    function emit() {
      onChange(snapshot());
    }

    function start(plan) {
      stages = (plan || stagePlan()).filter(isStageKey);
      startedAt = now();
      current = null;
      currentStart = 0;
      records = new Map();
      forecast = null;
      finished = false;
      emit();
    }

    function ensureKey(key) {
      if (key && !stages.includes(key)) stages.push(key);
    }

    // begin marks a stage active. A repeated active for the same stage (a
    // heartbeat) refreshes without restarting its clock, so the per-stage
    // elapsed keeps growing instead of resetting every few seconds.
    function begin(key) {
      if (!key || finished) return;
      ensureKey(key);
      if (current === key) {
        emit();
        return;
      }
      finishCurrent();
      current = key;
      currentStart = now();
      const record = recordFor(key);
      record.state = "active";
      record.elapsedMs = 0;
      records.set(key, record);
      emit();
    }

    function finishCurrent() {
      if (!current) return;
      const record = recordFor(current);
      record.state = "done";
      record.elapsedMs = now() - currentStart;
      records.set(current, record);
      current = null;
      currentStart = 0;
    }

    // finish closes a stage. elapsedMs lets a server-reported duration replace
    // the local wall clock; without it the tracker uses the time it observed.
    function finish(key, elapsedMs) {
      if (!key) return;
      ensureKey(key);
      const record = recordFor(key);
      record.state = "done";
      if (Number.isFinite(elapsedMs) && elapsedMs >= 0) record.elapsedMs = Number(elapsedMs);
      else if (current === key) record.elapsedMs = now() - currentStart;
      records.set(key, record);
      if (current === key) {
        current = null;
        currentStart = 0;
      }
      emit();
    }

    // progress reports a determinate count for the active stage (result pages,
    // rendered hunks). Stages without one stay indeterminate — the lane must
    // never invent a percentage the work does not support.
    function progress(done, total) {
      if (!current || finished) return;
      const record = recordFor(current);
      record.done = Math.max(0, Number(done) || 0);
      record.total = Math.max(0, Number(total) || 0);
      records.set(current, record);
      emit();
    }

    function setForecast(next) {
      forecast = next || null;
      emit();
    }

    function stop() {
      finishCurrent();
      finished = true;
      emit();
    }

    return {
      start,
      begin,
      finish,
      progress,
      setForecast,
      stop,
      snapshot,
    };
  }

  // degradationNotice turns a server truncation forecast into a localized-key
  // descriptor, or null when nothing is being cut short. reasons:
  //   hunk_limit — maxHunks dropped hunks from the result
  //   line_limit — maxLines cut long hunks short
  function degradationNotice(forecast) {
    if (!forecast || !forecast.degraded) return null;
    if (forecast.reason === "line_limit") {
      return {
        key: "degradedLines",
        params: {
          max: Number(forecast.max_lines) || 0,
          count: Number(forecast.truncated_hunks) || 0,
        },
      };
    }
    return {
      key: "degradedHunks",
      params: {
        omitted: Number(forecast.omitted_hunks) || 0,
        total: Number(forecast.hunk_count) || 0,
        max: Number(forecast.max_hunks) || 0,
      },
    };
  }

  return { STAGE_KEYS, stagePlan, isStageKey, formatElapsed, createProgressTracker, degradationNotice };
});
