(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameComparisonScope = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const VERSION = 1;
  // Comparisons are remembered most-recent-first; the oldest fall out first so
  // the store stays a fixed, small thing rather than growing forever.
  const STORE_LIMIT = 20;

  // Global display preferences (#260). These describe how the reader wants to
  // see a result, not what the comparison is, so they are persisted once and
  // shared by every comparison. `key` is the browser-storage key and `kind`
  // says how the stored string maps back onto the control.
  const GLOBAL_PREFERENCES = Object.freeze([
    { id: "lang", key: "ayame-lang", kind: "value" },
    { id: "theme", key: "ayame-theme", kind: "value" },
    { id: "scheme", key: "ayame-scheme", kind: "value" },
    { id: "wrap", key: "ayame-wrap", kind: "boolean" },
    { id: "viewMode", key: "ayame-view", kind: "value" },
    { id: "showWs", key: "ayame-showws", kind: "boolean" },
    { id: "syntax", key: "ayame-syntax", kind: "boolean" },
    { id: "word", key: "ayame-word", kind: "boolean" },
    { id: "contextLines", key: "ayame-context-lines", kind: "number" },
    { id: "contextToggle", key: "ayame-context-visible", kind: "boolean" },
    { id: "autoReload", key: "ayame-auto-reload", kind: "boolean" },
    { id: "sidebarToggle", key: "ayame-sidebar", kind: "boolean" },
  ]);

  // Controls that change what a comparison means. They are scoped to the
  // comparison's identity — mode plus input paths — and travel in the versioned
  // URL fragment (#254). This list is the single source of truth: app.js reads
  // it both for the URL state and for the per-comparison local memory.
  const COMPARISON_CONTROLS = Object.freeze([
    "encoding", "numeric", "reverse",
    "ignoreCase", "ignoreEOL", "ignoreTrailingEOL", "whitespace", "whitespaceScale", "lineFilters",
    "detectMoves", "moveMinLines", "window", "maxHunks", "maxLines",
    "hasHeader", "alignColumns", "leftFormat", "rightFormat", "leftParser", "rightParser",
    "leftDelimiter", "rightDelimiter", "lazyQuotes", "trimLeadingSpace", "keyMode",
    "ignoreColumns", "tolerance", "columnTolerances", "csvMaxRows",
    "memory", "tempDir", "partitions", "parseWorkers", "workers", "mergeFanIn",
    "partitionBuffer", "maxRecordBytes", "keepTemp", "changedColumnsOnly",
    "dirIncludes", "dirExcludes", "dirFilter", "dirFilterFile", "dirFilterSet",
    "dirCompareBy", "dirHidden", "dirWorkers", "dirStatus",
  ]);

  const globalByID = new Map(GLOBAL_PREFERENCES.map((preference) => [preference.id, preference]));
  const comparisonSet = new Set(COMPARISON_CONTROLS);

  function globalPreference(id) {
    return globalByID.get(id) || null;
  }

  function isGlobalPreference(id) {
    return globalByID.has(id);
  }

  function isComparisonControl(id) {
    return comparisonSet.has(id);
  }

  // comparisonKey is the identity the URL state and the local memory agree on:
  // the mode plus the trimmed input paths. It returns "" for anything that is
  // not a complete file-backed comparison, so scratch text and half-typed forms
  // never create a scope. Three-way modes require the base path.
  function comparisonKey(mode, paths) {
    if (typeof mode !== "string" || !mode) return "";
    const source = paths && typeof paths === "object" ? paths : {};
    const trim = (value) => (typeof value === "string" ? value.trim() : "");
    const base = trim(source.base);
    const old = trim(source.old);
    const next = trim(source.new);
    if (!old || !next) return "";
    if ((mode === "threeway" || mode === "threeway-csv") && !base) return "";
    return JSON.stringify([mode, base, old, next]);
  }

  // conditionSnapshot narrows an arbitrary controls object to the
  // per-comparison controls, dropping identity fields and display preferences.
  function conditionSnapshot(controls) {
    const snapshot = {};
    if (!controls || typeof controls !== "object" || Array.isArray(controls)) return snapshot;
    for (const id of COMPARISON_CONTROLS) {
      if (Object.prototype.hasOwnProperty.call(controls, id)) snapshot[id] = controls[id];
    }
    return snapshot;
  }

  function validSnapshot(value) {
    return Boolean(value) && typeof value === "object" && !Array.isArray(value);
  }

  // parseStore reads the persisted JSON into a plain object. It never throws:
  // a corrupted or foreign value reads as empty rather than taking the
  // comparison path down with it. Entries are opaque objects — app.js stores the
  // same versioned comparison state the URL fragment carries.
  function parseStore(raw) {
    if (typeof raw !== "string" || !raw) return {};
    try {
      const value = JSON.parse(raw);
      if (!validSnapshot(value)) return {};
      const store = {};
      for (const key of Object.keys(value)) {
        if (!key || key === "__proto__" || !validSnapshot(value[key])) continue;
        store[key] = value[key];
      }
      return store;
    } catch (_) {
      return {};
    }
  }

  // serializeStore keeps insertion order as recency and bounds the map.
  function serializeStore(store) {
    const source = validSnapshot(store) ? store : {};
    const keys = Object.keys(source);
    const kept = keys.slice(Math.max(0, keys.length - STORE_LIMIT));
    const bounded = {};
    for (const key of kept) bounded[key] = source[key];
    return JSON.stringify(bounded);
  }

  function readConditions(store, key) {
    if (!validSnapshot(store) || typeof key !== "string" || !key) return null;
    const snapshot = store[key];
    return validSnapshot(snapshot) ? { ...snapshot } : null;
  }

  // writeConditions records (or replaces) one comparison's state and moves it
  // to the newest position. Entries are copied through unchanged, so callers
  // decide what a comparison remembers.
  function writeConditions(store, key, snapshot) {
    const next = validSnapshot(store) ? { ...store } : {};
    if (typeof key !== "string" || !key || key === "__proto__") return next;
    delete next[key];
    next[key] = snapshot;
    const keys = Object.keys(next);
    for (const stale of keys.slice(0, Math.max(0, keys.length - STORE_LIMIT))) delete next[stale];
    return next;
  }

  return {
    VERSION,
    STORE_LIMIT,
    GLOBAL_PREFERENCES,
    COMPARISON_CONTROLS,
    globalPreference,
    isGlobalPreference,
    isComparisonControl,
    comparisonKey,
    conditionSnapshot,
    parseStore,
    serializeStore,
    readConditions,
    writeConditions,
  };
});
