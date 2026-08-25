// Windowing for the folder comparison's continuous view (#291).
//
// Reading a change set one file at a time means opening and returning once per
// file; a hundred differing files is a hundred round trips. The continuous view
// stacks every differing file in one scroll, which only works if the browser is
// never asked to hold all of them at once: a file's diff is fetched when it
// comes near the viewport, and files that have drifted far away give their DOM
// back.
//
// The decisions about which files that means are pure, so they run under
// node --test (#139) instead of being inferred from a scrollbar.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameContinuous = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // A file with no differences has nothing to read in a continuous pass. The
  // status filter still applies first, so this only removes what the filter
  // would have shown as an unchanged row.
  function continuousEntries(entries) {
    return (entries || []).filter((entry) => entry && entry.status !== "same");
  }

  function clampIndex(index, total) {
    if (!Number.isInteger(index)) return 0;
    return Math.max(0, Math.min(index, Math.max(0, total - 1)));
  }

  // The files worth having ready around the one being read: the current file,
  // a little of what is coming, and enough of what is behind that scrolling
  // back up does not re-fetch immediately.
  function windowAround(focusIndex, total, { ahead = 2, behind = 1 } = {}) {
    if (!Number.isInteger(total) || total <= 0) return [];
    const focus = clampIndex(focusIndex, total);
    const first = Math.max(0, focus - Math.max(0, behind));
    const last = Math.min(total - 1, focus + Math.max(0, ahead));
    const window = [];
    for (let index = first; index <= last; index++) window.push(index);
    return window;
  }

  // What to give back once more files are loaded than the budget allows:
  // whatever is furthest from the file being read, and never the focus itself.
  // Ties break towards the earlier file, so a long scroll down releases what is
  // behind before what is ahead.
  function unloadTargets(loadedIndexes, focusIndex, limit) {
    const loaded = [...new Set((loadedIndexes || []).filter(Number.isInteger))];
    const budget = Number.isInteger(limit) && limit > 0 ? limit : 1;
    if (loaded.length <= budget) return [];
    const focus = Number.isInteger(focusIndex) ? focusIndex : 0;
    const ranked = loaded
      .filter((index) => index !== focus)
      .sort((a, b) => {
        const distance = Math.abs(b - focus) - Math.abs(a - focus);
        if (distance !== 0) return distance;
        return a - b;
      });
    return ranked.slice(0, loaded.length - budget).sort((a, b) => a - b);
  }

  // Where next/prev goes. Movement inside a file is the caller's business; this
  // answers what happens at a file's edge, including refusing to run off either
  // end of the change set.
  function stepSection(currentIndex, direction, total) {
    if (!Number.isInteger(total) || total <= 0) return -1;
    const current = clampIndex(currentIndex, total);
    const next = current + (direction < 0 ? -1 : 1);
    if (next < 0 || next >= total) return -1;
    return next;
  }

  // The file a continuous scroll is "at": the last one whose top has passed the
  // reading line. Offsets arrive already measured, so this stays testable.
  function sectionAt(offsets, scrollTop) {
    if (!Array.isArray(offsets) || !offsets.length) return -1;
    const position = Number(scrollTop) || 0;
    let found = 0;
    for (let index = 0; index < offsets.length; index++) {
      if ((Number(offsets[index]) || 0) <= position) found = index;
      else break;
    }
    return found;
  }

  return { continuousEntries, windowAround, unloadTargets, stepSection, sectionAt };
});
