// Paging and column arithmetic for the CSV result table (#154).
//
// Turning a page used to discard the whole result — summary, pane headers,
// column headers, pager and all — to change a hundred rows. These are the
// decisions that survive that rebuild being removed: which page is in range,
// which columns are shown, and what the pager controls should say. They are
// pure, so node checks them rather than a rebuilt table (#139).
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameCSVView = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // There is always one page, even with nothing on it: an empty result still
  // renders a table frame rather than a pager that says "page 1 of 0".
  function csvPageCount(total, pageSize) {
    const rows = Math.max(0, Number(total) || 0);
    const size = Math.max(1, Number(pageSize) || 1);
    return Math.max(1, Math.ceil(rows / size));
  }

  function clampPage(page, pageCount) {
    const count = Math.max(1, Number(pageCount) || 1);
    const requested = Number.isFinite(Number(page)) ? Math.trunc(Number(page)) : 0;
    return Math.max(0, Math.min(requested, count - 1));
  }

  // "Changed columns only" hides the columns the comparison found nothing in.
  // The set is what the summary reported, so a column that changed outside the
  // rows on screen still counts.
  function visibleColumns(headerLength, changedIndexes, changedOnly) {
    const length = Math.max(0, Number(headerLength) || 0);
    const columns = [];
    const changed = new Set(changedIndexes || []);
    for (let index = 0; index < length; index++) {
      if (!changedOnly || changed.has(index)) columns.push(index);
    }
    return columns;
  }

  // What the pager shows for a page. Returned rather than applied so the
  // controls can be updated in place instead of rebuilt.
  function pagerState(page, pageCount) {
    const count = Math.max(1, Number(pageCount) || 1);
    const current = clampPage(page, count);
    return {
      page: current,
      pageCount: count,
      value: String(current + 1),
      atFirst: current === 0,
      atLast: current + 1 >= count,
    };
  }

  // The slice of differences a page shows, and where it starts, so a row can
  // still name its position in the whole result.
  function pageSlice(differences, page, pageSize) {
    const rows = Array.isArray(differences) ? differences : [];
    const size = Math.max(1, Number(pageSize) || 1);
    const current = clampPage(page, csvPageCount(rows.length, size));
    const start = current * size;
    return { start, rows: rows.slice(start, start + size) };
  }

  return { csvPageCount, clampPage, visibleColumns, pagerState, pageSlice };
});
