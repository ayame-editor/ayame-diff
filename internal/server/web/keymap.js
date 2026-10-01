// Keyboard shortcut bindings and the default map (#285, #277).
//
// The chord syntax, the presets, the conflict rules and the merge-flow advance
// rules live here so they can be exercised under node --test without a DOM;
// app.js owns what each action does and where the choice is persisted.
//
// #285 provides the remapping mechanism: presets, per-action overrides and the
// storage format, resolved through mergeBindings. #277 provides the default map
// itself, including the merge-flow actions (conflict navigation, side adoption,
// auto-advance and save), which stay part of that default map and resolve through
// the same preset/override layer.
//
// A "chord" is one physical key press: zero or more modifiers plus a key,
// written canonically as "Ctrl+Alt+Shift+Key". Command (Meta) is folded into
// Ctrl because the application has always accepted either for the shortcuts
// that predate this table; keeping one primary modifier avoids an ambiguous
// dispatch when both are bound.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameKeymap = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // The actions the help dialog and the dispatcher share. The order is the order
  // the help lists them, so it reads as the merge flow: navigate the result,
  // navigate conflicts, choose a side, then save. Keeping the list here means
  // mergeBindings can fill in an unbound entry for every action, so a preset that
  // omits one disables it rather than leaving a stale chord.
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
    "revertLine",
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
    revertLine: "Delete",
  };

  // "minimal" keeps only the actions a reader needs to move and search; every
  // other action is intentionally unbound. The default preset is the merge-flow
  // map above.
  const PRESETS = {
    default: DEFAULT_BINDINGS,
    minimal: {
      navigateNext: "Alt+ArrowDown",
      navigatePrev: "Alt+ArrowUp",
      search: "Ctrl+F",
      close: "Escape",
    },
  };

  const MODIFIER_ORDER = ["Ctrl", "Alt", "Shift"];

  // "down", "ArrowDown" and the like all name the same physical key.
  const NAMED_KEYS = {
    escape: "Escape", esc: "Escape", enter: "Enter", return: "Enter",
    space: "Space", spacebar: "Space", tab: "Tab", backspace: "Backspace",
    delete: "Delete", del: "Delete", insert: "Insert",
    home: "Home", end: "End", pageup: "PageUp", pagedown: "PageDown",
    up: "ArrowUp", down: "ArrowDown", left: "ArrowLeft", right: "ArrowRight",
    arrowup: "ArrowUp", arrowdown: "ArrowDown", arrowleft: "ArrowLeft", arrowright: "ArrowRight",
  };

  // Keys the browser reserves. They are rejected instead of silently swallowed.
  const RESERVED_CHORDS = [
    "Ctrl+W", "Ctrl+Shift+W", "Ctrl+T", "Ctrl+Shift+T", "Ctrl+N", "Ctrl+Shift+N",
    "Ctrl+Tab", "Ctrl+Shift+Tab", "Ctrl+L", "Ctrl+R", "Ctrl+Shift+R", "Ctrl+P",
    "Ctrl+J", "Ctrl+H", "Ctrl+D", "Ctrl+Q",
    "F5", "F11", "F12",
  ].map(formatChord);

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

  function canonicalChord(source) {
    const chord = { key: "", ctrl: false, alt: false, shift: false };
    if (!source || typeof source !== "object") return chord;
    chord.key = canonicalKey(source.key);
    chord.ctrl = !!source.ctrl;
    chord.alt = !!source.alt;
    chord.shift = !!source.shift;
    return chord;
  }

  // parseChord normalizes any accepted spelling to a canonical chord object, or
  // returns null when nothing usable is left. A chord needs a key: modifiers
  // alone cannot be bound.
  function parseChord(value) {
    if (value == null) return null;
    if (typeof value === "object") {
      const chord = canonicalChord(value);
      return chord.key ? chord : null;
    }
    const text = String(value).trim();
    if (!text) return null;
    const chord = { key: "", ctrl: false, alt: false, shift: false };
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
    for (const name of MODIFIER_ORDER) if (chord[name.toLowerCase()]) parts.push(name);
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

  function isReservedChord(binding) {
    const chord = formatChord(binding);
    return !!chord && RESERVED_CHORDS.includes(chord);
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

  function normalizeValue(value) {
    return value == null || value === "" ? null : (formatChord(value) || null);
  }

  // mergeBindings resolves a preset plus user overrides against the action list.
  // An override is authoritative even when null, which is how a user unbinds an
  // action a preset enabled. Actions present in neither come back null (unbound)
  // so callers always get one entry per action.
  function mergeBindings(actions, preset, overrides) {
    const ids = (Array.isArray(actions) ? actions : []).map((action) => (typeof action === "string" ? action : action && action.id));
    const base = preset && typeof preset === "object" ? preset : {};
    const custom = overrides && typeof overrides === "object" ? overrides : {};
    const result = {};
    for (const id of ids) {
      if (!id) continue;
      if (Object.prototype.hasOwnProperty.call(custom, id)) result[id] = normalizeValue(custom[id]);
      else if (Object.prototype.hasOwnProperty.call(base, id)) result[id] = normalizeValue(base[id]);
      else result[id] = null;
    }
    return result;
  }

  // Export/import are a plain { preset, overrides } document so a team can share
  // one. The action list is not embedded: an older export stays readable after
  // new actions are added, which simply fall back to the preset.
  function serializeBindings(state) {
    const preset = state && typeof state.preset === "string" ? state.preset : "default";
    const overrides = state && state.overrides && typeof state.overrides === "object" ? state.overrides : {};
    return JSON.stringify({ version: 1, preset, overrides }, null, 2);
  }

  function parseBindings(text) {
    const data = JSON.parse(String(text));
    if (!data || typeof data !== "object") return { preset: "default", overrides: {} };
    const preset = typeof data.preset === "string" && data.preset ? data.preset : "default";
    const overrides = data.overrides && typeof data.overrides === "object" ? data.overrides : {};
    return { preset, overrides };
  }

  return {
    ACTION_IDS,
    DEFAULT_BINDINGS,
    PRESETS,
    RESERVED_CHORDS,
    canonicalKey,
    parseChord,
    formatChord,
    chordFromEvent,
    eventMatchesChord,
    isReservedChord,
    findConflicts,
    mergeBindings,
    serializeBindings,
    parseBindings,
    nextUnresolved,
  };
});
