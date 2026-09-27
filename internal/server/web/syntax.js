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
  // Languages whose multi-line constructs the state machine understands. Every
  // other language returns null from createHighlighter, which is the caller's
  // cue to keep using the unchanged line-local highlightSpans fallback (#287).
  const STREAM_LANGS = new Set(["css", "go", "javascript", "python", "rust", "shell", "sql"]);
  // Strings that can cross a newline. `escaped: false` means a backslash does
  // not escape the closing delimiter (Go raw strings). Ordinary single-line
  // quotes are still handled by the shared tokenizer and are not listed.
  const MULTILINE_STRINGS = {
    go: [{ kind: "string", open: "`", close: "`", escaped: false }],
    javascript: [{ kind: "string", open: "`", close: "`", escaped: true }],
    python: [
      { kind: "string", open: '"""', close: '"""', escaped: false },
      { kind: "string", open: "'''", close: "'''", escaped: false },
    ],
  };
  // `/* ... */` is treated as a block comment for every code language, which
  // matches the line-local tokenizer below even where the language has no such
  // comment (e.g. Python); consistency with the fallback matters more than
  // language purity here (#287).
  const BLOCK_COMMENT = { kind: "comment", close: "*/" };
  // Identifier-style heredoc delimiters only (`EOF`, `_X`), optionally quoted or
  // after `<<-`. Numeric delimiters are excluded so shell arithmetic without
  // spaces (`$((1 << 2))`) is not mistaken for a heredoc (#287).
  const HEREDOC_RE = /<<(-?)\s*(['"]?)([A-Za-z_][A-Za-z0-9_]*)\2/uy;
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
  // ---- Multi-line state machine (#287) ----
  // These helpers extend codeSpans rather than replacing it: with state === null
  // the tokenizer is byte-for-byte the line-local fallback, and only a caller
  // that opted into createHighlighter gets state carried between lines.
  function matchMultilineOpen(text, i, lang) {
    const list = MULTILINE_STRINGS[lang];
    if (!list) return null;
    for (const spec of list) if (text.startsWith(spec.open, i)) return spec;
    return null;
  }
  function matchOpenClose(text, start, spec) {
    if (spec.close.length > 1 || !spec.escaped) return text.indexOf(spec.close, start);
    let i = start;
    while (i < text.length) {
      if (text[i] === "\\") { i += 2; continue; }
      if (text[i] === spec.close) return i;
      i++;
    }
    return -1;
  }
  function matchHeredoc(text, i) {
    HEREDOC_RE.lastIndex = i;
    const m = HEREDOC_RE.exec(text);
    if (!m) return null;
    return { end: i + m[0].length, delimiter: m[3], stripTabs: m[1] === "-" };
  }
  function heredocEnds(text, heredoc) {
    return (heredoc.stripTabs ? text.replace(/^\t+/u, "") : text) === heredoc.delimiter;
  }
  // consumeOpenState finishes a construct that began on an earlier line,
  // returning the index where ordinary tokenizing resumes. It leaves
  // state.open set when the line ends still inside the construct.
  function consumeOpenState(text, start, out, state) {
    const open = state.open;
    if (open.kind === "comment") {
      const close = text.indexOf(open.close, start);
      if (close < 0) { push(out, "comment", text.slice(start)); return text.length; }
      push(out, "comment", text.slice(start, close + open.close.length));
      state.open = null;
      return close + open.close.length;
    }
    const close = matchOpenClose(text, start, open);
    if (close < 0) { push(out, "string", text.slice(start)); return text.length; }
    push(out, "string", text.slice(start, close + open.close.length));
    state.open = null;
    return close + open.close.length;
  }
  // streamOpen starts a multi-line construct that opens at i, or returns null
  // when nothing stateful begins here. It returns -1 after consuming the rest of
  // the line, so the tokenizer stops.
  function streamOpen(text, i, lang, out, state) {
    if (lang === "shell") {
      const heredoc = matchHeredoc(text, i);
      if (heredoc) {
        push(out, "op", text.slice(i, heredoc.end));
        state.heredoc = { delimiter: heredoc.delimiter, stripTabs: heredoc.stripTabs };
        return heredoc.end;
      }
    }
    const spec = matchMultilineOpen(text, i, lang);
    if (!spec) return null;
    const close = matchOpenClose(text, i + spec.open.length, spec);
    if (close < 0) { push(out, "string", text.slice(i)); state.open = spec; return -1; }
    push(out, "string", text.slice(i, close + spec.close.length));
    return close + spec.close.length;
  }

  function codeSpans(text, lang, state) {
    const out = [], keywords = KEYWORDS[lang] || new Set(), lineComment = commentPrefix(lang);
    let i = 0;
    while (i < text.length) {
      if (state && state.open) {
        i = consumeOpenState(text, i, out, state);
        if (state.open) return out;
        continue;
      }
      if (lineComment && text.startsWith(lineComment, i)) { push(out, "comment", text.slice(i)); break; }
      if (text.startsWith("/*", i)) {
        const close = text.indexOf("*/", i + 2);
        if (close < 0) {
          push(out, "comment", text.slice(i));
          if (state) state.open = BLOCK_COMMENT;
          break;
        }
        push(out, "comment", text.slice(i, close + 2)); i = close + 2; continue;
      }
      if (state) {
        const next = streamOpen(text, i, lang, out, state);
        if (next === -1) return out;
        if (next !== null) { i = next; continue; }
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

  // createHighlighter returns a stateful highlighter that colors block comments,
  // template literals, Go raw strings, Python triple-quoted strings and shell
  // heredocs across lines, or null when the language has no construct this
  // machine understands. A null return is the degradation signal: the caller
  // keeps using highlightSpans unchanged (#287).
  //
  // The machine is deliberately shallow. It does not balance nested delimiters
  // (Rust nested block comments close at the first `*/`), it does not parse
  // `${...}` inside a template literal, and it cannot know a construct that
  // began on a line the diff did not render — callers reset it at such a gap.
  function createHighlighter(path) {
    const lang = languageForPath(path);
    if (!lang || !STREAM_LANGS.has(lang)) return null;
    const state = { open: null, heredoc: null };
    return {
      language: lang,
      reset() { state.open = null; state.heredoc = null; },
      // state() is a read-only snapshot for tests and callers.
      state() {
        return {
          open: state.open ? { kind: state.open.kind, close: state.open.close } : null,
          heredoc: state.heredoc ? { ...state.heredoc } : null,
        };
      },
      highlightLine(value) {
        const text = String(value ?? "");
        if (state.heredoc) {
          const out = [];
          push(out, "string", text);
          if (heredocEnds(text, state.heredoc)) state.heredoc = null;
          return out;
        }
        try {
          return codeSpans(text, lang, state);
        } catch {
          // Never let the accurate path take the view down with it: drop the
          // carried state and return the unchanged line-local result (#287).
          state.open = null;
          state.heredoc = null;
          return highlightSpans(text, path);
        }
      },
    };
  }

  function highlightLines(lines, path) {
    const highlighter = createHighlighter(path);
    if (!highlighter) return null;
    return lines.map((line) => highlighter.highlightLine(line));
  }

  // lineRuns groups the rendered lines of one diff side into contiguous runs.
  // The diff sends hunk lines only, so a gap in line numbers means unchanged
  // lines were omitted; a stateful highlighter must be reset at each gap because
  // it never sees the lines it skipped (#287). Returns [{start, end, lines}],
  // where each line is {line, text} and `line` is zero-based.
  function lineRuns(hunks, side) {
    const startKey = side === "old" ? "old_start" : "new_start";
    const runs = [];
    let current = null;
    for (const hunk of hunks || []) {
      const lines = hunk[side] || [];
      const start = hunk[startKey];
      if (typeof start !== "number" || !lines.length) continue;
      if (!current || start !== current.end) {
        current = { start, end: start, lines: [] };
        runs.push(current);
      }
      for (let k = 0; k < lines.length; k++) current.lines.push({ line: start + k, text: lines[k] });
      current.end = start + lines.length;
    }
    return runs;
  }

  // sliceSpans clips a line's spans to [start, end) and shifts the offsets back
  // to the slice. It lets a word-diff part keep its accurate coloring when the
  // line was highlighted as a whole (#287).
  function sliceSpans(spans, start, end) {
    const out = [];
    if (!spans) return out;
    let offset = 0;
    for (const span of spans) {
      const spanEnd = offset + span.text.length;
      if (spanEnd > start && offset < end) {
        const from = Math.max(start, offset), to = Math.min(end, spanEnd);
        push(out, span.kind, span.text.slice(from - offset, to - offset));
      }
      offset = spanEnd;
    }
    return out;
  }

  const api = { highlightSpans, languageForPath, createHighlighter, highlightLines, sliceSpans, lineRuns };
  root.AyameSyntax = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
