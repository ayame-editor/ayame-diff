// Colour-independent diff signalling (#298).
//
// A diff tool is colour-dependent by nature: add/delete/change are usually
// distinguished by a green/red/yellow wash alone. This module is the single
// place that answers "what else says which kind this line is?" — the gutter
// glyph and the assistive-technology kind name — so both views and the tests
// read one source of truth. It is pure, so node --test exercises it directly,
// following the worddiff.js / i18n.js wrapper.
(function (root, factory) {
  "use strict";
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameDiffMark = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // The glyph printed in the gutter for a line's role. It is the signal that
  // survives when colour does not, and it is deliberately the +/- a patch uses:
  // the unified view already printed it (#115), so side-by-side gains the same
  // reading rather than a second vocabulary. A changed pair keeps the
  // removal/addition reading instead of inventing a third ambiguous symbol.
  function cellMarker(cls, side) {
    if (cls === "add") return "+";
    if (cls === "del") return "-";
    if (cls === "chg") return side === "old" ? "-" : "+";
    return "";
  }

  // The i18n key naming the line's kind for assistive technology. The keys
  // already exist in both catalogues for the difference index (#110).
  function cellKindKey(cls) {
    if (cls === "add") return "added";
    if (cls === "del") return "deleted";
    if (cls === "chg") return "modified";
    return "";
  }

  // Hunk kinds, as the server names them. "+"/"−"/"~" mirror the difference
  // index (#110) so the header glyph, the index and the rows all name a change
  // the same way. Replace is the one kind its rows do not carry on their own —
  // they read as "-" then "+" — so "~" is what keeps a change from looking like
  // an unrelated insert next to a delete.
  const HUNK_MARK = { insert: "+", delete: "\u2212", replace: "~" };
  const HUNK_KEY = { insert: "added", delete: "deleted", replace: "modified" };

  function hunkMarker(kind) {
    return HUNK_MARK[kind] || "";
  }

  function hunkKindKey(kind) {
    return HUNK_KEY[kind] || "modified";
  }

  // A conflict is its own kind: neither side wins and it is not a plain replace,
  // so it gets a glyph that cannot be read as either.
  const CONFLICT_MARK = "\u2260"; // ≠

  return { cellMarker, cellKindKey, hunkMarker, hunkKindKey, HUNK_MARK, CONFLICT_MARK };
});
