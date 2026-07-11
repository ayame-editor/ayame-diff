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
    diffCounter: (v) => `差分 ${v.current} / ${v.total}（未読 ${v.unread}）`,
    navHelpText: "差分移動: Alt+↓ 次 / Alt+↑ 前 / Alt+End 最後 / Alt+Home 最初",
    detectMoves: "移動ブロック検出", moveMinLines: "移動の最小行数", moved: "移動",
    addSync: "同期点を追加", clearSync: "同期点を全削除", syncPoints: "同期点",
    ignoreHunk: "この差分を無視", restoreHunk: "無視を解除", ignored: "無視",
    syncSelect: "左右から対応させる行を1行ずつ選択してください。",
    syncOrderError: "同期点は左右とも昇順になるよう選択してください。",
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
    diffCounter: (v) => `Difference ${v.current} / ${v.total} (${v.unread} unread)`,
    navHelpText: "Navigate: Alt+↓ next / Alt+↑ previous / Alt+End last / Alt+Home first",
    detectMoves: "detect moves", moveMinLines: "move min lines", moved: "moved",
    addSync: "Add sync", clearSync: "Clear sync", syncPoints: "Sync points",
    ignoreHunk: "Ignore this difference", restoreHunk: "Restore difference", ignored: "ignored",
    syncSelect: "Select one corresponding line on each side.",
    syncOrderError: "Sync points must increase on both sides.",
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
  if (lastData) updateCounter();
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
let currentHunk = -1;
let readHunks = new Set();
let navObserver = null;
let syncSelection = { old: null, new: null };
let syncPoints = [];
let ignoredHunks = new Set();

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
function cell(cls, lineNo, node, side) {
  const c = document.createElement("div");
  c.className = "cell " + cls;
  const ln = document.createElement("span");
  ln.className = "ln";
  ln.textContent = lineNo == null ? "" : String(lineNo);
  c.append(ln, node);
  if (lineNo != null && side) {
    c.classList.add("selectable-line");
    c.dataset.side = side;
    c.dataset.line = String(lineNo - 1);
    c.addEventListener("click", () => selectSyncLine(c));
  }
  return c;
}
function row(left, right) {
  const r = document.createElement("div");
  r.className = "row";
  r.append(left, right);
  return r;
}

function renderHunk(h, useWord, index) {
  const box = document.createElement("div");
  box.className = "hunk";
  box.id = `hunk-${index}`;
  box.dataset.hunk = String(index);
  box.tabIndex = -1;
  if (h.move_id) {
    box.classList.add("moved");
    box.dataset.moveId = String(h.move_id);
  }
  const head = document.createElement("div");
  head.className = "hunk-head";
  const kind = h.kind.charAt(0).toUpperCase() + h.kind.slice(1);
  head.textContent = h.move_id
    ? `@@ -${h.old_start + 1},${h.old_len} +${h.new_start + 1},${h.new_len} MOVED #${h.move_id} ↔ ${h.move_peer + 1} @@`
    : `@@ -${h.old_start + 1},${h.old_len} +${h.new_start + 1},${h.new_len} ${kind} @@`;
  if (h.move_id) {
    const jump = document.createElement("button");
    jump.type = "button";
    jump.className = "move-jump";
    jump.textContent = "↔";
    jump.title = t("moved");
    jump.addEventListener("click", (event) => {
      event.stopPropagation();
      const peer = [...document.querySelectorAll(`.hunk[data-move-id="${h.move_id}"]`)]
        .find((node) => Number(node.dataset.hunk) !== index);
      if (peer) jumpToHunk(Number(peer.dataset.hunk));
    });
    head.append(jump);
  }
  const ignore = document.createElement("button");
  ignore.type = "button";
  ignore.className = "hunk-ignore";
  ignore.textContent = t("ignoreHunk");
  ignore.addEventListener("click", (event) => {
    event.stopPropagation();
    toggleIgnoredHunk(index);
  });
  if (ignoredHunks.has(index)) {
    box.classList.add("ignored");
    ignore.textContent = t("restoreHunk");
  }
  head.append(ignore);
  box.append(head);

  const rows = document.createElement("div");
  rows.className = "rows";
  const old = h.old || [], neu = h.new || [];

  if (h.kind === "insert") {
    for (let k = 0; k < neu.length; k++)
      rows.append(row(cell("empty", null, plainSpan("")), cell("add", h.new_start + k + 1, plainSpan(neu[k]), "new")));
  } else if (h.kind === "delete") {
    for (let k = 0; k < old.length; k++)
      rows.append(row(cell("del", h.old_start + k + 1, plainSpan(old[k]), "old"), cell("empty", null, plainSpan(""))));
  } else {
    const pairs = Math.min(old.length, neu.length);
    for (let k = 0; k < pairs; k++) {
      const wd = useWord ? inlineWordDiff(old[k], neu[k]) : null;
      const left = cell("chg", h.old_start + k + 1, wd ? textSpan(wd.oldParts, "w-del") : plainSpan(old[k]), "old");
      const right = cell("chg", h.new_start + k + 1, wd ? textSpan(wd.newParts, "w-add") : plainSpan(neu[k]), "new");
      rows.append(row(left, right));
    }
    for (let k = pairs; k < old.length; k++)
      rows.append(row(cell("del", h.old_start + k + 1, plainSpan(old[k]), "old"), cell("empty", null, plainSpan(""))));
    for (let k = pairs; k < neu.length; k++)
      rows.append(row(cell("empty", null, plainSpan("")), cell("add", h.new_start + k + 1, plainSpan(neu[k]), "new")));
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
  if (res.moved_blocks) el.append(stat("move", t("moved"), res.moved_blocks));
  if (ignoredHunks.size) el.append(stat("", t("ignored"), ignoredHunks.size));
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
  setupNavigation(data);
  if (!data.hunks.length) {
    const empty = document.createElement("div");
    empty.className = "empty-state";
    empty.textContent = t("noDiff");
    result.append(empty);
    return;
  }
  const useWord = $("word").checked;
  const frag = document.createDocumentFragment();
  for (let i = 0; i < data.hunks.length; i++) frag.append(renderHunk(data.hunks[i], useWord, i));
  result.append(frag);
  observeHunks();
  updateMinimapViewport();
}

function updateCounter() {
  const active = activeHunkIndexes();
  const total = active.length;
  const unread = active.filter((index) => !readHunks.has(index)).length;
  const position = active.indexOf(currentHunk);
  $("diffCounter").textContent = t("diffCounter", {
    current: position >= 0 ? position + 1 : "–", total, unread,
  });
  for (const button of [$("firstDiff"), $("prevDiff"), $("nextDiff"), $("lastDiff")])
    button.disabled = total === 0;
}

function activeHunkIndexes() {
  return (lastData?.hunks || []).map((_, index) => index).filter((index) => !ignoredHunks.has(index));
}

function stepHunk(delta) {
  const active = activeHunkIndexes();
  if (!active.length) return;
  const position = active.indexOf(currentHunk);
  const next = position < 0 ? (delta < 0 ? active.length - 1 : 0) : Math.max(0, Math.min(active.length - 1, position + delta));
  jumpToHunk(active[next]);
}

function jumpToHunk(index) {
  const total = lastData?.hunks?.length || 0;
  if (!total || ignoredHunks.has(index)) return;
  index = Math.max(0, Math.min(total - 1, index));
  document.querySelector(".hunk.current")?.classList.remove("current");
  document.querySelector(".minimap-marker.current")?.classList.remove("current");
  currentHunk = index;
  readHunks.add(index);
  const hunk = $(`hunk-${index}`);
  hunk.classList.add("current", "read");
  hunk.focus({ preventScroll: true });
  hunk.scrollIntoView({ behavior: "smooth", block: "center" });
  document.querySelector(`.minimap-marker[data-hunk="${index}"]`)?.classList.add("current", "read");
  updateCounter();
}

function toggleIgnoredHunk(index) {
  const hunk = lastData?.hunks?.[index];
  if (!hunk) return;
  const indexes = hunk.move_id
    ? lastData.hunks.map((item, i) => item.move_id === hunk.move_id ? i : -1).filter((i) => i >= 0)
    : [index];
  const restore = indexes.every((i) => ignoredHunks.has(i));
  for (const i of indexes) {
    if (restore) ignoredHunks.delete(i); else ignoredHunks.add(i);
    const box = $(`hunk-${i}`);
    box.classList.toggle("ignored", !restore);
    box.querySelector(".hunk-ignore").textContent = t(restore ? "ignoreHunk" : "restoreHunk");
    document.querySelector(`.minimap-marker[data-hunk="${i}"]`)?.classList.toggle("ignored", !restore);
  }
  if (ignoredHunks.has(currentHunk)) currentHunk = -1;
  renderSummary(lastData);
  updateCounter();
}

function buildMinimap(data) {
  const map = $("minimap");
  map.querySelectorAll(".minimap-marker").forEach((el) => el.remove());
  const totalLines = Math.max(1, data.old_lines, data.new_lines);
  data.hunks.forEach((h, index) => {
    const marker = document.createElement("button");
    marker.type = "button";
    marker.className = `minimap-marker ${h.kind}${h.move_id ? " moved" : ""}${ignoredHunks.has(index) ? " ignored" : ""}`;
    marker.dataset.hunk = String(index);
    marker.title = `${index + 1}: ${h.kind}`;
    marker.style.top = `${Math.min(99, (Math.max(h.old_start, h.new_start) / totalLines) * 100)}%`;
    marker.style.height = `${Math.max(0.7, (Math.max(h.old_len, h.new_len, 1) / totalLines) * 100)}%`;
    marker.addEventListener("click", () => jumpToHunk(index));
    map.append(marker);
  });
}

function updateMinimapViewport() {
  const result = $("result");
  const map = $("minimap");
  if (map.hidden || !result.offsetHeight) return;
  const rect = result.getBoundingClientRect();
  const visibleTop = Math.max(0, -rect.top);
  const visibleBottom = Math.min(rect.height, window.innerHeight - rect.top);
  const top = Math.min(1, visibleTop / rect.height);
  const height = Math.max(0.03, Math.min(1 - top, (visibleBottom - visibleTop) / rect.height));
  $("minimapViewport").style.top = `${top * 100}%`;
  $("minimapViewport").style.height = `${height * 100}%`;
}

function setupNavigation(data) {
  navObserver?.disconnect();
  navObserver = null;
  currentHunk = -1;
  readHunks = new Set();
  const hasHunks = data.hunks.length > 0;
  $("diffNav").hidden = !hasHunks;
  $("minimap").hidden = !hasHunks;
  if (hasHunks) buildMinimap(data);
  updateCounter();
  updateMinimapViewport();
}

function observeHunks() {
  navObserver = new IntersectionObserver((entries) => {
    let changed = false;
    for (const entry of entries) {
      if (!entry.isIntersecting || entry.intersectionRatio < 0.55) continue;
      const index = Number(entry.target.dataset.hunk);
      if (ignoredHunks.has(index)) continue;
      if (!readHunks.has(index)) {
        readHunks.add(index);
        entry.target.classList.add("read");
        document.querySelector(`.minimap-marker[data-hunk="${index}"]`)?.classList.add("read");
        changed = true;
      }
    }
    if (changed) updateCounter();
  }, { threshold: [0.55] });
  document.querySelectorAll(".hunk").forEach((hunk) => navObserver.observe(hunk));
}

function selectSyncLine(cell) {
  const side = cell.dataset.side;
  document.querySelector(`.cell.sync-selected[data-side="${side}"]`)?.classList.remove("sync-selected");
  syncSelection[side] = Number(cell.dataset.line);
  cell.classList.add("sync-selected");
  $("addSync").disabled = syncSelection.old == null || syncSelection.new == null;
  if ($("addSync").disabled) setStatus(t("syncSelect"), "");
}

function resetSyncSelection() {
  syncSelection = { old: null, new: null };
  document.querySelectorAll(".cell.sync-selected").forEach((cell) => cell.classList.remove("sync-selected"));
  $("addSync").disabled = true;
}

function renderSyncPoints() {
  const panel = $("syncPanel");
  const list = $("syncList");
  list.innerHTML = "";
  syncPoints.forEach((point, index) => {
    const chip = document.createElement("button");
    chip.type = "button";
    chip.className = "sync-chip";
    chip.textContent = `${point.old + 1}:${point.new + 1} ×`;
    chip.addEventListener("click", () => {
      syncPoints.splice(index, 1);
      renderSyncPoints();
      compare();
    });
    list.append(chip);
  });
  panel.hidden = syncPoints.length === 0;
  $("clearSync").hidden = syncPoints.length === 0;
}

function addSyncPoint() {
  if (syncSelection.old == null || syncSelection.new == null) {
    setStatus(t("syncSelect"), "error");
    return;
  }
  const candidate = [...syncPoints, { old: syncSelection.old, new: syncSelection.new }]
    .sort((a, b) => a.old - b.old);
  if (candidate.some((point, i) => i > 0 && (point.old <= candidate[i - 1].old || point.new <= candidate[i - 1].new))) {
    setStatus(t("syncOrderError"), "error");
    return;
  }
  syncPoints = candidate;
  resetSyncSelection();
  renderSyncPoints();
  compare();
}

function clearSyncPoints() {
  syncPoints = [];
  resetSyncSelection();
  renderSyncPoints();
  if (lastData) compare();
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
  ignoredHunks = new Set();
  resetSyncSelection();
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
    detectMoves: $("detectMoves").checked,
    moveMinLines: Math.max(1, Number($("moveMinLines").value) || 2),
    syncPoints: syncPoints.map((point) => ({ ...point })),
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
  body.ignoredHunks = [...ignoredHunks].sort((a, b) => a - b);
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
$("firstDiff").addEventListener("click", () => { const active = activeHunkIndexes(); if (active.length) jumpToHunk(active[0]); });
$("prevDiff").addEventListener("click", () => stepHunk(-1));
$("nextDiff").addEventListener("click", () => stepHunk(1));
$("lastDiff").addEventListener("click", () => { const active = activeHunkIndexes(); if (active.length) jumpToHunk(active[active.length - 1]); });
$("addSync").addEventListener("click", addSyncPoint);
$("clearSync").addEventListener("click", clearSyncPoints);
$("navHelp").addEventListener("click", () => alert(t("navHelpText")));
document.addEventListener("keydown", (event) => {
  if (!event.altKey || event.ctrlKey || event.metaKey || !lastData?.hunks?.length) return;
  let target = null;
  const active = activeHunkIndexes();
  if (event.key === "ArrowDown") { event.preventDefault(); stepHunk(1); return; }
  else if (event.key === "ArrowUp") { event.preventDefault(); stepHunk(-1); return; }
  else if (event.key === "Home") target = active[0];
  else if (event.key === "End") target = active[active.length - 1];
  if (target != null) {
    event.preventDefault();
    jumpToHunk(target);
  }
});
let viewportFrame = 0;
window.addEventListener("scroll", () => {
  if (viewportFrame) return;
  viewportFrame = requestAnimationFrame(() => { viewportFrame = 0; updateMinimapViewport(); });
}, { passive: true });
window.addEventListener("resize", updateMinimapViewport);
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
