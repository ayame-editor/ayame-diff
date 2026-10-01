// Multiple-comparison tab state (#281).
//
// The GUI used to hold exactly one comparison: starting another discarded the
// first. The bookkeeping a tab bar needs — which tabs exist, which is active,
// what each one remembers, and how the set serializes — is pure list
// arithmetic, so it lives here and runs under node --test instead of being
// inferred from a rendered toolbar.
//
// A tab records the same comparison state the URL codec already carries plus a
// logical scroll anchor. Results themselves are not stored: activation
// re-applies the state and recomputes, which keeps memory bounded (#281).
//
// Nothing here touches the DOM or the network: app.js owns rendering, storage
// and the comparison itself.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameTabs = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const VERSION = 1;
  const MAX_LABEL = 48;
  const DEFAULT_PREFIX = "tab";

  function clampIndex(value, length) {
    if (!Number.isInteger(value) || length <= 0) return 0;
    return Math.max(0, Math.min(value, length - 1));
  }

  function baseName(path) {
    const cleaned = String(path || "").replace(/[\\/]+$/, "");
    const cut = Math.max(cleaned.lastIndexOf("/"), cleaned.lastIndexOf("\\"));
    return cut >= 0 ? cleaned.slice(cut + 1) : cleaned;
  }

  function truncateLabel(label, max = MAX_LABEL) {
    const text = String(label == null ? "" : label);
    if (text.length <= max) return text;
    return `${text.slice(0, Math.max(1, max - 1))}…`;
  }

  // The label a comparison earns is the two file names, so a row of tabs reads
  // like the panes it opens. A narrower tab is CSS's business; this only bounds
  // the string so the URL and the DOM do not carry a whole path.
  function labelFromState(state, fallback = "") {
    if (!state || typeof state !== "object" || Array.isArray(state)) return fallback;
    const paths = state.paths || {};
    const left = baseName(paths.old);
    const right = baseName(paths.new);
    if (!left && !right) return fallback;
    const both = right && right !== left ? `${left} ⇄ ${right}` : left || right;
    return truncateLabel(both);
  }

  function emptyDoc() {
    return { v: VERSION, active: 0, tabs: [] };
  }

  function normalizeTab(tab) {
    if (!tab || typeof tab !== "object" || Array.isArray(tab)) return null;
    if (typeof tab.id !== "string" || !tab.id) return null;
    const state = tab.state && typeof tab.state === "object" && !Array.isArray(tab.state)
      ? tab.state
      : null;
    let scroll = null;
    if (tab.scroll && typeof tab.scroll === "object" && !Array.isArray(tab.scroll) &&
        typeof tab.scroll.group === "string" && typeof tab.scroll.key === "string") {
      scroll = {
        group: tab.scroll.group,
        key: tab.scroll.key,
        order: Number.isFinite(tab.scroll.order) ? tab.scroll.order : null,
        offset: Number.isFinite(tab.scroll.offset) ? tab.scroll.offset : 0,
      };
    }
    const label = truncateLabel(
      typeof tab.label === "string" && tab.label ? tab.label : labelFromState(state, tab.id),
    );
    return { id: tab.id, label, state, scroll };
  }

  // Unknown or damaged input yields null so callers can fall back to a single
  // comparison rather than render a half-built tab bar. Duplicate ids are
  // dropped: two tabs with one id would make activation ambiguous.
  function normalizeDoc(value) {
    if (!value || typeof value !== "object" || Array.isArray(value)) return null;
    if (value.v !== VERSION) return null;
    if (!Array.isArray(value.tabs)) return null;
    const seen = new Set();
    const tabs = [];
    for (const item of value.tabs) {
      const tab = normalizeTab(item);
      if (!tab || seen.has(tab.id)) continue;
      seen.add(tab.id);
      tabs.push(tab);
    }
    return { v: VERSION, active: clampIndex(value.active, tabs.length), tabs };
  }

  function nextId(doc, prefix = DEFAULT_PREFIX) {
    const used = new Set((doc?.tabs || []).map((tab) => tab.id));
    let index = used.size + 1;
    let id = `${prefix}-${index}`;
    while (used.has(id)) {
      index += 1;
      id = `${prefix}-${index}`;
    }
    return id;
  }

  function activeTab(doc) {
    if (!doc || !Array.isArray(doc.tabs) || !doc.tabs.length) return null;
    return doc.tabs[clampIndex(doc.active, doc.tabs.length)];
  }

  function addTab(doc, tab = {}, options = {}) {
    const base = normalizeDoc(doc) || emptyDoc();
    const entry = normalizeTab({ ...tab, id: tab.id || nextId(base) });
    if (!entry) return base;
    const tabs = [...base.tabs, entry];
    const active = options.activate === false ? clampIndex(base.active, tabs.length) : tabs.length - 1;
    return { v: VERSION, active, tabs };
  }

  // Closing the active tab activates its neighbour: the one to the right when
  // there is one, otherwise the one to the left, the way editor tabs behave.
  function removeTab(doc, id) {
    if (!doc || !Array.isArray(doc.tabs)) return emptyDoc();
    const index = doc.tabs.findIndex((tab) => tab.id === id);
    if (index < 0) return doc;
    const tabs = doc.tabs.filter((tab) => tab.id !== id);
    if (!tabs.length) return emptyDoc();
    let active = doc.active;
    if (index < active) active -= 1;
    else if (index === active) active = index;
    return { v: VERSION, active: clampIndex(active, tabs.length), tabs };
  }

  function activateTab(doc, id) {
    if (!doc || !Array.isArray(doc.tabs)) return emptyDoc();
    const index = doc.tabs.findIndex((tab) => tab.id === id);
    if (index < 0) return doc;
    if (index === doc.active) return doc;
    return { v: VERSION, active: index, tabs: doc.tabs };
  }

  function updateTab(doc, id, patch = {}) {
    if (!doc || !Array.isArray(doc.tabs)) return emptyDoc();
    let changed = false;
    const tabs = doc.tabs.map((tab) => {
      if (tab.id !== id) return tab;
      changed = true;
      const merged = { ...tab, ...patch };
      // A new comparison state re-earns the label unless the caller overrode it.
      if (patch.state !== undefined && patch.label === undefined) {
        merged.label = labelFromState(merged.state, tab.id);
      }
      return normalizeTab(merged) || tab;
    });
    return changed ? { v: VERSION, active: clampIndex(doc.active, tabs.length), tabs } : doc;
  }

  function serializeDoc(doc) {
    const base = normalizeDoc(doc) || emptyDoc();
    return { v: VERSION, active: base.active, tabs: base.tabs };
  }

  function parseDoc(value) {
    return normalizeDoc(value);
  }

  return {
    VERSION,
    MAX_LABEL,
    emptyDoc,
    activeTab,
    addTab,
    removeTab,
    activateTab,
    updateTab,
    nextId,
    labelFromState,
    truncateLabel,
    serializeDoc,
    parseDoc,
  };
});
