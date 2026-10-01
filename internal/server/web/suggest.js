// "How do I make it match?" suggestions for a CSV/TSV result (#121).
//
// After a comparison reports differences, the next question is what setting
// would clear them. This module owns that reasoning: it reads the result the
// server already returned, guesses the dominant causes (column order, trailing
// whitespace, letter case, numeric rounding, a column that could be ignored),
// and proposes the engine option changes that would clear each one. It is pure
// and small so node --test can check the reasoning without a browser or an
// engine run (#139).
//
// The proposals it returns are option patches only. The server is asked to run
// the engine once per bounded candidate to report the real residual count; this
// module never runs a comparison itself.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameSuggest = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // How many candidates a single "what would clear this?" pass may propose.
  // Each one costs a full engine run on the server, so the set stays small.
  const MAX_PROPOSALS = 8;
  // The server refuses more than this many candidates in one suggest request.
  const MAX_CANDIDATES = 8;

  // A cell counts as "concentrated" in one column when at least this share of
  // all changed cells are in it, and there are at least two of them.
  const CONCENTRATION_SHARE = 0.5;

  function collapse(value) {
    return String(value == null ? "" : value).replace(/\s+/g, " ").trim();
  }

  function finiteNumber(value) {
    if (typeof value === "number") return Number.isFinite(value) ? value : null;
    const text = String(value == null ? "" : value).trim();
    if (text === "") return null;
    const number = Number(text);
    return Number.isFinite(number) ? number : null;
  }

  function decimalPlaces(text) {
    const match = /^[+-]?\d*(?:\.(\d+))?(?:[eE][+-]?\d+)?$/.exec(String(text).trim());
    return match && match[1] ? match[1].length : 0;
  }

  // A tolerance that certainly covers the pair: round the floating-point
  // difference up to the pair's own decimal precision instead of proposing a
  // value a hair below the real difference (0.020000000000000018 -> 0.02).
  function toleranceFor(leftText, rightText, leftNumber, rightNumber) {
    const difference = Math.abs(leftNumber - rightNumber);
    if (difference === 0) return 0;
    const places = Math.min(12, Math.max(decimalPlaces(leftText), decimalPlaces(rightText)));
    const factor = Math.pow(10, places);
    return Math.ceil(difference * factor - 1e-6) / factor;
  }

  // Classify one changed cell pair. Returns null when the two values are equal,
  // and otherwise a cause plus, for numeric pairs, the absolute difference that
  // a tolerance would have to cover.
  //
  // Whitespace is checked before case and numbers so " 1" vs "1" is reported as
  // whitespace rather than as a numerically-equal pair, and a formatting-only
  // numeric pair such as "1.0" vs "1" is reported as numeric with magnitude 0.
  function classifyCell(oldValue, newValue) {
    const left = String(oldValue == null ? "" : oldValue);
    const right = String(newValue == null ? "" : newValue);
    if (left === right) return null;
    if (collapse(left) === collapse(right)) return { cause: "whitespace", magnitude: 0 };
    if (left.toLowerCase() === right.toLowerCase()) return { cause: "case", magnitude: 0 };
    const leftNumber = finiteNumber(left);
    const rightNumber = finiteNumber(right);
    if (leftNumber !== null && rightNumber !== null) {
      return { cause: "numeric", magnitude: toleranceFor(left, right, leftNumber, rightNumber) };
    }
    return { cause: "content", magnitude: 0 };
  }

  // A tally of one cause across cells and columns.
  function tally() {
    return { cause: "", count: 0, magnitude: 0, columns: new Map() };
  }

  function addToTally(entry, cause, magnitude, columnName, columnIndex) {
    if (!entry.cause) entry.cause = cause;
    entry.count += 1;
    if (magnitude > entry.magnitude) entry.magnitude = magnitude;
    const key = columnIndex == null ? columnName : String(columnIndex);
    const column = entry.columns.get(key) || { index: columnIndex, name: columnName, count: 0, magnitude: 0 };
    column.count += 1;
    if (magnitude > column.magnitude) column.magnitude = magnitude;
    if (column.index == null && columnIndex != null) column.index = columnIndex;
    entry.columns.set(key, column);
  }

  // The number of distinct differences a summary describes, matching the
  // server's difference_count: each changed row pair counts once, not per side.
  function differenceCount(summary) {
    const changed = Math.max(Number(summary && summary.changed_left) || 0, Number(summary && summary.changed_right) || 0);
    return changed + (Number(summary && summary.left_only) || 0) + (Number(summary && summary.right_only) || 0);
  }

  // estimateCauses reads a CSV result and returns the causes it can attribute
  // differences to, most supported first. `options` is the comparison setup the
  // result was produced with; it decides which structural causes still apply.
  function estimateCauses(result, options) {
    const data = result || {};
    const setup = options || {};
    const causes = [];
    const cellTallies = new Map();

    const differences = Array.isArray(data.differences) ? data.differences : [];
    for (const difference of differences) {
      for (const changed of (difference && difference.changed_columns) || []) {
        const classified = classifyCell(changed.old, changed.new);
        if (!classified) continue;
        if (!cellTallies.has(classified.cause)) cellTallies.set(classified.cause, tally());
        addToTally(cellTallies.get(classified.cause), classified.cause, classified.magnitude, changed.name, changed.index);
      }
    }

    for (const entry of cellTallies.values()) {
      // "content" is the absence of a normalization cause; it is reported but
      // never gets a proposal.
      causes.push({ cause: entry.cause, count: entry.count, magnitude: entry.magnitude, columns: [...entry.columns.values()] });
    }

    const summary = data.summary || {};
    const total = differenceCount(summary);

    if (setup.alignColumnsByName === false && total > 0) {
      causes.push({ cause: "columnOrder", count: total, magnitude: 0, columns: [] });
    }

    const leftOnly = Number(summary.left_only) || 0;
    const rightOnly = Number(summary.right_only) || 0;
    if (leftOnly > 0 && rightOnly > 0) {
      const larger = Math.max(leftOnly, rightOnly);
      if (Math.abs(leftOnly - rightOnly) <= Math.max(1, Math.round(larger * 0.2))) {
        causes.push({ cause: "rowOrder", count: Math.min(leftOnly, rightOnly), magnitude: 0, columns: [] });
      }
    }

    const columnChanges = Array.isArray(summary.column_changes) ? summary.column_changes : [];
    const changedCells = columnChanges.reduce((sum, column) => sum + (Number(column.count) || 0), 0);
    const ranked = [...columnChanges].sort((a, b) => (Number(b.count) || 0) - (Number(a.count) || 0));
    const top = ranked[0];
    if (top && changedCells > 0 && ((Number(top.count) || 0) >= 2) && (Number(top.count) || 0) / changedCells >= CONCENTRATION_SHARE) {
      causes.push({ cause: "columnConcentration", count: Number(top.count) || 0, magnitude: 0, columns: [{ index: top.index, name: top.name, count: Number(top.count) || 0, magnitude: 0 }] });
    }

    if (!causes.length && total > 0) {
      causes.push({ cause: "content", count: total, magnitude: 0, columns: [] });
    }

    const priority = ["columnOrder", "whitespace", "case", "numeric", "rowOrder", "columnConcentration", "content"];
    causes.sort((a, b) => (b.count - a.count) || (priority.indexOf(a.cause) - priority.indexOf(b.cause)));
    return causes;
  }

  // The engine option selector for a column: its header name when the
  // comparison has a header, otherwise its index.
  function columnSelector(column, hasHeader) {
    if (hasHeader && column.name) return { name: column.name };
    if (column.index != null) return { index: column.index, by_index: true };
    return { name: column.name };
  }

  function keyedColumn(column, options) {
    const setup = options || {};
    if (!column) return false;
    if (Array.isArray(setup.keyNames) && column.name != null && setup.keyNames.includes(column.name)) return true;
    if (Array.isArray(setup.keyIndexes) && column.index != null && setup.keyIndexes.map(Number).includes(Number(column.index))) return true;
    return false;
  }

  function alreadyIgnored(column, options) {
    const setup = options || {};
    if (Array.isArray(setup.ignoreColumnNames) && column.name != null && setup.ignoreColumnNames.includes(column.name)) return true;
    if (Array.isArray(setup.ignoreColumnIndexes) && column.index != null && setup.ignoreColumnIndexes.map(Number).includes(Number(column.index))) return true;
    return false;
  }

  function alreadyToleranced(column, options) {
    const setup = options || {};
    return (setup.columnTolerances || []).some((item) => {
      if (item.name != null && column.name != null) return item.name === column.name;
      return Number(item.index) === Number(column.index);
    });
  }

  // buildProposals turns the estimated causes into stackable option patches.
  // Each patch is a partial csvRequest the caller can apply and re-run; the
  // server reports the residual differences that remain.
  function buildProposals(result, options) {
    const setup = options || {};
    const hasHeader = setup.hasHeader !== false;
    const causes = estimateCauses(result, setup);
    const byCause = new Map(causes.map((cause) => [cause.cause, cause]));
    const proposals = [];
    const seen = new Set();
    const push = (proposal) => {
      if (seen.has(proposal.id) || proposals.length >= MAX_PROPOSALS) return;
      seen.add(proposal.id);
      proposals.push(proposal);
    };

    if (byCause.has("columnOrder")) {
      push({ id: "alignColumnsByName", cause: "columnOrder", patch: { alignColumnsByName: true } });
    }

    const whitespace = byCause.get("whitespace");
    if (whitespace && setup.whitespace !== "change" && setup.whitespace !== "all") {
      push({ id: "whitespace", cause: "whitespace", patch: { whitespace: "change" }, count: whitespace.count });
    }

    const caseCause = byCause.get("case");
    if (caseCause && !setup.ignoreCase) {
      push({ id: "ignoreCase", cause: "case", patch: { ignoreCase: true }, count: caseCause.count });
    }

    const numeric = byCause.get("numeric");
    if (numeric) {
      const columns = [...numeric.columns]
        .filter((column) => !keyedColumn(column, setup) && !alreadyToleranced(column, setup) && !alreadyIgnored(column, setup))
        .sort((a, b) => b.count - a.count)
        .slice(0, 3);
      for (const column of columns) {
        push({
          id: `tolerance:${hasHeader && column.name ? column.name : column.index}`,
          cause: "numeric",
          patch: { columnTolerances: [{ ...columnSelector(column, hasHeader), value: column.magnitude }] },
          column, value: column.magnitude, count: column.count,
        });
      }
      // A global tolerance is the simpler ask once more than one column rounds,
      // but only an explicit include-key leaves the numeric columns out of the
      // key so the tolerance can actually apply.
      const explicitKey = setup.keyMode === "include" ||
        (Array.isArray(setup.keyNames) && setup.keyNames.length > 0) ||
        (Array.isArray(setup.keyIndexes) && setup.keyIndexes.length > 0);
      if (columns.length > 1 && setup.tolerance == null && explicitKey) {
        push({ id: "tolerance:*", cause: "numeric", patch: { tolerance: numeric.magnitude }, value: numeric.magnitude, count: numeric.count });
      }
    }

    const concentration = byCause.get("columnConcentration");
    if (concentration) {
      for (const column of concentration.columns.slice(0, 2)) {
        if (alreadyIgnored(column, setup)) continue;
        const selector = columnSelector(column, hasHeader);
        const patch = selector.name != null ? { ignoreColumnNames: [selector.name] } : { ignoreColumnIndexes: [selector.index] };
        push({ id: `ignore:${selector.name != null ? selector.name : selector.index}`, cause: "columnConcentration", patch, column, count: column.count });
      }
    }

    return proposals;
  }

  function sameSelector(left, right) {
    if (left.name != null && right.name != null) return left.name === right.name;
    if (left.index != null && right.index != null) return Number(left.index) === Number(right.index);
    return false;
  }

  function unionSelectors(existing, added) {
    const result = [...(existing || [])];
    for (const item of added || []) {
      if (!result.some((current) => sameSelector(current, item))) result.push(item);
    }
    return result;
  }

  // applyProposal returns a new options object with one proposal's patch
  // merged in. Applied proposals stack: a second call layers onto the first.
  function applyProposal(options, proposal) {
    const next = { ...(options || {}) };
    for (const [key, value] of Object.entries(proposal.patch || {})) {
      if (key === "columnTolerances") next.columnTolerances = unionSelectors(next.columnTolerances, value);
      else if (key === "ignoreColumnNames" || key === "ignoreColumnIndexes") next[key] = unionSelectors(next[key], value);
      else next[key] = value;
    }
    return next;
  }

  function applyProposals(options, proposals) {
    return (proposals || []).reduce((current, proposal) => applyProposal(current, proposal), { ...(options || {}) });
  }

  // isApplied reports whether every part of a proposal is already in options,
  // so the UI can mark a card as in effect.
  function isApplied(options, proposal) {
    const setup = options || {};
    for (const [key, value] of Object.entries(proposal.patch || {})) {
      if (key === "columnTolerances") {
        const applied = value.every((item) => alreadyToleranced(item, setup));
        if (!applied) return false;
      } else if (key === "ignoreColumnNames" || key === "ignoreColumnIndexes") {
        const applied = value.every((item) => (key === "ignoreColumnNames" ? alreadyIgnored({ name: item }, setup) : alreadyIgnored({ index: item }, setup)));
        if (!applied) return false;
      } else if (setup[key] !== value) {
        return false;
      }
    }
    return true;
  }

  return {
    MAX_PROPOSALS,
    MAX_CANDIDATES,
    classifyCell,
    differenceCount,
    estimateCauses,
    buildProposals,
    applyProposal,
    applyProposals,
    isApplied,
  };
});
