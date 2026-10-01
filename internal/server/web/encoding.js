// Encoding choices and the detected/mismatch decisions the pane header needs
// (#278). The detection itself lives in Go (internal/encoding); this module is
// only the DOM-free part the browser and the node checks share.
//
// The engine reports one concrete name per side and no confidence score, so the
// UI must not invent one. Instead it shows what was detected and offers every
// other encoding as something to try, which is what these helpers describe.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameEncoding = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  // Mirrors internal/encoding.Supported and the #encoding <select> in
  // index.html. "auto" means "let the engine detect"; the rest are concrete.
  const ENCODING_OPTIONS = [
    "auto",
    "utf-8",
    "utf-16le",
    "utf-16be",
    "shift_jis",
    "euc-jp",
    "iso-2022-jp",
  ];

  // The Japanese codecs whose byte patterns overlap most, so a guess between
  // them is the one most worth doubting with a second look.
  const JAPANESE_LEGACY = ["shift_jis", "euc-jp", "iso-2022-jp"];

  function isSupported(name) {
    return ENCODING_OPTIONS.includes(name);
  }

  function isJapaneseLegacy(name) {
    return JAPANESE_LEGACY.includes(name);
  }

  // A left/right mismatch is the first clue that one side was decoded wrongly;
  // with one side absent (an added/removed file) there is nothing to compare.
  function encodingMismatch(left, right) {
    return Boolean(left && right && left !== right);
  }

  // encodingCandidates lists the encodings to try when the guess looks wrong:
  // every concrete option except the one already in use, in menu order. "auto"
  // is not a candidate (it is how you go back to detecting), and because the
  // engine exposes no confidence ranking the alternatives are unranked
  // suggestions, not probabilities.
  function encodingCandidates(detected) {
    const chosen = isSupported(detected) ? detected : "";
    return ENCODING_OPTIONS.filter((name) => name !== "auto" && name !== chosen);
  }

  // The correction picker shows the user's explicit override, or "auto" so the
  // engine keeps detecting; either way the detected name is shown beside it.
  function encodingPickerValue(override) {
    return isSupported(override) && override !== "auto" ? override : "auto";
  }

  return {
    ENCODING_OPTIONS,
    JAPANESE_LEGACY,
    isSupported,
    isJapaneseLegacy,
    encodingMismatch,
    encodingCandidates,
    encodingPickerValue,
  };
});
