"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
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
} = require("../conditionscope.js");

function factory() {
  return {
    ignoreCase: false,
    ignoreEOL: false,
    ignoreTrailingEOL: false,
    whitespace: "none",
    lineFilters: "",
    detectMoves: false,
  };
}

test("the stored default has its own key, apart from any session state", () => {
  assert.equal(STORAGE_KEY, "ayame-condition-defaults");
  assert.equal(CONDITION_CONTROLS.length, 6);
  assert.equal(controlById("whitespace").type, "select");
  assert.equal(controlById("nope"), null);
});

test("capture reads controls and apply writes them back", () => {
  const values = {
    ignoreCase: true,
    ignoreEOL: false,
    ignoreTrailingEOL: false,
    whitespace: "change",
    lineFilters: "^generated$",
    detectMoves: true,
  };
  const captured = capture((id) => values[id]);
  assert.deepEqual(captured, values);

  const written = {};
  apply(captured, (control, value) => { written[control.id] = value; });
  assert.deepEqual(written, values);
});

test("sanitize rejects impossible values and falls back per control", () => {
  const fallback = factory();
  const stored = {
    ignoreCase: true,
    whitespace: "sideways", // not one of the allowed options
    lineFilters: 42, // not a string
    detectMoves: "yes", // not a boolean
  };
  assert.deepEqual(sanitize(stored, fallback), {
    ignoreCase: true,
    ignoreEOL: false,
    ignoreTrailingEOL: false,
    whitespace: "none",
    lineFilters: "",
    detectMoves: false,
  });
  assert.equal(validValue(controlById("whitespace"), "all"), true);
  assert.equal(validValue(controlById("whitespace"), "ALL"), false);
});

test("changes reports which conditions leave the default", () => {
  const defaults = factory();
  assert.deepEqual(changes(defaults, defaults), []);
  assert.equal(isDefault(defaults, defaults), true);

  const tweaked = { ...defaults, ignoreCase: true, lineFilters: "TODO" };
  assert.deepEqual(changes(tweaked, defaults).sort(), ["ignoreCase", "lineFilters"]);
  assert.equal(isDefault(tweaked, defaults), false);
});

test("resolve applies session over the saved default over the factory default", () => {
  const factoryDefaults = factory();
  const saved = { ...factory(), whitespace: "change", ignoreCase: true };
  const session = { ignoreEOL: true };

  const effective = resolve(factoryDefaults, saved, session);
  assert.equal(effective.whitespace, "change", "saved default survived untouched by the session");
  assert.equal(effective.ignoreCase, true);
  assert.equal(effective.ignoreEOL, true, "session override won");
  assert.equal(effective.detectMoves, false, "factory value filled the rest");

  // No session layer: a fresh comparison gets the saved default verbatim.
  assert.deepEqual(resolve(factoryDefaults, saved, null), sanitize(saved, factoryDefaults));
  // No saved default: the factory default is used.
  assert.deepEqual(resolve(factoryDefaults, null, null), factoryDefaults);
});

test("promote snapshots a copy so later changes cannot mutate the default", () => {
  const defaults = factory();
  const current = { ...defaults, ignoreCase: true };
  const promoted = promote(current, defaults);
  current.ignoreCase = false;
  assert.equal(promoted.ignoreCase, true);
  assert.notEqual(promoted, current);
});

test("serialize and parse round-trip a default; bad input yields null", () => {
  const defaults = { ...factory(), whitespace: "all", lineFilters: "a\nb" };
  assert.deepEqual(parse(serialize(defaults), factory()), defaults);
  assert.equal(parse("", factory()), null);
  assert.equal(parse("{not json", factory()), null);
  assert.equal(parse("[]", factory()), null);
  assert.equal(parse("null", factory()), null);
  // A partial stored object is completed from the fallback, never left partial.
  assert.deepEqual(parse('{"ignoreCase":true}', factory()), {
    ...factory(),
    ignoreCase: true,
  });
});
