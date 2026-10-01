// Which actions a hunk-local toolbar offers (#293).
//
// The actions were once rendered inline in every hunk head, always visible,
// far from the merge panel and the navigation bar that carry the session-wide
// controls. They now live in a small toolbar anchored to the hunk itself, and
// which actions that toolbar holds is a pure function of the hunk's state.
// Keeping the decision here means it runs under node --test instead of being
// inferred from the DOM.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameHunkActions = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // The actions to show, in toolbar order. A moved hunk gains the jump to its
  // paired location; the ignore action's label flips once the hunk is ignored;
  // adopt left/right exist only where a two-way text diff can be merged. Merge
  // mode decides whether the adopt buttons are *visible*, not whether they are
  // built, so the same descriptor list serves both states.
  //
  // Each descriptor carries everything the DOM layer needs: the id to dispatch
  // on, the class the stylesheet keys off, and the i18n key for its label. A
  // glyph, when present, is shown instead of the label text.
  function hunkActions(state) {
    const s = state || {};
    const actions = [];
    if (s.moved) {
      actions.push({ id: "move-jump", className: "move-jump", labelKey: "moved", glyph: "↔" });
    }
    actions.push({
      id: "ignore",
      className: "hunk-ignore",
      labelKey: s.ignored ? "restoreHunk" : "ignoreHunk",
      glyph: null,
    });
    if (s.mergeable) {
      actions.push({ id: "choose-left", className: "choose-left hunk-adopt", labelKey: "chooseLeft", side: "left", glyph: null });
      actions.push({ id: "choose-right", className: "choose-right hunk-adopt", labelKey: "chooseRight", side: "right", glyph: null });
    }
    return actions;
  }

  return { hunkActions };
});
