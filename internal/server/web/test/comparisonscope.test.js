"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const scope = require("../comparisonscope.js");

test("separates display preferences from per-comparison controls", () => {
  const globals = scope.GLOBAL_PREFERENCES.map((preference) => preference.id);
  // The one setting #260 added to the global set.
  assert.ok(globals.includes("word"));
  assert.equal(scope.globalPreference("word").key, "ayame-word");
  assert.equal(scope.isGlobalPreference("word"), true);

  // The two classifications must not overlap: a control is either the reader's
  // preference or the comparison's condition, never both.
  for (const id of scope.COMPARISON_CONTROLS) {
    assert.equal(scope.isGlobalPreference(id), false, `${id} is in both classifications`);
    assert.equal(scope.isComparisonControl(id), true);
  }
  assert.equal(scope.isComparisonControl("wrap"), false);
  assert.equal(scope.isComparisonControl("word"), false);
});

test("builds a stable identity from the mode and trimmed input paths", () => {
  const paths = { base: "", old: " /tmp/old.txt ", new: "/tmp/new.txt" };
  const key = scope.comparisonKey("text", paths);
  assert.equal(key, scope.comparisonKey("text", { old: "/tmp/old.txt", new: "/tmp/new.txt" }));
  assert.notEqual(key, scope.comparisonKey("sorted", paths));
  assert.notEqual(key, scope.comparisonKey("text", { old: "/tmp/old.txt", new: "/tmp/other.txt" }));

  // Incomplete comparisons have no scope at all.
  assert.equal(scope.comparisonKey("text", { old: "/tmp/old.txt", new: "" }), "");
  assert.equal(scope.comparisonKey("threeway", { old: "/a", new: "/b" }), "");
  assert.ok(scope.comparisonKey("threeway", { base: "/base", old: "/a", new: "/b" }));
  assert.equal(scope.comparisonKey("", paths), "");
});

test("snapshots only the per-comparison controls", () => {
  const snapshot = scope.conditionSnapshot({
    mode: "text",
    old: "/tmp/old.txt",
    wrap: false,
    word: false,
    ignoreCase: true,
    whitespace: "all",
  });
  assert.deepEqual(snapshot, { ignoreCase: true, whitespace: "all" });
});

test("round-trips a bounded store and rejects malformed input", () => {
  let store = {};
  for (let index = 0; index < scope.STORE_LIMIT + 5; index++) {
    store = scope.writeConditions(store, `key-${index}`, { ignoreCase: index % 2 === 0 });
  }
  const parsed = scope.parseStore(scope.serializeStore(store));
  const keys = Object.keys(parsed);
  assert.equal(keys.length, scope.STORE_LIMIT);
  // The newest write is kept, the oldest has fallen out.
  assert.equal(keys[keys.length - 1], `key-${scope.STORE_LIMIT + 4}`);
  assert.equal(parsed[`key-${scope.STORE_LIMIT + 4}`].ignoreCase, true);

  assert.deepEqual(scope.parseStore("not json"), {});
  assert.deepEqual(scope.parseStore(JSON.stringify([1, 2, 3])), {});
  assert.deepEqual(scope.parseStore(JSON.stringify({ k: 5 })), {});
  assert.equal(scope.readConditions(parsed, "missing"), null);
});

test("stores a whole comparison state, not just its controls", () => {
  // app.js stores the same versioned state the URL fragment carries, so the
  // store must pass the extra fields (paths, csvKeys, syncPoints) through.
  const state = {
    v: 1,
    mode: "threeway-csv",
    paths: { base: "/b.csv", old: "/o.csv", new: "/n.csv" },
    controls: { ignoreCase: true, whitespace: "change" },
    csvKeys: [{ name: "顧客ID", index: 0 }],
    syncPoints: [{ old: 3, new: 4 }],
  };
  const store = scope.writeConditions({}, "identity", state);
  assert.deepEqual(scope.parseStore(scope.serializeStore(store)).identity, state);
  assert.deepEqual(scope.readConditions(store, "identity"), state);
});

test("readConditions returns an independent copy of a remembered comparison", () => {
  const store = scope.writeConditions({}, "k", { ignoreCase: true });
  const snapshot = scope.readConditions(store, "k");
  assert.deepEqual(snapshot, { ignoreCase: true });
  snapshot.ignoreCase = false;
  assert.equal(store.k.ignoreCase, true);
  // Rewriting a key moves it to the newest position.
  const rewritten = scope.writeConditions({ a: {}, b: {} }, "a", { ignoreCase: true });
  assert.deepEqual(Object.keys(rewritten), ["b", "a"]);
});
