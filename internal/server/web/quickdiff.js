// Local-change regions for the gutter while a diff pane is being edited (#292).
//
// Once a pane is editable there are two different questions on screen at once:
// how the two compared files differ, and what this session has typed over the
// file it loaded. The first is the hunk rendering. The second is the difference
// between the editable buffer's baseline and its current lines, which is a
// separate computation and has to be redone every time an edit lands.
//
// That arithmetic is pure, so it runs under node --test (#139) rather than
// being inferred from the DOM.
//
// The editable buffer changes lines in place, so the baseline and the current
// lines normally have the same length and every change is a substitution. A
// length mismatch is still handled so that a future insert or delete cannot
// throw: a line only the current buffer has is "added", and a line only the
// baseline has is "removed".
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameQuickDiff = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  function textLines(value) {
    return Array.isArray(value) ? value : [];
  }

  // Changed lines clustered into runs. Adjacent changed lines of the same kind
  // become one region, which is what a gutter bar spans; a run of lines all
  // restored to the baseline drops out entirely.
  function localChangeRegions(originalLines, editedLines) {
    const original = textLines(originalLines);
    const edited = textLines(editedLines);
    const length = Math.max(original.length, edited.length);
    const regions = [];
    let current = null;
    for (let index = 0; index < length; index++) {
      const hasOriginal = index < original.length;
      const hasEdited = index < edited.length;
      const changed = !hasOriginal || !hasEdited || original[index] !== edited[index];
      if (!changed) {
        current = null;
        continue;
      }
      const kind = !hasOriginal ? "added" : !hasEdited ? "removed" : "modified";
      if (current && current.kind === kind && current.end === index - 1) {
        current.end = index;
        continue;
      }
      current = { kind, start: index, end: index };
      regions.push(current);
    }
    return regions;
  }

  // A line -> kind lookup for the render path, which asks per cell and should
  // not re-walk the whole file once per line.
  function localChangeIndex(originalLines, editedLines) {
    const index = new Map();
    for (const region of localChangeRegions(originalLines, editedLines)) {
      for (let line = region.start; line <= region.end; line++) index.set(line, region.kind);
    }
    return index;
  }

  // The region a line belongs to, so clicking a bar can put the whole run back
  // rather than only the line the pointer landed on.
  function regionAt(regions, line) {
    if (!Number.isInteger(line)) return null;
    for (const region of Array.isArray(regions) ? regions : []) {
      if (line >= region.start && line <= region.end) return region;
    }
    return null;
  }

  return { localChangeRegions, localChangeIndex, regionAt };
});
