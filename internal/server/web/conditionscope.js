// Session-scoped comparison conditions with an explicit default (#262).
//
// The problem this solves: a condition change used to be indistinguishable
// from a saved preference. Either it persisted silently, or it silently did
// not, and nothing on screen said which. Following Beyond Compare's
// session-scope dropdown, this module keeps the two apart:
//
//   * the default is a plain object, saved on its own and applied to every new
//     comparison;
//   * a session override is whatever the controls hold right now, and it never
//     reaches storage unless the user explicitly promotes it with
//     "Make default".
//
// The module is pure: it reads and writes plain objects and never touches the
// DOM or browser storage, so the precedence rules run under node --test (#139).
// app.js owns the wiring.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameConditionScope = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // The controls in the "Comparison conditions" group. Each declares how a raw
  // DOM value is validated, so a hand-edited or stale stored default can never
  // put an impossible value into a control.
  const CONDITION_CONTROLS = [
    { id: "ignoreCase", type: "checkbox" },
    { id: "ignoreEOL", type: "checkbox" },
    { id: "ignoreTrailingEOL", type: "checkbox" },
    { id: "whitespace", type: "select", values: ["none", "change", "all"] },
    { id: "lineFilters", type: "text" },
    { id: "detectMoves", type: "checkbox" },
  ];

  // The stored default lives under its own key, separate from any session
  // value in the URL or a project file, so loading one can never rewrite it.
  const STORAGE_KEY = "ayame-condition-defaults";

  function controlById(id) {
    return CONDITION_CONTROLS.find((control) => control.id === id) || null;
  }

  // validValue reports whether a stored value is usable as-is. Anything else
  // falls back, so a corrupt entry degrades to the default instead of blanking
  // a control.
  function validValue(control, value) {
    if (control.type === "checkbox") return typeof value === "boolean";
    if (control.type === "select") return typeof value === "string" && control.values.includes(value);
    if (control.type === "text") return typeof value === "string";
    return false;
  }

  // sanitize builds a complete condition set. For each control, a valid value
  // from `raw` wins; otherwise the fallback supplies it.
  function sanitize(raw, fallback) {
    const source = raw && typeof raw === "object" ? raw : {};
    const base = fallback && typeof fallback === "object" ? fallback : {};
    const out = {};
    for (const control of CONDITION_CONTROLS) {
      const value = source[control.id];
      out[control.id] = validValue(control, value) ? value : base[control.id];
    }
    return out;
  }

  // capture reads a condition set out of a DOM-like reader, so the module can
  // be tested with a plain map.
  function capture(read) {
    const out = {};
    for (const control of CONDITION_CONTROLS) {
      const value = read(control.id);
      out[control.id] = control.type === "checkbox" ? Boolean(value) : String(value == null ? "" : value);
    }
    return out;
  }

  // apply hands each control value to a writer, keeping every DOM access in
  // app.js.
  function apply(conditions, write) {
    for (const control of CONDITION_CONTROLS) {
      if (!Object.prototype.hasOwnProperty.call(conditions, control.id)) continue;
      write(control, conditions[control.id]);
    }
  }

  // changes lists the control ids whose value differs between two sets — the
  // "is this comparison still the default?" question the scope indicator
  // answers.
  function changes(candidate, baseline) {
    const current = sanitize(candidate, baseline);
    const reference = sanitize(baseline, baseline);
    const out = [];
    for (const control of CONDITION_CONTROLS) {
      if (current[control.id] !== reference[control.id]) out.push(control.id);
    }
    return out;
  }

  function isDefault(candidate, baseline) {
    return changes(candidate, baseline).length === 0;
  }

  // resolve is the precedence rule: a session override wins, then the saved
  // default, then the factory default. URL and project state arrive as the
  // session layer, which is why loading one never rewrites the default.
  function resolve(factory, persisted, session) {
    return sanitize(session, sanitize(persisted, factory));
  }

  // promote snapshots the current conditions as a fresh default; a copy, so a
  // later control change cannot mutate what was saved.
  function promote(conditions, fallback) {
    return sanitize(conditions, fallback || conditions);
  }

  function serialize(conditions, fallback) {
    return JSON.stringify(sanitize(conditions, fallback || conditions));
  }

  // parse returns null rather than throwing on anything unusable, so a broken
  // stored value falls back to the factory default instead of breaking
  // start-up.
  function parse(text, fallback) {
    if (typeof text !== "string" || text === "") return null;
    try {
      const value = JSON.parse(text);
      if (!value || typeof value !== "object" || Array.isArray(value)) return null;
      return sanitize(value, fallback);
    } catch (_) {
      return null;
    }
  }

  return {
    CONDITION_CONTROLS,
    STORAGE_KEY,
    controlById,
    validValue,
    sanitize,
    capture,
    apply,
    changes,
    isDefault,
    resolve,
    promote,
    serialize,
    parse,
  };
});
