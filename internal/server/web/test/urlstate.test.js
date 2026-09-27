"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  VERSION,
  HASH_KEY,
  TABS_HASH_KEY,
  MAX_ENCODED_LENGTH,
  encodeComparisonState,
  decodeComparisonState,
  readComparisonState,
  readTabState,
  buildComparisonURL,
  buildTabStateURL,
  buildShareURL,
} = require("../urlstate.js");

function sampleState() {
  return {
    v: VERSION,
    mode: "threeway-csv",
    paths: {
      base: "C:\\比較\\基準.csv",
      old: "/tmp/以前.csv",
      new: "/tmp/現在.csv",
    },
    controls: {
      ignoreCase: true,
      whitespace: "change",
      lineFilters: "^generated,\n一時$",
      maxHunks: "400",
    },
    csvKeys: [{ name: "顧客ID", index: 0 }],
    syncPoints: [{ old: 3, new: 4 }],
  };
}

test("round-trips versioned Unicode comparison state", () => {
  const state = sampleState();
  assert.deepEqual(decodeComparisonState(encodeComparisonState(state)), state);
});

test("history URL keeps the token but removes legacy launch parameters", () => {
  const url = buildComparisonURL(
    "http://127.0.0.1:9000/?token=secret&old=legacy&new=legacy&mode=text&autorun=1&debug=1",
    sampleState(),
  );
  const parsed = new URL(url);
  assert.equal(parsed.searchParams.get("token"), "secret");
  assert.equal(parsed.searchParams.get("debug"), "1");
  for (const name of ["old", "new", "base", "mode", "autorun"]) {
    assert.equal(parsed.searchParams.has(name), false);
  }
  assert.deepEqual(readComparisonState(url), sampleState());
});

test("shared URL excludes the API token without losing comparison state", () => {
  const url = buildShareURL("http://127.0.0.1:9000/?token=do-not-share", sampleState());
  assert.equal(new URL(url).searchParams.has("token"), false);
  assert.deepEqual(readComparisonState(url), sampleState());
});

test("rejects malformed, unsupported, and oversized state", () => {
  assert.equal(decodeComparisonState("not+base64"), null);
  const encodedUnsupported = Buffer.from(JSON.stringify({
    ...sampleState(),
    v: VERSION + 1,
  })).toString("base64url");
  assert.equal(decodeComparisonState(encodedUnsupported), null);
  assert.throws(
    () => encodeComparisonState({ ...sampleState(), controls: { huge: "x".repeat(MAX_ENCODED_LENGTH) } }),
    (error) => error instanceof RangeError && error.code === "STATE_TOO_LARGE",
  );
});

test("the tab set and the active comparison coexist in one fragment", () => {
  const tabs = { v: 1, active: 1, tabs: [{ id: "tab-1", label: "a.txt ⇄ b.txt", state: sampleState(), scroll: null }] };
  const withTabs = buildTabStateURL("http://127.0.0.1:9000/?token=secret#", tabs);
  const withComparison = buildComparisonURL(withTabs, sampleState(), true);

  const parsed = new URL(withComparison);
  assert.equal(parsed.searchParams.get("token"), "secret");
  assert.deepEqual(readComparisonState(withComparison), sampleState());
  assert.deepEqual(readTabState(withComparison), tabs);
  assert.ok(parsed.hash.includes(`${TABS_HASH_KEY}=`));
  assert.ok(parsed.hash.includes(`${HASH_KEY}=`));
});

test("building the tab set preserves an existing comparison fragment", () => {
  const comparison = buildComparisonURL("http://127.0.0.1:9000/#", sampleState(), true);
  const both = buildTabStateURL(comparison, { v: 1, active: 0, tabs: [] }, true);
  assert.deepEqual(readComparisonState(both), sampleState());
  assert.deepEqual(readTabState(both), { v: 1, active: 0, tabs: [] });
});

test("clearing the tab set drops only that key", () => {
  const both = buildTabStateURL(
    buildComparisonURL("http://127.0.0.1:9000/#", sampleState(), true),
    { v: 1, active: 0, tabs: [{ id: "tab-1", state: sampleState(), scroll: null }] },
    true,
  );
  const cleared = buildTabStateURL(both, null, true);
  assert.equal(readTabState(cleared), null);
  assert.deepEqual(readComparisonState(cleared), sampleState());
});

test("a shared URL carries one comparison and never the open tab set", () => {
  const tabs = { v: 1, active: 0, tabs: [{ id: "tab-1", state: sampleState(), scroll: null }] };
  const start = buildTabStateURL(
    "http://127.0.0.1:9000/?token=do-not-share",
    tabs,
  );
  const shared = buildShareURL(start, sampleState());
  const parsed = new URL(shared);
  assert.equal(parsed.searchParams.has("token"), false);
  assert.equal(readTabState(shared), null);
  assert.deepEqual(readComparisonState(shared), sampleState());
});

test("an oversized tab set is reported rather than silently truncated", () => {
  const huge = { v: 1, active: 0, tabs: [{ id: "tab-1", state: { controls: { blob: "x".repeat(MAX_ENCODED_LENGTH) } }, scroll: null }] };
  assert.throws(
    () => buildTabStateURL("http://127.0.0.1:9000/#", huge),
    (error) => error instanceof RangeError && error.code === "STATE_TOO_LARGE",
  );
});
