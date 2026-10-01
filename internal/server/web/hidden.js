// What a collapsed unchanged region hides (#268). The bar used to say only how
// many lines were folded away, which tells a reader nothing about the content;
// this turns the hidden range into the line numbers, an excerpt of the first
// hidden line, a heuristic source symbol, or the key values of omitted CSV
// rows. Pure and DOM-free, so node --test can exercise it (#139).
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameHidden = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const EXCERPT_MAX = 80;
  const DEFAULT_KEY_SEPARATOR = " / ";

  function nonNegativeInteger(value) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.max(0, Math.trunc(number)) : 0;
  }

  // A 1-based inclusive range, or null when the hidden run is empty. The start
  // is a 0-based line index because that is what the diff response uses.
  function lineRange(start, count) {
    const first = nonNegativeInteger(start);
    const length = nonNegativeInteger(count);
    if (!length) return null;
    return { first: first + 1, last: first + length };
  }

  function formatRange(range) {
    if (!range) return "";
    return range.first === range.last ? String(range.first) : `${range.first}\u2013${range.last}`;
  }

  // One readable line from the hidden content: whitespace runs collapse so a
  // tab-indented line stays a single visual line in the bar.
  function excerpt(line, max) {
    const text = String(line == null ? "" : line).replace(/\s+/g, " ").trim();
    if (!text) return "";
    const limit = Math.max(1, nonNegativeInteger(max) || EXCERPT_MAX);
    return text.length > limit ? text.slice(0, limit - 1) + "\u2026" : text;
  }

  // Easy wins only: a first hidden line that itself declares or names a block.
  // Anything else falls back to the line range, which is always known, rather
  // than pretending a guessed enclosing symbol is the hidden content.
  function symbol(line) {
    const text = String(line == null ? "" : line).replace(/\s+$/, "");
    if (!text.trim()) return "";
    let match = text.match(
      /^\s*(?:export\s+|pub\s+|public\s+|private\s+|protected\s+|internal\s+|static\s+|async\s+)*(func|function|def|fn|class|interface|struct|enum|type|const|let|var|module|namespace)\s+([A-Za-z_$][\w$]*)/,
    );
    if (match) return `${match[1]} ${match[2]}`;
    match = text.match(/^\s{0,3}(#{1,6})\s+(.+?)\s*#*\s*$/);
    if (match) return match[2];
    return "";
  }

  // The key columns of a row, joined in key order so a composite key still
  // reads as one value.
  function keyValue(row, indexes) {
    if (!Array.isArray(row)) return "";
    const columns = Array.isArray(indexes) ? indexes : [];
    const parts = [];
    for (const index of columns) {
      const at = nonNegativeInteger(index);
      parts.push(row[at] == null ? "" : String(row[at]));
    }
    return parts.join(DEFAULT_KEY_SEPARATOR);
  }

  // The span of keys a set of rows covers. Rows arrive in the engine's key
  // order, so the first and last row are the natural endpoints; a single
  // numeric key column is compared as a number so "9" sorts before "10".
  function keyRange(rows, indexes) {
    const list = Array.isArray(rows) ? rows : [];
    const values = [];
    for (const row of list) {
      const value = keyValue(row, indexes);
      if (value !== "") values.push(value);
    }
    if (!values.length) return null;
    const columns = Array.isArray(indexes) ? indexes : [];
    const numeric = columns.length === 1 && values.every((value) => /^-?\d+(?:\.\d+)?$/.test(value));
    let first = values[0];
    let last = values[0];
    for (const value of values) {
      const before = numeric ? Number(value) < Number(first) : value < first;
      const after = numeric ? Number(value) > Number(last) : value > last;
      if (before) first = value;
      if (after) last = value;
    }
    return { first, last, single: first === last, count: values.length };
  }

  function formatKeyRange(range) {
    if (!range) return "";
    return range.single ? String(range.first) : `${range.first}\u2013${range.last}`;
  }

  // Map key column names (header mode) and raw 0-based indexes (headerless
  // mode) onto positions in the inspection header. Names win when present.
  function keyIndexes(header, names, indexes) {
    const head = Array.isArray(header) ? header : [];
    const result = [];
    const seen = new Set();
    for (const name of Array.isArray(names) ? names : []) {
      const at = head.indexOf(name);
      if (at >= 0 && !seen.has(at)) { seen.add(at); result.push(at); }
    }
    for (const index of Array.isArray(indexes) ? indexes : []) {
      const at = nonNegativeInteger(index);
      if (at < head.length && !seen.has(at)) { seen.add(at); result.push(at); }
    }
    return result;
  }

  // The collapsed bar's text. `t` is the UI translate function, so the module
  // stays independent of the catalog; `preview` is the first hidden line when
  // the server has returned it, and `source` selects the symbol heuristic.
  function describeGap(input, t) {
    const item = input || {};
    const count = nonNegativeInteger(item.count).toLocaleString();
    const oldRange = lineRange(item.oldStart, item.count);
    const newRange = lineRange(item.newStart, item.count);
    const oldText = formatRange(oldRange);
    const newText = formatRange(newRange);
    let text;
    if (oldText && newText && oldText !== newText) {
      text = t("contextHiddenRangeSides", { count, old: oldText, new: newText });
    } else if (oldText || newText) {
      text = t("contextHiddenRange", { count, range: oldText || newText });
    } else {
      text = t("contextHidden", { count });
    }
    // Source files name the folded block when the first hidden line declares
    // one; otherwise the line range above already says where it is. Plain text
    // has no declarations to find, so it shows the first hidden line itself.
    const line = item.source ? symbol(item.preview) : excerpt(item.preview);
    return line ? `${text} \u00b7 ${line}` : text;
  }

  return {
    lineRange,
    formatRange,
    excerpt,
    symbol,
    keyValue,
    keyRange,
    formatKeyRange,
    keyIndexes,
    describeGap,
  };
});
