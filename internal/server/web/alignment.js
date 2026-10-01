// The whitespace policy split from #270: "ignore whitespace for the diff" and
// "ignore whitespace only while lining lines up" are different decisions, and
// the setup form expresses the second as one intent toggle ("absorb
// re-indentation"). This module is the pure mapping from that toggle plus the
// comparison whitespace value to the value the request sends as
// `alignWhitespace`, so the policy can be unit-tested without a DOM (#270).
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameAlignment = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const VALID_WHITESPACE = ["none", "change", "all"];

  // "change" collapses each run of whitespace and trims the ends, which is what
  // absorbs a re-indentation (leading tabs/spaces) without merging words.
  const ABSORB_ALIGNMENT = "change";

  const RANK = { none: 0, change: 1, all: 2 };

  function normalizeWhitespace(value) {
    return VALID_WHITESPACE.includes(value) ? value : "none";
  }

  // alignmentWhitespace returns the whitespace normalization used only to
  // establish line correspondence. It only ever asks for a policy strictly
  // coarser than the comparison's own: when the comparison already ignores as
  // much whitespace (or more), alignment would be indistinguishable from
  // detection, so this returns "none" and the server keeps the plain path.
  function alignmentWhitespace(whitespace, absorbReindent) {
    const compare = normalizeWhitespace(whitespace);
    if (!absorbReindent) return "none";
    return RANK[compare] < RANK[ABSORB_ALIGNMENT] ? ABSORB_ALIGNMENT : "none";
  }

  return { VALID_WHITESPACE, ABSORB_ALIGNMENT, alignmentWhitespace };
});
