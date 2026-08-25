// Message lane for the GUI (#97). Progress lives in its own element; results —
// success, warning, error — stack here so one operation's outcome cannot erase
// the previous one. Success withdraws itself after a few seconds, while a
// failure stays until it is dismissed, so a failed run is still readable after
// the next attempt starts.
//
// The log is DOM-free on purpose: it owns entries and timers, and the caller
// renders. That keeps it testable under node --test (#139).
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameMessages = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // Tones that withdraw on their own. Everything else waits for a dismissal,
  // which is the whole point of the lane: an error must survive the retry.
  const SELF_CLEARING_TONES = ["success", "info"];

  const DEFAULT_AUTO_DISMISS_MS = 6000;
  const DEFAULT_MAX_ENTRIES = 5;

  function selfClearing(tone) {
    return SELF_CLEARING_TONES.indexOf(tone) !== -1;
  }

  function normalizeTone(tone) {
    switch (tone) {
      case "error":
      case "warning":
      case "success":
        return tone;
      default:
        return "info";
    }
  }

  function createMessageLog(options) {
    const opts = options || {};
    const now = opts.now || (() => Date.now());
    const schedule = opts.schedule || ((fn, delay) => setTimeout(fn, delay));
    const cancel = opts.cancel || ((handle) => clearTimeout(handle));
    const onChange = opts.onChange || (() => {});
    const autoDismissMs = opts.autoDismissMs > 0 ? opts.autoDismissMs : DEFAULT_AUTO_DISMISS_MS;
    const maxEntries = opts.maxEntries > 0 ? opts.maxEntries : DEFAULT_MAX_ENTRIES;

    let entries = [];
    let nextID = 1;
    const timers = new Map();

    function snapshot() {
      return entries.map((entry) => ({ ...entry }));
    }

    function publish() {
      onChange(snapshot());
    }

    function stopTimer(id) {
      const handle = timers.get(id);
      if (handle !== undefined) {
        cancel(handle);
        timers.delete(id);
      }
    }

    function startTimer(entry) {
      stopTimer(entry.id);
      if (!selfClearing(entry.tone)) return;
      timers.set(entry.id, schedule(() => {
        timers.delete(entry.id);
        remove(entry.id);
      }, autoDismissMs));
    }

    function remove(id) {
      const before = entries.length;
      entries = entries.filter((entry) => entry.id !== id);
      stopTimer(id);
      if (entries.length !== before) publish();
    }

    // A repeated message refreshes the entry it duplicates instead of stacking
    // an identical line: a poll that fails every second must not bury the lane.
    function existing(message, tone) {
      for (let i = entries.length - 1; i >= 0; i--) {
        if (entries[i].message === message && entries[i].tone === tone) return entries[i];
      }
      return null;
    }

    function post(message, tone) {
      const text = String(message == null ? "" : message);
      if (!text) return null;
      const normalized = normalizeTone(tone);
      const at = now();

      const repeated = existing(text, normalized);
      if (repeated) {
        repeated.count++;
        repeated.at = at;
        entries = entries.filter((entry) => entry.id !== repeated.id).concat(repeated);
        startTimer(repeated);
        publish();
        return repeated.id;
      }

      const entry = { id: nextID++, message: text, tone: normalized, at, count: 1 };
      entries = entries.concat(entry);
      startTimer(entry);
      // The cap drops the oldest self-clearing line first, so a stack of
      // successes can never push a failure out of view.
      while (entries.length > maxEntries) {
        const victim = entries.find((candidate) => selfClearing(candidate.tone)) || entries[0];
        entries = entries.filter((candidate) => candidate.id !== victim.id);
        stopTimer(victim.id);
      }
      publish();
      return entry.id;
    }

    function dismiss(id) {
      remove(id);
    }

    function clear() {
      if (!entries.length) return;
      for (const entry of entries) stopTimer(entry.id);
      entries = [];
      publish();
    }

    return {
      post,
      dismiss,
      clear,
      entries: snapshot,
      isSelfClearing: selfClearing,
    };
  }

  return { createMessageLog, selfClearing, normalizeTone, SELF_CLEARING_TONES };
});
