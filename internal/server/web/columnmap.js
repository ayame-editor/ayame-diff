// Manual CSV column mapping for the result view (#119).
//
// The engine aligns columns by header name, and falls back to nothing when the
// names disagree, when a column exists on only one side, or when several are
// swapped. This module is the pure half of the receiving surface: it builds a
// starting left-right pairing, validates a hand-edited one, and turns it into
// the request shape the engine accepts. It stays free of the DOM so node can
// check it, and the engine validates the same rules again server-side.
//
// #116's content-based estimate is not in this branch, so the starting value
// is the header-name alignment when it succeeds and positional pairing
// otherwise; the caller reports which, rather than pretending it is a guess.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameColumnMap = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // A side that has no such column. The engine's ColumnPair uses the same value.
  const ABSENT = -1;

  function count(value) {
    const n = Math.trunc(Number(value));
    return Number.isFinite(n) && n > 0 ? n : 0;
  }

  function normalizePair(raw) {
    const left = Number(raw && raw.left);
    const right = Number(raw && raw.right);
    return {
      left: Number.isInteger(left) ? left : ABSENT,
      right: Number.isInteger(right) ? right : ABSENT,
      ignore: Boolean(raw && raw.ignore),
    };
  }

  // positionalMapping pairs columns by position, then appends the columns the
  // other side does not have as one-sided entries, so a column-count mismatch
  // is representable instead of an error.
  function positionalMapping(leftCount, rightCount) {
    const n = count(leftCount), m = count(rightCount);
    const shared = Math.min(n, m);
    const mapping = [];
    for (let i = 0; i < shared; i++) mapping.push({ left: i, right: i, ignore: false });
    for (let i = shared; i < n; i++) mapping.push({ left: i, right: ABSENT, ignore: false });
    for (let j = shared; j < m; j++) mapping.push({ left: ABSENT, right: j, ignore: false });
    return mapping;
  }

  // initialMapping mirrors the engine's alignColumnsByName: when every left
  // header name has a right counterpart, pair by name. Anything else falls back
  // to position. The returned source says which rule was used.
  function initialMapping(leftHeader, rightHeader, options) {
    const left = Array.isArray(leftHeader) ? leftHeader : [];
    const right = Array.isArray(rightHeader) ? rightHeader : [];
    const alignByName = !options || options.alignByName !== false;
    const byName = new Map();
    right.forEach((name, index) => {
      if (!byName.has(name)) byName.set(name, index);
    });
    let nameAligned = alignByName && left.length > 0 && left.length <= right.length;
    if (nameAligned) {
      for (const name of left) {
        if (!byName.has(name)) { nameAligned = false; break; }
      }
    }
    if (nameAligned) {
      const mapping = left.map((name, index) => ({ left: index, right: byName.get(name), ignore: false }));
      return { mapping, source: "name" };
    }
    return { mapping: positionalMapping(left.length, right.length), source: "position" };
  }

  // validateMapping applies the engine's rules client-side so a bad row is
  // explained in place: indexes are -1 or in range, no index repeats, and each
  // row names at least one side.
  function validateMapping(mapping, leftCount, rightCount) {
    const errors = [];
    const leftSeen = new Set(), rightSeen = new Set();
    const list = Array.isArray(mapping) ? mapping : [];
    list.forEach((raw, position) => {
      const pair = normalizePair(raw);
      if (pair.left !== ABSENT && (pair.left < ABSENT || pair.left >= leftCount)) {
        errors.push({ position, error: "leftRange" });
      }
      if (pair.right !== ABSENT && (pair.right < ABSENT || pair.right >= rightCount)) {
        errors.push({ position, error: "rightRange" });
      }
      if (pair.left === ABSENT && pair.right === ABSENT) {
        errors.push({ position, error: "empty" });
      }
      if (pair.left !== ABSENT) {
        if (leftSeen.has(pair.left)) errors.push({ position, error: "leftDuplicate" });
        leftSeen.add(pair.left);
      }
      if (pair.right !== ABSENT) {
        if (rightSeen.has(pair.right)) errors.push({ position, error: "rightDuplicate" });
        rightSeen.add(pair.right);
      }
    });
    return { valid: errors.length === 0, errors };
  }

  // mappingToRequest drops rows that name neither side and normalizes the rest
  // into the engine request shape. Empty input means "no manual map".
  function mappingToRequest(mapping) {
    const list = Array.isArray(mapping) ? mapping : [];
    const request = [];
    for (const raw of list) {
      const pair = normalizePair(raw);
      if (pair.left === ABSENT && pair.right === ABSENT) continue;
      request.push({ left: pair.left, right: pair.right, ignore: pair.ignore });
    }
    return request;
  }

  // mappingFromRequest restores the editor from a saved request/project. A
  // project holds the engine shape; missing fields become absent columns.
  function mappingFromRequest(mapping) {
    const list = Array.isArray(mapping) ? mapping : [];
    return list.map(normalizePair);
  }

  // reorderColumns moves one displayed column to another position. It backs the
  // drag handles on the result header and changes presentation only.
  function reorderColumns(columns, from, to) {
    const list = Array.isArray(columns) ? columns.slice() : [];
    if (!Number.isInteger(from) || !Number.isInteger(to) || from < 0 || from >= list.length) return list;
    const target = Math.max(0, Math.min(to, list.length - 1));
    if (from === target) return list;
    const [moved] = list.splice(from, 1);
    list.splice(target, 0, moved);
    return list;
  }

  return {
    ABSENT,
    positionalMapping,
    initialMapping,
    validateMapping,
    mappingToRequest,
    mappingFromRequest,
    reorderColumns,
  };
});
