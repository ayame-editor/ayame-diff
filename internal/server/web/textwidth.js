// Display-width model for the web UI, mirroring internal/textwidth (#289).
//
// The CLI already aligns side-by-side columns with internal/textwidth; the GUI
// only ever relied on the browser's font metrics. That left the two halves of
// the product free to disagree about how wide a line is. This module is the
// GUI's half of the same decision: terminal-cell width, East Asian Ambiguous
// handling, and tab expansion. It is pure, so node --test runs it with no DOM
// (#139) and a Go test can check it against the CLI renderer.
//
// The East Asian Width tables below were generated from the same
// golang.org/x/text/width data that internal/textwidth consults, so the two
// implementations classify a code point identically rather than approximately.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameTextWidth = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const DEFAULT_TAB_SIZE = 8;
  const MIN_TAB_SIZE = 1;
  const MAX_TAB_SIZE = 16;

  // East Asian Wide + Fullwidth: two cells. Paired [start, end] inclusive.
  const WIDE = [
    0x1100, 0x115f, 0x231a, 0x231b, 0x2329, 0x232a, 0x23e9, 0x23ec,
    0x23f0, 0x23f0, 0x23f3, 0x23f3, 0x25fd, 0x25fe, 0x2614, 0x2615,
    0x2648, 0x2653, 0x267f, 0x267f, 0x2693, 0x2693, 0x26a1, 0x26a1,
    0x26aa, 0x26ab, 0x26bd, 0x26be, 0x26c4, 0x26c5, 0x26ce, 0x26ce,
    0x26d4, 0x26d4, 0x26ea, 0x26ea, 0x26f2, 0x26f3, 0x26f5, 0x26f5,
    0x26fa, 0x26fa, 0x26fd, 0x26fd, 0x2705, 0x2705, 0x270a, 0x270b,
    0x2728, 0x2728, 0x274c, 0x274c, 0x274e, 0x274e, 0x2753, 0x2755,
    0x2757, 0x2757, 0x2795, 0x2797, 0x27b0, 0x27b0, 0x27bf, 0x27bf,
    0x2b1b, 0x2b1c, 0x2b50, 0x2b50, 0x2b55, 0x2b55, 0x2e80, 0x2e99,
    0x2e9b, 0x2ef3, 0x2f00, 0x2fd5, 0x2ff0, 0x2ffb, 0x3000, 0x303e,
    0x3041, 0x3096, 0x3099, 0x30ff, 0x3105, 0x312f, 0x3131, 0x318e,
    0x3190, 0x31e3, 0x31f0, 0x321e, 0x3220, 0x3247, 0x3250, 0x4dbf,
    0x4e00, 0xa48c, 0xa490, 0xa4c6, 0xa960, 0xa97c, 0xac00, 0xd7a3,
    0xf900, 0xfaff, 0xfe10, 0xfe19, 0xfe30, 0xfe52, 0xfe54, 0xfe66,
    0xfe68, 0xfe6b, 0xff01, 0xff60, 0xffe0, 0xffe6, 0x16fe0, 0x16fe4,
    0x16ff0, 0x16ff1, 0x17000, 0x187f7, 0x18800, 0x18cd5, 0x18d00, 0x18d08,
    0x1aff0, 0x1aff3, 0x1aff5, 0x1affb, 0x1affd, 0x1affe, 0x1b000, 0x1b122,
    0x1b132, 0x1b132, 0x1b150, 0x1b152, 0x1b155, 0x1b155, 0x1b164, 0x1b167,
    0x1b170, 0x1b2fb, 0x1f004, 0x1f004, 0x1f0cf, 0x1f0cf, 0x1f18e, 0x1f18e,
    0x1f191, 0x1f19a, 0x1f200, 0x1f202, 0x1f210, 0x1f23b, 0x1f240, 0x1f248,
    0x1f250, 0x1f251, 0x1f260, 0x1f265, 0x1f300, 0x1f320, 0x1f32d, 0x1f335,
    0x1f337, 0x1f37c, 0x1f37e, 0x1f393, 0x1f3a0, 0x1f3ca, 0x1f3cf, 0x1f3d3,
    0x1f3e0, 0x1f3f0, 0x1f3f4, 0x1f3f4, 0x1f3f8, 0x1f43e, 0x1f440, 0x1f440,
    0x1f442, 0x1f4fc, 0x1f4ff, 0x1f53d, 0x1f54b, 0x1f54e, 0x1f550, 0x1f567,
    0x1f57a, 0x1f57a, 0x1f595, 0x1f596, 0x1f5a4, 0x1f5a4, 0x1f5fb, 0x1f64f,
    0x1f680, 0x1f6c5, 0x1f6cc, 0x1f6cc, 0x1f6d0, 0x1f6d2, 0x1f6d5, 0x1f6d7,
    0x1f6dc, 0x1f6df, 0x1f6eb, 0x1f6ec, 0x1f6f4, 0x1f6fc, 0x1f7e0, 0x1f7eb,
    0x1f7f0, 0x1f7f0, 0x1f90c, 0x1f93a, 0x1f93c, 0x1f945, 0x1f947, 0x1f9ff,
    0x1fa70, 0x1fa7c, 0x1fa80, 0x1fa88, 0x1fa90, 0x1fabd, 0x1fabf, 0x1fac5,
    0x1face, 0x1fadb, 0x1fae0, 0x1fae8, 0x1faf0, 0x1faf8, 0x20000, 0x3ffff,
  ];

  // East Asian Ambiguous: one cell by default, two when the locale says so.
  const AMBIGUOUS = [
    0xa1, 0xa1, 0xa4, 0xa4, 0xa7, 0xa8, 0xaa, 0xaa,
    0xad, 0xae, 0xb0, 0xb4, 0xb6, 0xba, 0xbc, 0xbf,
    0xc6, 0xc6, 0xd0, 0xd0, 0xd7, 0xd8, 0xde, 0xe1,
    0xe6, 0xe6, 0xe8, 0xea, 0xec, 0xed, 0xf0, 0xf0,
    0xf2, 0xf3, 0xf7, 0xfa, 0xfc, 0xfc, 0xfe, 0xfe,
    0x101, 0x101, 0x111, 0x111, 0x113, 0x113, 0x11b, 0x11b,
    0x126, 0x127, 0x12b, 0x12b, 0x131, 0x133, 0x138, 0x138,
    0x13f, 0x142, 0x144, 0x144, 0x148, 0x14b, 0x14d, 0x14d,
    0x152, 0x153, 0x166, 0x167, 0x16b, 0x16b, 0x1ce, 0x1ce,
    0x1d0, 0x1d0, 0x1d2, 0x1d2, 0x1d4, 0x1d4, 0x1d6, 0x1d6,
    0x1d8, 0x1d8, 0x1da, 0x1da, 0x1dc, 0x1dc, 0x251, 0x251,
    0x261, 0x261, 0x2c4, 0x2c4, 0x2c7, 0x2c7, 0x2c9, 0x2cb,
    0x2cd, 0x2cd, 0x2d0, 0x2d0, 0x2d8, 0x2db, 0x2dd, 0x2dd,
    0x2df, 0x2df, 0x300, 0x36f, 0x391, 0x3a1, 0x3a3, 0x3a9,
    0x3b1, 0x3c1, 0x3c3, 0x3c9, 0x401, 0x401, 0x410, 0x44f,
    0x451, 0x451, 0x2010, 0x2010, 0x2013, 0x2016, 0x2018, 0x2019,
    0x201c, 0x201d, 0x2020, 0x2022, 0x2024, 0x2027, 0x2030, 0x2030,
    0x2032, 0x2033, 0x2035, 0x2035, 0x203b, 0x203b, 0x203e, 0x203e,
    0x2074, 0x2074, 0x207f, 0x207f, 0x2081, 0x2084, 0x20ac, 0x20ac,
    0x2103, 0x2103, 0x2105, 0x2105, 0x2109, 0x2109, 0x2113, 0x2113,
    0x2116, 0x2116, 0x2121, 0x2122, 0x2126, 0x2126, 0x212b, 0x212b,
    0x2153, 0x2154, 0x215b, 0x215e, 0x2160, 0x216b, 0x2170, 0x2179,
    0x2189, 0x2189, 0x2190, 0x2199, 0x21b8, 0x21b9, 0x21d2, 0x21d2,
    0x21d4, 0x21d4, 0x21e7, 0x21e7, 0x2200, 0x2200, 0x2202, 0x2203,
    0x2207, 0x2208, 0x220b, 0x220b, 0x220f, 0x220f, 0x2211, 0x2211,
    0x2215, 0x2215, 0x221a, 0x221a, 0x221d, 0x2220, 0x2223, 0x2223,
    0x2225, 0x2225, 0x2227, 0x222c, 0x222e, 0x222e, 0x2234, 0x2237,
    0x223c, 0x223d, 0x2248, 0x2248, 0x224c, 0x224c, 0x2252, 0x2252,
    0x2260, 0x2261, 0x2264, 0x2267, 0x226a, 0x226b, 0x226e, 0x226f,
    0x2282, 0x2283, 0x2286, 0x2287, 0x2295, 0x2295, 0x2299, 0x2299,
    0x22a5, 0x22a5, 0x22bf, 0x22bf, 0x2312, 0x2312, 0x2460, 0x24e9,
    0x24eb, 0x254b, 0x2550, 0x2573, 0x2580, 0x258f, 0x2592, 0x2595,
    0x25a0, 0x25a1, 0x25a3, 0x25a9, 0x25b2, 0x25b3, 0x25b6, 0x25b7,
    0x25bc, 0x25bd, 0x25c0, 0x25c1, 0x25c6, 0x25c8, 0x25cb, 0x25cb,
    0x25ce, 0x25d1, 0x25e2, 0x25e5, 0x25ef, 0x25ef, 0x2605, 0x2606,
    0x2609, 0x2609, 0x260e, 0x260f, 0x261c, 0x261c, 0x261e, 0x261e,
    0x2640, 0x2640, 0x2642, 0x2642, 0x2660, 0x2661, 0x2663, 0x2665,
    0x2667, 0x266a, 0x266c, 0x266d, 0x266f, 0x266f, 0x269e, 0x269f,
    0x26bf, 0x26bf, 0x26c6, 0x26cd, 0x26cf, 0x26d3, 0x26d5, 0x26e1,
    0x26e3, 0x26e3, 0x26e8, 0x26e9, 0x26eb, 0x26f1, 0x26f4, 0x26f4,
    0x26f6, 0x26f9, 0x26fb, 0x26fc, 0x26fe, 0x26ff, 0x273d, 0x273d,
    0x2776, 0x277f, 0x2b56, 0x2b59, 0x3248, 0x324f, 0xe000, 0xf8ff,
    0xfe00, 0xfe0f, 0xfffd, 0xfffd, 0x1f100, 0x1f10a, 0x1f110, 0x1f12d,
    0x1f130, 0x1f169, 0x1f170, 0x1f18d, 0x1f18f, 0x1f190, 0x1f19b, 0x1f1ac,
    0xe0100, 0xe01ef, 0xf0000, 0xffffd, 0x100000, 0x10fffd,
  ];

  // inRanges does a binary search over a flat, sorted array of [start, end]
  // pairs, the same shape the generator emitted.
  function inRanges(codePoint, ranges) {
    let low = 0;
    let high = ranges.length / 2 - 1;
    while (low <= high) {
      const mid = (low + high) >> 1;
      if (codePoint < ranges[mid * 2]) high = mid - 1;
      else if (codePoint > ranges[mid * 2 + 1]) low = mid + 1;
      else return true;
    }
    return false;
  }

  // Unicode category tests use JavaScript's own property escapes rather than a
  // hand-copied table, exactly as Go uses unicode.Is.
  const MARK_RE = /^[\p{Mn}\p{Me}]$/u;
  const FORMAT_RE = /^[\p{Cf}]$/u;

  function isZeroWidth(codePoint) {
    if (codePoint === 0) return true;
    // Hangul Jamo medial/final and Extended-A/B are Lo, so the category checks
    // below would miss them; Go special-cases the same ranges.
    if (codePoint >= 0x1160 && codePoint <= 0x11ff) return true;
    if (codePoint >= 0xd7b0 && codePoint <= 0xd7ff) return true;
    const ch = String.fromCodePoint(codePoint);
    return MARK_RE.test(ch) || FORMAT_RE.test(ch);
  }

  function isEmojiModifier(codePoint) {
    return codePoint >= 0x1f3fb && codePoint <= 0x1f3ff;
  }

  function isRegionalIndicator(codePoint) {
    return codePoint >= 0x1f1e6 && codePoint <= 0x1f1ff;
  }

  function isClusterExtender(codePoint) {
    // ZWJ terminates the current extension run and joins the next base rune.
    return codePoint !== 0x200d && (isZeroWidth(codePoint) || isEmojiModifier(codePoint));
  }

  // runeWidth returns the cell width of one code point, before cluster
  // collapsing. It mirrors internal/textwidth.runeDisplayWidth.
  function runeWidth(codePoint, options) {
    if (isZeroWidth(codePoint)) return 0;
    if (codePoint < 0x20 || (codePoint >= 0x7f && codePoint < 0xa0)) return 0;
    if (inRanges(codePoint, WIDE)) return 2;
    if (options.eastAsianAmbiguousWide && inRanges(codePoint, AMBIGUOUS)) return 2;
    return 1;
  }

  // nextCluster returns the index just past a display cluster and its width.
  // This is the terminal-oriented subset of grapheme breaking that the CLI
  // uses: combining marks, emoji modifiers, regional-indicator flags, and ZWJ
  // emoji stay together.
  function nextCluster(codePoints, start, options) {
    let i = start + 1;
    let width = runeWidth(codePoints[start], options);
    const regionalPair = isRegionalIndicator(codePoints[start]);
    if (regionalPair && i < codePoints.length && isRegionalIndicator(codePoints[i])) {
      width = 2;
      i++;
    }
    for (;;) {
      while (i < codePoints.length && isClusterExtender(codePoints[i])) {
        if (codePoints[i] === 0xfe0f || codePoints[i] === 0x20e3) width = Math.max(width, 2);
        i++;
      }
      if (i + 1 >= codePoints.length || codePoints[i] !== 0x200d) break;
      i++; // zero-width joiner
      width = Math.max(width, runeWidth(codePoints[i], options));
      i++;
    }
    return { next: i, width };
  }

  function optionsOf(options) {
    return { eastAsianAmbiguousWide: Boolean(options && options.eastAsianAmbiguousWide) };
  }

  // displayWidth returns the number of terminal cells text occupies. A raw tab
  // counts as zero, matching internal/textwidth; the GUI renders tabs through
  // CSS tab-size, not through this number (see docs/gui.md).
  function displayWidth(text, options) {
    const opts = optionsOf(options);
    const codePoints = Array.from(String(text == null ? "" : text), (ch) => ch.codePointAt(0));
    let width = 0;
    for (let i = 0; i < codePoints.length; ) {
      const cluster = nextCluster(codePoints, i, opts);
      width += cluster.width;
      i = cluster.next;
    }
    return width;
  }

  function normalizeTabSize(size) {
    const value = Math.trunc(Number(size));
    if (!Number.isFinite(value) || value < MIN_TAB_SIZE) return DEFAULT_TAB_SIZE;
    return Math.min(value, MAX_TAB_SIZE);
  }

  // nextTabStop returns the column a tab at column advances to, so a line
  // rendered at one tab size and one ambiguous-width choice stays aligned.
  function nextTabStop(column, tabSize) {
    const size = normalizeTabSize(tabSize);
    const at = Math.max(0, Math.trunc(Number(column)) || 0);
    return at + (size - (at % size));
  }

  // expandTabs replaces each tab with enough spaces to reach its tab stop,
  // measured in display cells, so CJK text before a tab moves the stop the way
  // the CLI's width model would. Tabs only; other whitespace is left alone.
  function expandTabs(text, tabSize, options) {
    const opts = optionsOf(options);
    const codePoints = Array.from(String(text == null ? "" : text), (ch) => ch.codePointAt(0));
    let out = "";
    let column = 0;
    for (let i = 0; i < codePoints.length; ) {
      if (codePoints[i] === 0x09) {
        const stop = nextTabStop(column, tabSize);
        out += " ".repeat(stop - column);
        column = stop;
        i++;
        continue;
      }
      const cluster = nextCluster(codePoints, i, opts);
      for (let j = i; j < cluster.next; j++) out += String.fromCodePoint(codePoints[j]);
      column += cluster.width;
      i = cluster.next;
    }
    return out;
  }

  // truncate shortens text to at most maxWidth cells, appending "...", never
  // cutting a cluster in half. Mirrors internal/textwidth.TruncateWithOptions.
  function truncate(text, maxWidth, options) {
    const opts = optionsOf(options);
    const limit = Math.trunc(Number(maxWidth));
    if (!Number.isFinite(limit) || limit <= 0) return "";
    const source = String(text == null ? "" : text);
    if (displayWidth(source, opts) <= limit) return source;
    if (limit <= 3) return ".".repeat(limit);
    const target = limit - 3;
    const codePoints = Array.from(source, (ch) => ch.codePointAt(0));
    let out = "";
    let width = 0;
    for (let i = 0; i < codePoints.length; ) {
      const cluster = nextCluster(codePoints, i, opts);
      if (width + cluster.width > target) break;
      for (let j = i; j < cluster.next; j++) out += String.fromCodePoint(codePoints[j]);
      width += cluster.width;
      i = cluster.next;
    }
    return out + "...";
  }

  // padRight pads text with spaces to exactly width cells, truncating when it
  // is already wider. Mirrors internal/textwidth.PadRightWithOptions.
  function padRight(text, width, options) {
    const opts = optionsOf(options);
    const source = String(text == null ? "" : text);
    const target = Math.trunc(Number(width));
    if (!Number.isFinite(target)) return source;
    const current = displayWidth(source, opts);
    if (current >= target) return truncate(source, target, opts);
    return source + " ".repeat(target - current);
  }

  return {
    DEFAULT_TAB_SIZE,
    MIN_TAB_SIZE,
    MAX_TAB_SIZE,
    displayWidth,
    expandTabs,
    nextTabStop,
    normalizeTabSize,
    truncate,
    padRight,
  };
});
