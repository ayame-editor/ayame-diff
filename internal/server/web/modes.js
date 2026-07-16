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

  // Every comparison-condition control id in the shared setup pool.
  const COMPARE_CONDITIONS = [
    "ignoreCase", "whitespace", "ignoreEOL", "ignoreTrailingEOL", "lineFilters",
  ];

  // The subset each mode passes to its request body. Modes absent from this map
  // fall back to the full set: text / sorted spread requestBody(), and 3-way
  // text spreads it via threeWayRequestBody(), so they honor every condition.
  const LIVE_BY_MODE = {
    // csvRequestBody() reads ignoreCase / whitespace / lineFilters but not the
    // EOL toggles — row-keyed CSV comparison has no line-ending notion.
    csv: ["ignoreCase", "whitespace", "lineFilters"],
    "threeway-csv": ["ignoreCase", "whitespace", "lineFilters"],
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
