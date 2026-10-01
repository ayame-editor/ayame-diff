"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
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
} = require("../tabs.js");

function sampleState(mode = "text", old = "/tmp/left.txt", right = "/tmp/right.txt") {
  return {
    v: 1,
    mode,
    paths: { base: "", old, new: right },
    controls: { ignoreCase: true, whitespace: "change" },
    syncPoints: [],
  };
}

test("an empty document has no tabs and no active tab", () => {
  const doc = emptyDoc();
  assert.deepEqual(doc, { v: VERSION, active: 0, tabs: [] });
  assert.equal(activeTab(doc), null);
});

test("adding a tab makes it active and generates a unique id", () => {
  let doc = addTab(emptyDoc(), { state: sampleState() });
  assert.equal(doc.tabs.length, 1);
  assert.equal(doc.active, 0);
  assert.equal(activeTab(doc).id, "tab-1");

  doc = addTab(doc, { state: sampleState("csv", "/a/one.csv", "/a/two.csv") });
  assert.equal(doc.tabs.length, 2);
  assert.equal(doc.active, 1, "the new tab becomes active");
  assert.notEqual(doc.tabs[0].id, doc.tabs[1].id);
});

test("adding without activation leaves the current tab in front", () => {
  let doc = addTab(emptyDoc(), { state: sampleState() });
  doc = addTab(doc, { state: sampleState("csv", "/a/1.csv", "/a/2.csv") });
  doc = activateTab(doc, doc.tabs[0].id);
  doc = addTab(doc, { state: sampleState("sorted", "/a/a.txt", "/a/b.txt") }, { activate: false });
  assert.equal(doc.active, 0);
  assert.equal(doc.tabs.length, 3);
});

test("closing the active tab activates its right neighbour, then the left at the end", () => {
  let doc = emptyDoc();
  doc = addTab(doc, { id: "a", state: sampleState() });
  doc = addTab(doc, { id: "b", state: sampleState() });
  doc = addTab(doc, { id: "c", state: sampleState() });
  doc = activateTab(doc, "b");

  const middle = removeTab(doc, "b");
  assert.deepEqual(middle.tabs.map((tab) => tab.id), ["a", "c"]);
  assert.equal(activeTab(middle).id, "c", "the right neighbour takes over");

  const end = removeTab(middle, "c");
  assert.equal(activeTab(end).id, "a", "with no right neighbour the left one does");
});

test("closing a tab before the active one keeps the same comparison active", () => {
  let doc = emptyDoc();
  doc = addTab(doc, { id: "a", state: sampleState() });
  doc = addTab(doc, { id: "b", state: sampleState() });
  doc = addTab(doc, { id: "c", state: sampleState() });
  doc = activateTab(doc, "c");
  const next = removeTab(doc, "a");
  assert.equal(activeTab(next).id, "c");
  assert.equal(next.active, 1);
});

test("closing the last tab leaves an empty document", () => {
  const doc = addTab(emptyDoc(), { id: "only", state: sampleState() });
  assert.deepEqual(removeTab(doc, "only"), emptyDoc());
});

test("updating a tab state without a label relabels it from the paths", () => {
  let doc = addTab(emptyDoc(), { id: "a", state: sampleState("text", "/tmp/left.txt", "/tmp/right.txt") });
  doc = updateTab(doc, "a", { state: sampleState("text", "/tmp/renamed.txt", "/tmp/right.txt"), scroll: null });
  assert.match(activeTab(doc).label, /renamed\.txt/);
  assert.doesNotMatch(activeTab(doc).label, /left\.txt/);
});

test("labels use the two file names and truncate long ones", () => {
  assert.equal(labelFromState(sampleState("text", "/dir/a.txt", "/dir/b.txt")), "a.txt ⇄ b.txt");
  assert.equal(labelFromState(sampleState("text", "C:\\work\\same.txt", "C:\\work\\same.txt")), "same.txt");
  assert.equal(labelFromState(sampleState("text", "C:\\work\\a.txt", "D:/other/b.txt")), "a.txt ⇄ b.txt");
  assert.equal(labelFromState(null, "New tab"), "New tab");

  const long = labelFromState(sampleState("text", `/a/${"x".repeat(80)}.txt`, "/a/short.txt"));
  assert.equal(long.length, MAX_LABEL);
  assert.ok(long.endsWith("…"));
  assert.equal(truncateLabel("short", 48), "short");
});

test("a tab round-trips through serialize/parse with Unicode and a scroll anchor", () => {
  const state = sampleState("threeway-csv", "C:\\比較\\基準.csv", "/tmp/現在.csv");
  const scroll = { group: "row", key: "42", order: 42, offset: -3.5 };
  let doc = addTab(emptyDoc(), { id: "tab-1", state, scroll, label: "基準.csv ⇄ 現在.csv" });
  doc = addTab(doc, { id: "tab-2", state: sampleState(), scroll: null }, { activate: false });

  const restored = parseDoc(JSON.parse(JSON.stringify(serializeDoc(doc))));
  assert.deepEqual(restored, serializeDoc(doc));
  assert.deepEqual(activeTab(restored).scroll, scroll);
  assert.equal(activeTab(restored).label, "基準.csv ⇄ 現在.csv");
});

test("parse rejects a wrong version, malformed shapes, and duplicate ids", () => {
  assert.equal(parseDoc(null), null);
  assert.equal(parseDoc("nope"), null);
  assert.equal(parseDoc([]), null);
  assert.equal(parseDoc({ v: VERSION + 1, tabs: [] }), null);
  assert.equal(parseDoc({ v: VERSION }), null);

  const deduped = parseDoc({
    v: VERSION,
    active: 5,
    tabs: [
      { id: "a", state: sampleState(), scroll: { group: "row", key: "1", order: 1, offset: 0 } },
      { id: "a", state: sampleState() },
      { id: "b", state: "not an object" },
    ],
  });
  assert.deepEqual(deduped.tabs.map((tab) => tab.id), ["a", "b"]);
  assert.equal(deduped.active, 1, "an out-of-range active index is clamped");

  const noScroll = parseDoc({ v: VERSION, active: 0, tabs: [{ id: "a", state: sampleState(), scroll: { group: "row" } }] });
  assert.equal(activeTab(noScroll).scroll, null, "a partial anchor is dropped");
});

test("an unknown id leaves the document untouched", () => {
  const doc = addTab(emptyDoc(), { id: "a", state: sampleState() });
  assert.equal(activateTab(doc, "missing"), doc);
  assert.equal(removeTab(doc, "missing"), doc);
  assert.equal(updateTab(doc, "missing", { label: "x" }), doc);
});

test("nextId does not collide with existing ids", () => {
  let doc = emptyDoc();
  doc = addTab(doc, { id: "tab-1", state: sampleState() });
  doc = addTab(doc, { id: "tab-3", state: sampleState() }, { activate: false });
  const id = nextId(doc);
  assert.ok(!doc.tabs.some((tab) => tab.id === id));
  assert.equal(id, "tab-4", "the naive next number is taken, so it moves on");

  // Auto-generated ids stay unique as tabs accumulate.
  let auto = emptyDoc();
  for (let i = 0; i < 5; i++) auto = addTab(auto, { state: sampleState() });
  assert.equal(new Set(auto.tabs.map((tab) => tab.id)).size, 5);
});
