"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { apiErrorKey, KEY_BY_CODE } = require("../apierrors.js");

test("a server code decides the message", () => {
  assert.equal(apiErrorKey("file_not_found", 404), "errFileNotFound");
  assert.equal(apiErrorKey("permission_denied", 403), "errPermissionDenied");
  assert.equal(apiErrorKey("overwrite_refused", 400), "errOverwriteRefused");
  assert.equal(apiErrorKey("timeout", 408), "errTimeout");
  assert.equal(apiErrorKey("busy", 429), "errBusy");
  assert.equal(apiErrorKey("internal", 500), "errInternal");
});

test("the code wins over a status that would say something else", () => {
  assert.equal(apiErrorKey("overwrite_refused", 400), "errOverwriteRefused");
  assert.equal(apiErrorKey("unsupported_input", 400), "errUnsupportedInput");
});

test("an unclassified failure falls back to its status", () => {
  assert.equal(apiErrorKey("", 401), "errUnauthorized");
  assert.equal(apiErrorKey(undefined, 404), "errFileNotFound");
  assert.equal(apiErrorKey(null, 429), "errBusy");
  assert.equal(apiErrorKey("", 503), "errInternal");
  assert.equal(apiErrorKey("", 500), "errInternal");
});

test("an unknown failure keeps the server's own words", () => {
  assert.equal(apiErrorKey("", 400), "");
  assert.equal(apiErrorKey("something_new", 400), "");
  assert.equal(apiErrorKey("", 0), "");
});

test("every code maps to a distinct key", () => {
  const keys = Object.values(KEY_BY_CODE);
  assert.equal(new Set(keys).size, keys.length);
});
