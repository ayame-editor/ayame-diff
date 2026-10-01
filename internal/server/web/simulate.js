// Pure impact-plan helpers for destructive operations (#273). Before a merge
// save that may overwrite a compared input, the browser previews what would be
// written and which inputs would be touched. Keeping the path comparison and
// the plan shape here makes that preview testable without a DOM, in the spirit
// of KDiff3's "Simulate it / Do it".
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameSimulate = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // Fold separator styles and cosmetic path noise so two spellings of the same
  // target line up. The browser does not know the server's working directory,
  // so relative paths stay relative; matching the spellings the UI produced is
  // enough for an informational preview, and the server still enforces.
  function normalizePath(value) {
    const text = String(value == null ? "" : value).trim();
    if (!text) return "";
    const unified = text.replace(/\\/g, "/");
    const root = /^([A-Za-z]:)?\/+/.exec(unified);
    const drive = root && root[1] ? root[1].toUpperCase() + "/" : "";
    const leading = root && !root[1] ? "/" : "";
    const rest = root ? unified.slice(root[0].length) : unified;
    const parts = [];
    for (const part of rest.split("/")) {
      if (!part || part === ".") continue;
      if (part === ".." && parts.length && parts[parts.length - 1] !== "..") parts.pop();
      else if (part !== "..") parts.push(part);
    }
    return drive + leading + parts.join("/");
  }

  function samePath(left, right) {
    const a = normalizePath(left);
    return a !== "" && a === normalizePath(right);
  }

  // mergeImpact reads a plain description of a pending merge save and returns
  // the same plan the server would execute: the single output path it writes,
  // every compared input whose spelling resolves to that output, and the
  // ordered actions a confirmation should list.
  function mergeImpact(plan) {
    const source = plan || {};
    const output = normalizePath(source.output);
    const inputs = [];
    for (const [role, value] of [["base", source.base], ["old", source.old], ["new", source.new]]) {
      const path = normalizePath(value);
      if (path) inputs.push({ role, path });
    }
    const affected = output ? inputs.filter((input) => input.path === output) : [];
    const overwrite = Boolean(source.overwrite);
    const actions = [];
    if (output) actions.push({ type: "write", path: output, role: null, overwrite: affected.length > 0 });
    for (const input of affected) {
      actions.push({ type: "overwrite", path: input.path, role: input.role, overwrite });
    }
    return {
      mode: String(source.mode || ""),
      output,
      overwrite,
      unresolved: Math.max(0, Math.trunc(Number(source.unresolved) || 0)),
      inputs,
      affected,
      actions,
      deletes: [],
      summary: {
        writes: output ? 1 : 0,
        overwrites: affected.length,
        deletes: 0,
      },
      destructive: affected.length > 0,
    };
  }

  return { normalizePath, samePath, mergeImpact };
});
