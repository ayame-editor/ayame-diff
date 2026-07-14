// Lightweight, line-local syntax highlighting for the visible diff rows.
// It deliberately keeps no parser state and never scans an entire file.
(function (root) {
  "use strict";

  const EXT_LANGUAGE = {
    cjs: "javascript", css: "css", go: "go", htm: "html", html: "html",
    js: "javascript", json: "json", jsonc: "json", jsx: "javascript",
    log: "log", mjs: "javascript", md: "markdown", mdx: "markdown",
    py: "python", rs: "rust", sh: "shell", sql: "sql", ts: "javascript",
    tsx: "javascript", yaml: "yaml", yml: "yaml",
  };
  const COMMON_LITERALS = new Set(["false", "nil", "null", "None", "true"]);
  const KEYWORDS = {
    go: new Set("break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var".split(" ")),
    javascript: new Set("as async await break case catch class const continue default delete do else export extends finally for from function if import in instanceof interface let new of return switch throw try type typeof var void while with yield".split(" ")),
    python: new Set("and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield".split(" ")),
    rust: new Set("as async await break const continue crate dyn else enum extern fn for if impl in let loop match mod move mut pub ref return self Self static struct super trait type unsafe use where while".split(" ")),
    shell: new Set("case do done elif else esac export fi for function if in local set then while".split(" ")),
    sql: new Set("alter and as by case create delete desc distinct drop else end from group having in inner insert into is join left like limit not null on or order outer right select set table then union update values when where".split(" ")),
  };

  function basename(path) {
    const normalized = String(path || "").replaceAll("\\", "/");
    return normalized.slice(normalized.lastIndexOf("/") + 1);
  }
  const languageCache = new Map();
  function languageForPath(path) {
    // highlightSpans runs per line, but the path is constant for a whole diff
    // side, so memoize the resolution instead of re-deriving basename/extension
    // for every line. A diff has at most a couple of paths, so the cache stays
    // tiny; clear it if it somehow grows (e.g. a folder diff of many files).
    if (languageCache.has(path)) return languageCache.get(path);
    const name = basename(path).toLowerCase();
    let lang;
    if (name === "dockerfile" || name === "makefile") lang = "shell";
    else if (name === "cargo.lock" || name === "package-lock.json") lang = "json";
    else if (name === "pnpm-lock.yaml") lang = "yaml";
    else {
      const dot = name.lastIndexOf(".");
      lang = dot < 0 ? null : (EXT_LANGUAGE[name.slice(dot + 1)] || null);
    }
    if (languageCache.size > 64) languageCache.clear();
    languageCache.set(path, lang);
    return lang;
  }

  // Sticky (y) regexes are reused across every line: matching at .lastIndex
  // tokenizes in place, avoiding the O(L^2) substring garbage that text.slice(i)
  // produced once per character in the tokenizer loops below.
  const JSON_LITERAL_RE = /(?:true|false|null)\b/uy;
  const JSON_NUMBER_RE = /-?\d+(?:\.\d+)?(?:e[+-]?\d+)?/iuy;
  const CODE_NUMBER_RE = /(?:0x[\da-f]+|\d+(?:\.\d+)?(?:e[+-]?\d+)?)/iuy;
  const CODE_IDENT_RE = /[A-Za-z_$][\w$]*/uy;
  function stickyMatch(re, text, i) {
    re.lastIndex = i;
    return re.exec(text); // matches only at i (sticky), else null
  }
  function inferLanguage(text) {
    if (/^\s*(TRACE|DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL)\b/u.test(text)) return "log";
    if (/^\s*\d{4}-\d\d-\d\d[T\s]\d\d:\d\d:\d\d/u.test(text)) return "log";
    if (/^\s*[{[]\s*$/u.test(text) || /^\s*"[^"]+"\s*:/u.test(text)) return "json";
    return null;
  }
  function push(out, kind, text) {
    if (!text) return;
    const last = out[out.length - 1];
    if (last && last.kind === kind) last.text += text;
    else out.push({ kind, text });
  }
  function nextNonSpace(text, start) {
    for (let i = start; i < text.length; i++) if (!/\s/u.test(text[i])) return text[i];
    return "";
  }
  function quotedEnd(text, start, quote) {
    let i = start + 1;
    while (i < text.length) {
      if (text[i] === "\\") { i += 2; continue; }
      if (text[i] === quote) return i + 1;
      i++;
    }
    return text.length;
  }
  function commentPrefix(lang) {
    if (lang === "python" || lang === "shell" || lang === "yaml") return "#";
    if (lang === "sql") return "--";
    if (["css", "go", "javascript", "rust"].includes(lang)) return "//";
    return "";
  }

  function jsonSpans(text) {
    const out = [];
    let i = 0;
    while (i < text.length) {
      if (text.startsWith("//", i)) { push(out, "comment", text.slice(i)); break; }
      if (text[i] === '"') {
        const end = quotedEnd(text, i, '"');
        push(out, nextNonSpace(text, end) === ":" ? "key" : "string", text.slice(i, end));
        i = end; continue;
      }
      const literal = stickyMatch(JSON_LITERAL_RE, text, i);
      if (literal) { push(out, "literal", literal[0]); i += literal[0].length; continue; }
      const number = stickyMatch(JSON_NUMBER_RE, text, i);
      if (number) { push(out, "number", number[0]); i += number[0].length; continue; }
      push(out, /^[{}[\],:]+$/u.test(text[i]) ? "op" : "plain", text[i]); i++;
    }
    return out;
  }
  function markdownSpans(text) {
    const out = [];
    const heading = text.match(/^(#{1,6})(\s+.*)$/u);
    if (heading) { push(out, "heading", heading[1]); push(out, "plain", heading[2]); return out; }
    let offset = 0;
    const re = /(`[^`]*`|\[[^\]]+\]\([^)]+\)|https?:\/\/\S+)/gu;
    for (const match of text.matchAll(re)) {
      if (match.index > offset) push(out, "plain", text.slice(offset, match.index));
      push(out, match[0].startsWith("`") ? "string" : "link", match[0]);
      offset = match.index + match[0].length;
    }
    push(out, "plain", text.slice(offset));
    return out;
  }
  function logSpans(text) {
    const out = [];
    const level = text.match(/\b(TRACE|DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL)\b/u);
    if (!level || level.index == null) return [{ kind: "plain", text }];
    push(out, "plain", text.slice(0, level.index));
    const value = level[1];
    const kind = value === "TRACE" || value === "DEBUG" ? "level-debug"
      : value === "INFO" ? "level-info"
        : value === "WARN" || value === "WARNING" ? "level-warn" : "level-error";
    push(out, kind, value);
    push(out, "plain", text.slice(level.index + value.length));
    return out;
  }
  function codeSpans(text, lang) {
    const out = [], keywords = KEYWORDS[lang] || new Set(), lineComment = commentPrefix(lang);
    let i = 0;
    while (i < text.length) {
      if (lineComment && text.startsWith(lineComment, i)) { push(out, "comment", text.slice(i)); break; }
      if (text.startsWith("/*", i)) {
        const close = text.indexOf("*/", i + 2), end = close < 0 ? text.length : close + 2;
        push(out, "comment", text.slice(i, end)); i = end; continue;
      }
      const ch = text[i];
      if (ch === '"' || ch === "'" || (ch === "`" && lang === "javascript")) {
        const end = quotedEnd(text, i, ch); push(out, "string", text.slice(i, end)); i = end; continue;
      }
      const number = stickyMatch(CODE_NUMBER_RE, text, i);
      if (number) { push(out, "number", number[0]); i += number[0].length; continue; }
      const ident = stickyMatch(CODE_IDENT_RE, text, i);
      if (ident) {
        const word = ident[0], lower = word.toLowerCase();
        const kind = keywords.has(word) || keywords.has(lower) ? "keyword"
          : COMMON_LITERALS.has(word) ? "literal"
            : nextNonSpace(text, i + word.length) === "(" ? "function" : "plain";
        push(out, kind, word); i += word.length; continue;
      }
      push(out, /^[{}()[\].,;:+\-*/%=&|!<>?]+$/u.test(ch) ? "op" : "plain", ch); i++;
    }
    return out;
  }
  function highlightSpans(text, path) {
    const lang = languageForPath(path) || inferLanguage(text);
    if (!lang) return null;
    if (lang === "json") return jsonSpans(text);
    if (lang === "markdown") return markdownSpans(text);
    if (lang === "log") return logSpans(text);
    return codeSpans(text, lang);
  }

  const api = { highlightSpans, languageForPath };
  root.AyameSyntax = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
