"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const T = require("../theme.js");

// A minimal Storage stand-in so the persistence helpers can be exercised
// without a browser.
function fakeStorage(seed) {
  const values = new Map(Object.entries(seed || {}));
  return {
    getItem: (key) => (values.has(key) ? values.get(key) : null),
    setItem: (key, value) => values.set(key, String(value)),
    removeItem: (key) => values.delete(key),
    raw: values,
  };
}

test("contrastRatio matches the WCAG anchors", () => {
  assert.equal(T.contrastRatio("#000", "#fff"), 21);
  assert.equal(T.contrastRatio("#ffffff", "#ffffff"), 1);
  assert.equal(T.contrastRatio("#fff", "#000"), T.contrastRatio("#000", "#fff"));
  // The Ayame light accent against white text is a real, measured value.
  assert.equal(T.contrastRatio("#fff", "#7a5cc0"), 5.1);
});

test("relativeLuminance runs from black to white", () => {
  assert.equal(T.relativeLuminance(T.parseColor("#000")), 0);
  assert.equal(T.relativeLuminance(T.parseColor("#fff")), 1);
});

test("parseColor reads the forms a token or colour picker produces", () => {
  assert.deepEqual(T.parseColor("#abc"), { r: 170, g: 187, b: 204, a: 1 });
  assert.deepEqual(T.parseColor("#aabbcc"), { r: 170, g: 187, b: 204, a: 1 });
  assert.deepEqual(T.parseColor("#aabbcc80"), { r: 170, g: 187, b: 204, a: 128 / 255 });
  assert.deepEqual(T.parseColor("rgb(1 2 3)"), { r: 1, g: 2, b: 3, a: 1 });
  assert.deepEqual(T.parseColor("rgba(1, 2, 3, 0.5)"), { r: 1, g: 2, b: 3, a: 0.5 });
  assert.deepEqual(T.parseColor("rgb(50% 0% 0%)"), { r: 127.5, g: 0, b: 0, a: 1 });
  assert.deepEqual(T.parseColor("transparent"), { r: 0, g: 0, b: 0, a: 0 });
  assert.equal(T.parseColor("color-mix(in srgb, red, blue)"), null);
  assert.equal(T.parseColor(""), null);
});

test("resolveColor follows var() and the palette's color-mix wash", () => {
  const tokens = T.effectiveTokens(T.createTheme("t", "light", {
    "--success": "#000000",
    "--add-bg": "color-mix(in srgb, var(--success) 50%, transparent)",
  }));
  // Half of black over the cream ground is a mid grey with alpha 0.5.
  const wash = T.resolveColor("var(--add-bg)", tokens);
  assert.equal(wash.a, 0.5);
  assert.equal(T.resolveColor("var(--accent-bright)", tokens).r, 106);
  assert.equal(T.resolveColor("var(--no-such-token)", tokens), null);
  assert.equal(T.resolveColor("linear-gradient(#000, #fff)", tokens), null);
});

test("a translucent wash is measured over the ground it is painted on", () => {
  const black50 = { r: 0, g: 0, b: 0, a: 0.5 };
  const white = { r: 255, g: 255, b: 255, a: 1 };
  const grey = T.composite(black50, white);
  assert.equal(grey.a, 1);
  assert.equal(Math.round(grey.r), 128);
  // contrastRatio works on opaque colours only, so the un-composited black
  // would read as a 21:1 match for white; compositing is what makes the tint
  // measurable (3.98:1 here).
  assert.equal(T.contrastRatio(grey, white), 3.98);
  assert.notEqual(T.contrastRatio(black50, white), T.contrastRatio(grey, white));
});

test("checkContrast reports every implied pair and its thresholds", () => {
  const pairs = T.contrastPairs();
  const keys = pairs.map((pair) => `${pair.fg}|${pair.bg}`);
  for (const expected of ["--fg|--bg", "--on-accent|--accent", "--add-fg|--add-bg", "--del-fg|--del-bg", "--chg-fg|--chg-bg", "--move-fg|--move-bg"]) {
    assert.ok(keys.includes(expected), `${expected} is not validated`);
  }
  const light = T.checkContrast(T.effectiveTokens(T.createTheme("t", "light", {})));
  assert.equal(light.length, pairs.length);
  for (const row of light) {
    assert.equal(row.ratio !== null, true, `${row.fg} on ${row.bg} did not resolve`);
    assert.equal(row.aa, true, `${row.fg} on ${row.bg} unexpectedly fails AA in the default light palette`);
  }
});

test("checkContrast flags a pair a custom theme made unreadable", () => {
  const theme = T.createTheme("bad", "light", { "--add-fg": "#fbf8f1", "--add-bg": "#fdfcf8" });
  const rows = T.checkContrast(T.effectiveTokens(theme));
  const add = rows.find((row) => row.fg === "--add-fg" && row.bg === "--add-bg");
  assert.ok(add.ratio < T.AA_NORMAL, `expected a low ratio, got ${add.ratio}`);
  assert.equal(add.aa, false);
  assert.equal(add.large, false);
});

test("effectiveTokens merges overrides onto the chosen base", () => {
  const theme = T.createTheme("t", "dark", { "--bg": "#101010" });
  const tokens = T.effectiveTokens(theme);
  assert.equal(tokens["--bg"], "#101010");
  assert.equal(tokens["--fg"], T.DEFAULTS.dark["--fg"]);
  // The base map itself is never mutated.
  assert.equal(T.DEFAULTS.dark["--bg"], "#1e1e1e");
});

test("validateOverrides rejects unknown keys, bad values, and oversize themes", () => {
  assert.deepEqual(T.validateOverrides(undefined), {});
  assert.deepEqual(T.validateOverrides({ "--bg": "#fff" }), { "--bg": "#fff" });
  assert.throws(() => T.validateOverrides({ "--made-up": "#fff" }), (error) => error.code === "THEME_UNKNOWN_TOKEN");
  assert.throws(() => T.validateOverrides({ "--bg": "" }), (error) => error.code === "THEME_INVALID_VALUE");
  assert.throws(() => T.validateOverrides({ "--bg": 12 }), (error) => error.code === "THEME_INVALID_VALUE");
  const many = {};
  for (let i = 0; i < T.MAX_TOKENS + 1; i += 1) many[`--x${i}`] = "#fff";
  assert.throws(() => T.validateOverrides(many), (error) => error.code === "THEME_TOO_LARGE");
});

test("a theme round-trips through JSON", () => {
  const theme = T.createTheme("Share me", "light", { "--bg": "#ffffff", "--fg": "#000000" });
  const text = T.serializeTheme(theme);
  assert.deepEqual(T.parseTheme(text), { ok: true, theme });
  assert.deepEqual(T.parseTheme(JSON.stringify(theme)).theme.tokens, { "--bg": "#ffffff", "--fg": "#000000" });
  // An export without a version or a name still normalizes.
  const bare = T.parseTheme('{"tokens":{"--bg":"#000"}}');
  assert.equal(bare.ok, true);
  assert.equal(bare.theme.base, "light");
  assert.equal(bare.theme.v, T.VERSION);
});

test("parseTheme reports why a shared theme was rejected", () => {
  assert.equal(T.parseTheme("{not json").code, "THEME_INVALID_JSON");
  assert.equal(T.parseTheme('["array"]').code, "THEME_INVALID_SHAPE");
  assert.equal(T.parseTheme('{"tokens":{"--nope":"#fff"}}').code, "THEME_UNKNOWN_TOKEN");
  assert.equal(T.parseTheme(`{"v":${T.VERSION + 1},"tokens":{}}`).code, "THEME_UNSUPPORTED_VERSION");
  assert.equal(T.parseTheme("x".repeat(T.MAX_THEME_BYTES + 1)).code, "THEME_TOO_LARGE");
});

test("every shipped preset is a valid, complete colour theme", () => {
  const presets = T.presetList();
  assert.ok(presets.length >= 6);
  const ids = new Set();
  for (const preset of presets) {
    assert.ok(preset.name.length > 0);
    const tokens = T.effectiveTokens(preset);
    for (const key of T.TOKEN_KEYS) assert.ok(typeof tokens[key] === "string" && tokens[key].length > 0, `${preset.name} is missing ${key}`);
    assert.equal(T.parseTheme(T.serializeTheme(preset)).ok, true);
    assert.ok(ids.add(preset.name));
  }
  assert.equal(T.presetById("solarized-light").base, "light");
  assert.equal(T.presetById("vscode-dark").base, "dark");
  assert.equal(T.presetById("missing"), null);
});

test("named themes are stored, listed, and deleted in a storage stub", () => {
  const storage = fakeStorage();
  assert.deepEqual(T.loadThemeList(storage), {});
  const saved = T.saveNamedTheme(storage, T.createTheme("Team", "light", { "--bg": "#000" }));
  assert.equal(saved.name, "Team");
  assert.deepEqual(Object.keys(T.loadThemeList(storage)), ["Team"]);
  T.saveNamedTheme(storage, T.createTheme("Team", "light", { "--bg": "#111" }));
  assert.equal(T.loadThemeList(storage).Team.tokens["--bg"], "#111");
  T.deleteNamedTheme(storage, "Team");
  assert.deepEqual(T.loadThemeList(storage), {});
  assert.throws(() => T.saveNamedTheme(storage, T.createTheme("", "light", {})), (error) => error.code === "THEME_NAME_REQUIRED");
});

test("a damaged shelf entry is skipped rather than breaking the rest", () => {
  const storage = fakeStorage({
    [T.LIST_KEY]: JSON.stringify({ Good: { tokens: { "--bg": "#000" } }, Broken: { tokens: { "--nope": "#fff" } } }),
  });
  const list = T.loadThemeList(storage);
  assert.deepEqual(Object.keys(list), ["Good"]);
});

test("the active theme persists and clears", () => {
  const storage = fakeStorage();
  assert.equal(T.loadActiveTheme(storage), null);
  T.saveActiveTheme(storage, T.createTheme("Current", "dark", { "--bg": "#000" }));
  assert.equal(T.loadActiveTheme(storage).base, "dark");
  T.clearActiveTheme(storage);
  assert.equal(T.loadActiveTheme(storage), null);
});

test("sameTokens compares override maps regardless of key order", () => {
  assert.equal(T.sameTokens({ "--bg": "#000", "--fg": "#fff" }, { "--fg": "#fff", "--bg": "#000" }), true);
  assert.equal(T.sameTokens({ "--bg": "#000" }, { "--bg": "#111" }), false);
  assert.equal(T.sameTokens(null, {}), true);
});
