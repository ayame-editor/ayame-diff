// The single server `mode` value collapses two orthogonal questions (#263):
//
//   1. WHAT is compared — two files, three files (a 3-way merge), or a folder.
//   2. HOW the compared text is read — plain text, sorted lines, or CSV columns.
//
// The dropdown that used to hold `text / sorted / csv / dir / threeway /
// threeway-csv` made the second question a property of the first: choosing a
// reading meant deciding it before comparing, and the `threeway-csv` entry was
// the visible sign that the control was really a cross product. This module
// owns the (small, explicit) map between the two axes and the flat `mode` the
// API still expects, so the GUI never enumerates combinations itself and the
// server contract does not change.
//
// DOM-free on purpose, so it runs under node --test.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameModeAxes = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // Axis 1: the shape of the input the user chose.
  const SHAPES = ["two", "three", "folder"];

  // Axis 2: how the compared lines/rows are interpreted. This is deliberately
  // not a property of the input; it is a reading that can be switched and
  // reverted after a result exists.
  const INTERPRETATIONS = ["text", "sorted", "csv"];

  // Not every reading applies to every shape: a folder comparison compares
  // names and bytes, and the server has no sorted three-way mode. Listing the
  // supported readings per shape is what keeps the second axis an axis rather
  // than a flat list of modes — `threeway-sorted` is simply not offered.
  const INTERPRETATIONS_BY_SHAPE = {
    two: ["text", "sorted", "csv"],
    three: ["text", "csv"],
    folder: [],
  };

  // The one place the axes meet the flat API value. The server still receives
  // `text` / `sorted` / `csv` / `dir` / `threeway` / `threeway-csv`; nothing
  // about a request body changes.
  const MODE_BY_AXES = {
    "two/text": "text",
    "two/sorted": "sorted",
    "two/csv": "csv",
    "three/text": "threeway",
    "three/csv": "threeway-csv",
    "folder/": "dir",
  };

  const AXES_BY_MODE = {
    text: { shape: "two", interpretation: "text" },
    sorted: { shape: "two", interpretation: "sorted" },
    csv: { shape: "two", interpretation: "csv" },
    threeway: { shape: "three", interpretation: "text" },
    "threeway-csv": { shape: "three", interpretation: "csv" },
    dir: { shape: "folder", interpretation: "" },
  };

  // interpretationsFor returns the readings a shape supports, in the order the
  // toolbar should offer them. A copy, so callers cannot mutate the table.
  function interpretationsFor(shape) {
    return [...(INTERPRETATIONS_BY_SHAPE[shape] || [])];
  }

  function supportsInterpretation(shape, interpretation) {
    return interpretationsFor(shape).includes(interpretation);
  }

  // composeMode collapses the axes into the value the API expects. An
  // unsupported pair falls back to the shape's first reading rather than
  // inventing an unknown mode, so a stale toolbar value can never become a
  // malformed request.
  function composeMode(shape, interpretation) {
    const supported = interpretationsFor(shape);
    if (!supported.length) return "dir";
    const chosen = supported.includes(interpretation) ? interpretation : supported[0];
    return MODE_BY_AXES[`${shape}/${chosen}`] || MODE_BY_AXES[`${shape}/${supported[0]}`];
  }

  // decomposeMode is the inverse, used when a `mode` arrives from a launch
  // parameter, a shared URL, or a saved project. An unknown mode reads as a
  // plain two-file text comparison, the historical default.
  function decomposeMode(mode) {
    const axes = AXES_BY_MODE[mode];
    return axes ? { ...axes } : { shape: "two", interpretation: "text" };
  }

  return {
    SHAPES,
    INTERPRETATIONS,
    interpretationsFor,
    supportsInterpretation,
    composeMode,
    decomposeMode,
  };
});
