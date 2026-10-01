// The result toolbar's comparison-condition controls (#264).
//
// JetBrains keeps the settings that change how a diff is *read* permanently in
// the viewer's toolbar, so the policy a result was computed under stays visible
// while the result is on screen. This module derives the value each control
// shows in its label from the settings the dialog owns; app.js mirrors the two
// rather than keeping a second copy of the state.
//
// Only the toggles live here. Values that tune how much is computed — move min
// lines, context lines, and the line-filter definitions themselves — stay in the
// settings dialog, so the toolbar's item count is fixed at one row per
// condition and never grows with the input. The derivation is pure, so it runs
// under node --test (#139).
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameConditions = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const WHITESPACE_VALUES = ["none", "change", "all"];
  const CASE_VALUES = ["distinguish", "ignore"];
  // The settings dialog exposes whitespace and EOL as one monotonic scale
  // (#259); the toolbar mirrors it as two derived rows, so this stays bounded.
  const EOL_VALUES = ["as-is", "eol", "trailing", "both"];
  const SCALE_VALUES = ["strict", "eol", "eol-change", "eol-all"];
  const MOVE_VALUES = ["off", "detect"];

  // One entry per toolbar row, in display order. `element` is the control's id
  // in index.html; the name/value message keys are resolved by the caller so
  // this stays free of the catalog and of the DOM.
  const TOOLBAR_CONTROLS = [
    { id: "whitespace", element: "tbWhitespace", nameKey: "whitespace" },
    { id: "case", element: "tbCase", nameKey: "caseCondition" },
    { id: "eol", element: "tbEol", nameKey: "eolCondition" },
    { id: "filters", element: "tbFilters", nameKey: "lineFilters" },
    { id: "moves", element: "tbMoves", nameKey: "conditionMoves" },
  ];

  const VALUE_KEYS = {
    whitespace: { none: "whitespaceNone", change: "whitespaceChange", all: "whitespaceAll" },
    case: { distinguish: "caseDistinguish", ignore: "caseIgnore" },
    eol: { "as-is": "eolKeep", eol: "ignoreEOL", trailing: "ignoreTrailingEOL", both: "eolBoth" },
    moves: { off: "conditionMovesOff", detect: "conditionMovesOn" },
  };

  const ACTIVE = {
    whitespace: (policy) => policy.whitespace !== "none",
    case: (policy) => policy.case === "ignore",
    eol: (policy) => policy.eol !== "as-is",
    filters: (policy) => policy.filters > 0,
    moves: (policy) => policy.moves === "detect",
  };

  // The filter box is one textarea, one regexp per line; its count is the whole
  // of its state as far as the toolbar is concerned.
  function lineFilterCount(value) {
    if (Array.isArray(value)) return value.filter((item) => String(item).trim() !== "").length;
    if (value == null || value === "") return 0;
    if (typeof value === "number") return Number.isFinite(value) ? Math.max(0, Math.trunc(value)) : 0;
    return String(value).split(/\r?\n/).map((line) => line.trim()).filter(Boolean).length;
  }

  // scaleFor maps the two toolbar dimensions back onto the single settings
  // control. change/all always ignore line endings on the scale, so an
  // "as-is" EOL with a non-default whitespace is read as the closest level.
  function scaleFor(whitespace, eol) {
    if (whitespace === "all") return "eol-all";
    if (whitespace === "change") return "eol-change";
    return eol === "as-is" ? "strict" : "eol";
  }

  // readPolicy turns the raw settings-control state into the compact vocabulary
  // the toolbar speaks. The whitespaceScale control is the source of truth
  // (#259); the legacy individual booleans are still understood so stored
  // policy and the pure tests can describe either shape.
  function readPolicy(raw = {}) {
    let whitespace = WHITESPACE_VALUES.includes(raw.whitespace) ? raw.whitespace : "none";
    let eol = "as-is";
    if (raw.ignoreEOL && raw.ignoreTrailingEOL) eol = "both";
    else if (raw.ignoreEOL) eol = "eol";
    else if (raw.ignoreTrailingEOL) eol = "trailing";
    if (SCALE_VALUES.includes(raw.whitespaceScale)) {
      const scale = raw.whitespaceScale;
      whitespace = scale === "eol-change" ? "change" : scale === "eol-all" ? "all" : "none";
      eol = scale === "strict" ? "as-is" : "eol";
    }
    return {
      whitespace,
      case: raw.ignoreCase ? "ignore" : "distinguish",
      eol,
      filters: lineFilterCount(raw.lineFilters),
      moves: raw.detectMoves ? "detect" : "off",
    };
  }

  // writePolicy maps a toolbar selection back onto the settings control it
  // owns. Both the whitespace and EOL rows write the shared scale, so the
  // current policy is passed in to preserve the other dimension. The filter
  // definitions are not editable from the toolbar, and the numeric tuning is
  // not represented here at all.
  function writePolicy(control, value, current = {}) {
    const ws = WHITESPACE_VALUES.includes(current.whitespace) ? current.whitespace : "none";
    const curEol = current.eol === "as-is" ? "as-is" : "eol";
    switch (control) {
      case "whitespace":
        return WHITESPACE_VALUES.includes(value) ? { whitespaceScale: scaleFor(value, curEol) } : {};
      case "case":
        return CASE_VALUES.includes(value) ? { ignoreCase: value === "ignore" } : {};
      case "eol":
        if (!EOL_VALUES.includes(value)) return {};
        return { whitespaceScale: scaleFor(ws, value === "as-is" ? "as-is" : "eol") };
      case "moves":
        return MOVE_VALUES.includes(value) ? { detectMoves: value === "detect" } : {};
      default:
        return {};
    }
  }

  function isDefault(policy) {
    const p = policy || readPolicy({});
    return Object.keys(ACTIVE).every((id) => !ACTIVE[id](p));
  }

  function activeConditionIds(policy) {
    const p = policy || readPolicy({});
    return Object.keys(ACTIVE).filter((id) => ACTIVE[id](p));
  }

  // describeConditions returns exactly one row per toolbar control — never one
  // per filter — with the current value already resolved through `t`. The
  // current value is both the selected option and the row's accessible title, so
  // the policy stays readable without opening the dropdown.
  function describeConditions(policy, t) {
    const tr = typeof t === "function" ? t : (key) => key;
    const p = policy || readPolicy({});
    return TOOLBAR_CONTROLS.map(({ id, element, nameKey }) => {
      let text;
      let value;
      if (id === "filters") {
        value = String(p.filters);
        text = p.filters > 0
          ? tr("conditionFilterCount", { count: p.filters })
          : tr("conditionFilterOff");
      } else {
        value = p[id];
        text = tr(VALUE_KEYS[id][value]);
      }
      return {
        id,
        element,
        name: tr(nameKey),
        text,
        value: String(value),
        active: ACTIVE[id](p),
      };
    });
  }

  return {
    WHITESPACE_VALUES,
    CASE_VALUES,
    EOL_VALUES,
    SCALE_VALUES,
    MOVE_VALUES,
    TOOLBAR_CONTROLS,
    lineFilterCount,
    readPolicy,
    writePolicy,
    isDefault,
    activeConditionIds,
    describeConditions,
  };
});
