"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const {
  VERSION,
  MAX_ENCODED_LENGTH,
  encodeComparisonState,
  decodeComparisonState,
  readComparisonState,
  buildComparisonURL,
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
