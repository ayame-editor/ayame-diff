// Layout and resolution state for the three-way result (#282).
//
// The old three-way view drew a BASE column into every event and kept the
// merge output behind a path input in a separate panel, so the reader could
// not see what would be written without saving it first. Following
// Kaleidoscope's placement, the common view is LEFT | RESULT | RIGHT and BASE
// is available on demand; the result column is the merge output.
//
// Deciding what an event resolves to mirrors internal/threeway.MergeLines, so
// the result pane can be checked under node --test instead of only against a
// saved file.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameThreeWayView = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  function lines(value) {
    return Array.isArray(value) ? value.slice() : [];
  }

  // The lines a merge writes for one event, given the user's choice. This is
  // the same selection MergeLines makes: independent and identical changes
  // apply themselves, CSV merged groups use their combined rows, and a
  // conflict waits for LEFT/RIGHT/BASE. An unresolved conflict still previews
  // BASE, but is flagged so the pane cannot read as a finished merge.
  function resultLines(event, choice) {
    const kind = (event && event.kind) || "";
    if (kind === "conflict") {
      switch (choice) {
        case "left": return { lines: lines(event.left), unresolved: false };
        case "right": return { lines: lines(event.right), unresolved: false };
        case "base": return { lines: lines(event.base), unresolved: false };
        default: return { lines: lines(event.base), unresolved: true };
      }
    }
    if (kind === "right_only") return { lines: lines(event.right), unresolved: false };
    if (kind === "merged") return { lines: lines(event.combined), unresolved: false };
    // left_only and same_change both resolve to LEFT, which is also RIGHT for
    // a same_change event. Anything unrecognised falls back to BASE.
    if (kind === "same_change") return { lines: lines(event.left), unresolved: false };
    if (kind === "left_only") return { lines: lines(event.left), unresolved: false };
    return { lines: lines(event.base), unresolved: false };
  }

  // The panes one event draws, in reading order. BASE is on demand: without it
  // the common comparison is LEFT | RESULT | RIGHT, and asking for it adds a
  // leading BASE column. The result pane is always present and carries the
  // unresolved flag the view turns into a badge.
  function panes(event, choice, showBase) {
    const result = resultLines(event, choice);
    const list = [];
    if (showBase) list.push({ role: "base", lines: lines(event.base), unresolved: false });
    list.push({ role: "left", lines: lines(event.left), unresolved: false });
    list.push({ role: "result", lines: result.lines, unresolved: result.unresolved });
    list.push({ role: "right", lines: lines(event.right), unresolved: false });
    return list;
  }

  // Whether an event still needs the user to pick a side. Used to count what a
  // merge would leave unresolved without re-deriving the whole result.
  function unresolved(event, choice) {
    return resultLines(event, choice).unresolved;
  }

  return { resultLines, panes, unresolved };
});
