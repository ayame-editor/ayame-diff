// Pure geometry for unchanged regions around diff hunks (#267). The browser
// asks the server only for the small ranges that are visible; keeping the
// range arithmetic here makes it testable without a DOM.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameUnchanged = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  function nonNegativeInteger(value) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.max(0, Math.trunc(number)) : 0;
  }

  // A region precedes every hunk and one follows the last hunk. Diff hunks
  // consume different numbers of lines on each side, but the runs between
  // them are aligned unchanged lines and therefore normally have equal
  // lengths. Taking the smaller length keeps malformed or manually-aligned
  // input inside both source bounds.
  function buildUnchangedRegions(hunks, oldLines, newLines, includeTrailing = true) {
    const regions = [];
    let oldEnd = 0;
    let newEnd = 0;
    const items = Array.isArray(hunks) ? hunks : [];
    for (let index = 0; index < items.length; index++) {
      const hunk = items[index] || {};
      const oldStart = nonNegativeInteger(hunk.old_start);
      const newStart = nonNegativeInteger(hunk.new_start);
      const oldCount = Math.max(0, oldStart - oldEnd);
      const newCount = Math.max(0, newStart - newEnd);
      regions.push({
        index,
        oldStart: oldEnd,
        newStart: newEnd,
        oldCount,
        newCount,
        count: Math.min(oldCount, newCount),
      });
      oldEnd = oldStart + nonNegativeInteger(hunk.old_len);
      newEnd = newStart + nonNegativeInteger(hunk.new_len);
    }
    const oldCount = includeTrailing ? Math.max(0, nonNegativeInteger(oldLines) - oldEnd) : 0;
    const newCount = includeTrailing ? Math.max(0, nonNegativeInteger(newLines) - newEnd) : 0;
    regions.push({
      index: items.length,
      oldStart: oldEnd,
      newStart: newEnd,
      oldCount,
      newCount,
      count: Math.min(oldCount, newCount),
    });
    return regions;
  }

  // The first region contributes lines immediately before the first hunk, the
  // last contributes lines immediately after the last hunk, and an interior
  // region contributes both edges. Overlapping edges collapse into one range.
  function initialContextRanges(regions, lineCount) {
    const items = Array.isArray(regions) ? regions : [];
    const wanted = nonNegativeInteger(lineCount);
    if (!wanted || items.length < 2) return [];
    const ranges = [];
    for (const region of items) {
      const count = nonNegativeInteger(region.count);
      if (!count) continue;
      const hasLowerEdge = region.index > 0;
      const hasUpperEdge = region.index < items.length - 1;
      const lower = hasLowerEdge ? Math.min(wanted, count) : 0;
      const upper = hasUpperEdge ? Math.min(wanted, count - lower) : 0;
      if (lower + upper >= count) {
        ranges.push({ region: region.index, offset: 0, count });
        continue;
      }
      if (lower) ranges.push({ region: region.index, offset: 0, count: lower });
      if (upper) ranges.push({ region: region.index, offset: count - upper, count: upper });
    }
    return ranges;
  }

  // Return the holes that remain after the loaded segments. Segments may be
  // adjacent or overlap while two requests settle; both cases still produce a
  // stable, duplicate-free set of gaps for the controls.
  function missingContextSpans(count, segments) {
    const limit = nonNegativeInteger(count);
    const intervals = (Array.isArray(segments) ? segments : [])
      .map((segment) => ({
        start: Math.min(limit, nonNegativeInteger(segment.offset)),
        end: Math.min(limit, nonNegativeInteger(segment.offset) + nonNegativeInteger(segment.count)),
      }))
      .filter((interval) => interval.end > interval.start)
      .sort((left, right) => left.start - right.start || left.end - right.end);
    const missing = [];
    let cursor = 0;
    for (const interval of intervals) {
      if (interval.start > cursor) missing.push({ offset: cursor, count: interval.start - cursor });
      cursor = Math.max(cursor, interval.end);
    }
    if (cursor < limit) missing.push({ offset: cursor, count: limit - cursor });
    return missing;
  }

  function batchContextRanges(ranges, maxRanges, maxLines) {
    const rangeLimit = Math.max(1, nonNegativeInteger(maxRanges));
    const lineLimit = Math.max(1, nonNegativeInteger(maxLines));
    const batches = [];
    let batch = [];
    let lines = 0;
    for (const range of Array.isArray(ranges) ? ranges : []) {
      const count = nonNegativeInteger(range?.count);
      if (!count) continue;
      if (batch.length && (batch.length >= rangeLimit || lines + count > lineLimit)) {
        batches.push(batch);
        batch = [];
        lines = 0;
      }
      batch.push(range);
      lines += count;
    }
    if (batch.length) batches.push(batch);
    return batches;
  }

  return { buildUnchangedRegions, initialContextRanges, missingContextSpans, batchContextRanges };
});
