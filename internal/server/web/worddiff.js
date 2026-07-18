// Word-level diff used by the side-by-side view.
//
// It lives outside app.js so it can be executed in tests without a DOM: it is
// the one piece of client logic that is a real algorithm rather than DOM
// plumbing, and it had no execution-level coverage at all (#139). The wrapper
// mirrors syntax.js — a browser global plus a CommonJS export for node --test.
(function (root) {
  "use strict";

  const INLINE_MAX_CHARS = 2000;
  const INLINE_MAX_TOKENS = 260;
  // Word class includes combining marks (\p{Mark}) so a base letter keeps its
  // accent (e.g. "e"+U+0301) instead of stranding the mark as a separate token.
  const TOKEN_RE = /(\s+|[\p{Letter}\p{Mark}\p{Number}_]+|[^\s\p{Letter}\p{Mark}\p{Number}_]+)/gu;
  // CJK is written without spaces, so a whole run would be one token and the diff
  // could only mark it all changed; split each such character out so "日本語" vs
  // "日本国" highlights just the last character. Mirrors worddiff.go's cjkScripts.
  const CJK_RE = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/u;

  function inlineTokens(text) {
    const tokens = [];
    for (const m of String(text || "").matchAll(TOKEN_RE)) appendSplitCJK(tokens, m[0]);
    return tokens;
  }
  // appendSplitCJK pushes tok, emitting each CJK character as its own token while
  // keeping maximal non-CJK runs intact. Non-CJK tokens take a fast path.
  function appendSplitCJK(tokens, tok) {
    if (!CJK_RE.test(tok)) { tokens.push(tok); return; }
    let buf = "";
    for (const ch of tok) { // iterates by code point
      if (CJK_RE.test(ch)) {
        if (buf) { tokens.push(buf); buf = ""; }
        tokens.push(ch);
      } else {
        buf += ch;
      }
    }
    if (buf) tokens.push(buf);
  }
  function pushPart(parts, text, changed) {
    if (!text) return;
    const last = parts[parts.length - 1];
    if (last && last.changed === changed) last.text += text;
    else parts.push({ text, changed });
  }
  // inlineDPScratch is reused across calls so a full render does not allocate a
  // fresh table per changed line. It is cleared to the needed length each time,
  // since the algorithm reads cells it has not written yet (row m and column n
  // must be zero).
  let inlineDPScratch = new Uint16Array(0);

  function inlineDPBuffer(size) {
    if (inlineDPScratch.length < size) inlineDPScratch = new Uint16Array(size);
    inlineDPScratch.fill(0, 0, size);
    return inlineDPScratch;
  }

  function inlineWordDiff(oldText, newText) {
    oldText = String(oldText || "");
    newText = String(newText || "");
    if (oldText === newText) return null;
    if (oldText.length + newText.length > INLINE_MAX_CHARS) return null;
    const a = inlineTokens(oldText);
    const b = inlineTokens(newText);
    if (a.length + b.length > INLINE_MAX_TOKENS) return null;
    const m = a.length, n = b.length;
    // One flat Uint16Array rather than m+1 of them: the row-per-line form
    // allocated an array per token and turned every dp read into an
    // array-of-arrays double dereference. The table is bounded by
    // INLINE_MAX_TOKENS above, so a single buffer is reused across calls (#155).
    const stride = n + 1;
    const dp = inlineDPBuffer(stride * (m + 1));
    const at = (i, j) => i * stride + j;
    for (let i = m - 1; i >= 0; i--)
      for (let j = n - 1; j >= 0; j--)
        dp[at(i, j)] = a[i] === b[j] ? dp[at(i + 1, j + 1)] + 1 : Math.max(dp[at(i + 1, j)], dp[at(i, j + 1)]);
    const oldParts = [], newParts = [];
    let i = 0, j = 0;
    while (i < m || j < n) {
      if (i < m && j < n && a[i] === b[j]) {
        pushPart(oldParts, a[i], false);
        pushPart(newParts, b[j], false);
        i++; j++;
      } else if (j >= n || (i < m && dp[at(i + 1, j)] >= dp[at(i, j + 1)])) {
        pushPart(oldParts, a[i], true); i++;
      } else {
        pushPart(newParts, b[j], true); j++;
      }
    }
    return { oldParts, newParts };
  }

  const api = { inlineWordDiff, inlineTokens, pushPart, INLINE_MAX_CHARS, INLINE_MAX_TOKENS };
  root.AyameWordDiff = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
