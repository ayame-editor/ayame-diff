// Resident-memory status for a CSV comparison (#138).
//
// The engine reports the budget it resolved and whether it had to spill, but the
// bytes are raw and the display decisions (binary units, an optional cap, hiding
// the spill directory unless a spill actually happened) should be testable
// without a browser. This is that pure logic.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameMemoryBudget = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // Binary units match --memory and the engine's own FormatByteSize, so the
  // number the user typed is the number shown.
  const UNITS = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];

  function formatBytes(bytes) {
    const value = Number(bytes);
    if (!Number.isFinite(value) || value <= 0) return "0B";
    let n = value;
    let unit = 0;
    while (n >= 1024 && unit < UNITS.length - 1) {
      n /= 1024;
      unit += 1;
    }
    return unit === 0 ? `${Math.round(n)}B` : `${n.toFixed(1)}${UNITS[unit]}`;
  }

  // memoryStatus turns the API's `memory` object into what the summary note
  // needs. A missing or non-positive budget returns null so the caller renders
  // nothing rather than a bogus "0B" limit.
  function memoryStatus(report) {
    if (!report || !Number(report.budget_bytes)) return null;
    const spilled = Boolean(report.spilled);
    return {
      budget: formatBytes(report.budget_bytes),
      cap: report.cap ? String(report.cap) : "",
      spilled,
      dir: spilled && report.spill_dir ? String(report.spill_dir) : "",
    };
  }

  return { formatBytes, memoryStatus };
});
