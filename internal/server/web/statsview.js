// Pure statistics helpers for the result panel (#120). Turning a summary into
// rows, percentages, bar widths, and an exportable CSV/JSON artifact needs no
// DOM, so it lives here and runs under node --test like the other modules.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameStatsView = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  function number(value) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  function ratio(part, whole) {
    const total = number(whole);
    if (total <= 0) return 0;
    return number(part) / total;
  }

  // Percentages are shown at most to one decimal: a column that accounts for
  // less than a percent would otherwise read as "0%", so it is called out.
  function formatPercent(fraction, digits) {
    const value = Math.max(0, Math.min(1, number(fraction)));
    if (value > 0 && value < 0.01) return "<1%";
    const percent = value * 100;
    const places = Number.isFinite(digits) ? digits : (percent > 0 && percent < 10 ? 1 : 0);
    return `${percent.toFixed(places)}%`;
  }

  function barPercent(value, max) {
    const largest = number(max);
    if (largest <= 0) return 0;
    return Math.max(0, Math.min(100, (number(value) / largest) * 100));
  }

  // csvColumnRows normalizes the engine's column_changes. The cap lives on the
  // renderer, not here: every column the engine reports becomes a row.
  function csvColumnRows(summary) {
    const source = summary || {};
    const list = Array.isArray(source.column_changes) ? source.column_changes : [];
    let largest = 0;
    for (const column of list) largest = Math.max(largest, number(column && column.count));
    return list.map((column) => {
      const count = number(column && column.count);
      const row = {
        index: number(column && column.index),
        name: String(column && column.name != null ? column.name : ""),
        count,
        share: number(column && column.share),
        bar: barPercent(count, largest),
      };
      const numeric = column && column.numeric;
      if (numeric) {
        row.numeric = {
          count: number(numeric.count), sum: number(numeric.sum), mean: number(numeric.mean),
          min: number(numeric.min), max: number(numeric.max),
          increased: number(numeric.increased), decreased: number(numeric.decreased),
          unchanged: number(numeric.unchanged),
        };
      }
      return row;
    });
  }

  // textSummary gathers the whole-file context the server reports alongside the
  // kind totals, falling back to the totals when the response predates them.
  function textSummary(data) {
    const source = data || {};
    const added = number(source.added);
    const deleted = number(source.deleted);
    const modified = number(source.modified);
    const total = Math.max(number(source.old_lines), number(source.new_lines));
    const changed = source.changed_lines !== undefined ? number(source.changed_lines) : added + deleted + modified;
    return {
      old_lines: number(source.old_lines), new_lines: number(source.new_lines),
      added, deleted, modified, moved_blocks: number(source.moved_blocks),
      hunk_count: number(source.hunk_count),
      changed_lines: changed, total_lines: total,
      changed_share: source.changed_share !== undefined ? number(source.changed_share) : ratio(changed, total),
      largest_hunk: number(source.largest_hunk),
    };
  }

  // buildStats turns a comparison response into the artifact both the panel and
  // the exports consume, so the three cannot drift apart.
  function buildStats(kind, data) {
    if (kind === "csv") {
      const source = data || {};
      const summary = source.summary || {};
      return {
        kind: "csv",
        summary,
        header: Array.isArray(source.header) ? source.header.slice() : [],
        columns: csvColumnRows(summary),
      };
    }
    return { kind: "text", summary: textSummary(data) };
  }

  const SUMMARY_METRICS = {
    csv: ["left_rows", "right_rows", "equal_rows", "left_only", "right_only", "changed_left", "changed_right", "diff_rows", "changed_rows"],
    text: ["old_lines", "new_lines", "added", "deleted", "modified", "moved_blocks", "hunk_count", "changed_lines", "total_lines", "changed_share", "largest_hunk"],
  };

  const NUMERIC_METRICS = [
    ["delta_count", "count"], ["delta_sum", "sum"], ["delta_mean", "mean"],
    ["delta_min", "min"], ["delta_max", "max"],
    ["increased", "increased"], ["decreased", "decreased"], ["unchanged", "unchanged"],
  ];

  function csvCell(value) {
    const text = value == null ? "" : String(value);
    return /[",\n\r]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
  }

  // toCSV writes a long-format table (section,column,metric,value) rather than
  // one wide schema per mode: a spreadsheet pivot and a script both read it.
  function toCSV(document) {
    const stats = document || {};
    const summary = stats.summary || {};
    const rows = [["section", "column", "metric", "value"]];
    for (const metric of SUMMARY_METRICS[stats.kind] || []) {
      if (summary[metric] !== undefined) rows.push(["summary", "", metric, String(summary[metric])]);
    }
    if (stats.kind === "csv") {
      for (const column of stats.columns || []) {
        rows.push(["column", column.name, "changed", String(column.count)]);
        rows.push(["column", column.name, "share", String(column.share)]);
        if (column.numeric) {
          for (const [label, key] of NUMERIC_METRICS) {
            rows.push(["column", column.name, label, String(column.numeric[key])]);
          }
        }
      }
    }
    return rows.map((row) => row.map(csvCell).join(",")).join("\n") + "\n";
  }

  function toJSON(document) {
    return JSON.stringify(document, null, 2) + "\n";
  }

  return { ratio, formatPercent, barPercent, csvColumnRows, textSummary, buildStats, toCSV, toJSON };
});
