// The single monotonic whitespace/EOL scale (#259).
//
// The old setup form exposed three independent controls for one axis —
// ignoreEOL / ignoreTrailingEOL / whitespace — which asked the user to pick a
// meaningful point out of 2x2x3 combinations. P4Merge instead presents one
// ordered scale where each step ignores everything the previous step did. This
// module is the pure mapping from that scale to the request fields the server
// still accepts, so the API stays backward compatible.
//
// It is a UMD module: app.js consumes it as globalThis.AyameWhitespaceScale, and
// it is unit-tested under node --test with require().
(function (root, factory) {
  if (typeof module === "object" && module.exports) module.exports = factory();
  else root.AyameWhitespaceScale = factory();
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // Ordered from least to most ignored. The order is the contract: a later level
  // never ignores less than an earlier one.
  const LEVELS = ["strict", "eol", "eol-change", "eol-all"];

  // The legacy /api/diff fields each level is equivalent to. These are exactly
  // the values the old individual controls produced:
  //   strict     = no checkboxes, whitespace "none"
  //   eol        = ignoreEOL
  //   eol-change = ignoreEOL + whitespace "change"
  //   eol-all    = ignoreEOL + whitespace "all"
  // ignoreEOL makes ignoreTrailingEOL redundant, so it stays false.
  const LEGACY_FIELDS = {
    strict: { whitespace: "none", ignoreEOL: false, ignoreTrailingEOL: false },
    eol: { whitespace: "none", ignoreEOL: true, ignoreTrailingEOL: false },
    "eol-change": { whitespace: "change", ignoreEOL: true, ignoreTrailingEOL: false },
    "eol-all": { whitespace: "all", ignoreEOL: true, ignoreTrailingEOL: false },
  };

  // i18n keys for the option labels, in the same order as LEVELS.
  const LABEL_KEYS = {
    strict: "wsScaleStrict",
    eol: "wsScaleEOL",
    "eol-change": "wsScaleEOLChange",
    "eol-all": "wsScaleEOLAll",
  };

  // normalize clamps an unknown or missing value to the least-surprising level,
  // strict, which is also the default selection.
  function normalize(level) {
    return Object.prototype.hasOwnProperty.call(LEGACY_FIELDS, level) ? level : "strict";
  }

  // requestFields returns a fresh copy of the legacy request fields for a level.
  function requestFields(level) {
    return Object.assign({}, LEGACY_FIELDS[normalize(level)]);
  }

  // whitespaceMode returns just the --ignore-whitespace value. CSV comparison is
  // record-based and has no line-ending notion, so it only consumes this part.
  function whitespaceMode(level) {
    return LEGACY_FIELDS[normalize(level)].whitespace;
  }

  // labelKey returns the i18n key for a level's human label.
  function labelKey(level) {
    return LABEL_KEYS[normalize(level)];
  }

  // levelForWhitespace is the inverse used when restoring a saved CSV project,
  // whose body carries only the legacy whitespace value. CSV ignores line
  // endings, so the whitespace part picks the matching EOL level.
  function levelForWhitespace(mode) {
    switch (mode) {
      case "change": return "eol-change";
      case "all": return "eol-all";
      default: return "strict";
    }
  }

  return { LEVELS, LABEL_KEYS, requestFields, whitespaceMode, labelKey, levelForWhitespace, normalize };
});
