// Pure bookkeeping for the unresolved-difference list (#272). A merge save used
// to count unresolved hunks and then silently resolve every one of them to the
// left, without showing which they were. This module answers "which items still
// have no choice?" from the same data the merge UI already holds, so app.js can
// list them, jump to one, and pick an explicit implicit-resolution target.
// DOM-free so it is testable under node --test.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameUnresolved = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // Text merge choices are keyed by numeric hunk index, CSV and three-way
  // choices by their string IDs, so both a Map and a plain object are accepted.
  function choiceOf(choices, id) {
    if (choices instanceof Map) return choices.get(id);
    if (choices && typeof choices === "object") return choices[String(id)];
    return undefined;
  }

  // items is [{ id, label }] for every item that needs a side; choices holds the
  // sides already picked. A truthy fallback means every item is covered by an
  // all-left / all-right default, so nothing counts as unresolved.
  // Returns [{ index, id, label }] in display order.
  function unresolvedItems(items, choices, fallback) {
    if (fallback) return [];
    const list = Array.isArray(items) ? items : [];
    const unresolved = [];
    for (let index = 0; index < list.length; index++) {
      const item = list[index];
      if (!item || typeof item !== "object") continue;
      const choice = choiceOf(choices, item.id);
      if (choice === undefined || choice === null || choice === "") {
        unresolved.push({ index, id: item.id, label: item.label == null ? "" : String(item.label) });
      }
    }
    return unresolved;
  }

  function unresolvedCount(items, choices, fallback) {
    return unresolvedItems(items, choices, fallback).length;
  }

  // The implicit-resolution targets each merge kind can honor. Two-way text
  // hunks can keep either side or leave markers; two-way CSV rows can keep a
  // side but have no marker form; three-way adds BASE, and only three-way text
  // has a marker form.
  function targetsFor(kind) {
    switch (kind) {
      case "csv": return ["left", "right"];
      case "threeway-text": return ["left", "right", "base", "markers"];
      case "threeway-csv": return ["left", "right", "base"];
      default: return ["left", "right", "markers"];
    }
  }

  return { unresolvedItems, unresolvedCount, targetsFor };
});
