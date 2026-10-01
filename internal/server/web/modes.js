// Central policy for which comparison-condition controls each compare mode
// actually honors. The setup form shows one shared pool of comparison options
// (ignore case / whitespace / ignore EOL / ignore trailing EOL / line filters),
// but not every mode reads every option when it builds its request body (see
// requestBody / csvRequestBody / dirRequestBody in app.js). A control that is
// visible yet never read is a "dead" control: the user toggles it and nothing
// changes. syncModeOpts consults this policy to hide the dead controls for the
// active mode, keeping the visible set in lockstep with what the request
// actually applies (#124).
(function (root) {
  "use strict";

  // Every comparison-condition control id in the shared setup pool. Whitespace
  // and line endings are one monotonic scale (#259) rather than three controls.
  const COMPARE_CONDITIONS = [
    "ignoreCase", "whitespaceScale", "lineFilters",
  ];

  // The subset each mode passes to its request body. Modes absent from this map
  // fall back to the full set: text / sorted spread requestBody(), and 3-way
  // text spreads it via threeWayRequestBody(), so they honor every condition.
  //
  // csv / threeway-csv also spread the full set: csvRequestBody() reads
  // ignoreCase, the whitespace part of the scale, and lineFilters. The scale's
  // EOL part is structurally irrelevant to record-based CSV, but the single
  // control stays live because its whitespace part drives real differences.
  const LIVE_BY_MODE = {
    // dirRequestBody() reads none of them: folder compare works on names, size,
    // mtime, and byte content, with its own include/exclude globs.
    dir: [],
  };

  // liveCompareConditions returns the conditions the given mode sends to the
  // server (and therefore the controls that should stay visible).
  function liveCompareConditions(mode) {
    return LIVE_BY_MODE[mode] || COMPARE_CONDITIONS;
  }

  // deadCompareConditions returns the conditions shown in the form that the
  // given mode ignores — the controls syncModeOpts should hide.
  function deadCompareConditions(mode) {
    const live = new Set(liveCompareConditions(mode));
    return COMPARE_CONDITIONS.filter((id) => !live.has(id));
  }

  const api = { COMPARE_CONDITIONS, liveCompareConditions, deadCompareConditions };
  root.AyameModes = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
