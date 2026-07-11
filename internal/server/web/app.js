"use strict";

const $ = (id) => document.getElementById(id);

// ---- i18n (JA/EN) ----
const I18N = {
  ja: {
    mode: "モード", encoding: "文字コード", window: "ウィンドウ",
    maxHunks: "最大ハンク数", maxLines: "ハンクあたり最大行",
    word: "ワードハイライト", numeric: "数値", reverse: "逆順", compare: "比較",
    ignoreCase: "大小無視", whitespace: "空白", cancel: "キャンセル",
    cancelled: "キャンセルしました", scheme: "配色", wrap: "折り返し",
    showWs: "空白表示", scratch: "テキスト貼り付け",
    patchFormat: "patch形式", patchContext: "patch文脈行", exportPatch: "patchを書き出す",
    exporting: "patch生成中…", exported: "patchを書き出しました",
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
    ignoreCase: "ignore case", whitespace: "whitespace", cancel: "Cancel",
    cancelled: "Cancelled", scheme: "colors", wrap: "wrap",
    showWs: "show whitespace", scratch: "paste text",
    patchFormat: "patch format", patchContext: "patch context", exportPatch: "Export patch",
    exporting: "Exporting patch…", exported: "Patch exported",
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

// In-flight request controller, so the Cancel button can abort a long compare.
let currentAbort = null;
// Display prefs read at render time.
let showWS = false;
let lastData = null; // last diff response, for re-render on a display-option change

// ---- rendering ----
// appendText adds text to el, optionally rendering whitespace as dimmed marks
// (space -> ·, tab -> →) so leading/trailing spacing is visible.
function appendText(el, text) {
  if (!showWS) { el.appendChild(document.createTextNode(text)); return; }
  const re = /(\s+)|([^\s]+)/g;
  let m;
  while ((m = re.exec(text))) {
    if (m[1]) {
      const s = document.createElement("span");
      s.className = "ws";
      s.textContent = m[1].replace(/ /g, "·").replace(/\t/g, "→");
      el.appendChild(s);
    } else {
      el.appendChild(document.createTextNode(m[2]));
    }
  }
}
function textSpan(parts, changedClass) {
  const tx = document.createElement("span");
  tx.className = "tx";
  if (!parts) return tx;
  for (const p of parts) {
    const s = document.createElement("span");
    if (p.changed) s.className = changedClass;
    appendText(s, p.text);
    tx.append(s);
  }
  return tx;
}
function plainSpan(text) {
  const tx = document.createElement("span");
  tx.className = "tx";
  appendText(tx, text);
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

// renderResult draws a diff response into the summary + result areas, honoring
// the current display options (word highlight, show-whitespace).
function renderResult(data) {
  showWS = $("showWs").checked;
  renderSummary(data);
  const result = $("result");
  result.innerHTML = "";
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
}

function setStatus(msg, cls) {
  const el = $("status");
  if (!msg) { el.hidden = true; return; }
  el.className = "status " + (cls || "");
  el.textContent = msg;
  el.hidden = false;
}

async function compare() {
  const body = requestBody();
  if (!validateInputs(body)) return;
  const ac = new AbortController();
  currentAbort = ac;
  $("compare").disabled = true;
  $("cancel").hidden = false;
  $("summary").hidden = true;
  $("result").innerHTML = "";
  const started = Date.now();
  const tick = () => setStatus(t("comparing") + " " + ((Date.now() - started) / 1000).toFixed(1) + "s", "busy");
  tick();
  const timer = setInterval(tick, 100);
  try {
    const resp = await fetch("/api/diff", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal: ac.signal,
    });
    const data = await resp.json();
    if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
    setStatus("");
    lastData = data;
    renderResult(data);
  } catch (err) {
    if (err.name === "AbortError") setStatus(t("cancelled"), "");
    else setStatus(String(err.message || err), "error");
  } finally {
    clearInterval(timer);
    $("compare").disabled = false;
    $("cancel").hidden = true;
    currentAbort = null;
  }
}

function requestBody() {
  const scratch = $("scratch").checked;
  return {
    inline: scratch,
    old: $("old").value.trim(),
    new: $("new").value.trim(),
    oldText: $("oldText").value,
    newText: $("newText").value,
    mode: $("mode").value,
    encoding: $("encoding").value,
    window: Number($("window").value) || 128,
    maxHunks: Number($("maxHunks").value) || 200,
    maxLines: Number($("maxLines").value) || 200,
    numeric: $("numeric").checked,
    reverse: $("reverse").checked,
    ignoreCase: $("ignoreCase").checked,
    whitespace: $("whitespace").value,
  };
}

function validateInputs(body) {
  if (!body.inline && (!body.old || !body.new)) {
    setStatus(t("enterPaths"), "error");
    return false;
  }
  return true;
}

async function exportPatch() {
  const body = requestBody();
  if (!validateInputs(body)) return;
  body.patchFormat = $("patchFormat").value;
  body.context = Math.max(0, Number($("patchContext").value) || 0);
  $("exportPatch").disabled = true;
  setStatus(t("exporting"), "busy");
  try {
    const resp = await fetch("/api/patch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!resp.ok) {
      const data = await resp.json().catch(() => ({}));
      throw new Error(data.error || `HTTP ${resp.status}`);
    }
    const blob = await resp.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "ayame.patch";
    document.body.append(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
    setStatus(t("exported"), "");
  } catch (err) {
    setStatus(String(err.message || err), "error");
  } finally {
    $("exportPatch").disabled = false;
  }
}

function syncModeOpts() {
  const sorted = $("mode").value === "sorted";
  $("numericWrap").hidden = !sorted;
  $("reverseWrap").hidden = !sorted;
  $("exportPatch").disabled = sorted;
}
function syncPatchOpts() {
  $("patchContextWrap").hidden = $("patchFormat").value === "normal";
}

// Display preferences (color scheme + line wrap), persisted across visits.
function applyScheme(v) {
  document.documentElement.setAttribute("data-scheme", v === "default" ? "" : v);
  localStorage.setItem("ayame-scheme", v);
  $("scheme").value = v;
}
function applyWrap(on) {
  $("result").classList.toggle("nowrap", !on);
  localStorage.setItem("ayame-wrap", on ? "1" : "0");
  $("wrap").checked = on;
}

$("compare").addEventListener("click", compare);
$("exportPatch").addEventListener("click", exportPatch);
$("cancel").addEventListener("click", () => { if (currentAbort) currentAbort.abort(); });
$("mode").addEventListener("change", syncModeOpts);
$("patchFormat").addEventListener("change", syncPatchOpts);
$("scheme").addEventListener("change", () => applyScheme($("scheme").value));
$("wrap").addEventListener("change", () => applyWrap($("wrap").checked));
$("showWs").addEventListener("change", () => {
  localStorage.setItem("ayame-showws", $("showWs").checked ? "1" : "0");
  if (lastData) renderResult(lastData); // re-render so the change is immediate
});
function applyScratch() {
  const on = $("scratch").checked;
  $("paths").hidden = on;
  $("scratchArea").hidden = !on;
}
$("scratch").addEventListener("change", applyScratch);
applyScratch();
applyScheme(localStorage.getItem("ayame-scheme") || "default");
applyWrap(localStorage.getItem("ayame-wrap") !== "0");
$("showWs").checked = localStorage.getItem("ayame-showws") === "1";
$("lang").addEventListener("click", () => applyLang(lang === "ja" ? "en" : "ja"));
syncModeOpts();
syncPatchOpts();
applyLang(lang);

fetch("/api/health")
  .then((r) => r.json())
  .then((d) => { if (d.version) $("version").textContent = d.version; })
  .catch(() => {});
