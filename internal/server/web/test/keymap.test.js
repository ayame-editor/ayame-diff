"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  ACTION_IDS,
  DEFAULT_BINDINGS,
  PRESETS,
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
} = require("../keymap.js");

test("parseChord normalizes spelling and modifier aliases", () => {
  assert.deepEqual(parseChord("Alt+ArrowDown"), { key: "ArrowDown", ctrl: false, alt: true, shift: false });
  assert.deepEqual(parseChord("ctrl + f"), { key: "F", ctrl: true, alt: false, shift: false });
  assert.deepEqual(parseChord("Cmd+Enter"), { key: "Enter", ctrl: true, alt: false, shift: false });
  assert.deepEqual(parseChord("down"), { key: "ArrowDown", ctrl: false, alt: false, shift: false });
  assert.equal(parseChord("Alt"), null, "modifiers alone cannot be bound");
  assert.equal(parseChord(""), null);
  assert.equal(parseChord(null), null);
});

test("formatChord emits one canonical spelling", () => {
  assert.equal(formatChord("alt+ArrowDown"), "Alt+ArrowDown");
  assert.equal(formatChord("Meta+F"), "Ctrl+F", "Command folds into Ctrl");
  assert.equal(formatChord("Shift+Enter"), "Shift+Enter");
  assert.equal(formatChord("ctrl+alt+shift+b"), "Ctrl+Alt+Shift+B");
  assert.equal(formatChord("nonsense"), "nonsense", "unknown named keys are left as written");
  assert.equal(formatChord("Alt"), "");
});

test("chordFromEvent folds Command into Ctrl", () => {
  assert.deepEqual(chordFromEvent({ key: "f", metaKey: true }), { key: "F", ctrl: true, alt: false, shift: false });
  assert.equal(chordFromEvent({ key: "Dead" }).key, "Dead");
  assert.equal(chordFromEvent(null), null);
});

test("eventMatchesChord requires the exact modifier set", () => {
  const base = { key: "ArrowDown", altKey: true };
  assert.equal(eventMatchesChord(base, "Alt+ArrowDown"), true);
  assert.equal(eventMatchesChord({ key: "b", altKey: true }, "Alt+B"), true, "letters match case-insensitively");
  assert.equal(eventMatchesChord({ key: "b", ctrlKey: true }, "Ctrl+B"), true);
  assert.equal(eventMatchesChord({ key: "b", metaKey: true }, "Ctrl+B"), true, "Command satisfies Ctrl");
  assert.equal(eventMatchesChord({ key: "B", shiftKey: true, metaKey: true }, "Ctrl+B"), false, "an extra Shift must not match");
  assert.equal(eventMatchesChord({ key: "ArrowDown" }, "Alt+ArrowDown"), false);
  assert.equal(eventMatchesChord({ key: "ArrowDown", altKey: true }, null), false, "an unbound action never matches");
});

test("the default map binds every action without a collision", () => {
  assert.deepEqual(ACTION_IDS, Object.keys(DEFAULT_BINDINGS), "every documented action has a default key");
  for (const id of ACTION_IDS) assert.ok(DEFAULT_BINDINGS[id], `${id} is bound`);
  assert.deepEqual(findConflicts(DEFAULT_BINDINGS), [], "no default chord is claimed twice");
  assert.notEqual(DEFAULT_BINDINGS.navigateNext, DEFAULT_BINDINGS.nextConflict, "difference and conflict navigation are separate keys");
});

test("the keymap keeps the WinMerge navigation and side keys", () => {
  assert.equal(DEFAULT_BINDINGS.navigateNext, "Alt+ArrowDown");
  assert.equal(DEFAULT_BINDINGS.navigatePrev, "Alt+ArrowUp");
  assert.equal(DEFAULT_BINDINGS.firstDiff, "Alt+Home");
  assert.equal(DEFAULT_BINDINGS.lastDiff, "Alt+End");
  assert.equal(DEFAULT_BINDINGS.chooseLeft, "Alt+ArrowLeft");
  assert.equal(DEFAULT_BINDINGS.chooseRight, "Alt+ArrowRight");
  assert.equal(DEFAULT_BINDINGS.chooseBase, "Alt+B");
});

test("nextUnresolved walks forward, backward and wraps", () => {
  const active = [0, 2, 5, 7];
  assert.equal(nextUnresolved(active, 0, (i) => i === 2, 1), 5, "skips a resolved entry");
  assert.equal(nextUnresolved(active, 7, () => false, 1), 0, "wraps forward");
  assert.equal(nextUnresolved(active, 0, () => false, -1), 7, "wraps backward");
  assert.equal(nextUnresolved(active, 5, (i) => i === 2, -1), 0, "skips a resolved entry while walking backward");
  assert.equal(nextUnresolved(active, 5, () => false, -1), 2, "the current entry is not returned");
});

test("nextUnresolved handles an unknown current position and no candidates", () => {
  assert.equal(nextUnresolved([3, 6], 99, () => false, 1), 3, "an unknown position starts forward at the first entry");
  assert.equal(nextUnresolved([3, 6], 99, () => false, -1), 6, "an unknown position starts backward at the last entry");
  assert.equal(nextUnresolved([3, 6], 3, () => true, 1), null, "all resolved means no advance");
  assert.equal(nextUnresolved([], 0, () => false, 1), null, "an empty list never advances");
  assert.equal(nextUnresolved(null, 0, () => false, 1), null);
});

test("findConflicts names both actions and ignores unbound entries", () => {
  assert.deepEqual(findConflicts({
    navigateNext: "Alt+ArrowDown",
    navigatePrev: "Alt+ArrowDown",
    search: "Ctrl+F",
    close: null,
  }), [{ chord: "Alt+ArrowDown", actions: ["navigateNext", "navigatePrev"] }]);
  assert.deepEqual(findConflicts({ a: "Ctrl+F", b: "Meta+F" }), [{ chord: "Ctrl+F", actions: ["a", "b"] }], "Ctrl and Command are one chord");
  assert.deepEqual(findConflicts({ a: null, b: "" }), []);
});

test("isReservedChord flags browser shortcuts only", () => {
  assert.equal(isReservedChord("Ctrl+W"), true);
  assert.equal(isReservedChord("Cmd+T"), true);
  assert.equal(isReservedChord("F11"), true);
  assert.equal(isReservedChord("Ctrl+F"), false);
  assert.equal(isReservedChord("Ctrl+S"), false, "the app owns Ctrl+S");
  assert.equal(isReservedChord(null), false);
});

test("mergeBindings applies overrides over the preset and fills unbound actions", () => {
  const merged = mergeBindings(ACTION_IDS, PRESETS.default, {});
  assert.deepEqual(merged, DEFAULT_BINDINGS);

  const minimal = mergeBindings(ACTION_IDS, PRESETS.minimal, {});
  assert.equal(minimal.navigateNext, "Alt+ArrowDown");
  assert.equal(minimal.close, "Escape");
  assert.equal(minimal.compare, null, "an action the preset omits is unbound");
  assert.equal(Object.keys(minimal).length, ACTION_IDS.length);

  const overridden = mergeBindings(ACTION_IDS, PRESETS.default, { search: "Ctrl+G", close: null });
  assert.equal(overridden.search, "Ctrl+G");
  assert.equal(overridden.close, null, "an explicit null unbinds a preset action");
  assert.equal(overridden.navigateNext, "Alt+ArrowDown", "untouched actions keep the preset");

  const normalized = mergeBindings(["search"], {}, { search: "meta + g" });
  assert.equal(normalized.search, "Ctrl+G");
});

test("serializeBindings round-trips through parseBindings", () => {
  const text = serializeBindings({ preset: "minimal", overrides: { search: "Ctrl+G", close: null } });
  assert.deepEqual(parseBindings(text), { preset: "minimal", overrides: { search: "Ctrl+G", close: null } });
  assert.deepEqual(parseBindings("{}"), { preset: "default", overrides: {} });
});
