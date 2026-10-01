(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameURLState = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const VERSION = 1;
  const HASH_KEY = "compare";
  // The set of open comparison tabs (#281) rides alongside the active
  // comparison under its own fragment key, so the single-comparison key keeps
  // its exact old meaning and a reader without the tab code still restores.
  const TABS_HASH_KEY = "tabs";
  const MAX_ENCODED_LENGTH = 32 * 1024;
  const LEGACY_STATE_PARAMS = ["base", "old", "new", "mode", "autorun"];

  function bytesToBase64URL(bytes) {
    if (typeof Buffer !== "undefined") {
      return Buffer.from(bytes).toString("base64url");
    }
    let binary = "";
    for (let offset = 0; offset < bytes.length; offset += 0x8000) {
      binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
    }
    return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  }

  function base64URLToBytes(value) {
    if (!/^[A-Za-z0-9_-]+$/.test(value)) throw new Error("invalid comparison state encoding");
    if (typeof Buffer !== "undefined") return new Uint8Array(Buffer.from(value, "base64url"));
    const padded = value.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(value.length / 4) * 4, "=");
    const binary = atob(padded);
    return Uint8Array.from(binary, (char) => char.charCodeAt(0));
  }

  function validState(value) {
    return value && typeof value === "object" && !Array.isArray(value) &&
      value.v === VERSION &&
      typeof value.mode === "string" &&
      value.paths && typeof value.paths === "object" && !Array.isArray(value.paths) &&
      value.controls && typeof value.controls === "object" && !Array.isArray(value.controls);
  }

  function encodeValue(value) {
    const encoded = bytesToBase64URL(new TextEncoder().encode(JSON.stringify(value)));
    if (encoded.length > MAX_ENCODED_LENGTH) {
      const error = new RangeError("comparison state is too large for a reliable URL");
      error.code = "STATE_TOO_LARGE";
      throw error;
    }
    return encoded;
  }

  function decodeValue(encoded) {
    if (!encoded || encoded.length > MAX_ENCODED_LENGTH) return null;
    try {
      return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(base64URLToBytes(encoded)));
    } catch (_) {
      return null;
    }
  }

  function encodeComparisonState(state) {
    return encodeValue({ ...state, v: VERSION });
  }

  function decodeComparisonState(encoded) {
    const value = decodeValue(encoded);
    return validState(value) ? value : null;
  }

  function readComparisonState(urlValue) {
    const url = new URL(urlValue, "http://ayame.invalid/");
    const encoded = new URLSearchParams(url.hash.slice(1)).get(HASH_KEY);
    return decodeComparisonState(encoded);
  }

  // The raw tab document; tabs.js owns its shape and validation. A wrong or
  // damaged blob returns null and the caller falls back to the active
  // comparison alone.
  function readTabState(urlValue) {
    const url = new URL(urlValue, "http://ayame.invalid/");
    return decodeValue(new URLSearchParams(url.hash.slice(1)).get(TABS_HASH_KEY));
  }

  // buildTabStateURL preserves the active comparison fragment so the two
  // writers compose; only known keys are carried over.
  function buildTabStateURL(urlValue, doc, includeToken = true) {
    const url = new URL(urlValue, "http://ayame.invalid/");
    if (!includeToken) url.searchParams.delete("token");
    const current = new URLSearchParams(url.hash.slice(1));
    const params = new URLSearchParams();
    const comparison = current.get(HASH_KEY);
    if (comparison) params.set(HASH_KEY, comparison);
    if (doc) params.set(TABS_HASH_KEY, encodeValue({ ...doc, v: VERSION }));
    url.hash = params.toString();
    return url.toString();
  }

  function buildComparisonURL(urlValue, state, includeToken = true) {
    const url = new URL(urlValue, "http://ayame.invalid/");
    for (const name of LEGACY_STATE_PARAMS) url.searchParams.delete(name);
    if (!includeToken) url.searchParams.delete("token");
    // Keep the sibling tab set; the legacy codec wrote a single-key fragment
    // because it had no siblings.
    const tabs = new URLSearchParams(url.hash.slice(1)).get(TABS_HASH_KEY);
    const params = new URLSearchParams();
    params.set(HASH_KEY, encodeComparisonState(state));
    if (tabs) params.set(TABS_HASH_KEY, tabs);
    url.hash = params.toString();
    return url.toString();
  }

  // A shared link describes one comparison, not the reader's whole session, so
  // it drops the tab set as well as the token.
  function buildShareURL(urlValue, state) {
    const url = new URL(buildComparisonURL(urlValue, state, false), "http://ayame.invalid/");
    const params = new URLSearchParams(url.hash.slice(1));
    params.delete(TABS_HASH_KEY);
    url.hash = params.toString();
    return url.toString();
  }

  return {
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
  };
});
