// Ordered multi-side selection for merge hunks (#271). A hunk's adopted
// contributions are kept as a set, so selecting and deselecting are symmetric
// (P4Merge-style independent toggles) and a hunk can adopt more than one side
// ("both"). The canonical side order below fixes the order the engine
// concatenates the adopted contributions in; it is deliberately not the click
// order, so the output does not depend on which toggle was flipped first.
(function (root) {
  "use strict";

  // The contributions each merge surface can adopt, in concatenation order.
  // Two-way merges have no base; three-way merges concatenate base first so a
  // "both" conflict reads as the ancestor followed by the two descendants.
  const ORDERS = {
    text: ["left", "right"],
    csv: ["left", "right"],
    threeway: ["base", "left", "right"],
    "threeway-csv": ["base", "left", "right"],
  };

  function orderFor(kind) {
    return ORDERS[kind] || ORDERS.text;
  }

  // createMergeSelection returns the selection state for one comparison.
  function createMergeSelection(kind) {
    const order = orderFor(kind).slice();
    const known = new Set(order);
    let choices = new Map(); // String(id) -> Set(side)

    // sides returns the hunk's adopted contributions in canonical order.
    function sides(id) {
      const set = choices.get(String(id));
      if (!set || set.size === 0) return [];
      return order.filter((side) => set.has(side));
    }
    function has(id, side) { return sides(id).includes(side); }
    function selected(id) { return sides(id).length > 0; }
    function count(id) { return sides(id).length; }
    // isBoth reports a hunk adopting more than one contribution.
    function isBoth(id) { return count(id) > 1; }
    // size counts hunks with at least one contribution, the resolved count.
    function size() {
      let total = 0;
      for (const set of choices.values()) if (set.size > 0) total++;
      return total;
    }
    function ids() { return [...choices.keys()]; }
    function entries() { return ids().map((id) => [id, sides(id)]); }

    // toggle flips one contribution and returns the hunk's new sides.
    function toggle(id, side) {
      if (!known.has(side)) throw new Error("unknown merge side: " + side);
      const key = String(id);
      const set = new Set(choices.get(key) || []);
      if (set.has(side)) set.delete(side); else set.add(side);
      if (set.size > 0) choices.set(key, set); else choices.delete(key);
      return sides(key);
    }
    // set replaces the hunk's contributions with the given ones.
    function set(id, list) {
      const key = String(id);
      const next = new Set();
      for (const side of list || []) if (known.has(side)) next.add(side);
      if (next.size > 0) choices.set(key, next); else choices.delete(key);
      return sides(key);
    }
    // choose keeps the primary "pick a side" gesture but replaces rather than
    // merges, since a caller asking for exactly one side wants exactly one.
    function choose(id, side) { return set(id, [side]); }
    function clear(id) { choices.delete(String(id)); }
    function clearAll() { choices = new Map(); }
    function replace(list) {
      choices = new Map();
      for (const entry of list || []) set(entry[0], entry[1]);
    }
    function clone() {
      const copy = createMergeSelection(kind);
      copy.replace(entries());
      return copy;
    }
    // toWire encodes each resolved hunk as its comma-joined canonical sides.
    function toWire() {
      const out = {};
      for (const id of ids()) out[id] = sides(id).join(",");
      return out;
    }
    // unresolved counts the units without any contribution adopted.
    function unresolved(units) { return Math.max(0, (units || 0) - size()); }

    return {
      kind, order, sides, has, selected, count, isBoth, size, ids, entries,
      toggle, set, choose, clear, clearAll, replace, clone, toWire, unresolved,
    };
  }

  const api = { ORDERS, orderFor, createMergeSelection };
  root.AyameMergeSelect = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
