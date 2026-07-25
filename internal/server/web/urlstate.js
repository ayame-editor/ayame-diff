(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameURLState = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const VERSION = 1;
  const HASH_KEY = "compare";
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

  function encodeComparisonState(state) {
    const value = { ...state, v: VERSION };
    const encoded = bytesToBase64URL(new TextEncoder().encode(JSON.stringify(value)));
    if (encoded.length > MAX_ENCODED_LENGTH) {
      const error = new RangeError("comparison state is too large for a reliable URL");
      error.code = "STATE_TOO_LARGE";
      throw error;
    }
    return encoded;
  }

  function decodeComparisonState(encoded) {
    if (!encoded || encoded.length > MAX_ENCODED_LENGTH) return null;
    try {
      const value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(base64URLToBytes(encoded)));
      return validState(value) ? value : null;
    } catch (_) {
      return null;
    }
  }

  function readComparisonState(urlValue) {
    const url = new URL(urlValue, "http://ayame.invalid/");
    const encoded = new URLSearchParams(url.hash.slice(1)).get(HASH_KEY);
    return decodeComparisonState(encoded);
  }

  function buildComparisonURL(urlValue, state, includeToken = true) {
    const url = new URL(urlValue, "http://ayame.invalid/");
    for (const name of LEGACY_STATE_PARAMS) url.searchParams.delete(name);
    if (!includeToken) url.searchParams.delete("token");
    url.hash = new URLSearchParams({ [HASH_KEY]: encodeComparisonState(state) }).toString();
    return url.toString();
  }

  function buildShareURL(urlValue, state) {
    return buildComparisonURL(urlValue, state, false);
  }

  return {
    VERSION,
    HASH_KEY,
    MAX_ENCODED_LENGTH,
    encodeComparisonState,
    decodeComparisonState,
    readComparisonState,
    buildComparisonURL,
    buildShareURL,
  };
});
