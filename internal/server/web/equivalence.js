// Which message a CSV equivalence verdict resolves to (#116).
//
// A key-based comparison is row-order independent, so "no differences" alone
// cannot say whether the inputs were identical or simply written in a different
// order. The server now states that verdict; these are the presentation-only
// rules that turn it into a catalog key. They are pure, so node checks them
// rather than a rebuilt result card (#139).
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameEquivalence = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const TITLE_KEYS = {
    equal_columns_reordered: "dataEqualColumnsReordered",
    equal_row_order: "dataEqualRowOrder",
    equal_presentation: "dataEqualPresentation",
  };

  // The card title for a no-difference result. A plain "equal" keeps the
  // existing complete/filtered match wording; the presentation-only verdicts
  // name the difference the reader would otherwise have to infer.
  function equivalenceTitleKey(verdict, usesRules) {
    if (verdict && verdict.substantively_equal && TITLE_KEYS[verdict.status]) {
      return TITLE_KEYS[verdict.status];
    }
    return usesRules ? "filteredMatch" : "completeMatch";
  }

  // True when the comparison was withheld because the columns are reordered
  // and name alignment is off, so the result should offer the one-click fix.
  function alignmentProposal(verdict) {
    return Boolean(verdict && verdict.status === "columns_reordered" && verdict.alignable_columns);
  }

  return { equivalenceTitleKey, alignmentProposal };
});
