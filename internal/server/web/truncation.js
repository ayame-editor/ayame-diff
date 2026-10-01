// Choosing how far to lift hunk truncation (#261). The server reports how many
// hunks it dropped, so the reader only has to say "compute more": the next cap
// is derived from what was actually omitted rather than guessed. Kept out of
// app.js so the arithmetic runs under node without a DOM.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameTruncation = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // The setup form's default, used only when a run reports no usable cap.
  const DEFAULT_MAX_HUNKS = 200;
  // A hard ceiling so one click can never ask the server to store an unbounded
  // number of hunks. It sits far above any diff a person reads on screen.
  const MAX_MAX_HUNKS = 1000000;
  // Ask for a little more than the diff is known to contain, so the next run
  // completes instead of truncating again by a hunk or two.
  const HEADROOM = 1.25;

  function positiveInteger(value, fallback) {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? Math.trunc(number) : fallback;
  }

  // nextMaxHunks returns the cap to use after a truncated run. `current` is the
  // cap that run used and `omitted` the hunks it dropped, so `current + omitted`
  // is the whole diff. Growth is derived from what the server reported, and a
  // later truncated run reports again, so repeated calls keep raising the cap
  // for as long as the diff stays truncated.
  function nextMaxHunks(current, omitted) {
    const base = positiveInteger(current, DEFAULT_MAX_HUNKS);
    const missing = Math.max(0, Math.trunc(Number(omitted) || 0));
    const wanted = Math.ceil((base + missing) * HEADROOM);
    return Math.min(MAX_MAX_HUNKS, Math.max(base + 1, wanted));
  }

  return { nextMaxHunks, DEFAULT_MAX_HUNKS, MAX_MAX_HUNKS };
});
