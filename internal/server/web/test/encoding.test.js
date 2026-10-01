"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const {
  ENCODING_OPTIONS,
  JAPANESE_LEGACY,
  isSupported,
  isJapaneseLegacy,
  encodingMismatch,
  encodingCandidates,
  encodingPickerValue,
} = require("../encoding.js");

test("options match the engine dropdown in index.html", () => {
  // The page and the engine offer the same set: read the markup rather than
  // trusting a second hardcoded list. Go separately checks the markup against
  // encoding.Supported, so the three stay in step.
  const html = fs.readFileSync(path.join(__dirname, "..", "index.html"), "utf8");
  const block = html.slice(html.indexOf('<select id="encoding">'), html.indexOf("</select>", html.indexOf('<select id="encoding">')));
  const values = [...block.matchAll(/<option value="([^"]+)"/g)].map((match) => match[1]);
  assert.deepEqual(values, ENCODING_OPTIONS);
});

test("encodingMismatch only fires when both sides differ", () => {
  assert.equal(encodingMismatch("shift_jis", "utf-8"), true);
  assert.equal(encodingMismatch("shift_jis", "shift_jis"), false);
  assert.equal(encodingMismatch("shift_jis", ""), false);
  assert.equal(encodingMismatch("", "utf-8"), false);
  assert.equal(encodingMismatch("", ""), false);
});

test("candidates are every other concrete encoding, in menu order", () => {
  assert.deepEqual(encodingCandidates("shift_jis"), [
    "utf-8",
    "utf-16le",
    "utf-16be",
    "euc-jp",
    "iso-2022-jp",
  ]);
  // Never offer "auto" as a correction, and never offer the current one.
  assert.ok(!encodingCandidates("euc-jp").includes("euc-jp"));
  assert.ok(!encodingCandidates("euc-jp").includes("auto"));
  // An unknown or absent detection falls back to listing everything concrete.
  assert.deepEqual(encodingCandidates(""), encodingCandidates("auto"));
});

test("picker value is the explicit override or auto", () => {
  assert.equal(encodingPickerValue(""), "auto");
  assert.equal(encodingPickerValue("auto"), "auto");
  assert.equal(encodingPickerValue("euc-jp"), "euc-jp");
  assert.equal(encodingPickerValue("nonsense"), "auto");
});

test("supported and legacy sets are closed vocabularies", () => {
  assert.equal(isSupported("iso-2022-jp"), true);
  assert.equal(isSupported("cp932"), false);
  assert.equal(isJapaneseLegacy("shift_jis"), true);
  assert.equal(isJapaneseLegacy("utf-8"), false);
  assert.deepEqual(JAPANESE_LEGACY, ["shift_jis", "euc-jp", "iso-2022-jp"]);
});
