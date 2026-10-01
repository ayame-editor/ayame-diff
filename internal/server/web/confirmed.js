// Pure half of the explicit "confirmed" marks (#288). The automatic
// "read on scroll" state lives in app.js and is deliberately not mixed in here:
// a hunk that merely passed through the viewport is not verified, so the two
// must never collapse into one set. Storage also stays in app.js; this module
// only derives the stable keys and does the small set arithmetic, so it runs
// under node --test without a DOM.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameConfirmed = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  function nonNegativeInteger(value) {
    const number = Number(value);
    return Number.isFinite(number) ? Math.max(0, Math.trunc(number)) : 0;
  }

  // FNV-1a over UTF-16 code units. A plain 32-bit digest is enough here: the
  // goal is to notice that a hunk's content changed, not to resist an adversary.
  function hashText(text) {
    const value = String(text == null ? "" : text);
    let hash = 0x811c9dc5;
    for (let index = 0; index < value.length; index++) {
      hash ^= value.charCodeAt(index);
      hash = Math.imul(hash, 0x01000193) >>> 0;
    }
    return hash.toString(16).padStart(8, "0");
  }

  // A hunk's identity is its kind, its two ranges, and a digest of its changed
  // lines. The ranges keep two identical replacement bodies apart; the content
  // digest is what drops a confirmation when the compared lines change while
  // the hunk keeps the same place. The array index is deliberately not used, so
  // reordering or ignoring a hunk cannot move a confirmation onto another one.
  function hunkSignature(hunk) {
    const value = hunk || {};
    const old = Array.isArray(value.old) ? value.old : [];
    const neu = Array.isArray(value.new) ? value.new : [];
    return [
      String(value.kind || ""),
      nonNegativeInteger(value.old_start),
      nonNegativeInteger(value.old_len),
      nonNegativeInteger(value.new_start),
      nonNegativeInteger(value.new_len),
      hashText(JSON.stringify([old, neu])),
    ].join(":");
  }

  function hunkSignatures(hunks) {
    return (Array.isArray(hunks) ? hunks : []).map(hunkSignature);
  }

  // The comparison identity namespaces stored marks so two different files do
  // not share them. Inline (pasted) text has no path, so its content stands in.
  function comparisonIdentity(fields) {
    const value = fields || {};
    const parts = [String(value.mode || "")];
    if (value.inline) {
      parts.push("inline", hashText(value.oldText || ""), hashText(value.newText || ""));
    } else {
      parts.push(String(value.old || ""), String(value.new || ""), String(value.base || ""));
    }
    if (value.oldAbsent) parts.push("old-absent");
    if (value.newAbsent) parts.push("new-absent");
    return hashText(parts.join("\u0001"));
  }

  // Keep only the stored signatures that still describe a current hunk. A
  // changed hunk produces a different signature, so its confirmation is dropped
  // rather than carried onto content the user never verified.
  function restoreSignatures(signatures, stored) {
    const known = new Set(Array.isArray(stored) ? stored : []);
    return (Array.isArray(signatures) ? signatures : []).filter((signature) => known.has(signature));
  }

  // Indexes of the hunks that are not confirmed yet, in render order. Callers
  // apply their own scope (ignored hunks, three-way conflicts) before jumping.
  function unconfirmedIndexes(signatures, confirmed) {
    const done = confirmed && typeof confirmed.has === "function" ? confirmed : new Set();
    const items = Array.isArray(signatures) ? signatures : [];
    const indexes = [];
    for (let index = 0; index < items.length; index++) {
      if (!done.has(items[index])) indexes.push(index);
    }
    return indexes;
  }

  // Progress over a caller-chosen scope. Without a scope every hunk counts.
  function confirmProgress(signatures, confirmed, indexes) {
    const items = Array.isArray(signatures) ? signatures : [];
    const done = confirmed && typeof confirmed.has === "function" ? confirmed : new Set();
    const scope = Array.isArray(indexes) ? indexes : items.map((_, index) => index);
    let count = 0;
    for (const index of scope) if (done.has(items[index])) count++;
    return { confirmed: count, total: scope.length, unconfirmed: scope.length - count };
  }

  return {
    hashText,
    hunkSignature,
    hunkSignatures,
    comparisonIdentity,
    restoreSignatures,
    unconfirmedIndexes,
    confirmProgress,
  };
});
