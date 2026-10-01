// Theme model for the GUI (#286). Pure logic only: the token catalogue, the
// merge/validation rules for an imported theme, and the WCAG contrast maths.
// The DOM wiring lives in app.js; keeping the maths here lets it run under
// node --test and guards against the #150 class of bug, where a user-picked
// foreground and its tinted cell background end up the same colour.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameTheme = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  const VERSION = 1;
  const CUSTOM_SCHEME = "custom";
  const ACTIVE_KEY = "ayame-theme-custom";
  const LIST_KEY = "ayame-themes";
  const MAX_TOKENS = 64;
  const MAX_VALUE_LENGTH = 200;
  const MAX_THEME_BYTES = 64 * 1024;

  // WCAG 2.1 thresholds. Normal body text is 13px in this UI, so 4.5 is the
  // bar that matters; the others are reported so a large or decorative pair is
  // not called a failure when it meets the 3:1 large-text rule.
  const AA_NORMAL = 4.5;
  const AA_LARGE = 3.0;
  const AAA_NORMAL = 7.0;

  // The editable subset of tokens.css, grouped the way the grains of the UI
  // group: the ground, the accent/status ramp shared by chrome, the diff
  // palette that carries meaning, and the type tokens CJK display leans on.
  const TOKEN_GROUPS = [
    {
      id: "ground",
      tokens: [
        { key: "--bg", kind: "color" },
        { key: "--bg-elevated", kind: "color" },
        { key: "--fg", kind: "color" },
        { key: "--fg-dim", kind: "color" },
        { key: "--border", kind: "color" },
      ],
    },
    {
      id: "accent",
      tokens: [
        { key: "--accent", kind: "color" },
        { key: "--accent-bright", kind: "color" },
        { key: "--on-accent", kind: "color" },
        { key: "--danger", kind: "color" },
        { key: "--success", kind: "color" },
        { key: "--gold", kind: "color" },
      ],
    },
    {
      id: "diff",
      tokens: [
        { key: "--add-bg", kind: "color" },
        { key: "--add-fg", kind: "color" },
        { key: "--del-bg", kind: "color" },
        { key: "--del-fg", kind: "color" },
        { key: "--chg-bg", kind: "color" },
        { key: "--chg-fg", kind: "color" },
        { key: "--word-add", kind: "color" },
        { key: "--word-del", kind: "color" },
        { key: "--move-bg", kind: "color" },
        { key: "--move-fg", kind: "color" },
      ],
    },
    {
      id: "type",
      tokens: [
        { key: "--ui", kind: "font" },
        { key: "--mono", kind: "font" },
        { key: "--fs-ui", kind: "length" },
        { key: "--fs-data", kind: "length" },
        { key: "--fs-label", kind: "length" },
        { key: "--fs-caption", kind: "length" },
      ],
    },
  ];

  const TOKEN_KEYS = [];
  for (const group of TOKEN_GROUPS) for (const token of group.tokens) TOKEN_KEYS.push(token.key);
  const TOKEN_KEY_SET = new Set(TOKEN_KEYS);

  // Effective token values, mirroring tokens.css. The Go test
  // TestThemeModuleTokensMatchStylesheet re-reads the stylesheet and fails if
  // these drift, so the stylesheet stays the source of truth and this table is
  // only a model of it. Dark inherits the washes and the accent-derived move
  // colours unchanged, exactly as the cascade does.
  const LIGHT = {
    "--bg": "#fbf8f1",
    "--bg-elevated": "#fdfcf8",
    "--fg": "#2a2140",
    "--fg-dim": "#6e6383",
    "--border": "#e7e0d3",
    "--accent": "#7a5cc0",
    "--accent-bright": "#6a4cb0",
    "--on-accent": "#fff",
    "--danger": "#ca564f",
    "--success": "#4c9b45",
    "--gold": "#c79a2e",
    "--add-bg": "color-mix(in srgb, var(--success) 14%, transparent)",
    "--add-fg": "#296f23",
    "--del-bg": "color-mix(in srgb, var(--danger) 13%, transparent)",
    "--del-fg": "#993935",
    "--chg-bg": "color-mix(in srgb, var(--gold) 12%, transparent)",
    "--chg-fg": "#76570e",
    "--word-add": "color-mix(in srgb, var(--success) 55%, transparent)",
    "--word-del": "color-mix(in srgb, var(--danger) 55%, transparent)",
    "--move-bg": "color-mix(in srgb, var(--accent) 16%, transparent)",
    "--move-fg": "var(--accent-bright)",
    "--ui": "\"Segoe UI\", \"Hiragino Kaku Gothic ProN\", \"Noto Sans JP\", system-ui, sans-serif",
    "--mono": "\"SFMono-Regular\", Menlo, Consolas, \"DejaVu Sans Mono\", \"Noto Sans Mono CJK JP\", \"MS Gothic\", monospace",
    "--fs-ui": "13px",
    "--fs-data": "13px",
    "--fs-label": "12px",
    "--fs-caption": "11px",
  };

  const DARK = Object.assign({}, LIGHT, {
    "--bg": "#1e1e1e",
    "--bg-elevated": "#252526",
    "--fg": "#d4d4d4",
    "--fg-dim": "#9a9a9a",
    "--border": "#3c3c3c",
    "--accent": "#9b82d8",
    "--accent-bright": "#b49de6",
    "--danger": "#e66f66",
    "--success": "#5fae57",
    "--gold": "#d9b45a",
    "--add-fg": "#84c47d",
    "--del-fg": "#f49289",
    "--chg-fg": "#e4c46d",
    "--word-add": "color-mix(in srgb, var(--success) 40%, transparent)",
    "--word-del": "color-mix(in srgb, var(--danger) 40%, transparent)",
  });

  const DEFAULTS = { light: LIGHT, dark: DARK };

  // The foreground/background pairs the tokens imply. A wash is translucent, so
  // it is composited over the opaque ground it is painted on before the ratio
  // is taken; otherwise every tint would look like a near-match for white.
  const CONTRAST_PAIRS = [
    { fg: "--fg", bg: "--bg" },
    { fg: "--fg-dim", bg: "--bg" },
    { fg: "--on-accent", bg: "--accent" },
    { fg: "--add-fg", bg: "--add-bg", ground: "--bg" },
    { fg: "--del-fg", bg: "--del-bg", ground: "--bg" },
    { fg: "--chg-fg", bg: "--chg-bg", ground: "--bg" },
    { fg: "--move-fg", bg: "--move-bg", ground: "--bg" },
  ];

  const PRESETS = [
    { id: "ayame-light", name: "Ayame Light", base: "light", tokens: {} },
    { id: "ayame-dark", name: "Ayame Dark", base: "dark", tokens: {} },
    {
      id: "winmerge",
      name: "WinMerge",
      base: "light",
      tokens: {
        "--bg": "#ffffff",
        "--bg-elevated": "#f2f2f2",
        "--fg": "#1a1a1a",
        "--fg-dim": "#5a5a5a",
        "--border": "#c8c8c8",
        "--accent": "#2f5fb0",
        "--accent-bright": "#24488a",
        "--on-accent": "#ffffff",
        "--danger": "#b22222",
        "--success": "#1a7f1a",
        "--gold": "#7a6300",
        "--add-bg": "#ccffcc",
        "--add-fg": "#145a14",
        "--del-bg": "#ffd6d6",
        "--del-fg": "#8b1a1a",
        "--chg-bg": "#fff4cc",
        "--chg-fg": "#6b5600",
        "--word-add": "#99ee99",
        "--word-del": "#ffb0b0",
        "--move-bg": "#dcdcff",
        "--move-fg": "#333399",
      },
    },
    {
      id: "vscode-dark",
      name: "VS Code Dark+",
      base: "dark",
      tokens: {
        "--bg": "#1e1e1e",
        "--bg-elevated": "#252526",
        "--fg": "#d4d4d4",
        "--fg-dim": "#9a9a9a",
        "--border": "#3c3c3c",
        "--accent": "#569cd6",
        "--accent-bright": "#79b8ff",
        "--on-accent": "#0b0b0b",
        "--danger": "#f44747",
        "--success": "#4ec9b0",
        "--gold": "#dcdcaa",
        "--add-bg": "#17351d",
        "--add-fg": "#b5cea8",
        "--del-bg": "#3a1d1d",
        "--del-fg": "#f48771",
        "--chg-bg": "#37331c",
        "--chg-fg": "#e6dc9a",
        "--word-add": "#2d5a2d",
        "--word-del": "#5a2d2d",
        "--move-bg": "#232347",
        "--move-fg": "#9cdcfe",
      },
    },
    {
      id: "solarized-light",
      name: "Solarized Light",
      base: "light",
      tokens: {
        "--bg": "#fdf6e3",
        "--bg-elevated": "#eee8d5",
        "--fg": "#073642",
        "--fg-dim": "#5a6b6f",
        "--border": "#d5cdb6",
        "--accent": "#1f6fa8",
        "--accent-bright": "#155c8c",
        "--on-accent": "#fdf6e3",
        "--danger": "#c02a27",
        "--success": "#5f7300",
        "--gold": "#8a6a00",
        "--add-bg": "#e6f0d8",
        "--add-fg": "#3f5a00",
        "--del-bg": "#f7dfdc",
        "--del-fg": "#9c211e",
        "--chg-bg": "#f5ecd0",
        "--chg-fg": "#6d5300",
        "--word-add": "#c8dfa8",
        "--word-del": "#f0c4bf",
        "--move-bg": "#d6e6f5",
        "--move-fg": "#155c8c",
      },
    },
    {
      id: "high-contrast",
      name: "High contrast",
      base: "dark",
      tokens: {
        "--bg": "#000000",
        "--bg-elevated": "#0a0a0a",
        "--fg": "#ffffff",
        "--fg-dim": "#d0d0d0",
        "--border": "#ffffff",
        "--accent": "#ffff00",
        "--accent-bright": "#00ffff",
        "--on-accent": "#000000",
        "--danger": "#ff6b6b",
        "--success": "#6bff6b",
        "--gold": "#ffff66",
        "--add-bg": "#003300",
        "--add-fg": "#b6ffb6",
        "--del-bg": "#330000",
        "--del-fg": "#ffb6b6",
        "--chg-bg": "#333300",
        "--chg-fg": "#ffffb6",
        "--word-add": "#006600",
        "--word-del": "#660000",
        "--move-bg": "#000066",
        "--move-fg": "#b6b6ff",
      },
    },
  ];

  const NAMED_COLORS = {
    transparent: { r: 0, g: 0, b: 0, a: 0 },
    black: { r: 0, g: 0, b: 0, a: 1 },
    white: { r: 255, g: 255, b: 255, a: 1 },
  };

  function clamp(value, min, max) {
    return value < min ? min : value > max ? max : value;
  }

  function round(value, places) {
    const factor = 10 ** places;
    return Math.round(value * factor) / factor;
  }

  function hexPair(hex) {
    return parseInt(hex, 16);
  }

  // parseColor understands the forms tokens.css and a colour picker produce:
  // hex, rgb()/rgba(), the transparent keyword, and a few named colours. It
  // returns null for anything else (a var(), a color-mix(), a gradient) so the
  // caller can resolve references first or decline to score a pair.
  function parseColor(value) {
    if (!value || typeof value !== "string") return null;
    const text = value.trim().toLowerCase();
    if (NAMED_COLORS[text]) return Object.assign({}, NAMED_COLORS[text]);
    if (text.charAt(0) === "#") {
      const hex = text.slice(1);
      if (/^[0-9a-f]{3}$/.test(hex)) {
        return {
          r: hexPair(hex.charAt(0) + hex.charAt(0)),
          g: hexPair(hex.charAt(1) + hex.charAt(1)),
          b: hexPair(hex.charAt(2) + hex.charAt(2)),
          a: 1,
        };
      }
      if (/^[0-9a-f]{4}$/.test(hex)) {
        return {
          r: hexPair(hex.charAt(0) + hex.charAt(0)),
          g: hexPair(hex.charAt(1) + hex.charAt(1)),
          b: hexPair(hex.charAt(2) + hex.charAt(2)),
          a: hexPair(hex.charAt(3) + hex.charAt(3)) / 255,
        };
      }
      if (/^[0-9a-f]{6}$/.test(hex)) {
        return { r: hexPair(hex.slice(0, 2)), g: hexPair(hex.slice(2, 4)), b: hexPair(hex.slice(4, 6)), a: 1 };
      }
      if (/^[0-9a-f]{8}$/.test(hex)) {
        return {
          r: hexPair(hex.slice(0, 2)),
          g: hexPair(hex.slice(2, 4)),
          b: hexPair(hex.slice(4, 6)),
          a: hexPair(hex.slice(6, 8)) / 255,
        };
      }
      return null;
    }
    const rgb = /^rgba?\((.*)\)$/.exec(text);
    if (!rgb) return null;
    const body = rgb[1].replace(/\//g, " ").replace(/,/g, " ");
    const parts = body.split(/\s+/).filter(Boolean);
    if (parts.length < 3 || parts.length > 4) return null;
    const channel = (part) => {
      const percent = /%$/.test(part);
      const number = parseFloat(part);
      if (!Number.isFinite(number)) return null;
      return percent ? (number / 100) * 255 : number;
    };
    const r = channel(parts[0]);
    const g = channel(parts[1]);
    const b = channel(parts[2]);
    if (r === null || g === null || b === null) return null;
    let a = 1;
    if (parts.length === 4) {
      const parsed = parseFloat(parts[3]);
      if (!Number.isFinite(parsed)) return null;
      a = parsed <= 1 ? parsed : parsed / 255;
    }
    return { r: clamp(r, 0, 255), g: clamp(g, 0, 255), b: clamp(b, 0, 255), a: clamp(a, 0, 1) };
  }

  // resolveColor follows var() references and the one color-mix() shape the
  // palette uses (`in srgb, <color> N%, transparent`). Anything else is left
  // unresolved rather than guessed at.
  function resolveColor(value, tokens, depth) {
    if ((depth || 0) > 8) return null;
    if (typeof value !== "string") return null;
    const text = value.trim();
    const reference = /^var\(\s*(--[\w-]+)\s*\)$/.exec(text);
    if (reference) return resolveColor(tokens ? tokens[reference[1]] : undefined, tokens, (depth || 0) + 1);
    const direct = parseColor(text);
    if (direct) return direct;
    const mix = /^color-mix\(\s*in\s+srgb\s*,\s*(.+)\)$/i.exec(text);
    if (!mix) return null;
    const body = mix[1];
    // Split on the top-level comma that separates the two stops.
    let comma = -1;
    let depthParen = 0;
    for (let i = 0; i < body.length; i += 1) {
      const char = body.charAt(i);
      if (char === "(") depthParen += 1;
      else if (char === ")") depthParen -= 1;
      else if (char === "," && depthParen === 0) { comma = i; break; }
    }
    if (comma < 0) return null;
    const stops = [body.slice(0, comma), body.slice(comma + 1)];
    const parsed = stops.map((stop) => {
      const percent = /\s+([\d.]+)%\s*$/.exec(stop);
      const colorText = percent ? stop.slice(0, percent.index) : stop;
      return { color: resolveColor(colorText, tokens, (depth || 0) + 1), weight: percent ? parseFloat(percent[1]) / 100 : null };
    });
    if (!parsed[0].color || !parsed[1].color) return null;
    let weight = parsed[0].weight;
    if (weight === null) weight = parsed[1].weight === null ? 0.5 : 1 - parsed[1].weight;
    return mixColors(parsed[0].color, parsed[1].color, weight);
  }

  // mixColors interpolates with premultiplied alpha, the way CSS color-mix does,
  // so mixing a colour into transparent keeps the colour and only lowers alpha.
  function mixColors(a, b, weightA) {
    const weightB = 1 - weightA;
    const alpha = a.a * weightA + b.a * weightB;
    if (alpha <= 0) return { r: 0, g: 0, b: 0, a: 0 };
    return {
      r: (a.r * a.a * weightA + b.r * b.a * weightB) / alpha,
      g: (a.g * a.a * weightA + b.g * b.a * weightB) / alpha,
      b: (a.b * a.a * weightA + b.b * b.a * weightB) / alpha,
      a: alpha,
    };
  }

  // composite lays a translucent colour over an opaque ground. The palette's
  // washes are declared as translucent tints, so this is what makes their
  // contrast measurable at all.
  function composite(fg, bg) {
    if (!fg) return bg;
    if (!bg || bg.a >= 1) {
      if (bg && bg.a >= 1) {
        return {
          r: fg.r * fg.a + bg.r * (1 - fg.a),
          g: fg.g * fg.a + bg.g * (1 - fg.a),
          b: fg.b * fg.a + bg.b * (1 - fg.a),
          a: 1,
        };
      }
      return fg;
    }
    const over = composite(fg, { r: bg.r, g: bg.g, b: bg.b, a: 1 });
    return {
      r: over.r * bg.a + bg.r * (1 - bg.a),
      g: over.g * bg.a + bg.g * (1 - bg.a),
      b: over.b * bg.a + bg.b * (1 - bg.a),
      a: 1,
    };
  }

  function linearize(component) {
    const c = component / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  }

  function relativeLuminance(color) {
    return 0.2126 * linearize(color.r) + 0.7152 * linearize(color.g) + 0.0722 * linearize(color.b);
  }

  function asColor(value) {
    if (value && typeof value === "object" && typeof value.r === "number") return value;
    return parseColor(value);
  }

  // formatColor renders an opaque #rrggbb, the only form an <input type=color>
  // can hold. Alpha is dropped: the swatch is a picker, not storage.
  function formatColor(color) {
    const c = asColor(color);
    if (!c) return null;
    const channel = (value) => Math.round(clamp(value, 0, 255)).toString(16).padStart(2, "0");
    return `#${channel(c.r)}${channel(c.g)}${channel(c.b)}`;
  }

  // contrastRatio returns the WCAG ratio between two colours, 1.0 to 21.0.
  // Opaque colours are expected; callers composite translucent ones first.
  function contrastRatio(first, second) {
    const a = asColor(first);
    const b = asColor(second);
    if (!a || !b) return null;
    const la = relativeLuminance(a);
    const lb = relativeLuminance(b);
    const lighter = Math.max(la, lb);
    const darker = Math.min(la, lb);
    return round((lighter + 0.05) / (darker + 0.05), 2);
  }

  function contrastPairs() {
    return CONTRAST_PAIRS.map((pair) => Object.assign({}, pair));
  }

  // checkContrast resolves every foreground/background pair against an
  // effective token map and reports the ratio and which thresholds it meets.
  // A pair the resolver cannot follow is returned with ratio null rather than
  // silently dropped, so the UI can say so instead of showing a false pass.
  function checkContrast(tokens) {
    const map = tokens || LIGHT;
    return CONTRAST_PAIRS.map((pair) => {
      const fg = resolveColor(`var(${pair.fg})`, map);
      let bg = resolveColor(`var(${pair.bg})`, map);
      if (bg && pair.ground) bg = composite(bg, resolveColor(`var(${pair.ground})`, map));
      const ratio = fg && bg ? contrastRatio(fg, bg) : null;
      return {
        fg: pair.fg,
        bg: pair.bg,
        ratio,
        aa: ratio !== null && ratio >= AA_NORMAL,
        aaa: ratio !== null && ratio >= AAA_NORMAL,
        large: ratio !== null && ratio >= AA_LARGE,
      };
    });
  }

  function normalizeBase(base) {
    return base === "dark" ? "dark" : base === "system" ? "system" : "light";
  }

  function cloneTokens(tokens) {
    const copy = {};
    for (const key of Object.keys(tokens || {})) copy[key] = tokens[key];
    return copy;
  }

  function defaultTokens(base) {
    return cloneTokens(DEFAULTS[normalizeBase(base) === "dark" ? "dark" : "light"]);
  }

  function effectiveTokens(theme) {
    const tokens = defaultTokens(theme && theme.base);
    const overrides = theme && theme.tokens ? theme.tokens : {};
    for (const key of Object.keys(overrides)) tokens[key] = overrides[key];
    return tokens;
  }

  function themeError(code, message) {
    const error = new Error(message || code);
    error.code = code;
    return error;
  }

  function validateOverrides(tokens) {
    if (tokens === undefined || tokens === null) return {};
    if (typeof tokens !== "object" || Array.isArray(tokens)) throw themeError("THEME_INVALID_SHAPE", "theme tokens must be an object");
    const keys = Object.keys(tokens);
    if (keys.length > MAX_TOKENS) throw themeError("THEME_TOO_LARGE", "theme has too many tokens");
    const clean = {};
    for (const key of keys) {
      if (!TOKEN_KEY_SET.has(key)) throw themeError("THEME_UNKNOWN_TOKEN", `unknown theme token: ${key}`);
      const value = tokens[key];
      if (typeof value !== "string" || value.length === 0) throw themeError("THEME_INVALID_VALUE", `invalid value for ${key}`);
      if (value.length > MAX_VALUE_LENGTH) throw themeError("THEME_VALUE_TOO_LONG", `value for ${key} is too long`);
      clean[key] = value;
    }
    return clean;
  }

  function normalizeTheme(theme) {
    if (!theme || typeof theme !== "object" || Array.isArray(theme)) {
      throw themeError("THEME_INVALID_SHAPE", "a theme must be an object");
    }
    if (theme.v !== undefined && theme.v !== VERSION) throw themeError("THEME_UNSUPPORTED_VERSION", "unsupported theme version");
    const name = typeof theme.name === "string" ? theme.name.slice(0, 120) : "";
    return {
      v: VERSION,
      name,
      base: normalizeBase(theme.base),
      tokens: validateOverrides(theme.tokens),
    };
  }

  function createTheme(name, base, tokens) {
    return normalizeTheme({ v: VERSION, name: name || "", base: base || "light", tokens: tokens || {} });
  }

  function serializeTheme(theme) {
    return JSON.stringify(normalizeTheme(theme));
  }

  function parseTheme(text) {
    if (typeof text !== "string") return { ok: false, code: "THEME_INVALID_JSON", error: "theme must be a JSON string" };
    if (text.length > MAX_THEME_BYTES) return { ok: false, code: "THEME_TOO_LARGE", error: "theme is too large" };
    let parsed;
    try {
      parsed = JSON.parse(text);
    } catch (_) {
      return { ok: false, code: "THEME_INVALID_JSON", error: "theme is not valid JSON" };
    }
    try {
      return { ok: true, theme: normalizeTheme(parsed) };
    } catch (error) {
      return { ok: false, code: error.code || "THEME_INVALID_SHAPE", error: error.message };
    }
  }

  function presetList() {
    return PRESETS.map((preset) => Object.assign({ id: preset.id }, createTheme(preset.name, preset.base, preset.tokens)));
  }

  function presetById(id) {
    const preset = PRESETS.find((entry) => entry.id === id);
    return preset ? createTheme(preset.name, preset.base, preset.tokens) : null;
  }

  function sameTokens(a, b) {
    const left = a || {};
    const right = b || {};
    const keys = new Set(Object.keys(left).concat(Object.keys(right)));
    for (const key of keys) if (left[key] !== right[key]) return false;
    return true;
  }

  // ---- Named-theme storage. A storage object is passed in so the module stays
  // independent of the browser and the round-trip is testable with a stub.

  function readJSON(storage, key, fallback) {
    if (!storage) return fallback;
    try {
      const text = storage.getItem(key);
      if (!text) return fallback;
      const value = JSON.parse(text);
      return value === null || value === undefined ? fallback : value;
    } catch (_) {
      return fallback;
    }
  }

  function loadThemeList(storage) {
    const raw = readJSON(storage, LIST_KEY, {});
    if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};
    const list = {};
    for (const name of Object.keys(raw)) {
      try {
        list[name] = normalizeTheme(raw[name]);
      } catch (_) {
        // A damaged entry is skipped rather than taking the whole shelf down.
      }
    }
    return list;
  }

  function storeThemeList(storage, list) {
    if (!storage) return;
    storage.setItem(LIST_KEY, JSON.stringify(list || {}));
  }

  function saveNamedTheme(storage, theme) {
    const normalized = normalizeTheme(theme);
    if (!normalized.name) throw themeError("THEME_NAME_REQUIRED", "a saved theme needs a name");
    const list = loadThemeList(storage);
    list[normalized.name] = normalized;
    storeThemeList(storage, list);
    return normalized;
  }

  function deleteNamedTheme(storage, name) {
    const list = loadThemeList(storage);
    delete list[name];
    storeThemeList(storage, list);
    return list;
  }

  function loadActiveTheme(storage) {
    const raw = readJSON(storage, ACTIVE_KEY, null);
    if (!raw) return null;
    try {
      return normalizeTheme(raw);
    } catch (_) {
      return null;
    }
  }

  function saveActiveTheme(storage, theme) {
    if (!storage) return;
    storage.setItem(ACTIVE_KEY, serializeTheme(theme));
  }

  function clearActiveTheme(storage) {
    if (storage) storage.removeItem(ACTIVE_KEY);
  }

  return {
    VERSION,
    CUSTOM_SCHEME,
    ACTIVE_KEY,
    LIST_KEY,
    MAX_TOKENS,
    MAX_VALUE_LENGTH,
    MAX_THEME_BYTES,
    AA_NORMAL,
    AA_LARGE,
    AAA_NORMAL,
    TOKEN_GROUPS,
    TOKEN_KEYS,
    DEFAULTS,
    CONTRAST_PAIRS,
    parseColor,
    formatColor,
    resolveColor,
    mixColors,
    composite,
    relativeLuminance,
    contrastRatio,
    contrastPairs,
    checkContrast,
    normalizeBase,
    defaultTokens,
    effectiveTokens,
    validateOverrides,
    normalizeTheme,
    createTheme,
    serializeTheme,
    parseTheme,
    presetList,
    presetById,
    sameTokens,
    loadThemeList,
    storeThemeList,
    saveNamedTheme,
    deleteNamedTheme,
    loadActiveTheme,
    saveActiveTheme,
    clearActiveTheme,
  };
});
