// The presentation half of per-line merge provenance (#257).
//
// The server owns the merge itself: it resolves the choices and reports each
// output line's origin and a stable key. This module only turns that into a
// gutter marker, a class, and a breakdown, so it stays DOM-free and runs under
// node --test (#139).
//
// KDiff3 prints a letter in its summary column and gives manual lines an `m`;
// the marker here keeps that vocabulary.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameMergeProvenance = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const ORIGINS = ["base", "left", "right", "manual"];

  // Anything the server did not name is treated as base rather than dropped, so
  // a forgotten origin still shows a marker instead of an empty gutter.
  function normalizeOrigin(value) {
    const name = String(value == null ? "" : value);
    return ORIGINS.indexOf(name) >= 0 ? name : "base";
  }

  // KDiff3's manual marker `m`; the three input sides keep their initial.
  function marker(value) {
    switch (normalizeOrigin(value)) {
      case "left": return "L";
      case "right": return "R";
      case "manual": return "m";
      default: return "B";
    }
  }

  function originClass(value) {
    return "origin-" + normalizeOrigin(value);
  }

  function isManual(value) {
    return normalizeOrigin(value) === "manual";
  }

  // i18n key for the origin's name, so a tooltip can spell out the marker.
  function labelKey(value) {
    const name = normalizeOrigin(value);
    return "origin" + name.charAt(0).toUpperCase() + name.slice(1);
  }

  function nonNegative(value) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.max(0, Math.trunc(number)) : 0;
  }

  // Adopted is every line the result took from an input; manual is what the
  // user typed. The two partition the total, so the report cannot disagree
  // with the preview.
  function breakdown(provenance) {
    const source = provenance || {};
    const base = nonNegative(source.base);
    const left = nonNegative(source.left);
    const right = nonNegative(source.right);
    const manual = nonNegative(source.manual);
    return { base, left, right, manual, adopted: base + left + right, total: base + left + right + manual };
  }

  // countLines derives the same breakdown from a preview's line list, which is
  // what is actually on screen. A mismatch with the server's own count is a
  // bug worth seeing, so this is kept independent of it.
  function countLines(lines) {
    const items = Array.isArray(lines) ? lines : [];
    let base = 0;
    let left = 0;
    let right = 0;
    let manual = 0;
    for (const line of items) {
      switch (normalizeOrigin(line && line.origin)) {
        case "left": left++; break;
        case "right": right++; break;
        case "manual": manual++; break;
        default: base++; break;
      }
    }
    return { base, left, right, manual, adopted: base + left + right, total: items.length };
  }

  return { ORIGINS, normalizeOrigin, marker, originClass, isManual, labelKey, breakdown, countLines };
});
