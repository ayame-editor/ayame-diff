"use strict";

const $ = (id) => document.getElementById(id);

// ---- i18n (JA/EN) ----
const I18N = {
  ja: {
    mode: "モード", encoding: "文字コード", window: "ウィンドウ",
    maxHunks: "最大ハンク数", maxLines: "ハンクあたり最大行",
    word: "ワードハイライト", numeric: "数値", reverse: "逆順", compare: "比較",
    hunks: "ハンク", added: "追加", deleted: "削除", modified: "変更",
    omitted: (n) => `（${n} ハンク省略。最大ハンク数を上げてください）`,
    comparing: "比較中…", noDiff: "差分はありません。",
    enterPaths: "OLD と NEW のパスを入力してください。",
    langButton: "EN",
  },
  en: {
    mode: "mode", encoding: "encoding", window: "window", maxHunks: "max hunks",
    maxLines: "max lines/hunk", word: "word highlight", numeric: "numeric",
    reverse: "reverse", compare: "Compare",
    hunks: "hunks", added: "added", deleted: "deleted", modified: "modified",
    omitted: (n) => `(${n} hunks omitted; raise max hunks)`,
    comparing: "Comparing…", noDiff: "No differences.",
    enterPaths: "Enter both OLD and NEW paths.",
    langButton: "日本語",
  },
};
let lang = localStorage.getItem("ayame-lang");
if (lang !== "ja" && lang !== "en") {
  lang = (navigator.language || "").startsWith("ja") ? "ja" : "en";
}

function t(key, arg) {
  const v = (I18N[lang] || I18N.ja)[key];
  return typeof v === "function" ? v(arg) : v != null ? v : key;
}
function applyLang(next) {
  lang = next;
  localStorage.setItem("ayame-lang", lang);
  document.documentElement.lang = lang;
  for (const el of document.querySelectorAll("[data-i18n]")) {
    el.textContent = t(el.getAttribute("data-i18n"));
  }
  $("lang").textContent = t("langButton");
}

// ---- word-level diff (ported from ayame-editor web/src/search.ts) ----
const INLINE_MAX_CHARS = 2000;
const INLINE_MAX_TOKENS = 260;
const TOKEN_RE = /(\s+|[\p{Letter}\p{Number}_]+|[^\s\p{Letter}\p{Number}_]+)/gu;

function inlineTokens(text) {
  const tokens = [];
  for (const m of String(text || "").matchAll(TOKEN_RE)) tokens.push(m[0]);
  return tokens;
}
function pushPart(parts, text, changed) {
  if (!text) return;
  const last = parts[parts.length - 1];
  if (last && last.changed === changed) last.text += text;
  else parts.push({ text, changed });
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
  const dp = Array.from({ length: m + 1 }, () => new Uint16Array(n + 1));
  for (let i = m - 1; i >= 0; i--)
    for (let j = n - 1; j >= 0; j--)
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
  const oldParts = [], newParts = [];
  let i = 0, j = 0;
  while (i < m || j < n) {
    if (i < m && j < n && a[i] === b[j]) {
      pushPart(oldParts, a[i], false);
      pushPart(newParts, b[j], false);
      i++; j++;
    } else if (j >= n || (i < m && dp[i + 1][j] >= dp[i][j + 1])) {
      pushPart(oldParts, a[i], true); i++;
    } else {
      pushPart(newParts, b[j], true); j++;
    }
  }
  return { oldParts, newParts };
}

// ---- rendering ----
function textSpan(parts, changedClass) {
  const tx = document.createElement("span");
  tx.className = "tx";
  if (!parts) return tx;
  for (const p of parts) {
    const s = document.createElement("span");
    if (p.changed) s.className = changedClass;
    s.textContent = p.text;
    tx.append(s);
  }
  return tx;
}
function plainSpan(text) {
  const tx = document.createElement("span");
  tx.className = "tx";
  tx.textContent = text;
  return tx;
}
function cell(cls, lineNo, node) {
  const c = document.createElement("div");
  c.className = "cell " + cls;
  const ln = document.createElement("span");
  ln.className = "ln";
  ln.textContent = lineNo == null ? "" : String(lineNo);
  c.append(ln, node);
  return c;
}
function row(left, right) {
  const r = document.createElement("div");
  r.className = "row";
  r.append(left, right);
  return r;
}

function renderHunk(h, useWord) {
  const box = document.createElement("div");
  box.className = "hunk";
  const head = document.createElement("div");
  head.className = "hunk-head";
  const kind = h.kind.charAt(0).toUpperCase() + h.kind.slice(1);
  head.textContent = `@@ -${h.old_start + 1},${h.old_len} +${h.new_start + 1},${h.new_len} ${kind} @@`;
  box.append(head);

  const rows = document.createElement("div");
  rows.className = "rows";
  const old = h.old || [], neu = h.new || [];

  if (h.kind === "insert") {
    for (let k = 0; k < neu.length; k++)
      rows.append(row(cell("empty", null, plainSpan("")), cell("add", h.new_start + k + 1, plainSpan(neu[k]))));
  } else if (h.kind === "delete") {
    for (let k = 0; k < old.length; k++)
      rows.append(row(cell("del", h.old_start + k + 1, plainSpan(old[k])), cell("empty", null, plainSpan(""))));
  } else {
    const pairs = Math.min(old.length, neu.length);
    for (let k = 0; k < pairs; k++) {
      const wd = useWord ? inlineWordDiff(old[k], neu[k]) : null;
      const left = cell("chg", h.old_start + k + 1, wd ? textSpan(wd.oldParts, "w-del") : plainSpan(old[k]));
      const right = cell("chg", h.new_start + k + 1, wd ? textSpan(wd.newParts, "w-add") : plainSpan(neu[k]));
      rows.append(row(left, right));
    }
    for (let k = pairs; k < old.length; k++)
      rows.append(row(cell("del", h.old_start + k + 1, plainSpan(old[k])), cell("empty", null, plainSpan(""))));
    for (let k = pairs; k < neu.length; k++)
      rows.append(row(cell("empty", null, plainSpan("")), cell("add", h.new_start + k + 1, plainSpan(neu[k]))));
  }
  box.append(rows);
  return box;
}

function renderSummary(res) {
  const el = $("summary");
  el.innerHTML = "";
  const stat = (cls, label, n) => {
    const s = document.createElement("span");
    s.className = "stat " + cls;
    s.innerHTML = `<b>${n.toLocaleString()}</b> ${label}`;
    return s;
  };
  el.append(
    stat("", t("hunks"), res.hunk_count),
    stat("add", t("added"), res.added),
    stat("del", t("deleted"), res.deleted),
    stat("chg", t("modified"), res.modified),
  );
  if (res.omitted_hunks) {
    const n = document.createElement("span");
    n.className = "note";
    n.textContent = t("omitted", res.omitted_hunks.toLocaleString());
    el.append(n);
  }
  el.hidden = false;
}

function setStatus(msg, cls) {
  const el = $("status");
  if (!msg) { el.hidden = true; return; }
  el.className = "status " + (cls || "");
  el.textContent = msg;
  el.hidden = false;
}

async function compare() {
  const body = {
    old: $("old").value.trim(),
    new: $("new").value.trim(),
    mode: $("mode").value,
    encoding: $("encoding").value,
    window: Number($("window").value) || 128,
    maxHunks: Number($("maxHunks").value) || 200,
    maxLines: Number($("maxLines").value) || 200,
    numeric: $("numeric").checked,
    reverse: $("reverse").checked,
  };
  if (!body.old || !body.new) {
    setStatus(t("enterPaths"), "error");
    return;
  }
  $("compare").disabled = true;
  setStatus(t("comparing"), "busy");
  $("summary").hidden = true;
  $("result").innerHTML = "";
  try {
    const resp = await fetch("/api/diff", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await resp.json();
    if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
    setStatus("");
    renderSummary(data);
    const result = $("result");
    if (!data.hunks.length) {
      const empty = document.createElement("div");
      empty.className = "empty-state";
      empty.textContent = t("noDiff");
      result.append(empty);
      return;
    }
    const useWord = $("word").checked;
    const frag = document.createDocumentFragment();
    for (const h of data.hunks) frag.append(renderHunk(h, useWord));
    result.append(frag);
  } catch (err) {
    setStatus(String(err.message || err), "error");
  } finally {
    $("compare").disabled = false;
  }
}

function syncModeOpts() {
  const sorted = $("mode").value === "sorted";
  $("numericWrap").hidden = !sorted;
  $("reverseWrap").hidden = !sorted;
}

$("compare").addEventListener("click", compare);
$("mode").addEventListener("change", syncModeOpts);
$("lang").addEventListener("click", () => applyLang(lang === "ja" ? "en" : "ja"));
syncModeOpts();
applyLang(lang);

fetch("/api/health")
  .then((r) => r.json())
  .then((d) => { if (d.version) $("version").textContent = d.version; })
  .catch(() => {});
