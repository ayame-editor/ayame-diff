// Classification shared by the difference navigation and the dismiss state
// (#269). A "downgraded" hunk is a whitespace- or case-only difference that the
// active ignore options consider equal: it stays visible (subdued) but is not a
// difference, so navigation and the counts skip it. Keeping the rule here — a
// pure function over the result data — lets node exercise it without a DOM.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameDowngrade = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  function isDowngraded(hunk) {
    return Boolean(hunk && hunk.downgraded);
  }

  // navigableIndexes returns the hunk indexes difference navigation may visit.
  // Only real differences qualify: a downgraded hunk is skipped, as is a hunk
  // the user ignored by hand. When eventKinds is given (a three-way result),
  // only "conflict" events are targets.
  function navigableIndexes(hunks, ignored, eventKinds) {
    const excluded = ignored instanceof Set ? ignored : new Set(ignored || []);
    const list = hunks || [];
    return list
      .map((_, index) => index)
      .filter((index) => {
        if (isDowngraded(list[index])) return false;
        if (excluded.has(index)) return false;
        if (eventKinds && eventKinds[index] !== "conflict") return false;
        return true;
      });
  }

  // countDowngraded is a fallback for a result that predates the server's
  // downgraded_hunks field; the field is authoritative because it also counts
  // dismissed hunks the max-hunk cap kept out of the response.
  function countDowngraded(hunks) {
    return (hunks || []).filter(isDowngraded).length;
  }

  // essentialIndex maps one index into the rendered hunk list — which includes
  // dismissed hunks — to the index a server-side result without dismissed hunks
  // uses. It returns -1 for a dismissed hunk. The patch and merge APIs compute
  // over real differences only, so an index after a dismissed hunk must be
  // shifted down.
  function essentialIndex(hunks, index) {
    const list = hunks || [];
    if (isDowngraded(list[index])) return -1;
    let rank = 0;
    for (let i = 0; i < index && i < list.length; i++) {
      if (!isDowngraded(list[i])) rank++;
    }
    return rank;
  }

  // essentialIndexes applies essentialIndex to a list, dropping dismissed ones.
  function essentialIndexes(hunks, indexes) {
    return (indexes || [])
      .map((index) => essentialIndex(hunks, index))
      .filter((rank) => rank >= 0);
  }

  return { isDowngraded, navigableIndexes, countDowngraded, essentialIndex, essentialIndexes };
});
