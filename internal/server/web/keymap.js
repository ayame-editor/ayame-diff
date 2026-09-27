// Default keyboard map and merge-flow advance rules (#277).
//
// The chord syntax lives here so a binding can be parsed, normalized and matched
// without a DOM. app.js owns what each action does and which controls it binds;
// this module is the one table the help dialog and the handlers both read, so a
// documented default cannot drift from the keys that fire.
//
// It deliberately stops short of the remapping mechanism owned by #285: there
// are no presets, overrides or storage here. When that work lands it can resolve
// the same DEFAULT_BINDINGS through its preset/override layer, and the merge
// actions added below stay part of the default map.
//
// A "chord" is one physical key press: zero or more modifiers plus a key,
// written canonically as "Ctrl+Alt+Shift+Key". Command (Meta) is folded into
// Ctrl because the application has always accepted either; one primary modifier
// avoids an ambiguous dispatch when both are held.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameKeymap = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // The actions the help dialog and the dispatcher share. The order is the order
  // the help lists them, so it reads as the merge flow: navigate the result,
  // navigate conflicts, choose a side, then save.
  const ACTION_IDS = [
    "navigateNext",
    "navigatePrev",
    "firstDiff",
    "lastDiff",
    "nextConflict",
    "prevConflict",
    "chooseLeft",
    "chooseRight",
    "chooseBase",
    "chooseBoth",
    "toggleAutoAdvance",
    "saveMerge",
    "search",
    "searchNext",
    "searchPrev",
    "close",
    "compare",
  ];

  // The WinMerge defaults are kept exactly for the four navigator keys and the
  // side-adoption keys (Alt+Left/Right/B), so a WinMerge user does not relearn
  // them. Conflict navigation is a separate task from difference navigation in a
  // merge, so it gets its own binding; F8 / Shift+F8 follows DiffMerge, whose
  // layout is the closest to this one. Ctrl+PageUp/PageDown (KDiff3) is not used:
  // browsers reserve it for tab switching.
  const DEFAULT_BINDINGS = {
    navigateNext: "Alt+ArrowDown",
    navigatePrev: "Alt+ArrowUp",
    firstDiff: "Alt+Home",
    lastDiff: "Alt+End",
    nextConflict: "F8",
    prevConflict: "Shift+F8",
    chooseLeft: "Alt+ArrowLeft",
    chooseRight: "Alt+ArrowRight",
    chooseBase: "Alt+B",
    chooseBoth: "Alt+A",
    toggleAutoAdvance: "Alt+Shift+A",
    saveMerge: "Ctrl+Shift+S",
    search: "Ctrl+F",
    searchNext: "Enter",
    searchPrev: "Shift+Enter",
    close: "Escape",
    compare: "Ctrl+Enter",
  };

  // "down", "ArrowDown" and the like all name the same physical key.
  const NAMED_KEYS = {
    escape: "Escape", esc: "Escape", enter: "Enter", return: "Enter",
    space: "Space", spacebar: "Space", tab: "Tab", backspace: "Backspace",
    delete: "Delete", del: "Delete", insert: "Insert",
    home: "Home", end: "End", pageup: "PageUp", pagedown: "PageDown",
    up: "ArrowUp", down: "ArrowDown", left: "ArrowLeft", right: "ArrowRight",
    arrowup: "ArrowUp", arrowdown: "ArrowDown", arrowleft: "ArrowLeft", arrowright: "ArrowRight",
  };

  function modifierName(part) {
    switch (String(part).toLowerCase()) {
      case "ctrl": case "control":
      case "meta": case "cmd": case "command": case "win": case "super":
        return "ctrl";
      case "alt": case "option": case "opt":
        return "alt";
      case "shift":
        return "shift";
      default:
        return null;
    }
  }

  function canonicalKey(raw) {
    if (raw === " ") return "Space";
    const key = String(raw == null ? "" : raw).trim();
    if (!key) return "";
    const lower = key.toLowerCase();
    if (Object.prototype.hasOwnProperty.call(NAMED_KEYS, lower)) return NAMED_KEYS[lower];
    if (/^f([1-9]|1[0-2])$/i.test(key)) return key.toUpperCase();
    return key.length === 1 ? key.toUpperCase() : key;
  }

  // parseChord normalizes any accepted spelling to a canonical chord object, or
  // returns null when nothing usable is left. A chord needs a key: modifiers
  // alone cannot be bound.
  function parseChord(value) {
    if (value == null) return null;
    const chord = { key: "", ctrl: false, alt: false, shift: false };
    if (typeof value === "object") {
      chord.key = canonicalKey(value.key);
      chord.ctrl = !!value.ctrl;
      chord.alt = !!value.alt;
      chord.shift = !!value.shift;
      return chord.key ? chord : null;
    }
    const text = String(value).trim();
    if (!text) return null;
    for (const part of text.split("+")) {
      const token = part.trim();
      if (!token) continue;
      const modifier = modifierName(token);
      if (modifier) chord[modifier] = true;
      else chord.key = canonicalKey(token);
    }
    return chord.key ? chord : null;
  }

  function formatChord(value) {
    const chord = parseChord(value);
    if (!chord) return "";
    const parts = [];
    if (chord.ctrl) parts.push("Ctrl");
    if (chord.alt) parts.push("Alt");
    if (chord.shift) parts.push("Shift");
    parts.push(chord.key);
    return parts.join("+");
  }

  // chordFromEvent reads a KeyboardEvent without touching the event's other
  // shape, so the same function works in the browser and in a test literal.
  function chordFromEvent(event) {
    if (!event || event.key == null) return null;
    const key = canonicalKey(event.key);
    if (!key) return null;
    return {
      key,
      ctrl: !!(event.ctrlKey || event.metaKey),
      alt: !!event.altKey,
      shift: !!event.shiftKey,
    };
  }

  function eventMatchesChord(event, binding) {
    const target = parseChord(binding);
    if (!target) return false;
    const actual = chordFromEvent(event);
    if (!actual) return false;
    return formatChord(target) === formatChord(actual);
  }

  // findConflicts reports every chord claimed by more than one action, sorted by
  // chord so the warning is stable between renders. Unbound entries are ignored.
  function findConflicts(bindings) {
    const byChord = new Map();
    for (const id of Object.keys(bindings || {})) {
      const chord = formatChord(bindings[id]);
      if (!chord) continue;
      if (!byChord.has(chord)) byChord.set(chord, []);
      byChord.get(chord).push(id);
    }
    const conflicts = [];
    for (const [chord, actions] of byChord) {
      if (actions.length > 1) conflicts.push({ chord, actions });
    }
    conflicts.sort((left, right) => (left.chord < right.chord ? -1 : left.chord > right.chord ? 1 : 0));
    return conflicts;
  }

  // nextUnresolved picks the next entry of `active` in `direction` (+1 / -1)
  // whose value the `isResolved` predicate has not claimed, wrapping once. It
  // returns null when every entry is resolved or the list is empty.
  //
  // `active` is the ordered list of positions that can be visited (a conflict
  // index for a three-way result, a hunk index for a two-way diff). `current`
  // does not have to be in the list; an unknown position starts the search at
  // the beginning (forward) or end (backward). Auto-advance calls it with the
  // position that was just adopted, which is now resolved, so the first match is
  // the next unresolved one.
  function nextUnresolved(active, current, isResolved, direction) {
    const list = Array.isArray(active) ? active : [];
    if (!list.length) return null;
    const step = direction < 0 ? -1 : 1;
    const resolved = typeof isResolved === "function" ? isResolved : function () { return false; };
    let start = list.indexOf(current);
    if (start < 0) start = step < 0 ? list.length : -1;
    for (let count = 1; count <= list.length; count++) {
      const pos = (((start + step * count) % list.length) + list.length) % list.length;
      const value = list[pos];
      if (!resolved(value)) return value;
    }
    return null;
  }

  return {
    ACTION_IDS,
    DEFAULT_BINDINGS,
    canonicalKey,
    parseChord,
    formatChord,
    chordFromEvent,
    eventMatchesChord,
    findConflicts,
    nextUnresolved,
  };
});
