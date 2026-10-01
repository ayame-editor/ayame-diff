// Human-readable diff hunk headers (#266). The patch form "@@ -1204,15 +1198,17 @@"
// makes a reader decode four numbers and count lines to learn one thing: roughly
// where they are. This turns it into readable line ranges plus the change kind.
// The patch export keeps the "@@" form; only the on-screen header changes.
//
// Pure, so it runs under node --test (#139).
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameHunkHeader = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // A 1-based inclusive range such as "1204–1218". An empty side (a pure insert
  // has no old lines) returns "", which the caller replaces with its own label.
  function lineRange(start, count) {
    const first = Number(start || 0);
    const length = Number(count || 0);
    if (length <= 0) return "";
    const last = first + length;
    return last > first + 1 ? `${first + 1}\u2013${last}` : `${first + 1}`;
  }

  const KIND_KEYS = { insert: "hunkInsert", delete: "hunkDelete", change: "hunkChange" };

  // header(hunk, t) -> { text, moved }
  //
  // t is the UI translate function, so this module stays independent of the
  // catalog. moved is null unless the hunk is a detected move, in which case the
  // caller renders its jump affordance beside the text.
  function header(hunk, t) {
    const item = hunk || {};
    const parts = [
      t("hunkOldLabel"),
      lineRange(item.old_start, item.old_len) || t("hunkEmptyRange"),
      t("hunkNewLabel"),
      lineRange(item.new_start, item.new_len) || t("hunkEmptyRange"),
      t(KIND_KEYS[item.kind] || "hunkChange"),
    ];
    let moved = null;
    if (item.move_id) {
      moved = { id: item.move_id, peer: Number(item.move_peer || 0) + 1 };
      parts.push(t("hunkMoved", moved));
    }
    return { text: parts.join(" "), moved };
  }

  return { lineRange, header };
});
