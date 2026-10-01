"use strict";

const $ = (id) => document.getElementById(id);
const {
  captureScrollAnchor,
  restoreScrollAnchor,
} = globalThis.AyameScrollAnchor;
const {
  watchPathsForMode,
  createLongPollWatcher,
} = globalThis.AyameFileWatch;
const {
  HASH_KEY: COMPARISON_HASH_KEY,
  readComparisonState,
  readTabState,
  buildComparisonURL,
  buildTabStateURL,
  buildShareURL,
} = globalThis.AyameURLState;
// #260: which settings are the reader's (global) and which belong to a
// comparison, plus the helpers that keep the latter scoped to their identity.
const {
  COMPARISON_CONTROLS,
  comparisonKey,
  parseStore: parseConditionStore,
  serializeStore: serializeConditionStore,
  readConditions,
  writeConditions,
} = globalThis.AyameComparisonScope;
const {
  STORAGE_KEY: CONDITION_DEFAULTS_KEY,
  capture: captureConditionValues,
  apply: applyConditionValues,
  changes: changedConditionIds,
  isDefault: conditionsAreDefault,
  resolve: resolveConditionDefaults,
  promote: promoteConditionDefaults,
  serialize: serializeConditionDefaults,
  parse: parseConditionDefaults,
} = globalThis.AyameConditionScope;
const {
  emptyDoc: emptyTabDoc,
  activeTab: activeTabOf,
  addTab: addTabToDoc,
  removeTab: removeTabFromDoc,
  activateTab: activateTabInDoc,
  updateTab: updateTabInDoc,
  labelFromState: tabLabelFromState,
  serializeDoc: serializeTabDoc,
  parseDoc: parseTabDoc,
} = globalThis.AyameTabs;
const {
  calculateMinimapSegments,
  calculateMinimapViewport,
  scrollTopForMinimapPointer,
} = globalThis.AyameMinimap;
const { isDowngraded, navigableIndexes, essentialIndex, essentialIndexes } = globalThis.AyameDowngrade;
const { apiErrorKey } = globalThis.AyameAPIErrors;
const {
  differenceCount,
  estimateCauses,
  buildProposals,
  applyProposals,
  MAX_CANDIDATES: MAX_SUGGEST_CANDIDATES,
} = globalThis.AyameSuggest;
const { resultLines: threeWayResultLines, panes: threeWayPanes } = globalThis.AyameThreeWayView;
const { createEditBuffer, editableComparison } = globalThis.AyameEditBuffer;
// The two axes behind the flat `mode` value (#263): the input shape and the
// reading. composeMode/decomposeMode keep the GUI from enumerating the cross
// product the old dropdown exposed.
const {
  interpretationsFor,
  composeMode,
  decomposeMode,
} = globalThis.AyameModeAxes;
const {
  isJapaneseLegacy,
  encodingMismatch,
  encodingCandidates,
  encodingPickerValue,
} = globalThis.AyameEncoding;
const {
  marker: provenanceMarker,
  originClass: provenanceOriginClass,
  labelKey: provenanceLabelKey,
  breakdown: provenanceBreakdown,
} = globalThis.AyameMergeProvenance;
const { localChangeRegions, localChangeIndex, regionAt } = globalThis.AyameQuickDiff;
const { csvPageCount, clampPage, visibleColumns, pagerState, pageSlice } = globalThis.AyameCSVView;
const { memoryStatus } = globalThis.AyameMemoryBudget;
const { equivalenceTitleKey, alignmentProposal } = globalThis.AyameEquivalence;
const {
  requestFields: whitespaceRequestFields,
  whitespaceMode: whitespaceScaleMode,
  labelKey: whitespaceScaleLabelKey,
  levelForWhitespace: whitespaceScaleLevel,
} = globalThis.AyameWhitespaceScale;
const {
  ABSENT: COLUMN_ABSENT,
  initialMapping,
  validateMapping,
  mappingToRequest,
  mappingFromRequest,
  reorderColumns,
} = globalThis.AyameColumnMap;
const {
  buildUnchangedRegions,
  initialContextRanges,
  missingContextSpans,
  batchContextRanges,
} = globalThis.AyameUnchanged;
const { nextMaxHunks } = globalThis.AyameTruncation;
const {
  comparisonIdentity,
  hunkSignatures,
  restoreSignatures,
  unconfirmedIndexes,
  confirmProgress,
} = globalThis.AyameConfirmed;
const {
  unresolvedItems: computeMergeUnresolved,
  targetsFor: mergeTargetsFor,
} = globalThis.AyameUnresolved;
const {
  ACTION_IDS: KEYMAP_ACTION_IDS,
  DEFAULT_BINDINGS,
  PRESETS: KEYMAP_PRESETS,
  parseChord,
  formatChord,
  chordFromEvent,
  eventMatchesChord,
  isReservedChord,
  findConflicts,
  mergeBindings,
  serializeBindings,
  nextUnresolved,
} = globalThis.AyameKeymap;
const {
  describeGap,
  keyRange,
  formatKeyRange,
  keyIndexes: hiddenKeyIndexes,
} = globalThis.AyameHidden;
const {
  continuousEntries,
  windowAround,
  unloadTargets,
  stepSection,
  sectionAt,
} = globalThis.AyameContinuous;
const { hunkActions } = globalThis.AyameHunkActions;
const { createMessageLog } = globalThis.AyameMessages;
// Visual filter builder model/compiler (#129). Kept pure in rowfilter.js so the
// node:test suite can exercise the condition semantics without a DOM.
const {
  OPERATORS: ROW_FILTER_OPERATORS,
  operatorInfo: rowFilterOperatorInfo,
  newCondition: newRowFilterCondition,
  newGroup: newRowFilterGroup,
  newFilter: newRowFilter,
  normalizeFilter: normalizeRowFilter,
  validateFilter: validateRowFilter,
  compileFilter: compileRowFilter,
  countConditions: rowFilterCount,
} = globalThis.AyameRowFilter;
const {
  stagePlan: comparisonStagePlan,
  createProgressTracker,
  formatElapsed,
  degradationNotice,
} = globalThis.AyameProgress;
// Colour-independent diff signalling (#298): one mapping from a class or hunk
// kind to the gutter glyph and the accessible kind name.
const {
  cellMarker,
  cellKindKey,
  hunkMarker,
  hunkKindKey,
  CONFLICT_MARK,
} = globalThis.AyameDiffMark;
const {
  CUSTOM_SCHEME: THEME_CUSTOM_SCHEME,
  TOKEN_GROUPS,
  TOKEN_KEYS,
  effectiveTokens,
  resolveColor,
  formatColor,
  checkContrast,
  createTheme,
  normalizeTheme,
  serializeTheme,
  parseTheme,
  presetList,
  presetById,
  loadThemeList,
  saveNamedTheme,
  deleteNamedTheme,
  loadActiveTheme,
  saveActiveTheme,
} = globalThis.AyameTheme;
const {
  formatPercent: formatStatsPercent,
  buildStats,
  toCSV: statsToCSV,
  toJSON: statsToJSON,
} = globalThis.AyameStatsView;
const {
  isPlainComparison: optimisticIsPlain,
  planLineApproximation: optimisticPlanLine,
  createRecomputeCoordinator,
} = globalThis.AyameOptimistic;
// Declared with the other module wiring: setStatus runs during start-up, before
// the lane helpers further down the file are reached.
const messageLog = createMessageLog({ onChange: renderMessages });

function resultScrollInset() {
  const inset = document.querySelector("#result > .pane-heads")?.getBoundingClientRect().height || 0;
  // The continuous view's file headers stick below these, so the measurement
  // has to reach CSS as well (#291).
  $("result").style.setProperty("--pane-heads-height", `${Math.round(inset)}px`);
  return inset;
}

function captureResultScrollAnchor() {
  return captureScrollAnchor($("result"), resultScrollInset());
}

function restoreResultScrollAnchor(anchor, announceFailure = false) {
  const restored = restoreScrollAnchor($("result"), anchor, resultScrollInset());
  // A result with no logical rows already explains itself with an empty/match
  // state card. Otherwise make the unavoidable jump to the top explicit.
  if (!restored && anchor && announceFailure && $("result").querySelector("[data-scroll-anchor]")) {
    setStatus(t("scrollRestoreUnavailable"), "");
  }
  return restored;
}

// ---- API token (#108) ----
// The server requires this token on every /api call. It arrives once in the
// URL the ayame-diff command opens or prints, and is kept in sessionStorage so
// a reload or an in-page navigation that drops the query string still works.
// sessionStorage is per-origin and per-tab, so a page on another site cannot
// read it.
//
// It travels as a request header, never as a cookie: a cookie would be attached
// automatically to a cross-site request, whereas another origin cannot set a
// custom header without a CORS preflight that this server refuses. That is what
// makes the token double as CSRF protection.
const API_TOKEN_KEY = "ayame-api-token";
const apiToken = (() => {
  const fromURL = new URLSearchParams(location.search).get("token");
  if (fromURL) {
    try { sessionStorage.setItem(API_TOKEN_KEY, fromURL); } catch { /* private mode: keep it in memory only */ }
    return fromURL;
  }
  try { return sessionStorage.getItem(API_TOKEN_KEY) || ""; } catch { return ""; }
})();

// apiFetch is the single door to the API: it adds the token to every request so
// no call site can forget it.
function apiFetch(input, init = {}) {
  const headers = new Headers(init.headers || {});
  if (apiToken) headers.set("X-Ayame-Token", apiToken);
  return fetch(input, { ...init, headers });
}

// ---- Browser/server lifecycle (#96) ----
// One lease belongs to one loaded tab, rather than sessionStorage, so closing
// one of several tabs cannot stop a server that another tab still uses.
const BROWSER_HEARTBEAT_INTERVAL_MS = 30_000;
const browserSessionID = globalThis.crypto?.randomUUID?.() ||
  `tab-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
let browserHeartbeatTimer = 0;

async function postBrowserLifecycle(path) {
  if (!apiToken) return;
  const response = await apiFetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ session: browserSessionID }),
    keepalive: true,
  });
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
}

function heartbeatBrowserSession() {
  void postBrowserLifecycle("/api/lifecycle/heartbeat").catch(() => {});
}

function startBrowserSession() {
  if (!apiToken || browserHeartbeatTimer) return;
  heartbeatBrowserSession();
  browserHeartbeatTimer = setInterval(heartbeatBrowserSession, BROWSER_HEARTBEAT_INTERVAL_MS);
}

function stopBrowserHeartbeat() {
  clearInterval(browserHeartbeatTimer);
  browserHeartbeatTimer = 0;
}

function releaseBrowserSession() {
  stopBrowserHeartbeat();
  void postBrowserLifecycle("/api/lifecycle/release").catch(() => {});
}

// The message catalog lives in i18n.js; this file keeps the current choice.
const { CATALOG: I18N, translate, pickLanguage, languages, languageMeta, localeTag, direction } = globalThis.AyameI18N;
// The condition-toolbar derivation lives in conditions.js (#264).
const { readPolicy: readConditionPolicy, writePolicy: writeConditionPolicy, describeConditions } = globalThis.AyameConditions;
let lang = pickLanguage(localStorage.getItem("ayame-lang"), navigator.language);

function t(key, arg) {
  return translate(I18N, lang, key, arg);
}

// Numbers follow the chosen UI language, not the runtime's locale (#144).
function fmt(value) {
  return Number(value || 0).toLocaleString(localeTag(lang));
}

function applyLang(next) {
  lang = next;
  localStorage.setItem("ayame-lang", lang);
  document.documentElement.lang = lang;
  // A right-to-left language added to the catalog flips the layout here (#144).
  document.documentElement.dir = direction(lang);
  for (const el of document.querySelectorAll("[data-i18n]")) {
    el.textContent = t(el.getAttribute("data-i18n"));
  }
	for (const el of document.querySelectorAll("[data-i18n-placeholder]")) el.placeholder = t(el.getAttribute("data-i18n-placeholder"));
  for (const el of document.querySelectorAll("[data-i18n-title]")) el.title = t(el.getAttribute("data-i18n-title"));
  for (const el of document.querySelectorAll("[data-i18n-aria-label]")) el.setAttribute("aria-label", t(el.getAttribute("data-i18n-aria-label")));
  syncLanguageOptions();
  if (lastData) updateCounter();
	if (csvData && $("mode").value === "csv") renderCSV(csvData);
	refreshContextTranslations();
	syncConditionToolbar();
	renderRecentComparisons();
	renderFilterBuilders();
	// The scope chip is set from a message function, not a fixed data-i18n
	// string, so re-resolve it after the catalog swap.
	updateConditionScope();
	renderTabs();
}

// The switcher lists every catalog language under its own name, so a third
// language is a data-only change (#144).
function syncLanguageOptions() {
  const select = $("lang");
  if (!select) return;
  const wanted = languages();
  const existing = Array.from(select.options).map((option) => option.value);
  if (existing.length !== wanted.length || existing.some((value, index) => value !== wanted[index])) {
    select.textContent = "";
    for (const code of wanted) {
      const option = document.createElement("option");
      option.value = code;
      option.textContent = languageMeta(code).name;
      select.append(option);
    }
  }
  select.value = lang;
}


// ---- editable panes (#255) ----
//
// The diff response carries hunk slices, not whole files, so editing loads both
// files in full and compares the buffers instead of the paths. Nothing reaches
// disk until a save, which sends the file's own encoding, BOM and terminator
// back with it so an edited file is not quietly normalized.
//
// The editor is a textarea mounted on the line being changed rather than
// contenteditable: contenteditable and IME are a bad pair, and Japanese input
// is a first-class case here, not an afterthought.
let editSession = null;
let lineEditor = null;
let editComposing = false;

const EDIT_RECOMPARE_DELAY = 150;

// Optimistic re-computation (#258). A committed edit paints the edited line
// immediately and marks it provisional; the coordinator then folds the burst of
// edits that follow into one authoritative comparison, plus one catch-up run for
// whatever arrived while that comparison was in flight. The run is `compare`,
// which already cancels the previous request and restores the scroll anchor.
let provisionalActive = false;
const editRecompute = createRecomputeCoordinator({
  delay: EDIT_RECOMPARE_DELAY,
  run: async () => {
    if (editComposing || !editingEnabled() || lineEditor) return false;
    if (busyOperation) return "busy";
    return (await compare()) ? true : false;
  },
});

function editingEnabled() {
  return editSession !== null;
}

function editBufferFor(side) {
  return editSession ? editSession[side] || null : null;
}

function editedSides() {
  return editSession ? ["old", "new"].filter((side) => editSession[side]?.isDirty()) : [];
}

function syncEditControls() {
  const button = $("editMode");
  const available = editableComparison($("mode").value, $("scratch").checked) && hasComparisonResult();
  button.hidden = !available;
  button.setAttribute("aria-pressed", editingEnabled() ? "true" : "false");
  button.classList.toggle("active", editingEnabled());
  // The gutter's comparison stripe belongs to editing mode (#292); reading a
  // diff must not suddenly grow a stripe it never had.
  document.body.classList.toggle("editing", editingEnabled());
}

async function toggleEditMode() {
  if (editingEnabled()) {
    await leaveEditMode();
    return;
  }
  await enterEditMode();
}

async function enterEditMode(options = {}) {
  if (!editableComparison($("mode").value, $("scratch").checked)) return false;
  const paths = { old: $("old").value.trim(), new: $("new").value.trim() };
  if (!paths.old || !paths.new) return false;
  setStatus(t("editLoading"), "busy");
  try {
    const loaded = {};
    for (const side of ["old", "new"]) {
      const response = await apiFetch("/api/file/read", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          path: paths[side],
          encoding: encodingOverrideFor(side, paths[side]) || $("encoding").value,
        }),
      });
      const data = await response.json();
      if (!response.ok) throw apiError(data, response);
      loaded[side] = createEditBuffer(data);
    }
    editSession = loaded;
  } catch (err) {
    editSession = null;
    setStatus(String(err.message || err), "error");
    return false;
  }
  setStatus("");
  syncEditControls();
  const readOnly = ["old", "new"].filter((side) => editSession[side].readOnly());
  if (readOnly.length) {
    setStatus(t("editReadOnly", { sides: readOnly.map(sideLabel).join(" / ") }), "warning");
  }
  markEditedPanes();
  if (options.recompare !== false) await compare();
  return true;
}

// Reloading after an external change is an explicit discard: drop the buffers,
// let the comparison run on what is now on disk, then re-open the panes on that
// content so the user stays in the mode they chose.
let editReloadPending = false;

document.addEventListener("ayame:discard-unsaved-changes", () => {
  if (!editingEnabled()) return;
  closeLineEditor({ revert: false });
  clearEditSession();
  editReloadPending = true;
});

async function resumeEditingAfterReload() {
  if (!editReloadPending || editingEnabled()) return;
  editReloadPending = false;
  await enterEditMode({ recompare: false });
}

// Leaving keeps whatever is on disk: unsaved lines are the user's, so ask
// before dropping them rather than deciding for them.
async function leaveEditMode(options = {}) {
  if (!editingEnabled()) return true;
  closeLineEditor({ commit: true });
  const dirty = editedSides();
  if (dirty.length && !options.discard) {
    const confirmed = await askConfirm(t("editDiscardConfirm", { sides: dirty.map(sideLabel).join(" / ") }));
    if (!confirmed) return false;
  }
  clearEditSession();
  await compare();
  return true;
}

// Anything that replaces what is being compared also throws away lines the user
// typed, so it asks first. Re-comparing, changing display or comparison options
// and adding sync points all keep the buffers, and do not (#256).
// Ending a session in any of its three ways — the toggle, a refused route, an
// explicit discard — has to clear the watcher flag too, or auto-reload stays
// blocked for the rest of the session.
function clearEditSession() {
  editRecompute.cancel();
  clearProvisional();
  editSession = null;
  markEditedPanes();
  syncEditControls();
}

async function guardUnsavedEdits() {
  const dirty = editedSides();
  if (!dirty.length) return true;
  closeLineEditor({ commit: true });
  if (!(await askConfirm(t("editDiscardConfirm", { sides: dirty.map(sideLabel).join(" / ") })))) return false;
  clearEditSession();
  return true;
}

function sideLabel(side) {
  return side === "old" ? t("sideLeft") : t("sideRight");
}

// A line cell knows its side and its zero-based file line, so the buffer is
// addressed by exactly what the renderer already put on the element.
function openLineEditor(cellElement) {
  if (!editingEnabled() || !cellElement?.dataset?.side) return false;
  const side = cellElement.dataset.side;
  const index = Number(cellElement.dataset.line);
  const buffer = editBufferFor(side);
  if (!buffer || buffer.readOnly() || !Number.isInteger(index)) return false;
  if (index >= buffer.count()) return false;
  closeLineEditor({ commit: true });

  const initial = buffer.line(index) ?? "";
  const editor = document.createElement("textarea");
  editor.className = "line-editor";
  editor.rows = 1;
  editor.spellcheck = false;
  editor.value = initial;
  editor.setAttribute("aria-label", t("editLine", { line: index + 1, side: sideLabel(side) }));
  const content = cellElement.querySelector(".tx");
  if (content) content.hidden = true;
  cellElement.classList.add("editing");
  cellElement.append(editor);
  lineEditor = { element: editor, cell: cellElement, side, index, initial };

  // An IME composition is a sequence of intermediate values, none of which is
  // what the user means. Re-comparing on them would fight the input method.
  editor.addEventListener("compositionstart", () => { editComposing = true; });
  editor.addEventListener("compositionend", () => {
    editComposing = false;
    applyLineEdit();
  });
  editor.addEventListener("input", () => {
    if (editComposing) return;
    applyLineEdit();
  });
  // The editor sits inside the cell that opens it, so every key and click in it
  // would otherwise bubble up and re-open the editor on top of itself, taking
  // the caret to the end of the line each time.
  editor.addEventListener("click", (event) => event.stopPropagation());
  editor.addEventListener("keydown", (event) => {
    event.stopPropagation();
    if (event.isComposing || editComposing) return;
    if (event.key === "Escape") {
      event.preventDefault();
      closeLineEditor({ revert: true });
      cellElement.focus();
      return;
    }
    if (event.key === "Enter") {
      event.preventDefault();
      closeLineEditor({ commit: true });
      cellElement.focus();
    }
  });
  editor.addEventListener("blur", () => closeLineEditor({ commit: true }));
  editor.focus();
  editor.setSelectionRange(initial.length, initial.length);
  return true;
}

// Typing updates the buffer and the pane's unsaved marker, but not the
// comparison: re-rendering mid-word would tear the editor out of the DOM, and a
// line that stops differing would take its own hunk off the screen while the
// caret is still in it. The textarea is the live feedback while the editor is
// open; the optimistic render happens when the line is committed.
function applyLineEdit() {
  if (!lineEditor) return;
  const buffer = editBufferFor(lineEditor.side);
  if (!buffer) return;
  if (buffer.setLine(lineEditor.index, lineEditor.element.value)) {
    markEditedPanes();
  }
}

function closeLineEditor(options = {}) {
  if (!lineEditor) return;
  const { element, cell, side, index, initial } = lineEditor;
  lineEditor = null;
  editComposing = false;
  const buffer = editBufferFor(side);
  let moved = false;
  if (options.revert) moved = Boolean(buffer && buffer.setLine(index, initial));
  else if (options.commit) moved = Boolean(buffer && buffer.setLine(index, element.value));
  // A line that was typed and then closed differs from the comparison on
  // screen; anything the caller changed here, including a revert, has to reach
  // it. The value may also have moved during the edit without moving now.
  const differs = buffer && buffer.line(index) !== renderedLine(side, index);
  if (moved) markEditedPanes();
  element.remove();
  cell.classList.remove("editing");
  const content = cell.querySelector(".tx");
  if (content) content.hidden = false;
  if (differs) {
    // Paint the committed line before the server answers, then let the
    // authoritative comparison replace it (#258). `differs` was read from the
    // pre-optimistic DOM above, so the catch-up is still requested.
    applyOptimisticLineEdit(side, index);
    scheduleEditRecompare();
  }
}

// renderedLine reports what the comparison on screen was built from, so a
// closed editor only triggers a re-comparison when the two disagree.
function renderedLine(side, index) {
  const cellElement = document.querySelector(
    `#result .cell.selectable-line[data-side="${side}"][data-line="${index}"] .tx`);
  return cellElement ? cellElement.textContent : null;
}

// Typing must not re-run the comparison on every keystroke, and never while an
// IME is mid-word. The coordinator coalesces the burst: an idle burst arms one
// timer, and edits that land while a comparison is running raise one dirty flag
// for a single catch-up run. It also cancels nothing explicitly — `compare`
// aborts the previous request through `currentAbort` and its generation guard
// rejects a superseded response (#169, #258).
function scheduleEditRecompare() {
  editRecompute.request();
}

// ---- Optimistic line render (#258) ----
//
// A cheap approximation of the new result, painted without the server: the
// edited cell shows the line the user committed, and, only when the comparison
// is a plain exact diff with a counterpart on screen, the pair is re-judged as
// same/changed. The server's answer replaces the whole result when it arrives,
// and `compare` restores the scroll anchor, so the reader's position survives.
//
// This is only honest for a single committed line edit. Comparison-condition
// changes (ignore rules, move detection, sync points, paths) change the whole
// result and cannot be guessed from the buffers, so those keep the existing
// skeleton-and-wait behaviour; the code comments at `runCompare` mark that.
let provisionalBar = null;

function lineCell(side, index) {
  return document.querySelector(
    `#result .cell.selectable-line[data-side="${side}"][data-line="${index}"]`);
}

function siblingCell(cell) {
  const row = cell?.parentElement;
  if (!row) return null;
  return [...row.children].find((node) => node !== cell && node.classList.contains("cell")) || null;
}

function cellText(cell) {
  const tx = cell?.querySelector(".tx");
  return tx ? tx.textContent : null;
}

function markProvisionalCell(cell, plan) {
  if (!cell) return;
  cell.classList.add("provisional");
  cell.classList.toggle("provisional-same", plan.classify && plan.same);
  cell.dataset.provisional = plan.classify ? (plan.same ? "same" : "changed") : "unknown";
}

function showProvisionalNotice(plan) {
  const result = $("result");
  if (!result) return;
  if (!provisionalBar) {
    provisionalBar = document.createElement("div");
    provisionalBar.className = "provisional-bar";
    provisionalBar.setAttribute("role", "status");
  }
  if (provisionalBar.parentElement !== result) result.prepend(provisionalBar);
  provisionalBar.textContent = plan.classify
    ? t("provisionalNotice")
    : t("provisionalNoticeApprox");
  provisionalActive = true;
}

// clearProvisional drops every provisional mark. The whole result is rebuilt
// from the authoritative data on success, so this mostly covers failures,
// cancellations and leaving edit mode, where the approximation would otherwise
// stay on screen with nothing coming to correct it.
function clearProvisional() {
  provisionalActive = false;
  if (provisionalBar) {
    provisionalBar.remove();
    provisionalBar = null;
  }
  for (const cell of document.querySelectorAll("#result .cell.provisional")) {
    cell.classList.remove("provisional", "provisional-same");
    delete cell.dataset.provisional;
  }
}

function applyOptimisticLineEdit(side, index) {
  if (!provisionalRenderingPossible()) return false;
  const buffer = editBufferFor(side);
  const cell = lineCell(side, index);
  const counterpart = siblingCell(cell);
  const plan = optimisticPlanLine({
    editedValue: buffer.line(index) ?? "",
    counterpartValue: counterpart ? cellText(counterpart) : null,
    plain: optimisticIsPlain(requestBody()),
  });
  if (!plan.render) return false;
  const tx = cell.querySelector(".tx");
  if (tx) tx.textContent = buffer.line(index) ?? "";
  markProvisionalCell(cell, plan);
  if (counterpart) markProvisionalCell(counterpart, plan);
  showProvisionalNotice(plan);
  return true;
}

// A provisional render needs a text diff that is on screen and an active edit
// session; the line itself also has to be rendered, or there is no cell to
// paint. Any failure here leaves the caller on the existing compare path.
function provisionalRenderingPossible() {
  return editingEnabled() && $("mode").value === "text" &&
    Boolean(lastData?.hunks?.length) && $("result").children.length > 0;
}

// ---- Gutter change bars (#292) ----
//
// A pane being edited carries two different signals. The comparison's own
// difference is the hunk rendering: the cell shading, and the -/+ marker in the
// unified view. On top of that, a line whose current text differs from the file
// the buffer loaded carries a distinct gutter handle, so "this was already a
// difference" and "I typed this" never blur together while confirming data.
//
// The per-side regions are cached here and refreshed whenever an edit lands, so
// a render that asks once per cell does not recompute them once per line.
let localChangeMaps = { old: new Map(), new: new Map() };

function refreshLocalChangeMaps() {
  for (const side of ["old", "new"]) {
    const buffer = editBufferFor(side);
    localChangeMaps[side] = buffer
      ? localChangeIndex(buffer.original(), buffer.lines())
      : new Map();
  }
}

// A glyph, not only a colour: the mark has to survive a monochrome screen and
// colour blindness, and it names what happened to the line.
const GUTTER_MARKS = { added: "+", removed: "\u2212", modified: "~" };

// applyLocalChangeMark gives one cell its gutter handle. It is a real element
// so it can be an accessible marker (role, label, tooltip) and a click target,
// and so it reads apart from the comparison's shading. kind is null when the
// line matches the baseline, which removes a handle the line used to carry.
function applyLocalChangeMark(cellElement, kind) {
  const existing = cellElement.querySelector(".gutter-change");
  if (!kind) {
    if (existing) existing.remove();
    cellElement.classList.remove("local-change");
    delete cellElement.dataset.localChange;
    return;
  }
  const line = Number(cellElement.dataset.line);
  const label = t("gutterLocalChange", { line: line + 1 });
  cellElement.classList.add("local-change");
  cellElement.dataset.localChange = kind;
  let mark = existing;
  if (!mark) {
    const gutter = cellElement.querySelector(".ln");
    if (!gutter) return;
    mark = document.createElement("span");
    mark.className = "gutter-change";
    mark.setAttribute("role", "img");
    // The handle and the click that opens the line editor share the cell, so
    // this must not bubble or it would start editing instead of reverting.
    mark.addEventListener("click", (event) => {
      event.stopPropagation();
      revertLocalChange(cellElement.dataset.side, Number(cellElement.dataset.line));
    });
    gutter.append(mark);
  }
  mark.dataset.kind = kind;
  mark.textContent = GUTTER_MARKS[kind] || GUTTER_MARKS.modified;
  // The label describes the mark for a screen reader; the tooltip names the
  // action a pointer can take on the same element.
  mark.title = t("gutterRevert");
  mark.setAttribute("aria-label", label);
}

// Clicking a bar puts back the whole run it spans, not only the line under the
// pointer. The comparison catches up on the same debounce a keystroke uses, so
// a revert costs one comparison even when several bars are cleared in a row.
function revertLocalChange(side, line) {
  const buffer = editBufferFor(side);
  if (!buffer || buffer.readOnly() || !Number.isInteger(line)) return false;
  const region = regionAt(localChangeRegions(buffer.original(), buffer.lines()), line);
  if (!region) return false;
  closeLineEditor({ commit: true });
  const original = buffer.original();
  let moved = false;
  for (let index = region.start; index <= region.end && index < buffer.count(); index++) {
    if (buffer.setLine(index, original[index])) moved = true;
  }
  if (!moved) return false;
  markEditedPanes();
  scheduleEditRecompare();
  setStatus(t("editReverted", { side: sideLabel(side) }), "success");
  return true;
}

function markEditedPanes() {
  // The file watcher refuses to auto-reload over unsaved work by reading this
  // flag; owning it here is what connects the editor to that guard.
  document.body.dataset.unsavedChanges = editedSides().length ? "true" : "false";
  refreshLocalChangeMaps();
  for (const side of ["old", "new"]) {
    const buffer = editBufferFor(side);
    for (const marked of document.querySelectorAll(`#result .cell.selectable-line[data-side="${side}"]`)) {
      applyLocalChangeMark(marked, localChangeMaps[side].get(Number(marked.dataset.line)) || null);
    }
    const head = document.querySelector(`.pane-head.${side}`);
    if (!head) continue;
    head.classList.toggle("dirty", Boolean(buffer?.isDirty()));
    const marker = head.querySelector(".pane-head-dirty");
    if (marker) marker.hidden = !buffer?.isDirty();
    const save = head.querySelector(".pane-head-save");
    if (save) save.disabled = !buffer?.isDirty();
  }
}

async function saveEditedPane(side, options = {}) {
  const buffer = editBufferFor(side);
  if (!buffer || buffer.readOnly() || !buffer.isDirty()) return false;
  closeLineEditor({ commit: true });
  const body = {
    path: buffer.path(),
    lines: buffer.lines(),
    profile: buffer.profile(),
    expect: buffer.stamp(),
    overwrite: true,
    force: Boolean(options.force),
  };
  try {
    const response = await apiFetch("/api/file/save", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const data = await response.json();
    if (!response.ok) {
      // The file moved under the editor. That is the one case worth asking
      // about: overwriting silently would discard someone else's work.
      if (data?.code === "stale_write" && !options.force) {
        if (await askConfirm(t("editStaleConfirm", { path: buffer.path() }))) {
          return saveEditedPane(side, { force: true });
        }
        return false;
      }
      throw apiError(data, response);
    }
    buffer.accept({ stamp: data.stamp });
    markEditedPanes();
    // The watcher would otherwise report this very write as an external change
    // and re-compare on top of it, so give it the new baseline (#251, #255).
    const prepared = await prepareFileWatch();
    if (prepared) fileWatcher.start(prepared.paths, prepared.snapshot);
    setStatus(t("editSaved", { side: sideLabel(side), path: buffer.path() }), "success");
    return true;
  } catch (err) {
    setStatus(String(err.message || err), "error");
    return false;
  }
}

// ---- folder comparison: continuous view (#291) ----
//
// The tree opens one file and comes back; a hundred differing files is a
// hundred round trips. This stacks every differing file in one scroll, and only
// works by never holding all of them: a file's diff is fetched as it nears the
// viewport, and files that drift far away give their DOM back. Which files that
// means is decided in continuous.js, which node exercises directly.
let continuousView = null;

const CONTINUOUS_LOADED_LIMIT = 12;
const CONTINUOUS_TRACK_DELAY = 60;

function continuousActive() {
  return continuousView !== null;
}

function syncContinuousControls() {
  const button = $("dirContinuous");
  if (!button) return;
  button.hidden = !directoryData;
  button.setAttribute("aria-pressed", continuousActive() ? "true" : "false");
  button.classList.toggle("active", continuousActive());
}

// ---- Flattened folder view (#275) ----
// A deep tree scattered with differences means opening folders until the files
// appear. Flattening drops the hierarchy and lists the files by relative path
// in a Location column, the way Meld's flattened view does. It is orthogonal to
// the status filter, which still decides what is in the list.
function dirFlatActive() {
  const button = $("dirFlat");
  return button ? button.getAttribute("aria-pressed") === "true" : false;
}

function applyDirFlat(on, persist = true) {
  const button = $("dirFlat");
  if (!button) return;
  button.setAttribute("aria-pressed", on ? "true" : "false");
  button.classList.toggle("active", on);
  if (persist) localStorage.setItem("ayame-dirflat", on ? "1" : "0");
}

function syncDirFlatControls() {
  const button = $("dirFlat");
  if (!button) return;
  // The continuous view already has no hierarchy to flatten.
  button.hidden = !directoryData || continuousActive();
  button.classList.toggle("active", dirFlatActive());
}

async function toggleContinuousView() {
  if (!directoryData || !directoryBody) return;
  if (continuousActive()) {
    teardownContinuousView();
    await renderDirectory(directoryData, directoryBody);
    return;
  }
  await renderContinuous(directoryData, directoryBody);
}

function teardownContinuousView() {
  if (continuousView?.frame) clearTimeout(continuousView.frame);
  continuousView = null;
  syncContinuousControls();
}

async function renderContinuous(data, body) {
  teardownContinuousView();
  hideSuggestions();
  const filtered = filterDirectoryEntries(data.entries, $("dirStatus").value, $("dirSearch").value);
  const entries = continuousEntries(filtered);

  csvData = null; lastData = null; lastComparedRequest = null;
  clearUnchangedContext();
  $("syncPanel").hidden = true; $("mergePanel").hidden = true;
  $("diffNav").hidden = false;
  // Folder results have no reading axis (#263).
  syncInterpretationVisibility();
  $("dirStatusWrap").hidden = false;
  $("dirSearchWrap").hidden = false;
  syncDirFlatControls();
  for (const id of ["addSync", "clearSync", "viewModeWrap", "sidebarToggle", "confirmCounter", "prevUnconfirmed", "nextUnconfirmed"]) {
    const node = $(id); if (node) node.hidden = true;
  }
  // Navigation crosses file boundaries here, so the buttons that the tree hides
  // are exactly the ones this view needs.
  for (const id of ["firstDiff", "prevDiff", "nextDiff", "lastDiff", "diffCounter"]) {
    const node = $(id); if (node) node.hidden = false;
  }
  syncExportPatchVisibility();

  const result = $("result");
  result.innerHTML = "";
  result.append(paneHeads(data));
  if (!entries.length) {
    result.append(resultStateCard(t("completeMatch"), t("folderMatchScope", { total: fmt(data.entries.length) })));
    continuousView = { entries: [], sections: [], loaded: new Set(), pending: new Map(), focus: 0, frame: 0 };
    syncContinuousControls();
    updateContinuousCounter();
    return;
  }

  const sections = [];
  continuousView = { entries, sections, loaded: new Set(), pending: new Map(), focus: 0, body, frame: 0 };

  if (!(await renderInSlices(result, entries, (entry, index) => {
    const section = buildContinuousSection(entry, index);
    sections.push(section);
    return section;
  }))) return;

  syncContinuousControls();
  updateContinuousCounter();
  buildContinuousMinimap();
  await focusContinuousSection(0);
  setStatus("");
}

// Which file the scroll is at is measured rather than observed: the result pane
// is its own scroll container, and reading its offsets keeps the decision in
// sectionAt(), which node exercises directly. Work is done once per frame.
function trackContinuousScroll() {
  const view = continuousView;
  if (!view || !view.entries.length || view.frame) return;
  // A timer rather than an animation frame: a hidden tab never runs the frame,
  // which would leave the scroll position it was scheduled for unanswered and
  // the tracker wedged until the tab came back.
  view.frame = setTimeout(() => {
    view.frame = 0;
    if (continuousView !== view) return;
    const result = $("result");
    const offsets = view.sections.map((section) => section.offsetTop - result.offsetTop);
    const index = sectionAt(offsets, result.scrollTop + resultScrollInset() + 1);
    if (index >= 0 && index !== view.focus) void focusContinuousSection(index);
    else if (index >= 0) void primeContinuousWindow();
  }, CONTINUOUS_TRACK_DELAY);
}

// Loading only on a change of file would leave the first screen half-empty
// after a jump; priming keeps the window around the current file filled.
async function primeContinuousWindow() {
  const view = continuousView;
  if (!view) return;
  for (const target of windowAround(view.focus, view.entries.length)) {
    void loadContinuousSection(target);
  }
}

function buildContinuousSection(entry, index) {
  const section = document.createElement("section");
  section.className = `file-diff ${entry.status}`;
  section.dataset.sectionIndex = String(index);
  section.dataset.dirPath = entry.path;
  section.dataset.scrollAnchor = "continuous";
  section.dataset.scrollKey = entry.path;
  section.dataset.scrollOrder = String(index);

  const head = document.createElement("button");
  head.type = "button";
  head.className = "file-diff-head";
  head.setAttribute("aria-expanded", "true");
  const marker = document.createElement("span");
  marker.className = "dir-marker";
  marker.textContent = DIR_MARKERS[entry.status];
  marker.setAttribute("aria-hidden", "true");
  const path = document.createElement("span");
  path.className = "file-diff-path";
  path.textContent = entry.path;
  const meta = document.createElement("span");
  meta.className = "file-diff-meta";
  meta.textContent = t(entry.status);
  head.append(marker, path, meta);
  head.setAttribute("aria-label", `${t(entry.status)} ${entry.path}`);
  head.addEventListener("click", () => toggleContinuousSection(index));

  const bodyElement = document.createElement("div");
  bodyElement.className = "file-diff-body";

  section.append(head, bodyElement);
  return section;
}

// Collapsing is what makes a long change set navigable, and it also releases
// the file's DOM: a collapsed file is not worth holding.
function toggleContinuousSection(index) {
  const section = continuousView?.sections[index];
  if (!section) return;
  const head = section.querySelector(".file-diff-head");
  const open = head.getAttribute("aria-expanded") === "true";
  head.setAttribute("aria-expanded", open ? "false" : "true");
  section.classList.toggle("collapsed", open);
  if (open) unloadContinuousSection(index, { keepHeight: false });
  else void loadContinuousSection(index);
}

async function focusContinuousSection(index) {
  const view = continuousView;
  if (!view || !view.entries.length) return;
  view.focus = Math.max(0, Math.min(index, view.entries.length - 1));
  updateContinuousCounter();
  for (const target of windowAround(view.focus, view.entries.length)) {
    void loadContinuousSection(target);
  }
  trimContinuousWindow();
}

// The budget is enforced after each load rather than only on a focus change:
// a load that lands later would otherwise push the set past the limit and stay
// there until the reader moved on again.
function trimContinuousWindow() {
  const view = continuousView;
  if (!view) return;
  for (const target of unloadTargets([...view.loaded], view.focus, CONTINUOUS_LOADED_LIMIT)) {
    unloadContinuousSection(target, { keepHeight: true });
  }
}

async function loadContinuousSection(index) {
  const view = continuousView;
  const section = view?.sections[index];
  if (!view || !section) return;
  if (view.loaded.has(index) || view.pending.has(index)) return;
  if (section.classList.contains("collapsed")) return;

  const entry = view.entries[index];
  const request = directoryEntryRequest(entry, view.body.old, view.body.new);
  const payload = {
    ...requestBody(),
    inline: false,
    mode: "text",
    old: request.old,
    new: request.new,
    oldAbsent: request.oldAbsent,
    newAbsent: request.newAbsent,
    oldText: "",
    newText: "",
    syncPoints: [],
  };
  const bodyElement = section.querySelector(".file-diff-body");
  bodyElement.textContent = "";
  bodyElement.style.minHeight = "";
  section.classList.add("loading");

  const pending = (async () => {
    try {
      const response = await apiFetch("/api/diff", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      const data = await response.json();
      if (!response.ok) throw apiError(data, response);
      if (continuousView !== view) return;
      renderContinuousSectionBody(section, data, index);
      view.loaded.add(index);
      trimContinuousWindow();
    } catch (error) {
      if (continuousView !== view) return;
      const failure = document.createElement("p");
      failure.className = "file-diff-error";
      failure.textContent = String(error.message || error);
      bodyElement.append(failure);
      // A file that cannot be read is answered once, not retried on every
      // scroll frame that brings it back into view.
      view.loaded.add(index);
    } finally {
      section.classList.remove("loading");
      view.pending.delete(index);
    }
  })();
  view.pending.set(index, pending);
  await pending;
}

function renderContinuousSectionBody(section, data, index) {
  const bodyElement = section.querySelector(".file-diff-body");
  bodyElement.textContent = "";
  const meta = section.querySelector(".file-diff-meta");
  const entry = continuousView?.entries[index];
  if (!data.hunks?.length) {
    meta.textContent = `${t(entry?.status || "changed")} · ${t("completeMatch")}`;
    bodyElement.append(resultStateCard(t("completeMatch"), t("textMatchScope", {
      old: fmt(Number(data.old_lines || 0)),
      new: fmt(Number(data.new_lines || 0)),
    })));
    return;
  }
  meta.textContent = `${t(entry?.status || "changed")} · ${t("hunkCount", { count: fmt(data.hunks.length) })}`;
  prepareAccurateSyntax(data.hunks, syntaxPath("old"), syntaxPath("new"));
  for (const [hunkIndex, hunk] of data.hunks.entries()) {
    const node = renderHunk(hunk, hunkIndex);
    // renderHunk names a hunk for the single-file view; here the same number
    // repeats once per file, so the id would no longer be unique.
    node.id = `file-${index}-hunk-${hunkIndex}`;
    node.dataset.sectionIndex = String(index);
    bodyElement.append(node);
  }
}

function unloadContinuousSection(index, { keepHeight = true } = {}) {
  const view = continuousView;
  const section = view?.sections[index];
  if (!view || !section || !view.loaded.has(index)) return;
  const bodyElement = section.querySelector(".file-diff-body");
  // Releasing a file above the viewport would pull the page up under the
  // reader, so the space it took is held until it is loaded again.
  if (keepHeight) {
    const height = bodyElement.getBoundingClientRect().height;
    if (height > 0) bodyElement.style.minHeight = `${Math.round(height)}px`;
  } else {
    bodyElement.style.minHeight = "";
  }
  bodyElement.textContent = "";
  view.loaded.delete(index);
}

// Navigation crosses file boundaries a file at a time: it moves hunk to hunk
// inside the file being read, and steps to the next file at its edge. It walks
// the change set rather than the DOM, so a file that is not loaded yet is
// visited rather than skipped over.
async function continuousStep(direction, options = {}) {
  const view = continuousView;
  if (!view || !view.entries.length) return;
  if (options.edge === "first") return continuousGoToSection(0, "first");
  if (options.edge === "last") return continuousGoToSection(view.entries.length - 1, "last");

  const section = view.sections[view.focus];
  const hunks = section && !section.classList.contains("collapsed")
    ? [...section.querySelectorAll(".hunk")]
    : [];
  const current = continuousCurrentHunk(hunks);
  const next = current + (direction < 0 ? -1 : 1);
  if (next >= 0 && next < hunks.length) {
    scrollContinuousInto(hunks[next]);
    updateContinuousCounter();
    return;
  }
  const sectionIndex = stepSection(view.focus, direction, view.entries.length);
  if (sectionIndex < 0) return;
  await continuousGoToSection(sectionIndex, direction < 0 ? "last" : "first");
}

async function continuousGoToSection(index, edge) {
  const view = continuousView;
  const section = view?.sections[index];
  if (!section) return;
  await focusContinuousSection(index);
  await loadContinuousSection(index);
  if (continuousView !== view) return;
  const hunks = section.classList.contains("collapsed") ? [] : [...section.querySelectorAll(".hunk")];
  // Arriving at a file shows the file: scrolling straight to its first hunk
  // would leave the header that names it just above the viewport.
  const target = edge === "last" ? hunks[hunks.length - 1] || section : section;
  scrollContinuousInto(target);
  updateContinuousCounter();
}

function continuousCurrentHunk(hunks) {
  const result = $("result");
  const line = result.getBoundingClientRect().top + resultScrollInset() + 1;
  let current = -1;
  for (const [index, hunk] of hunks.entries()) {
    if (hunk.getBoundingClientRect().top <= line) current = index;
    else break;
  }
  return current;
}

function scrollContinuousInto(node) {
  if (!node) return;
  const result = $("result");
  const top = node.getBoundingClientRect().top - result.getBoundingClientRect().top + result.scrollTop - resultScrollInset();
  result.scrollTo({ top: Math.max(0, top), behavior: "auto" });
  if (typeof node.focus === "function") node.focus({ preventScroll: true });
}

function updateContinuousCounter() {
  const view = continuousView;
  const counter = $("diffCounter");
  if (!view || !counter) return;
  counter.textContent = t("continuousCounter", {
    current: fmt((view.entries.length ? view.focus + 1 : 0)),
    total: fmt(view.entries.length),
  });
}

// The minimap marks file boundaries here rather than hunks: it is the change
// set that is being scrolled, and a file is the unit worth aiming at.
function buildContinuousMinimap() {
  const view = continuousView;
  const map = $("minimap");
  map.querySelectorAll(".minimap-marker").forEach((el) => el.remove());
  if (!view?.entries.length) { map.hidden = true; minimapHasMarkers = false; return; }
  const wasHidden = map.hidden;
  map.hidden = false;
  const trackPixels = Math.max(1, Math.floor(map.getBoundingClientRect().height || 1));
  map.hidden = wasHidden;
  const segments = calculateMinimapSegments(view.entries.map((entry, index) => ({
    index,
    kind: entry.status === "changed" ? "Replace" : entry.status === "added" ? "Insert" : "Delete",
    displayLength: 1,
  })), trackPixels);
  for (const segment of segments) {
    const marker = document.createElement("button");
    marker.type = "button";
    marker.className = `minimap-marker ${segment.kind}`;
    marker.dataset.hunk = String(segment.index);
    marker.title = view.entries[segment.index]?.path || "";
    marker.style.top = `${segment.top * 100}%`;
    marker.style.height = `${segment.height * 100}%`;
    marker.addEventListener("click", () => void continuousGoToSection(segment.index, "first"));
    map.append(marker);
  }
  minimapHasMarkers = map.querySelector(".minimap-marker") !== null;
  updateMinimapViewport();
}

// ---- word-level diff (ported from ayame-editor web/src/search.ts) ----
// The word diff lives in worddiff.js so it can be tested without a DOM (#139).
const { inlineWordDiff, inlineTokens, pushPart } = globalThis.AyameWordDiff;
// The display-width model lives in textwidth.js, mirroring internal/textwidth,
// so the tab stops the GUI renders match the width the CLI computes (#289).
const { displayWidth, nextTabStop, normalizeTabSize } = globalThis.AyameTextWidth;
const {
  DIR_MARKERS,
  DIR_AUTO_EXPAND_LIMIT,
  buildDirTree,
  dirEntrySize,
  dirEntryStamp,
  directoryEntryRequest,
  filterDirectoryEntries,
  sortDirectoryEntries,
  locationOf,
  formatBytes,
  formatEpochNanos,
  rememberPlace,
} = globalThis.AyameDirectory;

// In-flight request controller, so the Cancel button can abort a long compare.
let currentAbort = null;

// ---- Mutual exclusion between long operations (#128) ----
// Each flow used to disable only its own button, so a comparison and an
// export, or two comparisons, could run at once and race each other's results
// into the same DOM and status line. Disabling #compare was not enough on its
// own either: drag and drop, folder-entry clicks, and sync-point edits all
// call compare() directly, never touching the button.

// busyOperation names the running operation, or null when idle.
let busyOperation = null;

// exclusiveControls are every trigger that must not fire while another
// operation is in flight. Cancel is deliberately absent: stopping the running
// operation is the one thing that must stay available.
const EXCLUSIVE_CONTROLS = [
  "compare", "exportPatch", "exportReport", "inspectCSV", "exportCSV", "saveMerge", "simulateMerge",
  "saveProject", "loadProject", "dirPreview", "saveDirProject", "loadDirProject",
  "addSync", "clearSync", "allLeft", "allRight", "allBase", "copyComparisonURL",
];

// controlsDisabledBeforeRun remembers which controls were already disabled for
// their own reasons (an empty selection, a mode that does not apply) so
// releasing the lock does not enable something that should stay off.
let controlsDisabledBeforeRun = null;

function lockExclusiveControls() {
  controlsDisabledBeforeRun = new Set();
  for (const id of EXCLUSIVE_CONTROLS) {
    const el = $(id);
    if (!el) continue;
    if (el.disabled) controlsDisabledBeforeRun.add(id);
    el.disabled = true;
  }
}

function unlockExclusiveControls() {
  for (const id of EXCLUSIVE_CONTROLS) {
    const el = $(id);
    if (!el) continue;
    el.disabled = controlsDisabledBeforeRun ? controlsDisabledBeforeRun.has(id) : false;
  }
  controlsDisabledBeforeRun = null;
}

// runExclusive runs fn with every competing trigger disabled. A second call
// while one is running is dropped rather than queued: the user pressed
// something that was already unavailable, and silently running it later would
// be more surprising than ignoring it.
async function runExclusive(name, fn) {
  if (busyOperation) return false;
  busyOperation = name;
  lockExclusiveControls();
  try {
    return await fn();
  } finally {
    busyOperation = null;
    unlockExclusiveControls();
  }
}

// requestGeneration invalidates the result of a superseded request. Even with
// the lock, an operation that was already in flight when the lock was taken
// (or one aborted and restarted) must not paint over a newer result.
let requestGeneration = 0;
function beginRequest() { return ++requestGeneration; }
function isCurrentRequest(generation) { return generation === requestGeneration; }
let lastData = null; // last diff response, retained for navigation and merge actions
let lastComparedRequest = null;
let unchangedRegions = [];
let contextLoadToken = 0;
let contextRequestID = 0;
let currentHunk = -1;
let readHunks = new Set();
let navObserver = null;
// Explicit confirmed marks (#288) are a third state beside read and ignored.
// confirmedSignatures holds one content-derived signature per current hunk and
// confirmedHunks the signatures the user confirmed; storage is keyed by the
// comparison identity so the same comparison resumes across sessions. Read-on-
// scroll stays in readHunks and is never treated as confirmation.
const CONFIRMED_STORAGE_KEY = "ayame-confirmed";
const CONFIRMED_COMPARISONS_MAX = 200;
let confirmedComparison = "";
let confirmedSignatures = [];
let confirmedHunks = new Set();
let syncSelection = { old: null, new: null };
let syncPoints = [];
let ignoredHunks = new Set();
let csvInspection = null;
let csvData = null;
// Two independent builders share one condition model: row conditions filter
// rows by value, column conditions drop columns by name (#129).
let filterState = { rows: newRowFilter(), columns: newRowFilter() };
// The header positions the comparison used as keys, so the result can name the
// key range when it truncates (#268). Empty unless the user chose key columns.
let csvKeyColumns = [];
let csvPage = 0;
const CSV_PAGE_SIZE = 100;
let browserTarget = null;
let browserAfterSelect = null;
let directoryData = null, directoryBody = null;
// Retains the explicit absent side when an added/removed folder entry opens as
// a text diff. Exact path matching prevents later manual edits from inheriting
// the flag accidentally.
let directoryEntryView = null;
let directorySearchTimer = 0;
// Merge selection is a toggle model (#271): each hunk holds the set of
// contributions it adopts, so clicking a chosen side again clears it and a hunk
// can adopt both sides. The kind fixes the canonical concatenation order.
const { createMergeSelection } = globalThis.AyameMergeSelect;
let mergeSelection = createMergeSelection("text"), mergeUndo = [], mergeRedo = [];
function resetMergeSelection(kind) { mergeSelection = createMergeSelection(kind); }
// Merge (adopt-left/right) controls are opt-in: most sessions only read diffs,
// so the per-hunk adopt buttons and the merge panel stay hidden until the user
// enters merge mode. setMergeMode syncs the body class the CSS keys off and the
// toggle's aria-pressed; updateMergeUI decides when the toggle is offered (#100).
let mergeMode = false;
// Auto-advance (#277) is opt-in: after adopting a choice it jumps to the next
// unresolved conflict, which turns adoption into a keyboard-only loop. Off by
// default so a one-off choice does not move the page under the reader.
let mergeAutoAdvance = false;
function setMergeMode(on) {
  mergeMode = on;
  document.body.classList.toggle("merge-mode", on);
  $("mergeMode").setAttribute("aria-pressed", on ? "true" : "false");
}
let threeWayData = null;
// Logical pane names supplied by a difftool/mergetool launch (#295). A VCS
// passes temporary files, so the path is not something a reader recognizes;
// these override the pane heading without changing the comparison path.
let launchLabels = { base: "", old: "", new: "" };
// BASE is on demand in the three-way view (#282): the common comparison is
// LEFT | RESULT | RIGHT and this remembers whether the reader asked for the
// ancestor column. It survives a re-compare so the choice is not lost.
let threeWayShowBase = false;

// ---- Merge provenance (#257) ----
// The server resolves the merge and reports each line's origin; the preview
// only renders it. The manual map carries typed lines by their stable key, so
// changing a conflict choice does not silently move an edit to another line.
let mergePreview = null;
let mergeManual = new Map();
let mergePreviewToken = 0;
let mergePreviewTimer = 0;
let mergeLineEditor = null;
let mergeComposing = false;

// ---- External file changes (#251) ----
// The server long-polls with os.Stat while the browser keeps the authenticated
// fetch open. EventSource is deliberately not used: it cannot attach the
// X-Ayame-Token header that protects every filesystem API call.
const AUTO_RELOAD_KEY = "ayame-auto-reload";
const EXTERNAL_CHANGE_DEBOUNCE_MS = 350;
let pendingExternalChange = null;
let externalChangeTimer = 0;

function currentWatchPaths() {
  return watchPathsForMode($("mode").value, {
    base: $("base").value,
    old: $("old").value,
    new: $("new").value,
  }, $("scratch").checked);
}

function sameWatchPaths(left, right) {
  return left.length === right.length && left.every((path, index) => path === right[index]);
}

async function requestFileWatch(paths, baseline = [], signal) {
  const response = await apiFetch("/api/watch", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ paths, baseline }),
    signal,
  });
  const data = await response.json();
  if (!response.ok) throw apiError(data, response);
  return data;
}

function hasUnsavedResultChanges() {
  // Direct pane editing (#255) will own this flag. Keeping the gate here now
  // means file watching cannot later start discarding edits merely because the
  // editor and watcher were implemented in separate changes.
  return document.body.dataset.unsavedChanges === "true";
}

function hideExternalChangeBar() {
  $("externalChangeBar").hidden = true;
}

function showExternalChangeBar() {
  $("externalChangeBar").hidden = false;
}

async function reloadExternalChange(change) {
  hideExternalChangeBar();
  pendingExternalChange = null;
  // Refresh the snapshot after the debounce. Editors often save via several
  // writes or an atomic replacement; this collapses them into one comparison.
  let prepared = change;
  try {
    const current = await requestFileWatch(change.paths);
    prepared = { ...change, snapshot: current.snapshot };
  } catch (error) {
    setStatus(t("watchFailed", { message: String(error.message || error) }), "warning");
  }
  if (!sameWatchPaths(prepared.paths, currentWatchPaths())) return false;
  return compare({ watch: prepared, external: true });
}

async function flushExternalChange() {
  externalChangeTimer = 0;
  const change = pendingExternalChange;
  if (!change || !$("autoReload").checked) return;
  if (!sameWatchPaths(change.paths, currentWatchPaths())) {
    pendingExternalChange = null;
    return;
  }
  if (hasUnsavedResultChanges()) {
    showExternalChangeBar();
    return;
  }
  if (busyOperation) {
    externalChangeTimer = setTimeout(flushExternalChange, EXTERNAL_CHANGE_DEBOUNCE_MS);
    return;
  }
  await reloadExternalChange(change);
}

function queueExternalChange(change) {
  pendingExternalChange = change;
  clearTimeout(externalChangeTimer);
  externalChangeTimer = setTimeout(flushExternalChange, EXTERNAL_CHANGE_DEBOUNCE_MS);
}

const fileWatcher = createLongPollWatcher({
  request: requestFileWatch,
  onChange: queueExternalChange,
  onError: (error) => {
    if ($("autoReload").checked) {
      setStatus(t("watchFailed", { message: String(error.message || error) }), "warning");
    }
  },
});

function stopFileWatch() {
  fileWatcher.stop();
  clearTimeout(externalChangeTimer);
  externalChangeTimer = 0;
  pendingExternalChange = null;
  hideExternalChangeBar();
}

async function prepareFileWatch() {
  stopFileWatch();
  if (!$("autoReload").checked) return null;
  const paths = currentWatchPaths();
  if (!paths.length) return null;
  try {
    const response = await requestFileWatch(paths);
    return { paths, snapshot: response.snapshot };
  } catch (error) {
    setStatus(t("watchFailed", { message: String(error.message || error) }), "warning");
    return null;
  }
}

async function armFileWatchFromCurrentState() {
  if (!(lastData || csvData || threeWayData) || !$("autoReload").checked) return;
  const prepared = await prepareFileWatch();
  if (prepared && sameWatchPaths(prepared.paths, currentWatchPaths())) {
    fileWatcher.start(prepared.paths, prepared.snapshot);
  }
}

// ---- Comparison state in the URL (#254) ----
// State lives in the fragment: browsers never send it to the local server, so
// paths do not enter access logs or Referer headers. The normal in-tab URL keeps
// its API token so reload works; the explicit copy action always removes it.
const URL_STATE_MODES = new Set(["text", "sorted", "csv", "threeway", "threeway-csv", "dir"]);
// The per-comparison controls are classified once in comparisonscope.js, so the
// URL state (#254) and the local per-comparison memory (#260) can never drift.
const URL_STATE_CONTROL_IDS = COMPARISON_CONTROLS;
let restoringComparisonURL = false;
let comparisonURLReplaceTimer = 0;
let comparisonURLRestoreGeneration = 0;

// Per-comparison conditions (#260). Display preferences are global, but what a
// comparison means is scoped to its identity — mode plus input paths, the same
// identity the versioned URL fragment carries. Switching back to a comparison
// restores the conditions it was run with instead of inheriting the last one's,
// including entry points that never touch the URL.
const CONDITION_STORE_KEY = "ayame-conditions";
let activeComparisonKey = "";
// The comparison controls' load-time values. A comparison with no memory resets
// to these rather than inheriting whatever the previous comparison left behind.
// (main's saved condition default is applied before this snapshot is taken, so
// the baseline a fresh comparison resets to is that default.)
const conditionBaseline = new Map();

// ---- Multiple comparisons (#281) ----
// Several comparisons stay open as tabs. The active tab is mirrored into the
// setup form and the result; before switching away, its comparison state and
// scroll anchor are stashed, and the target recomputes from its own state. That
// trades recomputation for bounded memory, which the issue explicitly allows as
// long as scroll position and edits survive.
let tabDoc = emptyTabDoc();
let tabSwitching = false;

function renderTabs() {
  const nav = $("comparisonTabs");
  const list = $("tabList");
  if (!nav || !list) return;
  nav.hidden = tabDoc.tabs.length === 0;
  list.innerHTML = "";
  const active = activeTabOf(tabDoc);
  for (const tab of tabDoc.tabs) {
    const selected = tab.id === active?.id;
    const item = document.createElement("div");
    item.className = "tab";
    item.classList.toggle("active", selected);
    item.dataset.tab = tab.id;

    const label = document.createElement("button");
    label.type = "button";
    label.className = "tab-label";
    label.setAttribute("role", "tab");
    label.setAttribute("aria-selected", String(selected));
    label.tabIndex = selected ? 0 : -1;
    label.textContent = tab.label;
    label.title = tab.label;
    label.addEventListener("click", () => { void switchTab(tab.id); });
    item.append(label);

    if (tabDoc.tabs.length > 1) {
      const close = document.createElement("button");
      close.type = "button";
      close.className = "tab-close";
      close.setAttribute("aria-label", t("closeTab", { label: tab.label }));
      close.textContent = "×";
      close.addEventListener("click", (event) => {
        event.stopPropagation();
        void closeTab(tab.id);
      });
      item.append(close);
    }
    list.append(item);
  }
  list.querySelector(".tab.active")?.scrollIntoView({ block: "nearest", inline: "nearest" });
}

// rememberActiveTab keeps the active tab's stored state and label in step with
// the form after a successful compare or a condition edit.
function rememberActiveTab(state) {
  const active = activeTabOf(tabDoc);
  const label = tabLabelFromState(state);
  if (!active) {
    tabDoc = addTabToDoc(tabDoc, { state, label }, { activate: true });
  } else {
    tabDoc = updateTabInDoc(tabDoc, active.id, { state, label });
  }
  renderTabs();
}

// stashActiveTab records what the outgoing tab was showing, including where it
// was scrolled to, so switching back returns to the same line.
function stashActiveTab() {
  const active = activeTabOf(tabDoc);
  if (!active) return;
  const patch = { scroll: captureResultScrollAnchor() };
  const state = captureComparisonState();
  if (state) {
    patch.state = state;
    patch.label = tabLabelFromState(state);
  }
  tabDoc = updateTabInDoc(tabDoc, active.id, patch);
}

async function loadActiveTab() {
  const active = activeTabOf(tabDoc);
  if (!active) return;
  if (!active.state || !validComparisonPaths(active.state)) {
    updateComparisonURL("replace");
    renderTabs();
    return;
  }
  restoringComparisonURL = true;
  try {
    if (!(await applyComparisonState(active.state))) {
      setStatus(t("urlStateInvalid"), "warning");
      return;
    }
    await compare({ urlHistory: "none", scrollAnchor: active.scroll || null });
  } finally {
    restoringComparisonURL = false;
  }
  updateComparisonURL("replace");
  renderTabs();
}

async function switchTab(id) {
  if (tabSwitching) return;
  const active = activeTabOf(tabDoc);
  if (!active || active.id === id || !tabDoc.tabs.some((tab) => tab.id === id)) {
    renderTabs();
    return;
  }
  if (editingEnabled() && !(await guardUnsavedEdits())) return;
  tabSwitching = true;
  try {
    stashActiveTab();
    tabDoc = activateTabInDoc(tabDoc, id);
    renderTabs();
    await loadActiveTab();
  } finally {
    tabSwitching = false;
  }
}

// openTab duplicates the current comparison: the usual next step is to change
// one side, so starting from the open state is cheaper than retyping it.
async function openTab() {
  if (tabSwitching) return;
  if (editingEnabled() && !(await guardUnsavedEdits())) return;
  stashActiveTab();
  const state = captureComparisonState();
  const label = state ? tabLabelFromState(state) : t("newTab");
  tabDoc = addTabToDoc(tabDoc, {
    state,
    label,
    scroll: state ? captureResultScrollAnchor() : null,
  }, { activate: true });
  renderTabs();
  updateComparisonURL("replace");
}

async function closeTab(id) {
  if (tabSwitching) return;
  const active = activeTabOf(tabDoc);
  if (!active || tabDoc.tabs.length <= 1) return;
  const closing = tabDoc.tabs.find((tab) => tab.id === id);
  if (!closing) return;
  if (id === active.id && editingEnabled() && !(await guardUnsavedEdits())) return;
  const wasActive = id === active.id;
  tabDoc = removeTabFromDoc(tabDoc, id);
  renderTabs();
  if (!wasActive) {
    updateComparisonURL("replace");
    return;
  }
  tabSwitching = true;
  try {
    await loadActiveTab();
  } finally {
    tabSwitching = false;
  }
}

function hasComparisonResult() {
  return Boolean(lastData || csvData || threeWayData || directoryData);
}

function captureComparisonState() {
  if ($("scratch").checked) return null;
  const mode = $("mode").value;
  if (!URL_STATE_MODES.has(mode)) return null;
  const paths = {
    base: $("base").value.trim(),
    old: $("old").value.trim(),
    new: $("new").value.trim(),
  };
  if (!paths.old || !paths.new || ((mode === "threeway" || mode === "threeway-csv") && !paths.base)) {
    return null;
  }
  const controls = {};
  for (const id of URL_STATE_CONTROL_IDS) {
    const node = $(id);
    if (!node) continue;
    controls[id] = node.type === "checkbox" ? node.checked : node.value;
  }
  const state = {
    v: 1,
    mode,
    paths,
    controls,
    syncPoints: syncPoints.map((point) => ({ old: point.old, new: point.new })),
  };
  if (mode === "csv" || mode === "threeway-csv") {
    state.csvKeys = selectedCSVColumns().map((column) => ({
      name: column.name,
      index: column.index,
    }));
  }
  return state;
}

function validComparisonPaths(state) {
  if (!URL_STATE_MODES.has(state?.mode)) return false;
  const paths = state?.paths;
  if (!paths || typeof paths.old !== "string" || typeof paths.new !== "string" ||
      typeof paths.base !== "string" || !paths.old.trim() || !paths.new.trim()) {
    return false;
  }
  return !((state.mode === "threeway" || state.mode === "threeway-csv") && !paths.base.trim());
}

async function applyComparisonState(state) {
  if (!validComparisonPaths(state)) return false;
  stopFileWatch();
  $("scratch").checked = false;
  applyScratch();
  $("mode").value = state.mode;
  for (const side of ["base", "old", "new"]) $(side).value = state.paths[side] || "";
  for (const id of URL_STATE_CONTROL_IDS) {
    const node = $(id);
    if (!node || !Object.prototype.hasOwnProperty.call(state.controls, id)) continue;
    const value = state.controls[id];
    if (node.type === "checkbox") {
      if (typeof value === "boolean") node.checked = value;
    } else if (typeof value === "string" || typeof value === "number") {
      node.value = String(value);
    }
  }
  syncPoints = Array.isArray(state.syncPoints)
    ? state.syncPoints.filter((point) =>
      Number.isInteger(point?.old) && point.old >= 0 &&
      Number.isInteger(point?.new) && point.new >= 0)
    : [];
  resetSyncSelection();
  renderSyncPoints();
  csvInspection = null;
  $("inspection").textContent = "";
  $("keySetup").hidden = true;
  syncModeOpts();
  syncCompareReady();
  updateSetupSummary();
  updateDetailsBadges();

  const keyMode = state.controls.keyMode;
  const csvMode = state.mode === "csv" || state.mode === "threeway-csv";
  if (csvMode && (keyMode === "include" || keyMode === "exclude")) {
    if (!(await inspectCSV())) return false;
    const keys = Array.isArray(state.csvKeys) ? state.csvKeys : [];
    const names = new Set(keys.map((key) => typeof key?.name === "string" ? key.name : ""));
    const indexes = new Set(keys.map((key) => Number(key?.index)).filter(Number.isInteger));
    document.querySelectorAll("#columnList input").forEach((input) => {
      input.checked = names.has(input.dataset.name) || indexes.has(Number(input.dataset.index));
    });
    $("keyMode").value = keyMode;
    syncKeyMode();
  }
  // The URL handed us a complete comparison, so it becomes the active scope and
  // seeds the local memory. compare({urlHistory:"none"}) then sees the same
  // identity and leaves these conditions alone.
  adoptCurrentComparison();
  return syncCompareReady();
}

// ---- Per-comparison conditions (#260) ----
// The local memory stores the same versioned state the URL fragment carries, so
// a comparison can be restored through applyComparisonState — CSV key
// selections and sync points included — without touching the address bar. The
// identity is the mode plus the input paths, and display preferences are never
// part of it.
function captureConditionDefaults() {
  for (const id of COMPARISON_CONTROLS) {
    const node = $(id);
    if (!node) continue;
    conditionBaseline.set(id, node.type === "checkbox" ? node.checked : node.value);
  }
}

function resetConditions() {
  for (const [id, value] of conditionBaseline) {
    const node = $(id);
    if (!node) continue;
    if (node.type === "checkbox") node.checked = value;
    else node.value = value;
  }
  refreshConditionDependents();
}

// refreshConditionDependents brings the mode-specific visibility and the
// "changed settings" badges back in step after conditions are swapped wholesale.
function refreshConditionDependents() {
  syncModeOpts();
  syncCompareReady();
  updateSetupSummary();
  updateDetailsBadges();
}

function currentComparisonIdentity() {
  if ($("scratch").checked) return "";
  const mode = $("mode").value;
  if (!URL_STATE_MODES.has(mode)) return "";
  return comparisonKey(mode, { base: $("base").value, old: $("old").value, new: $("new").value });
}

// adoptCurrentComparison marks the form as belonging to the comparison now in
// the inputs. Entry points that restore a complete state themselves — a URL
// fragment, a loaded project — call this so a later Compare does not overwrite
// what they set.
function adoptCurrentComparison() {
  const key = currentComparisonIdentity();
  if (!key) return "";
  activeComparisonKey = key;
  storeComparisonState(key, captureComparisonState());
  return key;
}

function conditionStore() {
  try { return parseConditionStore(localStorage.getItem(CONDITION_STORE_KEY)); } catch (_) { return {}; }
}

function storeComparisonState(key, state) {
  if (!key || !state) return;
  const store = writeConditions(conditionStore(), key, state);
  try { localStorage.setItem(CONDITION_STORE_KEY, serializeConditionStore(store)); } catch (_) { /* storage full or blocked */ }
}

// scopeConditionsForRun resolves the state a run should start from. The first
// comparison of a visit keeps whatever the form shows — the user may have just
// set it. Once a comparison is active, switching to a different one restores
// that comparison's own remembered state, or the defaults when it has none: it
// never quietly inherits the previous comparison's. keepConditions skips the
// swap for a re-run that is the same comparison reversed (#90).
async function scopeConditionsForRun(keepConditions) {
  const key = currentComparisonIdentity();
  if (!key) return "";
  if (keepConditions || !activeComparisonKey || key === activeComparisonKey) {
    activeComparisonKey = key;
    return key;
  }
  const remembered = readConditions(conditionStore(), key);
  if (!remembered || !(await applyComparisonState(remembered))) {
    resetConditions();
    activeComparisonKey = key;
  }
  return key;
}

function comparisonURLHasState() {
  return new URLSearchParams(location.hash.slice(1)).has(COMPARISON_HASH_KEY);
}

function updateComparisonURL(action = "replace") {
  if (restoringComparisonURL || action === "none") return false;
  const state = captureComparisonState();
  if (!state) return false;
  rememberActiveTab(state);
  try {
    const comparisonURL = buildComparisonURL(location.href, state, true);
    let next = comparisonURL;
    try {
      next = buildTabStateURL(comparisonURL, serializeTabDoc(tabDoc), true);
    } catch (error) {
      // The active comparison still fits, but the whole tab set does not: keep
      // the single-comparison URL rather than lose the comparison too (#281).
      if (error?.code !== "STATE_TOO_LARGE") throw error;
      next = buildTabStateURL(comparisonURL, null, true);
    }
    if (next === location.href) return true;
    const metadata = { ayameComparison: true };
    if (action === "push") history.pushState(metadata, "", next);
    else history.replaceState(metadata, "", next);
    return true;
  } catch (error) {
    setStatus(t(error?.code === "STATE_TOO_LARGE" ? "urlStateTooLarge" : "urlStateInvalid"), "warning");
    return false;
  }
}

function scheduleComparisonURLReplace() {
  if (restoringComparisonURL || !hasComparisonResult()) return;
  clearTimeout(comparisonURLReplaceTimer);
  comparisonURLReplaceTimer = setTimeout(() => {
    comparisonURLReplaceTimer = 0;
    updateComparisonURL("replace");
  }, 100);
}

async function waitForIdleOperation() {
  cancelCurrentOperation();
  while (busyOperation) {
    await new Promise((resolve) => setTimeout(resolve, 20));
  }
}

async function restoreComparisonFromURL(state) {
  const generation = ++comparisonURLRestoreGeneration;
  await waitForIdleOperation();
  if (generation !== comparisonURLRestoreGeneration) return false;
  restoringComparisonURL = true;
  try {
    if (!(await applyComparisonState(state))) {
      setStatus(t("urlStateInvalid"), "warning");
      return false;
    }
    if (generation !== comparisonURLRestoreGeneration) return false;
    const restored = await compare({ urlHistory: "none" });
    // A legacy or shared URL carries one comparison and no tab set; give it a
    // tab so the bar reflects what is on screen (#281).
    if (restored) rememberActiveTab(state);
    return restored;
  } finally {
    if (generation === comparisonURLRestoreGeneration) restoringComparisonURL = false;
  }
}

function fallbackCopyText(value) {
  const input = document.createElement("textarea");
  input.value = value;
  input.readOnly = true;
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.append(input);
  input.select();
  let copied = false;
  try { copied = document.execCommand("copy"); } catch (_) { /* unavailable */ }
  input.remove();
  return copied;
}

async function copyComparisonURL() {
  const state = captureComparisonState();
  if (!state || !(await askConfirm(t("shareURLWarning")))) return;
  try {
    const url = buildShareURL(location.href, state);
    if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(url);
    else if (!fallbackCopyText(url)) throw new Error("clipboard unavailable");
    setStatus(t("sharedURLCopied"), "success");
  } catch (error) {
    setStatus(t(error?.code === "STATE_TOO_LARGE" ? "urlStateTooLarge" : "sharedURLCopyFailed"), "error");
  }
}

function syncCopyComparisonURLVisibility() {
  $("copyComparisonURL").hidden = !hasComparisonResult() || $("scratch").checked;
}

// ---- rendering ----
// appendText emits both the original whitespace and its visible representation.
// CSS swaps between them so display toggles never rebuild the diff DOM.
// appendText writes a line's text, building the whitespace-marker structure
// only when the display is actually on.
//
// Each whitespace run costs three elements (.ws wrapping .ws-original and
// .ws-visible) so CSS can swap the raw run for dots and arrows. Building them
// unconditionally made them 82% of the DOM on a large diff — 660,000 of
// 801,400 elements — for a display that is off by default, and it was the
// layout of all those nodes that froze the page (#127). Toggling the option
// re-renders, which is what applyDisplayPreferences now arranges.
function appendText(el, text, column) {
  let col = Number.isFinite(column) ? column : 0;
  // The column is only consumed to place whitespace markers, so skip the width
  // walk entirely when they are off. A large diff renders many tokens and the
  // count would otherwise be paid on every one of them (#127).
  if (!showWhitespace()) {
    el.appendChild(document.createTextNode(text));
    return col;
  }
  const opts = widthOptions();
  const re = /(\s+)|([^\s]+)/g;
  let m;
  while ((m = re.exec(text))) {
    if (m[1]) {
      const s = document.createElement("span");
      s.className = "ws";
      const original = document.createElement("span");
      original.className = "ws-original";
      original.textContent = m[1];
      const visible = document.createElement("span");
      visible.className = "ws-visible";
      const marked = visibleWhitespace(m[1], col, opts);
      visible.textContent = marked.text;
      col = marked.column;
      s.append(original, visible);
      el.appendChild(s);
    } else {
      el.appendChild(document.createTextNode(m[2]));
      col += displayWidth(m[2], opts);
    }
  }
  return col;
}

// visibleWhitespace renders a whitespace run for the "show whitespace" mode.
// Raw tabs are hidden behind the markers, so the marker itself has to reach the
// tab stop: "→" plus one "·" per remaining cell. That is what makes tabbed lines
// stay aligned while whitespace is visible, and it is where the East Asian
// Ambiguous width choice changes the result (#289).
function visibleWhitespace(run, column, options) {
  const tabSize = currentTabSize();
  let col = column;
  let out = "";
  for (const ch of run) {
    if (ch === "\t") {
      const stop = nextTabStop(col, tabSize);
      out += "→" + "·".repeat(Math.max(0, stop - col - 1));
      col = stop;
    } else if (ch === " ") {
      out += "·";
      col += 1;
    } else {
      out += ch;
      col += displayWidth(ch, options);
    }
  }
  return { text: out, column: col };
}

function widthOptions() {
  return { eastAsianAmbiguousWide: Boolean($("ambiguousWide")?.checked) };
}

function currentTabSize() {
  return normalizeTabSize($("tabSize")?.value);
}

function showWhitespace() { return $("showWs").checked; }
function syntaxPath(side) {
  if ($("scratch").checked) return "";
  return side === "old" ? $("old").value : $("new").value;
}
// ---- Accurate multi-line highlighting (#287) ----
// highlightSpans is line-local: a block comment or multi-line string that opens
// on one rendered line does not color the next. When syntax.js can carry state
// (createHighlighter), the diff highlights each hunk's lines in order, per side,
// and keeps the resulting spans in a lookup. Anywhere the state is not
// trustworthy — omitted lines between hunks, context fetched out of order, a
// language with no constructs, an older module — the lookup misses and
// appendSyntax falls back to highlightSpans, unchanged.
let syntaxAccurateSpans = new Map();
function resetAccurateSyntax() { syntaxAccurateSpans = new Map(); }
function accurateSpansFor(side, lineNo) {
  return lineNo == null ? undefined : syntaxAccurateSpans.get(`${side}:${lineNo}`);
}
function prepareAccurateSyntax(hunks, oldPath, newPath) {
  resetAccurateSyntax();
  const api = globalThis.AyameSyntax;
  if (!api?.createHighlighter || !api.lineRuns) return;
  for (const [side, path] of [["old", oldPath], ["new", newPath]]) {
    if (!api.createHighlighter(path)) continue;
    try {
      // lineRuns splits the side at every omitted-line gap; a fresh highlighter
      // per run means a construct is never carried across lines we never saw.
      for (const run of api.lineRuns(hunks, side)) {
        const highlighter = api.createHighlighter(path);
        for (const entry of run.lines) {
          syntaxAccurateSpans.set(`${side}:${entry.line}`, highlighter.highlightLine(entry.text));
        }
      }
    } catch {
      // Degradation path: an accurate pass that throws is abandoned wholesale,
      // and every lookup then misses into the unchanged line-local behavior.
      resetAccurateSyntax();
      return;
    }
  }
}

function appendSyntaxSpans(el, spans, text, column) {
  let col = Number.isFinite(column) ? column : 0;
  if (!spans) return appendText(el, text, col);
  for (const part of spans) {
    if (part.kind === "plain") { col = appendText(el, part.text, col); continue; }
    const token = document.createElement("span");
    token.className = `syn syn-${part.kind}`;
    col = appendText(token, part.text, col);
    el.append(token);
  }
  return col;
}
// appendSyntax resolves the accurate per-line spans when they are available
// (#287) and otherwise falls back to the unchanged line-local highlighting,
// threading the column offset main's renderer tracks.
function appendSyntax(el, text, path, column, spans) {
  const resolved = spans || globalThis.AyameSyntax?.highlightSpans(text, path);
  return appendSyntaxSpans(el, resolved, text, column);
}
function textSpan(parts, changedClass, path, spans) {
  const tx = document.createElement("span");
  tx.className = "tx";
  if (!parts) return tx;
  let offset = 0;
  for (const p of parts) {
    const s = document.createElement("span");
    if (p.changed) s.className = changedClass;
    // The word-diff parts concatenate back to the whole line, so clipping the
    // line's accurate spans to each part keeps both signals; the column offset
    // is threaded so the renderer keeps its position.
    const partSpans = spans ? globalThis.AyameSyntax?.sliceSpans?.(spans, offset, offset + p.text.length) : null;
    appendSyntax(s, p.text, path, offset, partSpans && partSpans.length ? partSpans : undefined);
    tx.append(s);
    offset += p.text.length;
  }
  return tx;
}

function plainSpan(text, path, spans) {
  const tx = document.createElement("span");
  tx.className = "tx";
  appendSyntax(tx, text, path, 0, spans);
  return tx;
}
function cell(cls, lineNo, node, side) {
  const c = document.createElement("div");
  c.className = "cell " + cls;
  // The kind is carried by data, not only by the background wash, so it
  // survives a colour-blind theme or a monochrome display (#298). The glyph is
  // printed in the gutter by CSS in both views; the sr-only word names the kind
  // for a screen reader, since a bare "-" is not a word. A changed pair is two
  // cells that share "chg", so the side is what tells removal from addition.
  const marker = cellMarker(cls, side);
  const kindKey = cellKindKey(cls);
  if (marker) c.dataset.marker = marker;
  if (kindKey) c.dataset.kind = kindKey;
  const ln = document.createElement("span");
  ln.className = "ln";
  ln.textContent = lineNo == null ? "" : String(lineNo);
  if (marker) ln.dataset.marker = marker;
  if (kindKey) {
    const kind = document.createElement("span");
    kind.className = "sr-only diff-kind";
    kind.textContent = t(kindKey);
    c.append(kind);
  }
  c.append(ln, node);
  if (lineNo != null && side) {
    c.classList.add("selectable-line");
    c.dataset.side = side;
    c.dataset.line = String(lineNo - 1);
    c.dataset.scrollAnchor = side;
    c.dataset.scrollKey = String(lineNo - 1);
    c.dataset.scrollOrder = String(lineNo - 1);
    // A line that differs from the file the buffer loaded carries the local
    // change handle (#292), which is a different mark from the comparison's
    // own shading on the cell.
    applyLocalChangeMark(c, localChangeMaps[side]?.get(lineNo - 1) || null);
    c.tabIndex = 0;
    c.setAttribute("role", "button");
    c.setAttribute("aria-pressed", "false");
    // Editing and manual alignment both want this click, so the mode decides:
    // while editing, a line opens for typing; otherwise it marks a sync point.
    const activate = () => {
      if (editingEnabled() && openLineEditor(c)) return;
      selectSyncLine(c);
    };
    c.addEventListener("click", activate);
    c.addEventListener("keydown", (event) => {
      // Delete puts a locally changed line back, so the gutter handle has a
      // keyboard path (#292). It only acts on a line that carries a mark, and
      // it goes through the keymap so the chord can be remapped (#285).
      if (matchesShortcut(event, "revertLine") && c.dataset.localChange) {
        event.preventDefault();
        if (revertLocalChange(side, Number(c.dataset.line))) c.focus();
        return;
      }
      if (event.key !== "Enter" && event.key !== " ") return;
      event.preventDefault();
      activate();
    });
  }
  return c;
}
function row(left, right) {
  const r = document.createElement("div");
  r.className = "row";
  r.append(left, right);
  return r;
}

// ---- Unchanged context (#267) ----
// The diff response deliberately contains hunk lines only. Context is fetched
// by bounded range after the hunk geometry is known, so a ten-million-line
// input stays a small initial response and expanding one boundary never loads
// the rest of the file by accident.
const CONTEXT_EXPAND_CHUNK = 20;
const CONTEXT_MAX_LINES = 50;
const CONTEXT_BATCH_RANGES = 400;
const CONTEXT_BATCH_LINES = 10_000;

function contextIsVisible() {
  return $("contextToggle")?.getAttribute("aria-pressed") === "true";
}

function contextLineCount() {
  const value = Number($("contextLines")?.value);
  return Number.isFinite(value) ? Math.max(0, Math.min(CONTEXT_MAX_LINES, Math.trunc(value))) : 3;
}

// A recognised language means the hidden range can name a symbol or heading;
// plain data/text falls back to the first-line excerpt (#268).
function hiddenSourceMode() {
  const language = globalThis.AyameSyntax?.languageForPath;
  if (!language) return false;
  return Boolean(language(syntaxPath("old")) || language(syntaxPath("new")));
}

function gapLabel(region, gap) {
  return describeGap({
    count: gap.count,
    oldStart: region.oldStart + gap.offset,
    newStart: region.newStart + gap.offset,
    preview: region.previews?.get(gap.offset),
    source: hiddenSourceMode(),
  }, t);
}

function clearUnchangedContext() {
  contextLoadToken++;
  unchangedRegions = [];
  syncContextVisibility();
}

function prepareUnchangedContext(data) {
  contextLoadToken++;
  // When maxHunks truncated the result, anything after the last returned hunk
  // is unclassified rather than unchanged. Do not label or fetch that tail.
  unchangedRegions = buildUnchangedRegions(data?.hunks, data?.old_lines, data?.new_lines, !data?.omitted_hunks)
    .map((region) => {
      const node = document.createElement("section");
      node.className = "context-region";
      node.dataset.contextRegion = String(region.index);
      node.setAttribute("aria-label", t("contextRegion"));
      // previews holds one hidden line per gap, fetched for the bar label only
      // (#268). It is deliberately not a segment: the line names what is hidden
      // without unfolding it.
      return { ...region, node, segments: [], previews: new Map(), pending: false };
    });
  renderAllContextRegions();
}

function appendContextRows(target, region, segment, skip = 0) {
  const oldPath = syntaxPath("old"), newPath = syntaxPath("new");
  const count = Math.min(segment.count, segment.old.length, segment.new.length);
  for (let index = skip; index < count; index++) {
    const oldLine = region.oldStart + segment.offset + index;
    const newLine = region.newStart + segment.offset + index;
    const left = cell("same", oldLine + 1, plainSpan(segment.old[index], oldPath), "old");
    const right = cell("same", newLine + 1, plainSpan(segment.new[index], newPath), "new");
    left.classList.add("old");
    right.classList.add("new");
    if (segment.old[index] === segment.new[index]) right.classList.add("context-duplicate");
    target.append(row(left, right));
  }
}

function contextGapControl(region, gap) {
  const control = document.createElement("div");
  control.className = "context-gap";
  control.dataset.contextCount = String(gap.count);
  control.dataset.contextOffset = String(gap.offset);
  const chunk = Math.min(CONTEXT_EXPAND_CHUNK, gap.count);
  const canExpandUp = region.index < unchangedRegions.length - 1;
  const canExpandDown = region.index > 0;
  const addDirection = (direction, label, title) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `context-expand ${direction}`;
    button.textContent = label;
    button.title = title;
    button.setAttribute("aria-label", title);
    button.disabled = region.pending;
    button.addEventListener("click", () => void expandContextSpan(region, gap, direction));
    control.append(button);
  };
  if (canExpandUp) addDirection("up", "↑", t("contextExpandUp", { count: fmt(chunk) }));
  const label = document.createElement("button");
  label.type = "button";
  label.className = "context-gap-label";
  label.textContent = gapLabel(region, gap);
  label.title = canExpandUp && canExpandDown
    ? t("contextExpandBoth", { count: fmt(chunk) })
    : t(canExpandUp ? "contextExpandUp" : "contextExpandDown", { count: fmt(chunk) });
  label.disabled = region.pending;
  const defaultDirection = canExpandUp && canExpandDown ? "both" : (canExpandUp ? "up" : "down");
  let dragStart = null;
  let dragged = false;
  label.addEventListener("pointerdown", (event) => {
    if (event.button !== 0) return;
    dragStart = event.clientY;
    dragged = false;
    label.setPointerCapture?.(event.pointerId);
  });
  label.addEventListener("pointerup", (event) => {
    if (dragStart == null) return;
    const distance = event.clientY - dragStart;
    dragStart = null;
    if (Math.abs(distance) < 12) return;
    dragged = true;
    const direction = distance < 0
      ? (canExpandUp ? "up" : defaultDirection)
      : (canExpandDown ? "down" : defaultDirection);
    void expandContextSpan(region, gap, direction);
    setTimeout(() => { dragged = false; }, 0);
  });
  label.addEventListener("pointercancel", () => { dragStart = null; });
  label.addEventListener("click", () => {
    if (dragged) { dragged = false; return; }
    void expandContextSpan(region, gap, defaultDirection);
  });
  control.append(label);
  if (canExpandDown) addDirection("down", "↓", t("contextExpandDown", { count: fmt(chunk) }));
  return control;
}

function renderContextRegion(region) {
  const node = region?.node;
  if (!node) return;
  node.textContent = "";
  node.hidden = !contextIsVisible() || region.count === 0;
  if (node.hidden) return;

  const segments = region.segments.slice().sort((left, right) => left.offset - right.offset);
  const gaps = new Map(missingContextSpans(region.count, segments).map((gap) => [gap.offset, gap]));
  let cursor = 0;
  for (const segment of segments) {
    const start = Math.max(0, segment.offset);
    const end = Math.min(region.count, start + segment.count);
    if (end <= cursor) continue;
    if (start > cursor) node.append(contextGapControl(region, gaps.get(cursor) || { offset: cursor, count: start - cursor }));
    const rows = document.createElement("div");
    rows.className = "context-rows";
    appendContextRows(rows, region, segment, Math.max(0, cursor - start));
    node.append(rows);
    cursor = end;
  }
  if (cursor < region.count) node.append(contextGapControl(region, gaps.get(cursor) || { offset: cursor, count: region.count - cursor }));
}

function renderAllContextRegions() {
  for (const region of unchangedRegions) renderContextRegion(region);
}

function refreshContextTranslations() {
  for (const region of unchangedRegions) {
    region.node.setAttribute("aria-label", t("contextRegion"));
    for (const gap of region.node.querySelectorAll(".context-gap")) {
      const count = Number(gap.dataset.contextCount) || 0;
      const chunk = fmt(Math.min(CONTEXT_EXPAND_CHUNK, count));
      const label = gap.querySelector(".context-gap-label");
      if (label) {
        const offset = Number(gap.dataset.contextOffset) || 0;
        label.textContent = gapLabel(region, { offset, count });
        const hasUp = Boolean(gap.querySelector(".context-expand.up"));
        const hasDown = Boolean(gap.querySelector(".context-expand.down"));
        label.title = hasUp && hasDown
          ? t("contextExpandBoth", { count: chunk })
          : t(hasUp ? "contextExpandUp" : "contextExpandDown", { count: chunk });
      }
      const up = gap.querySelector(".context-expand.up");
      if (up) {
        up.title = t("contextExpandUp", { count: chunk });
        up.setAttribute("aria-label", up.title);
      }
      const down = gap.querySelector(".context-expand.down");
      if (down) {
        down.title = t("contextExpandDown", { count: chunk });
        down.setAttribute("aria-label", down.title);
      }
    }
  }
}

function setContextPending(region, pending) {
  region.pending = pending;
  for (const button of region.node.querySelectorAll(".context-gap button")) button.disabled = pending;
}

async function renderContextRegionsSliced(regions, token) {
  let started = performance.now();
  for (const region of regions) {
    renderContextRegion(region);
    if (performance.now() - started < RENDER_BUDGET_MS) continue;
    await yieldToBrowser();
    if (token !== contextLoadToken) return false;
    started = performance.now();
  }
  return true;
}

async function loadContextRanges(ranges, options = {}) {
  if (!ranges.length || !lastComparedRequest) return false;
  const token = options.token ?? contextLoadToken;
  let source;
  try { source = JSON.parse(lastComparedRequest); } catch (_) { return false; }
  const byID = new Map();
  const prepared = ranges.map((range) => {
    const region = unchangedRegions[range.region];
    if (!region || range.count <= 0) return null;
    const id = ++contextRequestID;
    byID.set(id, { region, offset: range.offset, count: range.count });
    return {
      id,
      old_start: region.oldStart + range.offset,
      new_start: region.newStart + range.offset,
      count: range.count,
    };
  }).filter(Boolean);
  if (!prepared.length) return false;

  const affected = new Set([...byID.values()].map((item) => item.region));
  for (const region of affected) setContextPending(region, true);
  const anchor = options.preserveAnchor ? captureResultScrollAnchor() : null;
  try {
    if (options.announce) setStatus(t("contextLoading"), "busy");
    for (const batch of batchContextRanges(prepared, CONTEXT_BATCH_RANGES, CONTEXT_BATCH_LINES)) {
      const response = await apiFetch("/api/diff/context", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...source, ranges: batch }),
      });
      const data = await response.json();
      if (!response.ok) throw apiError(data, response);
      if (token !== contextLoadToken) return false;
      for (const returned of data.ranges || []) {
        const requested = byID.get(returned.id);
        if (!requested || !Array.isArray(returned.old) || !Array.isArray(returned.new)) continue;
        const count = Math.min(requested.count, returned.old.length, returned.new.length);
        if (!count) continue;
        requested.region.segments.push({
          offset: requested.offset,
          count,
          old: returned.old.slice(0, count),
          new: returned.new.slice(0, count),
        });
      }
      if (!(await renderContextRegionsSliced(affected, token))) return false;
      if (token !== contextLoadToken) return false;
    }
    if (searchOpen()) runSearch();
    updateMinimapViewport();
    return true;
  } catch (error) {
    if (token === contextLoadToken) setStatus(String(error.message || error), "error");
    return false;
  } finally {
    if (token === contextLoadToken) {
      for (const region of affected) setContextPending(region, false);
      if (anchor) restoreResultScrollAnchor(anchor);
    }
  }
}

async function loadInitialContext(options = {}) {
  if (!contextIsVisible() || !unchangedRegions.length) return true;
  const ranges = initialContextRanges(unchangedRegions, contextLineCount());
  const loaded = ranges.length ? await loadContextRanges(ranges, { ...options, token: contextLoadToken }) : true;
  if (loaded) await loadHiddenPreviews();
  return loaded;
}

// loadHiddenPreviews fetches one line at the head of every gap so the collapsed
// bar can name what it hides (#268). Previews are label-only: they are never
// appended as context rows, so folding a ten-million-line run still costs one
// line per gap rather than the whole run. Bounded like the expansion ranges.
async function loadHiddenPreviews() {
  if (!contextIsVisible() || !lastComparedRequest) return;
  let source;
  try { source = JSON.parse(lastComparedRequest); } catch (_) { return; }
  const token = contextLoadToken;
  const byID = new Map();
  const prepared = [];
  for (const region of unchangedRegions) {
    if (!region.count) continue;
    const gaps = missingContextSpans(region.count, region.segments);
    const live = new Set(gaps.map((gap) => gap.offset));
    for (const offset of [...region.previews.keys()]) {
      if (!live.has(offset)) region.previews.delete(offset);
    }
    for (const gap of gaps) {
      if (region.previews.has(gap.offset)) continue;
      const id = ++contextRequestID;
      byID.set(id, { region, offset: gap.offset });
      prepared.push({ id, old_start: region.oldStart + gap.offset, new_start: region.newStart + gap.offset, count: 1 });
    }
  }
  if (!prepared.length) return;
  try {
    for (const batch of batchContextRanges(prepared, CONTEXT_BATCH_RANGES, CONTEXT_BATCH_LINES)) {
      const response = await apiFetch("/api/diff/context", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...source, ranges: batch }),
      });
      const data = await response.json();
      if (!response.ok) throw apiError(data, response);
      if (token !== contextLoadToken) return;
      for (const returned of data.ranges || []) {
        const requested = byID.get(returned.id);
        if (!requested) continue;
        const text = returned.old?.[0] ?? returned.new?.[0];
        if (typeof text === "string") requested.region.previews.set(requested.offset, text);
      }
    }
  } catch (_) {
    // A preview is an adornment: failing to load one must not fail the context
    // the user did ask for, so the bar simply keeps its line range.
    return;
  }
  if (token !== contextLoadToken) return;
  for (const { region } of byID.values()) renderContextRegion(region);
}

async function expandContextSpan(region, gap, direction) {
  if (region.pending || gap.count <= 0) return;
  const count = Math.min(CONTEXT_EXPAND_CHUNK, gap.count);
  let ranges;
  if (direction === "up") {
    ranges = [{ region: region.index, offset: gap.offset + gap.count - count, count }];
  } else if (direction === "down") {
    ranges = [{ region: region.index, offset: gap.offset, count }];
  } else if (gap.count <= count * 2) {
    ranges = [{ region: region.index, offset: gap.offset, count: gap.count }];
  } else {
    ranges = [
      { region: region.index, offset: gap.offset, count },
      { region: region.index, offset: gap.offset + gap.count - count, count },
    ];
  }
  await loadContextRanges(ranges, { preserveAnchor: true });
  await loadHiddenPreviews();
}

function setContextVisibility(on, persist = true) {
  const button = $("contextToggle");
  button.setAttribute("aria-pressed", on ? "true" : "false");
  button.classList.toggle("active", on);
  if (persist) localStorage.setItem("ayame-context-visible", on ? "1" : "0");
  const anchor = captureResultScrollAnchor();
  renderAllContextRegions();
  restoreResultScrollAnchor(anchor);
  if (on && unchangedRegions.every((region) => region.segments.length === 0)) {
    void loadInitialContext({ preserveAnchor: true });
  }
  updateMinimapViewport();
}

function resetContextRanges() {
  contextLoadToken++;
  for (const region of unchangedRegions) {
    region.segments = [];
    region.previews.clear();
    region.pending = false;
  }
  renderAllContextRegions();
  if (contextIsVisible()) void loadInitialContext({ preserveAnchor: true });
}

function syncContextVisibility() {
  const button = $("contextToggle");
  if (!button) return;
  const mode = $("mode").value;
  button.hidden = !(Boolean(lastData?.hunks?.length) && (mode === "text" || mode === "sorted") && !continuousActive());
}

// A toolbar opened from its handle stays open until the pointer goes outside
// the hunk or Escape is pressed. Hover and focus reveal it on their own, so the
// pinned state only matters for touch and for a deliberate click on the handle.
function setHunkToolbarOpen(box, open) {
  box.classList.toggle("toolbar-open", open);
  // Closing while focus is still inside would leave :focus-within holding the
  // toolbar open, so move focus out as part of closing.
  if (!open && box.contains(document.activeElement)) document.activeElement.blur();
  box.querySelector(".hunk-toolbar-toggle")?.setAttribute("aria-expanded", open ? "true" : "false");
}

function renderHunk(h, index, confirm) {
  const box = document.createElement("div");
  box.className = "hunk";
  box.id = `hunk-${index}`;
  box.dataset.hunk = String(index);
  box.tabIndex = -1;
  if (confirm?.confirmed) box.classList.add("confirmed");
  if (h.move_id) {
    box.classList.add("moved");
    box.dataset.moveId = String(h.move_id);
  }
  if (h.downgraded) {
    // A dismissed whitespace/case-only difference: visible but subdued, and
    // not a navigation target (#269).
    box.classList.add("downgraded");
    box.title = t("downgradedHint");
  }
  const head = document.createElement("div");
  head.className = "hunk-head";
  const { text: headText } = AyameHunkHeader.header(h, t);
  // The header names the kind in words; the glyph in front adds the same
  // non-colour signal the rows carry (#298), and marks a replace as "~" so it
  // is not mistaken for a stray delete plus insert.
  head.textContent = h.move_id ? headText : `${hunkMarker(h.kind)} ${headText}`;
  if (h.downgraded) {
    const mark = document.createElement("span");
    mark.className = "hunk-downgraded";
    mark.textContent = ` ${t("downgraded")}`;
    head.append(mark);
  }
  box.dataset.kind = h.move_id ? "moved" : h.kind;
  box.setAttribute("role", "group");
  box.setAttribute("aria-label", head.textContent);

  // The hunk's actions sit in a toolbar anchored to the hunk, where the change
  // is, instead of always-visible in the head (#293). A small handle stays
  // visible so they are discoverable; pointer hover, keyboard focus inside the
  // hunk, and a touch tap all reveal the same toolbar. The action set itself is
  // pure and tested in hunkactions.js. A dismissed (downgraded) hunk has no
  // adopt/ignore affordances — it was already ignored by the comparison
  // options (#269) — so its toolbar holds only the move jump, if any.
  if (!h.downgraded || h.move_id) {
    const toolbarId = `hunk-actions-${index}`;
    const toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "hunk-toolbar-toggle";
    toggle.textContent = "⋯";
    toggle.title = t("hunkActions");
    toggle.setAttribute("aria-label", t("hunkActions"));
    toggle.setAttribute("aria-expanded", "false");
    toggle.setAttribute("aria-controls", toolbarId);
    toggle.addEventListener("click", (event) => {
      event.stopPropagation();
      setHunkToolbarOpen(box, !box.classList.contains("toolbar-open"));
    });
    const toolbar = document.createElement("div");
    toolbar.className = "hunk-toolbar";
    toolbar.id = toolbarId;
    toolbar.setAttribute("role", "group");
    toolbar.setAttribute("aria-label", t("hunkActions"));
    const actions = h.downgraded
      ? hunkActions({ moved: Boolean(h.move_id), ignored: false, mergeable: false })
        .filter((action) => action.id === "move-jump")
      : hunkActions({
        moved: Boolean(h.move_id),
        ignored: ignoredHunks.has(index),
        mergeable: $("mode").value === "text",
      });
    for (const action of actions) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = action.className;
      button.textContent = action.glyph || t(action.labelKey);
      button.title = t(action.labelKey);
      button.setAttribute("aria-label", t(action.labelKey));
      if (action.id === "move-jump") {
        button.addEventListener("click", (event) => {
          event.stopPropagation();
          const peer = [...document.querySelectorAll(`.hunk[data-move-id="${h.move_id}"]`)]
            .find((node) => Number(node.dataset.hunk) !== index);
          if (peer) jumpToHunk(Number(peer.dataset.hunk));
        });
      } else if (action.id === "ignore") {
        button.addEventListener("click", (event) => {
          event.stopPropagation();
          toggleIgnoredHunk(index);
        });
      } else if (action.side) {
        button.addEventListener("click", (event) => { event.stopPropagation(); chooseMerge(index, action.side); });
      }
      toolbar.append(button);
    }
    if (ignoredHunks.has(index)) box.classList.add("ignored");
    head.append(toggle, toolbar);
  }
  if (confirm) {
    const confirmToggle = document.createElement("button");
    confirmToggle.type = "button";
    confirmToggle.className = "hunk-confirm";
    confirmToggle.setAttribute("aria-pressed", confirm.confirmed ? "true" : "false");
    const label = t(confirm.confirmed ? "unconfirmHunk" : "confirmHunk");
    confirmToggle.textContent = t(confirm.confirmed ? "confirmed" : "confirmHunk");
    confirmToggle.title = label;
    confirmToggle.setAttribute("aria-label", label);
    confirmToggle.addEventListener("click", (event) => {
      event.stopPropagation();
      confirm.onToggle();
    });
    head.append(confirmToggle);
  }
  box.append(head);

  const rows = document.createElement("div");
  rows.className = "rows";
  const old = h.old || [], neu = h.new || [];
  const oldPath = syntaxPath("old"), newPath = syntaxPath("new");
  // Accurate spans were precomputed for this render (prepareAccurateSyntax).
  // They miss for context/ad-hoc rows, which fall back to highlightSpans.
  const oldSpans = (lineNo) => accurateSpansFor("old", lineNo);
  const newSpans = (lineNo) => accurateSpansFor("new", lineNo);

  if (h.kind === "insert") {
    for (let k = 0; k < neu.length; k++)
      rows.append(row(cell("empty", null, plainSpan("")), cell("add", h.new_start + k + 1, plainSpan(neu[k], newPath, newSpans(h.new_start + k)), "new")));
  } else if (h.kind === "delete") {
    for (let k = 0; k < old.length; k++)
      rows.append(row(cell("del", h.old_start + k + 1, plainSpan(old[k], oldPath, oldSpans(h.old_start + k)), "old"), cell("empty", null, plainSpan(""))));
  } else {
    const pairs = Math.min(old.length, neu.length);
    for (let k = 0; k < pairs; k++) {
      // The DP and its per-part spans are pure cost when the highlight is
      // off, since CSS only hid the result (#127).
      const wd = $("word").checked ? inlineWordDiff(old[k], neu[k]) : null;
      const left = cell("chg", h.old_start + k + 1, wd ? textSpan(wd.oldParts, "w-del", oldPath, oldSpans(h.old_start + k)) : plainSpan(old[k], oldPath, oldSpans(h.old_start + k)), "old");
      const right = cell("chg", h.new_start + k + 1, wd ? textSpan(wd.newParts, "w-add", newPath, newSpans(h.new_start + k)) : plainSpan(neu[k], newPath, newSpans(h.new_start + k)), "new");
      rows.append(row(left, right));
    }
    for (let k = pairs; k < old.length; k++)
      rows.append(row(cell("del", h.old_start + k + 1, plainSpan(old[k], oldPath, oldSpans(h.old_start + k)), "old"), cell("empty", null, plainSpan(""))));
    for (let k = pairs; k < neu.length; k++)
      rows.append(row(cell("empty", null, plainSpan("")), cell("add", h.new_start + k + 1, plainSpan(neu[k], newPath, newSpans(h.new_start + k)), "new")));
  }
  box.append(rows);
  return box;
}

// The server says how many hunks it dropped; the reader should not have to turn
// that into a number. One click derives the next cap from what was omitted,
// writes it to #maxHunks (which stays reachable for manual control), and runs
// the comparison again — compare() captures and restores the scroll anchor, so
// the reader keeps their place (#261).
function computeMoreButton(res) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "note-action compute-more";
  button.textContent = t("computeMoreHunks");
  button.title = t("computeMoreHunksTitle");
  button.setAttribute("aria-label", t("computeMoreHunksTitle"));
  button.addEventListener("click", () => {
    // hunk_count counts every hunk, including the omitted ones, so
    // hunk_count - omitted is the cap this run actually used.
    const used = Number(res.hunk_count) - Number(res.omitted_hunks);
    const next = nextMaxHunks(used, res.omitted_hunks);
    $("maxHunks").value = String(next);
    updateDetailsBadges();
    void compare();
  });
  return button;
}

function renderSummary(res) {
  const el = $("summary");
  el.innerHTML = "";
  // A count that names differences should reach them: the numbers were inert,
  // so finding "the deleted lines" meant scrolling (#110).
  const stat = (cls, label, n, kind) => {
    const jumpable = kind && n > 0;
    const s = document.createElement(jumpable ? "button" : "span");
    if (jumpable) {
      s.type = "button";
      s.dataset.jumpKind = kind;
      s.title = t("jumpToKind", { label });
      s.addEventListener("click", () => jumpToKind(kind));
    }
    s.className = "stat " + cls + (jumpable ? " stat-jump" : "");
    const count = document.createElement("b");
    count.textContent = fmt(n);
    s.append(count, ` ${label}`);
    return s;
  };
  el.append(
    stat("", t("hunks"), res.hunk_count),
    stat("add", t("added"), res.added, "insert"),
    stat("del", t("deleted"), res.deleted, "delete"),
    stat("chg", t("modified"), res.modified, "replace"),
  );
  // Whitespace/case-only differences are dismissed from the counts above but
  // still shown, so the summary has to name them separately (#269).
  if (res.downgraded_hunks) el.append(stat("downgraded", t("downgradedCount"), res.downgraded_hunks));
  if (res.moved_blocks) el.append(stat("move", t("moved"), res.moved_blocks));
  if (res.move_detection_skipped) {
    const skipped = document.createElement("span");
    skipped.className = "note";
    skipped.textContent = t("moveDetectionSkipped");
    el.append(skipped);
  }
  if (ignoredHunks.size) el.append(stat("", t("ignored"), ignoredHunks.size));
	const filters = activeFilters();
	if (filters.length) {
	  const applied = document.createElement("span");
	  applied.className = "note filters-active";
	  applied.textContent = `${t("activeFilters")}: ${filters.join(", ")}`;
	  el.append(applied);
	}
  if (res.omitted_hunks) {
    const n = document.createElement("span");
    n.className = "note";
    n.textContent = t("omitted", fmt(res.omitted_hunks));
    el.append(n, computeMoreButton(res));
  }
  // Show what `encoding: auto` decoded each file as, and flag a left/right
  // mismatch — the material clue when output looks garbled (#130). Present only
  // for file inputs; inline text carries no detected encoding.
  if (res.old_encoding || res.new_encoding) {
    const mismatch = encodingMismatch(res.old_encoding, res.new_encoding);
    const enc = document.createElement("span");
    enc.className = mismatch ? "note encoding-mismatch" : "note";
    enc.textContent = t("encodingDetected", { old: res.old_encoding || "—", new: res.new_encoding || "—" });
    if (mismatch) enc.textContent += ` — ${t("encodingMismatch")}`;
    // No confidence is available from the engine. Say that plainly and point at
    // the pane-header candidates rather than implying a certainty we do not have
    // (#278).
    if ([res.old_encoding, res.new_encoding].some(isJapaneseLegacy)) {
      enc.textContent += ` — ${t("encodingNoConfidence")}`;
    }
    el.append(enc);
  }
  el.hidden = false;
}

function resultStateCard(title, scope, kind = "match") {
  const card = document.createElement("div");
  card.className = `empty-state result-empty result-${kind}`;
  const heading = document.createElement("strong"); heading.textContent = title;
  const detail = document.createElement("p"); detail.textContent = scope;
  card.append(heading, detail);
  return card;
}

function comparisonUsesRules(csvMode = false) {
  return activeFilters().length > 0 || (csvMode && (
    $("tolerance").value !== "" || $("ignoreColumns").value.trim() !== "" || $("columnTolerances").value.trim() !== ""
  ));
}

// ---- Incremental rendering (#127) ----
// A large diff is far more DOM than one task can afford: 200 insert hunks of
// 100 lines each — reachable at the default caps — is 20,000 rows and roughly
// 800,000 elements, which took ~15.7s of blocked main thread when built in a
// single loop. Nothing could paint in that window, so the elapsed counter
// froze and the page ignored input.
//
// Rendering is therefore sliced: build for a few milliseconds, hand the frame
// back to the browser, continue. Content appears progressively instead of all
// at once at the end, and scrolling and typing keep working throughout.

// The slice budget and the render gate are pure and live in renderqueue.js,
// where node exercises them (#127, #128). RENDER_BUDGET_MS is how long one
// slice may build before yielding; the budget is comfortably inside a frame, so
// a slice cannot itself cause a dropped frame.
const { createSliceBudget, createRenderGate } = globalThis.AyameRenderQueue;
const RENDER_BUDGET_MS = globalThis.AyameRenderQueue.DEFAULT_BUDGET_MS;

// renderGate coalesces renders. A newer render supersedes the one in progress;
// Cancel stops it outright. The two are distinct so a cancelled comparison is
// not reported as a finished one.
const renderGate = createRenderGate();

// cancelRendering stops a render that is already painting. The request has
// resolved by then, so aborting it does nothing; without this, Cancel during a
// large render left the result to finish building (#128).
function cancelRendering() { renderGate.cancel(); }

// cancelCurrentOperation stops whatever is running: the request if one is in
// flight, and the render if its response has arrived and is being painted.
function cancelCurrentOperation() {
  if (currentAbort) currentAbort.abort();
  cancelRendering();
}

// yieldToBrowser hands the thread back, resuming on the next macrotask so the
// browser can paint and process input in between.
//
// MessageChannel rather than requestAnimationFrame, which does not fire in a
// hidden tab: a user who switches away mid-render would come back to a diff
// frozen part-way through. And rather than setTimeout, which browsers clamp to
// about a second in background tabs, turning a 3-second render into minutes.
const yieldWaiters = [];
const yieldChannel = new MessageChannel();
yieldChannel.port1.onmessage = () => {
  const resolve = yieldWaiters.shift();
  if (resolve) resolve();
};
function yieldToBrowser() {
  return new Promise((resolve) => {
    yieldWaiters.push(resolve);
    yieldChannel.port2.postMessage(0);
  });
}

// renderInSlices appends items.length nodes to target, yielding between
// slices. build(item, index) returns the node. Returns false if a newer render
// superseded this one or the user cancelled it, in which case the caller must
// not run its finishing steps.
async function renderInSlices(target, items, build) {
  // The nodes any search hits pointed at are being replaced.
  clearSearchHits();
  const token = renderGate.begin();
  const budget = createSliceBudget({ budgetMs: RENDER_BUDGET_MS, now: () => performance.now() });
  let index = 0;
  while (index < items.length) {
    const frag = document.createDocumentFragment();
    // Always place at least one node, so a single very expensive item cannot
    // stall the loop forever.
    do {
      frag.append(build(items[index], index));
      index++;
    } while (index < items.length && !budget.doneOne());
    target.append(frag);
    if (index >= items.length) break;
    setStatus(t("rendering", { done: fmt(index), total: fmt(items.length) }), "busy");
    await yieldToBrowser();
    if (!renderGate.isCurrent(token)) return false;
    budget.reset();
  }
  return true;
}

// showResultSkeleton fills the result area with placeholder cards while a
// comparison is in flight. The area used to sit blank from the moment it was
// cleared until the render finished, which read as a hung page (#127).
function showResultSkeleton() {
  const result = $("result");
  result.innerHTML = "";
  const wrap = document.createElement("div");
  wrap.className = "skeleton";
  wrap.setAttribute("aria-hidden", "true");
  for (let i = 0; i < 3; i++) {
    const card = document.createElement("div");
    card.className = "skeleton-hunk";
    const head = document.createElement("div");
    head.className = "skeleton-head";
    card.append(head);
    for (let row = 0; row < 3; row++) {
      const line = document.createElement("div");
      line.className = "skeleton-row";
      card.append(line);
    }
    wrap.append(card);
  }
  result.append(wrap);
}

// commitPanePath makes the header the working path control after a comparison.
// The setup form remains the empty-state entry point, but changing one side no
// longer means expanding it, finding the same field, and re-running by hand.
async function commitPanePath(input, side, comparedPath) {
  const value = input.value.trim();
  if (value === comparedPath || busyOperation) return;
  if (!(await guardUnsavedEdits())) {
    input.value = comparedPath;
    return;
  }
  $(side).value = value;
  csvInspection = null;
  $("inspection").textContent = "";
  $("keySetup").hidden = true;
  syncCompareReady();
  if (!value || $("compare").disabled) {
    setStatus(t("enterPaths"), "error");
    input.focus();
    return;
  }
  await compare();
}

// Per-side encoding corrections (#278). Detection is per side, so a fix must be
// too: each entry remembers the encoding chosen for one concrete path, which
// keeps a correction attached to the file it was made for even after a path
// edit or a swap. An absent entry, or one for a different path, means "auto".
let encodingOverrides = { old: null, new: null };

function encodingOverrideFor(side, path) {
  const entry = encodingOverrides[side];
  return entry && entry.path === path ? entry.encoding : "";
}

function setEncodingOverride(side, path, encoding) {
  encodingOverrides[side] = { path, encoding };
}

function clearEncodingOverrides() {
  encodingOverrides = { old: null, new: null };
}

// paneHeads keeps every input identified and editable at any scroll position.
// Text, CSV, and folder comparisons have two sides; 3-way comparisons add BASE.
// The full path, line count, and detected encoding remain available as a
// tooltip even when the narrow input has to elide the path.
function paneHeads(data = {}) {
  const heads = document.createElement("div");
  heads.className = "pane-heads";
  const scratch = $("scratch").checked;
  const threeWay = $("mode").value === "threeway" || $("mode").value === "threeway-csv";
  const specs = threeWay
    ? [["base", t("sideBase")], ["old", t("sideLeft")], ["new", t("sideRight")]]
    : [["old", t("sideLeft")], ["new", t("sideRight")]];
  heads.classList.toggle("three", threeWay);
  // One left/right comparison for the whole header so each side can mark a
  // mismatch with the other (#130, #278).
  const mismatch = encodingMismatch(data.old_encoding, data.new_encoding);
  for (const [side, labelText] of specs) {
    const path = $(side).value;
    const head = document.createElement("div");
    head.className = `pane-head ${side}`;
    const label = document.createElement("span");
    label.className = "pane-head-label";
    label.textContent = labelText;
    if (side === "old") label.dataset.oppositeLabel = t("sideRight");
    const alias = launchLabels[side];
    let aliasNode = null;
    if (alias) {
      aliasNode = document.createElement("span");
      aliasNode.className = "pane-head-alias";
      aliasNode.textContent = alias;
      aliasNode.title = `${alias}\n${path}`;
    }
    const name = document.createElement(scratch ? "span" : "input");
    name.className = "pane-head-path";
    if (scratch) {
      name.textContent = t("scratch");
    } else {
      name.type = "text";
      name.value = path;
      name.setAttribute("list", `${side}History`);
      name.setAttribute("aria-label", t("panePath", { side: labelText }));
      name.spellcheck = false;
      name.addEventListener("change", () => commitPanePath(name, side, path));
      name.addEventListener("keydown", (event) => {
        if (event.key !== "Enter" || event.isComposing) return;
        event.preventDefault();
        name.blur();
      });
    }
    const encoding = data[`${side}_encoding`] || "";
    const lines = data[`${side}_lines`];
    const details = [path];
    if (lines != null) details.push(t("lineCount", { count: fmt(Number(lines)) }));
    if (encoding) details.push(`${t("encoding")}: ${encoding}`);
    name.title = details.filter(Boolean).join("\n");
    head.append(label);
    if (aliasNode) head.append(aliasNode);
    head.append(name);
    if (encoding || lines != null) {
      const meta = document.createElement("span");
      meta.className = "pane-head-meta";
      if (encoding && mismatch) {
        meta.classList.add("encoding-mismatch");
        meta.title = t("encodingMismatch");
      }
      meta.textContent = [
        encoding,
        lines != null ? t("lineCount", { count: fmt(Number(lines)) }) : "",
      ].filter(Boolean).join(" · ");
      head.append(meta);
    }
    // A file on disk can be decoded wrongly; this picker re-reads one side with
    // a different codec and re-runs the comparison, which keeps the scroll
    // anchor (#278). Offered only where a detected encoding exists — a file text
    // diff — so scratch text and structured results (which report none) do not
    // grow a control that would do nothing.
    if (encoding && !scratch && (side === "old" || side === "new")) {
      const picker = document.createElement("select");
      picker.className = "pane-head-encoding";
      picker.title = t("encodingSwitchTitle");
      picker.setAttribute("aria-label", t("encodingSwitch", { side: labelText }));
      const selected = encodingPickerValue(encodingOverrideFor(side, path));
      // The alternatives the engine cannot rank: auto (re-detect) plus every
      // concrete codec other than the guessed one. Keep the user's own choice
      // in the list even after it becomes the detected one.
      const values = ["auto", ...encodingCandidates(encoding)];
      if (!values.includes(selected)) values.splice(1, 0, selected);
      for (const value of values) {
        const option = document.createElement("option");
        option.value = value;
        // "auto" keeps detecting; naming the current guess only in its label
        // keeps it readable on narrow layouts, where the meta is hidden.
        option.textContent = value === "auto" ? t("encodingAutoDetected", { encoding }) : value;
        picker.append(option);
      }
      picker.value = selected;
      picker.addEventListener("change", () => {
        setEncodingOverride(side, path, picker.value);
        void compare();
      });
      head.append(picker);
    }
    // While editing, the header is where the pane's state and its save live:
    // a marker for unsaved lines, a save button for this side alone (Meld saves
    // the file the cursor is in), and a read-only badge when there is no point
    // offering either (#255).
    const buffer = editBufferFor(side);
    if (buffer) {
      if (buffer.readOnly()) {
        const locked = document.createElement("span");
        locked.className = "pane-head-readonly";
        locked.textContent = t("editReadOnlyBadge");
        locked.title = t("editReadOnlyTitle");
        head.append(locked);
      } else {
        const dirty = document.createElement("span");
        dirty.className = "pane-head-dirty";
        dirty.textContent = "\u25cf";
        dirty.title = t("editUnsaved");
        dirty.setAttribute("aria-label", t("editUnsaved"));
        dirty.hidden = !buffer.isDirty();
        const save = document.createElement("button");
        save.type = "button";
        save.className = "pane-head-save";
        save.textContent = t("editSave");
        save.title = t("editSaveTitle", { side: labelText });
        save.setAttribute("aria-label", t("editSaveTitle", { side: labelText }));
        save.disabled = !buffer.isDirty();
        save.addEventListener("click", () => void saveEditedPane(side));
        head.append(dirty, save);
      }
      head.classList.toggle("dirty", Boolean(buffer.isDirty()));
    }
    if (!scratch) {
      const browse = document.createElement("button");
      browse.type = "button";
      browse.className = "pane-head-browse";
      browse.textContent = "…";
      browse.title = t("browseFile");
      browse.setAttribute("aria-label", t("browseFile"));
      // Keep the input from committing its old typed value before the browser
      // dialog gets a chance to return the selected path.
      browse.addEventListener("pointerdown", (event) => event.preventDefault());
      browse.addEventListener("click", () => openBrowser(side, async (selected) => {
        name.value = selected;
        await commitPanePath(name, side, path);
      }));
      head.append(browse);
    }
    heads.append(head);
  }
  const swap = document.createElement("button");
  swap.type = "button";
  swap.className = "pane-head-swap";
  swap.textContent = "⇄";
  swap.title = t("swapSides");
  swap.setAttribute("aria-label", t("swapSides"));
  if (threeWay) swap.style.left = "66.666%";
  swap.addEventListener("click", swapSides);
  heads.append(swap);
  return heads;
}

// ---- Statistics panel (#120) ----
// The result opens with a breakdown of what changed, beyond the kind totals in
// the status bar: every changed CSV column with its share and numeric delta, or
// the whole-file context for a text diff. The exports reuse the summary already
// in the browser, so downloading an artifact never re-runs the comparison.
let statsDocument = null;

function statsCell(text, className) {
  const cell = document.createElement("td");
  if (className) cell.className = className;
  cell.textContent = text;
  return cell;
}

function statsRow(label, cells) {
  const row = document.createElement("tr");
  const head = document.createElement("th");
  head.scope = "row";
  head.textContent = label;
  row.append(head, ...cells);
  return row;
}

function statsBarCell(fraction) {
  const cell = document.createElement("td");
  cell.className = "stat-bar";
  const fill = document.createElement("span");
  fill.className = "stat-bar-fill";
  const value = Math.max(0, Math.min(1, Number(fraction) || 0));
  fill.style.width = `${Math.round(value * 100)}%`;
  cell.append(fill);
  return cell;
}

// A delta carries its sign so an increase is distinguishable from a decrease;
// -0 is normalized to 0 because the sum of opposite deltas can be negative zero.
function statsDeltaText(value) {
  const number = Number(value) || 0;
  const rounded = Math.abs(number) < 1e-9 ? 0 : number;
  return `${rounded > 0 ? "+" : ""}${Number(rounded.toFixed(6))}`;
}

function buildStatsTable(kind) {
  const table = document.createElement("table");
  table.className = "stats-table";
  const head = document.createElement("thead");
  const body = document.createElement("tbody");
  table.append(head, body);
  const headerRow = (labels) => {
    const row = document.createElement("tr");
    for (const label of labels) {
      const th = document.createElement("th");
      th.textContent = label;
      row.append(th);
    }
    return row;
  };
  if (kind === "csv") {
    head.append(headerRow([t("statsColumn"), t("statsChanged"), t("statsShare"), "", t("statsDeltaSum"), t("statsDeltaMean"), t("statsDeltaMax"), t("statsDirection")]));
    for (const column of statsDocument.columns) {
      const cells = [
        statsCell(column.count.toLocaleString()),
        statsCell(formatStatsPercent(column.share, 1)),
        statsBarCell(column.bar / 100),
      ];
      if (column.numeric) {
        cells.push(
          statsCell(statsDeltaText(column.numeric.sum)),
          statsCell(statsDeltaText(column.numeric.mean)),
          statsCell(statsDeltaText(column.numeric.max)),
          statsCell(`${column.numeric.increased} / ${column.numeric.decreased} / ${column.numeric.unchanged}`),
        );
      } else {
        for (let i = 0; i < 4; i++) cells.push(statsCell("—"));
      }
      body.append(statsRow(column.name, cells));
    }
    return table;
  }
  head.append(headerRow([t("statsMetric"), t("statsCount"), t("statsShare"), ""]));
  const summary = statsDocument.summary;
  const total = summary.total_lines || 0;
  const rows = [
    [t("hunks"), summary.hunk_count, null],
    [t("added"), summary.added, summary.added],
    [t("deleted"), summary.deleted, summary.deleted],
    [t("modified"), summary.modified, summary.modified],
  ];
  if (summary.moved_blocks) rows.push([t("moved"), summary.moved_blocks, null]);
  rows.push([t("statsChangedLines"), summary.changed_lines, summary.changed_lines]);
  rows.push([t("statsLargestHunk"), summary.largest_hunk, null]);
  for (const [label, value, counted] of rows) {
    const share = counted === null || !total ? 0 : counted / total;
    body.append(statsRow(label, [
      statsCell(value.toLocaleString()),
      statsCell(counted === null ? "—" : formatStatsPercent(share, 1)),
      statsBarCell(share),
    ]));
  }
  return table;
}

function statsPanel(kind, data) {
  statsDocument = buildStats(kind, data);
  const section = document.createElement("section");
  section.className = "stats-view";
  section.id = "statsView";
  const head = document.createElement("header");
  head.className = "stats-head";
  const title = document.createElement("strong");
  title.textContent = t("statisticsTitle");
  const actions = document.createElement("span");
  actions.className = "stats-actions";
  for (const [format, label] of [["csv", t("statsExportCSV")], ["json", t("statsExportJSON")]]) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "stats-export";
    button.textContent = label;
    button.addEventListener("click", () => exportStats(format));
    actions.append(button);
  }
  head.append(title, actions);
  const scroll = document.createElement("div");
  scroll.className = "stats-scroll";
  scroll.append(buildStatsTable(kind));
  section.append(head, scroll);
  return section;
}

function statsArtifactName(format) {
  return `ayame-stats.${format}`;
}

function exportStats(format) {
  if (!statsDocument) return;
  const json = format === "json";
  const name = statsArtifactName(json ? "json" : "csv");
  const blob = new Blob([json ? statsToJSON(statsDocument) : statsToCSV(statsDocument)], {
    type: `${json ? "application/json" : "text/csv"};charset=utf-8`,
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
  setStatus(t("statsExported", name), "success");
}

// renderResult draws a diff response once. Display preferences only toggle
// classes on the completed DOM via applyDisplayPreferences. It returns false
// when the render was superseded or cancelled before it finished, so the caller
// does not treat a stopped render as a completed comparison.
async function renderResult(data) {
  // The authoritative result replaces the optimistic one wholesale; the
  // provisional banner and marks are inside #result and go with it (#258).
  clearProvisional();
  hideSuggestions();
  applyDisplayPreferences();
  renderSummary(data);
  // The cells about to be built ask for their local change mark, so the cached
  // regions have to describe the buffers as they are now (#292).
  if (editingEnabled()) refreshLocalChangeMaps();
  const result = $("result");
  result.innerHTML = "";
  resetMergeRowIndex();
  setupNavigation(data);
  prepareConfirmations(data.hunks);
  updateCounter();
  syncExportPatchVisibility();
  result.append(paneHeads(data));
  result.append(statsPanel("text", data));
  if (!data.hunks.length) {
    clearUnchangedContext();
    const scope = t("textMatchScope", { old: fmt(data.old_lines), new: fmt(data.new_lines) });
    result.append(resultStateCard(t(comparisonUsesRules() ? "filteredMatch" : "completeMatch"), scope));
    return true;
  }
  prepareUnchangedContext(data);
  prepareAccurateSyntax(data.hunks, syntaxPath("old"), syntaxPath("new"));
  const complete = await renderInSlices(result, data.hunks, (hunk, index) => {
    const fragment = document.createDocumentFragment();
    fragment.append(
      unchangedRegions[index].node,
      renderHunk(hunk, index, {
        confirmed: isConfirmed(index),
        onToggle: () => toggleConfirmedHunk(index),
      }),
    );
    if (index === data.hunks.length - 1) fragment.append(unchangedRegions[index + 1].node);
    return fragment;
  });
  if (!complete) return false;
  syncContextVisibility();
  const contextComplete = await loadInitialContext({ announce: true });
  if (contextComplete) setStatus("");
  // Re-apply an open search to the diff that just replaced the old one (#118).
  if (searchOpen()) runSearch();
  updateMergeUI();
  observeHunks();
  buildMinimap(data);
  updateMinimapViewport();
  return true;
}

function mutateMerge(mutator) {
  mergeUndo.push(mergeSelection.clone());
  if (mergeUndo.length > 100) mergeUndo.shift();
  mergeRedo = [];
  mutator();
  updateMergeUI();
  // The remaining count is the merge flow's position indicator, so it is the
  // one thing worth announcing after a choice (#298).
  announce($("mergeUnresolved").textContent);
}
// chooseMerge toggles one contribution: clicking a chosen side clears it, so
// selecting and deselecting are the same gesture (P4Merge-style, #271). The
// "both" side is #277's explicit union choice and selects left and right
// together; auto-advance then jumps to the next unresolved conflict.
function chooseMerge(id, side) {
  mutateMerge(() => {
    if (side === "both") mergeSelection.set(id, ["left", "right"]);
    else mergeSelection.toggle(id, side);
  });
  if (mergeAutoAdvance) advanceAfterAdopt(id);
}
// syncMergeRow paints the adopted sides on one hunk or CSV row: the border
// classes the CSS keys off and the buttons' aria-pressed state.
function syncMergeRow(row, id) {
  if (!row) return;
  for (const side of mergeSelection.order) {
    const adopted = mergeSelection.has(id, side);
    row.classList.toggle(`merge-${side}`, adopted);
    const button = row.querySelector(`.choose-${side}`);
    if (button) button.setAttribute("aria-pressed", adopted ? "true" : "false");
  }
  // "both" is not a stored side; it is the union of left and right (#277), so
  // its pressed state and the row's union highlight come from isBoth.
  const both = row.querySelector(".choose-both");
  const isBoth = mergeSelection.isBoth(id);
  if (both) both.setAttribute("aria-pressed", isBoth ? "true" : "false");
  row.classList.toggle("merge-both", isBoth);
}

// ---- Unresolved-difference list and implicit-resolution target (#272) ----
// A save used to confirm once and silently resolve every remaining conflict to
// the left. These helpers list the unresolved items so the user can jump to
// them, and read the target a save uses when they still choose to continue.

// mergeKind names the active merge so the target list and the item labels can
// be chosen without re-reading the mode at every call site.
function mergeKind() {
  const mode = $("mode").value;
  if (mode === "threeway") return "threeway-text";
  if (mode === "threeway-csv") return "threeway-csv";
  if (mode === "csv") return "csv";
  return "text";
}

function mergeTargetLabel(target) {
  switch (target) {
    case "right": return t("right");
    case "base": return t("sideBase");
    case "markers": return t("targetMarkersShort");
    default: return t("left");
  }
}

function mergeTargetOptions() { return mergeTargetsFor(mergeKind()); }

function currentUnresolvedTarget() {
  const value = $("mergeUnresolvedTarget")?.value || "left";
  return mergeTargetOptions().includes(value) ? value : "left";
}

function textHunkLabel(hunk, index) {
  const kind = t(hunk.kind === "insert" ? "added" : hunk.kind === "delete" ? "deleted" : "modified");
  return `#${index + 1} · ${kind} · ${hunk.new_start + 1}`;
}
function csvDifferenceLabel(diff, index) {
  return `#${index + 1} · ${diff.kind} · ${String(diff.id).slice(0, 8)}`;
}
function threeWayEventLabel(event) {
  const id = String(event.id).slice(0, 10);
  if (Array.isArray(event.key) && event.key.length) return `#${id} · ${event.key.join(" / ")}`;
  const at = Number.isFinite(event.base_start) ? ` · ${t("sideBase")} ${event.base_start + 1}` : "";
  return `#${id} · ${t("conflicts")}${at}`;
}

// mergeDecidableItems lists the items a save must decide and the total the
// server will count. Capped text hunks have no row to list; carrying the total
// keeps the confirmation honest about them.
function mergeDecidableItems() {
  const kind = mergeKind();
  if (kind === "threeway-text" || kind === "threeway-csv") {
    const events = (threeWayData?.events || []).filter((event) => event.kind === "conflict");
    return { kind, items: events.map((event) => ({ id: event.id, label: threeWayEventLabel(event) })),
      total: threeWayData?.conflicts || 0 };
  }
  if (kind === "csv") {
    const diffs = csvData?.differences || [];
    return { kind, items: diffs.map((diff, index) => ({ id: diff.id, label: csvDifferenceLabel(diff, index) })),
      total: csvData?.difference_count || diffs.length };
  }
  const hunks = lastData?.hunks || [];
  return { kind, items: hunks.map((hunk, index) => ({ id: index, label: textHunkLabel(hunk, index) })),
    total: lastData?.hunk_count ?? hunks.length };
}

function mergeUnresolvedState() {
  const { kind, items, total } = mergeDecidableItems();
  const list = [];
  for (let index = 0; index < items.length; index++) {
    const item = items[index];
    if (!mergeSelection.selected(item.id)) list.push({ index, id: item.id, label: item.label });
  }
  const count = Math.max(0, total - mergeSelection.size());
  return { kind, list, count, hidden: Math.max(0, count - list.length) };
}

function jumpToMergeItem(id) {
  let node = mergeRowIndex.get(String(id));
  if (!node || node.isConnected === false) node = document.getElementById(`hunk-${id}`);
  if (!node) return;
  node.scrollIntoView({ block: "center", behavior: "smooth" });
  if (typeof node.focus === "function") node.focus({ preventScroll: true });
}

function renderMergeUnresolved(state) {
  const box = $("mergeUnresolvedBox"), list = $("mergeUnresolvedList");
  list.innerHTML = "";
  box.hidden = state.count === 0;
  for (const item of state.list) {
    const entry = document.createElement("li");
    const button = document.createElement("button");
    button.type = "button";
    button.className = "merge-unresolved-jump";
    button.textContent = item.label;
    button.addEventListener("click", () => jumpToMergeItem(item.id));
    entry.append(button);
    list.append(entry);
  }
  if (state.hidden > 0) {
    const entry = document.createElement("li");
    entry.className = "merge-unresolved-more";
    entry.textContent = t("unresolvedMore", { count: state.hidden });
    list.append(entry);
  }
  const go = $("mergeGoUnresolved");
  go.disabled = state.list.length === 0;
  go.onclick = state.list.length ? () => jumpToMergeItem(state.list[0].id) : null;
}

// The target select offers only what the active endpoint can honor: two-way CSV
// has no marker form, and only three-way text has BASE.
function updateUnresolvedTargetOptions() {
  const select = $("mergeUnresolvedTarget");
  if (!select) return;
  const allowed = mergeTargetOptions();
  for (const option of select.options) option.hidden = !allowed.includes(option.value);
  if (!allowed.includes(select.value)) select.value = "left";
}

function refreshMergeUnresolved() {
  updateUnresolvedTargetOptions();
  const state = mergeUnresolvedState();
  $("mergeUnresolved").textContent = t("unresolved", state.count);
  renderMergeUnresolved(state);
}

// reportMergeSaved keeps "saved" distinct from "no conflicts remain": a file
// written with implicit side choices or leftover markers says so, and only a
// fully decided merge reads as a plain success (#272).
function reportMergeSaved(data, target, path) {
  const markers = typeof data?.conflictsRemaining === "number" ? data.conflictsRemaining : (Number(data?.conflictMarkers) || 0);
  const implicit = Array.isArray(data?.implicitlyResolved) ? data.implicitlyResolved.length : (Number(data?.unresolved) || 0);
  if (markers > 0) { setStatus(t("mergeSavedWithMarkers", { path, count: markers }), "warning"); return; }
  if (implicit > 0) { setStatus(t("mergeSavedWithImplicit", { path, count: implicit, target: mergeTargetLabel(target) }), "warning"); return; }
  setStatus(t("mergeSaved", data.output), "success");
}
// mergeChoiceKey maps a position in the current result to the key mergeSelection
// stores: a hunk index for a two-way text diff, an event/difference ID for a
// three-way or CSV result.
function mergeChoiceKey(index) {
  if (threeWayData) return threeWayData.events[index]?.id ?? index;
  if (csvData && $("mode").value === "csv") return csvData.differences[index]?.id ?? index;
  return index;
}

// mergeIndexForKey is the inverse, used to know where auto-advance starts. It
// returns -1 when the key is not part of the current result.
function mergeIndexForKey(key) {
  if (threeWayData) return threeWayData.events.findIndex((event) => String(event.id) === String(key));
  if (csvData && $("mode").value === "csv") return csvData.differences.findIndex((item) => String(item.id) === String(key));
  const index = Number(key);
  return Number.isInteger(index) ? index : -1;
}

// mergeUnresolvedIndexes lists the positions auto-advance may visit: conflicts
// for a three-way result, every difference for a two-way text diff. CSV rows are
// not hunk elements, so they are left to the buttons.
function mergeUnresolvedIndexes() {
  if (threeWayData) return activeConflictIndexes();
  if (csvData && $("mode").value === "csv") return [];
  return activeHunkIndexes();
}

// advanceAfterAdopt moves to the next unresolved position after the one just
// resolved. nextUnresolved wraps, so the loop keeps going instead of dead-ending
// at the last conflict.
function advanceAfterAdopt(key) {
  const from = mergeIndexForKey(key);
  if (from < 0) return;
  const next = nextUnresolved(
    mergeUnresolvedIndexes(),
    from,
    (index) => mergeSelection.selected(mergeChoiceKey(index)),
    1,
  );
  if (next != null) jumpToHunk(next);
}

function setMergeAutoAdvance(on) {
  mergeAutoAdvance = Boolean(on);
  const box = $("mergeAutoAdvance");
  if (box) box.checked = mergeAutoAdvance;
  setStatus(t(mergeAutoAdvance ? "autoAdvanceOn" : "autoAdvanceOff"), "");
}

function mergePanelVisible() {
  const panel = $("mergePanel");
  return Boolean(panel) && !panel.hidden;
}
function updateMergeUI() {
	$("mergeMode").hidden = true; // the merge-mode toggle is a text-diff affordance (#100)
  const autoAdvanceBox = $("mergeAutoAdvance");
  if (autoAdvanceBox) autoAdvanceBox.checked = mergeAutoAdvance;
	if (threeWayData && ($("mode").value === "threeway" || $("mode").value === "threeway-csv")) { updateThreeWayMergeUI(); return; }
	$("allBase").hidden = true;
	$("toggleBase").hidden = true; // BASE is a three-way-only column (#282)
	if ($("mode").value === "csv" && csvData) { updateCSVMergeUI(); return; }
  const mergeable = Boolean(lastData?.hunks?.some((hunk) => !isDowngraded(hunk))) && $("mode").value === "text";
  // Offer the toggle whenever a text diff can be merged, but keep the adopt
  // buttons (CSS) and the merge panel hidden until the user opts into merge mode.
  $("mergeMode").hidden = !mergeable;
  $("mergePanel").hidden = !(mergeable && mergeMode);
  if (!mergeable) return;
  lastData.hunks.forEach((hunk, index) => {
    if (isDowngraded(hunk)) return;
    syncMergeRow($(`hunk-${index}`), index);
  });
  refreshMergeUnresolved();
  $("mergeUndo").disabled = mergeUndo.length === 0; $("mergeRedo").disabled = mergeRedo.length === 0;
}
// mergeRowIndex maps a merge id to the row that represents it, filled while
// rendering. Both merge UIs used to find their rows with a document-wide
// attribute selector, once per event, on every merge click. The three-way
// result has no cap on its event count, so a file with thousands of conflicts
// meant thousands of full-document scans per click (#154).
let mergeRowIndex = new Map();
// The three-way result pane for each event, so a choice rewrites just that
// pane instead of re-rendering the comparison (#282).
let mergeResultIndex = new Map();

function resetMergeRowIndex() { mergeRowIndex = new Map(); }
function indexMergeRow(id, node) { mergeRowIndex.set(String(id), node); }
function resetMergeResultIndex() { mergeResultIndex = new Map(); }
function indexMergeResult(id, node) { mergeResultIndex.set(String(id), node); }

function updateCSVMergeUI() {
  $("mergePanel").hidden = false;
  for (const [id, row] of mergeRowIndex) syncMergeRow(row, id);
  refreshMergeUnresolved();
  $("mergeUndo").disabled = mergeUndo.length === 0; $("mergeRedo").disabled = mergeRedo.length === 0;
}
function undoMerge() {
  if (!mergeUndo.length) return;
  mergeRedo.push(mergeSelection.clone());
  mergeSelection = mergeUndo.pop(); updateMergeUI();
  announce($("mergeUnresolved").textContent);
}
function redoMerge() {
  if (!mergeRedo.length) return;
  mergeUndo.push(mergeSelection.clone());
  mergeSelection = mergeRedo.pop(); updateMergeUI();
  announce($("mergeUnresolved").textContent);
}
// mergeSelection's wire choices index the rendered hunk list, which may include
// dismissed hunks; the merge API computes over real differences only, so remap
// the keys before sending them (#269).
function essentialChoices(wire, hunks) {
  const out = {};
  for (const [index, side] of Object.entries(wire || {})) {
    const rank = essentialIndex(hunks || [], Number(index));
    if (rank >= 0) out[rank] = side;
  }
  return out;
}

// ---- Simulate / Do impact preview (#273) ----
// Overwriting an input is the one destructive thing this screen does. Before
// the overwrite is confirmed the user sees what will be written and where, as a
// list, in the same spirit as KDiff3's "Simulate it / Do it". The simulation is
// pure (AyameSimulate); only the wording is resolved here.
function mergeUnresolvedCount(mode) {
  if (mode === "threeway" || mode === "threeway-csv")
    return mergeSelection.unresolved(threeWayData?.conflicts || 0);
  if (mode === "csv")
    return mergeSelection.unresolved(csvData?.difference_count || csvData?.differences?.length || 0);
  return mergeSelection.unresolved(lastData?.hunk_count || 0);
}

function buildMergeImpact(mode, unresolved) {
  const threeWay = mode === "threeway" || mode === "threeway-csv";
  return globalThis.AyameSimulate.mergeImpact({
    mode,
    output: $("mergeOutput").value.trim(),
    old: $("old").value.trim(),
    new: $("new").value.trim(),
    base: threeWay ? $("base").value.trim() : "",
    overwrite: $("mergeOverwrite").checked,
    unresolved,
  });
}

function impactRoleLabel(role) {
  if (role === "base") return t("sideBase");
  return role === "old" ? t("sideLeft") : t("sideRight");
}

// Each row is plain data for askConfirm; the dialog renders it as a list.
function mergeImpactRows(impact) {
  const rows = [];
  if (impact.output) rows.push({ kind: "write", text: t("impactWrite", { path: impact.output }) });
  for (const input of impact.affected)
    rows.push({ kind: "overwrite", text: t("impactOverwrite", { role: impactRoleLabel(input.role), path: input.path }) });
  if (impact.unresolved > 0) rows.push({ kind: "info", text: t("unresolved", impact.unresolved) });
  return rows;
}

function mergeImpactPrompt(impact) {
  return impact.affected.length && impact.overwrite ? t("overwriteWarning") : t("impactLead");
}

// The distinct "Simulate" step: show the list, then let Proceed execute. The
// save is told the overwrite was already confirmed so it does not ask again.
async function previewMergeImpact() {
  const mode = $("mode").value;
  if (!$("mergeOutput").value.trim()) { setStatus(t("requiredField", { field: t("outputPath") }), "error"); return; }
  const impact = buildMergeImpact(mode, mergeUnresolvedCount(mode));
  if (!await askConfirm(mergeImpactPrompt(impact), mergeImpactRows(impact))) return;
  mergeImpactConfirmed = true;
  try { await saveMergeResult(); } finally { mergeImpactConfirmed = false; }
}

async function saveTextMerge(previewConfirmed) {
  const output = $("mergeOutput").value.trim();
  if (!output) { setStatus(t("requiredField", { field: t("outputPath") }), "error"); return; }
  const target = currentUnresolvedTarget();
  const unresolved = mergeUnresolvedState().count;
  const allowUnresolved = unresolved > 0 && await askConfirm(t("unresolvedWarning", { count: unresolved, target: mergeTargetLabel(target) }));
  if (unresolved > 0 && !allowUnresolved) return;
  const overwrite = $("mergeOverwrite").checked;
  const impact = buildMergeImpact("text", unresolved);
  const confirmOverwrite = previewConfirmed || !overwrite || await askConfirm(mergeImpactPrompt(impact), mergeImpactRows(impact));
  if (!confirmOverwrite) return;
  const body = { ...requestBody(), output, choices: essentialChoices(mergeSelection.toWire(), lastData?.hunks), allowUnresolved, unresolvedTarget: allowUnresolved ? target : "", overwrite, confirmOverwrite };
  $("saveMerge").disabled = true;
  try {
    const response = await apiFetch("/api/merge/text", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const data = await response.json(); if (!response.ok) throw apiError(data, response);
    reportMergeSaved(data, target, data.output);
  } catch (err) { setStatus(String(err.message || err), "error"); }
  finally { $("saveMerge").disabled = false; }
}
async function saveCSVMerge(previewConfirmed) {
  const output = $("mergeOutput").value.trim();
  if (!output) { setStatus(t("requiredField", { field: t("outputPath") }), "error"); return; }
  const target = currentUnresolvedTarget();
  const unresolved = mergeUnresolvedState().count;
  const allowUnresolved = unresolved > 0 && await askConfirm(t("unresolvedWarning", { count: unresolved, target: mergeTargetLabel(target) }));
  if (unresolved > 0 && !allowUnresolved) return;
  const overwrite = $("mergeOverwrite").checked;
  const impact = buildMergeImpact("csv", unresolved);
  const confirmOverwrite = previewConfirmed || !overwrite || await askConfirm(mergeImpactPrompt(impact), mergeImpactRows(impact));
  if (!confirmOverwrite) return;
  const body = { ...csvRequestBody(), output, choices: mergeSelection.toWire(), allowUnresolved, unresolvedTarget: allowUnresolved ? target : "", overwrite, confirmOverwrite };
  $("saveMerge").disabled = true;
  try {
    const response = await apiFetch("/api/merge/csv", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const data = await response.json(); if (!response.ok) throw apiError(data, response);
    reportMergeSaved({ output: data.output, unresolved: data.summary?.unresolved_rows }, target, data.output);
  } catch (err) { setStatus(String(err.message || err), "error"); }
  finally { $("saveMerge").disabled = false; }
}
// mergeImpactConfirmed is set by the Simulate dialog's Proceed so the save it
// starts does not ask the same question twice. It is consumed exactly once.
let mergeImpactConfirmed = false;

function runSaveMergeResult() {
  const previewConfirmed = mergeImpactConfirmed;
  mergeImpactConfirmed = false;
  if ($("mode").value === "threeway" || $("mode").value === "threeway-csv") return saveThreeWayMerge(previewConfirmed);
  return $("mode").value === "csv" ? saveCSVMerge(previewConfirmed) : saveTextMerge(previewConfirmed);
}
// A merge writes a file from the current inputs, so it must not run while a
// comparison for different inputs is still in flight (#128).
async function saveMergeResult() { return runExclusive("saveMerge", runSaveMergeResult); }

function threeWayRequestBody() { return { ...requestBody(), base: $("base").value.trim() }; }
function threeLines(value, csvMode) { return csvMode ? (value || []).map((row) => row.join("\t")) : (value || []); }
async function renderThreeWay(data, csvMode) {
  threeWayData = { ...data, csvMode }; csvData = null;
  hideSuggestions();
  lastComparedRequest = null;
  clearUnchangedContext();
  const summary = $("summary"); summary.innerHTML = "";
  const add = (label, value, cls = "") => { const item = document.createElement("span"); item.className = `stat ${cls}`; const b = document.createElement("b"); b.textContent = value; item.append(b, ` ${label}`); summary.append(item); };
  add(t("conflicts"), data.conflicts, "del"); add(t("left"), data.left_only); add(t("right"), data.right_only); add(t("same"), data.same_change); if (data.merged) add(t("autoMerged"), data.merged, "add"); summary.hidden = false;
  const result = $("result"); result.innerHTML = "";
  result.append(paneHeads(data));
  resetMergeRowIndex(); resetMergeResultIndex();
  setThreeWayBase(threeWayShowBase);
  lastData = {
    old_lines: data.base_lines || data.events.length,
    new_lines: data.base_lines || data.events.length,
    hunks: data.events.map((event) => ({
      kind: event.kind === "conflict" ? "replace" : "insert",
      minimap_kind: event.kind === "conflict" ? "conflict" : "insert",
      old_start: event.base_start || 0,
      new_start: event.base_start || 0,
      old_len: event.base_len || 1,
      new_len: event.base_len || 1,
      display_len: Math.max(
        event.base?.length || event.base_len || 1,
        event.left?.length || 0,
        event.right?.length || 0,
        event.combined?.length || 0,
      ),
    })),
  };
  syncExportPatchVisibility();
  setupNavigation(lastData);
  const buildEvent = (event, index) => {
    const box = document.createElement("section");
    box.className = `hunk three-event ${event.kind}`; box.id = `hunk-${index}`; box.dataset.hunk = String(index); box.dataset.mergeId = String(event.id); box.tabIndex = -1;
    box.dataset.scrollAnchor = csvMode ? "threeway-csv" : "threeway";
    box.dataset.scrollKey = String(event.id);
    box.dataset.scrollOrder = String(event.base_start ?? index);
    indexMergeRow(event.id, box);
    const head = document.createElement("header"); head.className = "hunk-head";
    // A conflict is colour-coded purple by default; the ≠ glyph makes it
    // readable without that colour, and `data-kind` gives tests and CSS a hook
    // to the same fact (#298).
    const kindLabel = event.kind === "conflict" ? `${CONFLICT_MARK} ${event.kind}` : event.kind;
    head.append(document.createTextNode(`${kindLabel} #${String(event.id).slice(0, 10)} · ${csvMode ? event.key.join(" / ") : `${t("sideBase")} ${event.base_start + 1},${event.base_len}`}`));
    box.dataset.kind = event.kind;
    box.setAttribute("role", "group");
    if (event.kind === "conflict") {
      const actions = document.createElement("span"); actions.className = "hunk-merge";
      for (const [side, label] of [["left", t("chooseLeft")], ["base", t("chooseBase")], ["right", t("chooseRight")], ["both", t("chooseBoth")]]) { const button = document.createElement("button"); button.type = "button"; button.className = `choose-${side}`; button.textContent = label; button.setAttribute("aria-pressed", "false"); button.title = t("mergeToggleHint"); button.onclick = () => chooseMerge(event.id, side); actions.append(button); }
      head.append(actions);
    }
    const grid = document.createElement("div"); grid.className = "three-grid";
    // LEFT | RESULT | RIGHT, with BASE only when asked for. The result pane
    // is the merge output: what a save would write for this event (#282).
    const choice = mergeSelection.sides(event.id).join(",");
    for (const paneData of threeWayPanes(event, choice, threeWayShowBase)) {
      const pane = renderThreeWayPane(document.createElement("section"), paneData, csvMode);
      if (paneData.role === "result") {
        pane.dataset.choice = choice;
        indexMergeResult(event.id, pane);
      }
      grid.append(pane);
    }
    box.append(head, grid);
    return box;
  };
  // Sliced like the text path: a three-way result can carry as many events as
  // a diff carries hunks, and this loop appended straight into the live DOM
  // rather than a fragment, so it was the heavier of the two (#127). Returns
  // false when the render was stopped before it finished.
  if (data.events.length && !(await renderInSlices(result, data.events, buildEvent))) return false;
  if (data.events.length) setStatus("");
  if (!data.events.length) {
    const scope = csvMode
      ? t("threeWayCSVMatchScope", { columns: fmt((data.header || []).length) })
      : t("threeWayTextMatchScope", { lines: fmt(Number(data.base_lines || 0)) });
    result.append(resultStateCard(t(comparisonUsesRules(csvMode) ? "filteredMatch" : "completeMatch"), scope));
  }
  observeHunks(); updateThreeWayMergeUI(); buildMinimap(lastData); updateMinimapViewport();
  return true;
}
// setThreeWayBase shows or hides the ancestor column and keeps the toggle's
// pressed state in step with it (#282).
function setThreeWayBase(show) {
  threeWayShowBase = Boolean(show);
  $("result").classList.toggle("show-base", threeWayShowBase);
  $("toggleBase").setAttribute("aria-pressed", threeWayShowBase ? "true" : "false");
}

function threeWayPaneLabel(role) {
  if (role === "base") return t("sideBase");
  if (role === "left") return t("sideLeft");
  if (role === "right") return t("sideRight");
  return t("mergeResult");
}

// One column of a three-way event. The result column is the merge output and
// is rebuilt in place when a conflict choice changes; the other columns are
// drawn once at render time.
function renderThreeWayPane(pane, paneData, csvMode) {
  pane.className = `three-pane ${paneData.role}`;
  pane.classList.toggle("unresolved", Boolean(paneData.unresolved));
  pane.innerHTML = "";
  const title = document.createElement("h3");
  title.textContent = threeWayPaneLabel(paneData.role);
  if (paneData.unresolved) {
    const badge = document.createElement("span");
    badge.className = "three-pane-badge";
    badge.textContent = t("unresolvedBadge");
    title.append(" ", badge);
  }
  pane.append(title);
  for (const line of threeLines(paneData.lines, csvMode)) {
    const row = document.createElement("div");
    row.className = "three-line";
    row.textContent = line;
    pane.append(row);
  }
  return pane;
}

function updateThreeWayMergeUI() {
	$("allBase").hidden = false;
  $("toggleBase").hidden = !threeWayData;
  $("mergePanel").hidden = !threeWayData;
  syncMergeProvenancePanel();
  if (!threeWayData) return;
  for (const event of threeWayData.events) {
    syncMergeRow(mergeRowIndex.get(String(event.id)), event.id);
    // Rewrite only the event whose choice changed; a three-way result has no
    // cap on its event count, so a full pass would be O(events) per click (#154).
    const side = mergeSelection.sides(event.id).join(",");
    const pane = mergeResultIndex.get(String(event.id));
    if (pane && pane.dataset.choice !== side) {
      pane.dataset.choice = side;
      renderThreeWayPane(pane, { role: "result", ...threeWayResultLines(event, side) }, threeWayData.csvMode);
    }
  }
  refreshMergeUnresolved();
  $("mergeUndo").disabled = mergeUndo.length === 0; $("mergeRedo").disabled = mergeRedo.length === 0;
  $("toggleBase").setAttribute("aria-pressed", threeWayShowBase ? "true" : "false");
  // A choice change rewrites the result, so an open preview has to follow it.
  if (mergeProvenanceOpen()) scheduleMergePreview();
}

// The preview shows the result the save would write, one line per row, with
// KDiff3's summary-column marker in the gutter. Manual is `m` and a grey wash,
// so a typed line is never mistaken for an adopted one (#257).
function mergeProvenanceOpen() {
  const panel = $("mergeProvenance");
  return Boolean(panel && !panel.hidden && panel.open);
}

function syncMergeProvenancePanel() {
  const panel = $("mergeProvenance");
  if (!panel) return;
  // A CSV row is a record rather than a line and already carries its own
  // origin, so per-line provenance is a text-mode view (#257).
  const show = Boolean(threeWayData && !threeWayData.csvMode);
  panel.hidden = !show;
  if (!show) resetMergePreview();
}

function resetMergePreview() {
  mergePreview = null;
  mergeManual = new Map();
  mergePreviewToken++;
  if (mergePreviewTimer) { clearTimeout(mergePreviewTimer); mergePreviewTimer = 0; }
  const counts = $("mergeProvenanceCounts");
  if (counts) counts.textContent = "";
  const lines = $("mergePreviewLines");
  if (lines) lines.innerHTML = "";
}

function scheduleMergePreview() {
  if (mergePreviewTimer) clearTimeout(mergePreviewTimer);
  mergePreviewTimer = setTimeout(() => { mergePreviewTimer = 0; void refreshMergePreview(); }, 120);
}

async function refreshMergePreview() {
  if (!threeWayData || threeWayData.csvMode || !mergeProvenanceOpen()) return;
  const token = ++mergePreviewToken;
  try {
    const body = {
      ...threeWayRequestBody(),
      choices: mergeSelection.toWire(),
      manual: Object.fromEntries(mergeManual),
    };
    const response = await apiFetch("/api/three-way/text/preview", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const data = await response.json();
    if (!response.ok) throw apiError(data, response);
    if (token !== mergePreviewToken || !mergeProvenanceOpen()) return;
    mergePreview = data;
    // Drop edits whose input line the current choices no longer produce, so a
    // stale override cannot come back with a later choice.
    const keys = new Set((data.lines || []).map((line) => line.key));
    for (const key of [...mergeManual.keys()]) if (!keys.has(key)) mergeManual.delete(key);
    renderMergePreview();
  } catch (err) {
    if (token === mergePreviewToken) setStatus(String(err.message || err), "error");
  }
}

function renderMergePreview() {
  const container = $("mergePreviewLines");
  if (!container) return;
  closeMergeLineEditor({ commit: false });
  container.innerHTML = "";
  if (!mergePreview) return;
  const fragment = document.createDocumentFragment();
  (mergePreview.lines || []).forEach((line, index) => fragment.append(mergePreviewRow(line, index)));
  container.append(fragment);
  updateMergeProvenanceCounts();
}

function mergePreviewRow(line, index) {
  const row = document.createElement("div");
  row.className = `merge-preview-line ${provenanceOriginClass(line.origin)}`;
  row.dataset.key = String(line.key || "");
  row.dataset.index = String(index);
  row.tabIndex = 0;
  const gutter = document.createElement("span");
  gutter.className = "merge-origin-marker";
  const text = document.createElement("span");
  text.className = "merge-line-text";
  text.textContent = line.text;
  row.append(gutter, text);
  styleMergePreviewRow(row, line.origin);
  row.addEventListener("click", () => openMergeLineEditor(row, line));
  row.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") { event.preventDefault(); openMergeLineEditor(row, line); }
  });
  return row;
}

function styleMergePreviewRow(row, origin) {
  row.classList.remove("origin-base", "origin-left", "origin-right", "origin-manual");
  row.classList.add(provenanceOriginClass(origin));
  const marker = row.querySelector(".merge-origin-marker");
  if (!marker) return;
  marker.textContent = provenanceMarker(origin);
  const label = t(provenanceLabelKey(origin));
  marker.title = label;
  marker.setAttribute("aria-label", label);
}

function updateMergeProvenanceCounts() {
  const el = $("mergeProvenanceCounts");
  if (!el) return;
  if (!mergePreview) { el.textContent = ""; return; }
  const counts = provenanceBreakdown(mergePreview.provenance);
  el.textContent = t("mergeProvenanceCounts", {
    adopted: counts.adopted.toLocaleString(),
    base: counts.base.toLocaleString(),
    left: counts.left.toLocaleString(),
    right: counts.right.toLocaleString(),
    manual: counts.manual.toLocaleString(),
    unresolved: Number(mergePreview.unresolved || 0).toLocaleString(),
  });
}

// A merged line is editable in place. The edit is held in mergeManual under the
// line's key and sent with the save, so the written file and the report agree
// on what was typed (#257).
function openMergeLineEditor(row, line) {
  if (!line || !row) return;
  closeMergeLineEditor({ commit: true });
  const text = row.querySelector(".merge-line-text");
  if (!text) return;
  const editor = document.createElement("textarea");
  editor.className = "line-editor merge-line-editor";
  editor.rows = 1;
  editor.spellcheck = false;
  editor.value = line.text;
  editor.setAttribute("aria-label", t("mergeEditLine", { line: Number(row.dataset.index) + 1 }));
  text.hidden = true;
  row.classList.add("editing");
  row.append(editor);
  mergeLineEditor = { element: editor, row, line, before: line.text, beforeOrigin: line.origin };
  editor.addEventListener("compositionstart", () => { mergeComposing = true; });
  editor.addEventListener("compositionend", () => { mergeComposing = false; applyMergeLineEdit(); });
  editor.addEventListener("input", () => { if (!mergeComposing) applyMergeLineEdit(); });
  editor.addEventListener("click", (event) => event.stopPropagation());
  editor.addEventListener("keydown", (event) => {
    event.stopPropagation();
    if (event.isComposing || mergeComposing) return;
    if (event.key === "Escape") { event.preventDefault(); closeMergeLineEditor({ revert: true }); row.focus(); }
    else if (event.key === "Enter") { event.preventDefault(); closeMergeLineEditor({ commit: true }); row.focus(); }
  });
  editor.addEventListener("blur", () => closeMergeLineEditor({ commit: true }));
  editor.focus();
  editor.setSelectionRange(editor.value.length, editor.value.length);
}

function applyMergeLineEdit() {
  if (!mergeLineEditor) return;
  const { element, row, line } = mergeLineEditor;
  mergeManual.set(line.key, element.value);
  line.text = element.value;
  line.origin = "manual";
  const text = row.querySelector(".merge-line-text");
  if (text) text.textContent = element.value;
  styleMergePreviewRow(row, "manual");
}

function closeMergeLineEditor(options = {}) {
  if (!mergeLineEditor) return;
  const { element, row, line, before, beforeOrigin } = mergeLineEditor;
  mergeLineEditor = null;
  mergeComposing = false;
  if (options.revert) {
    line.text = before;
    line.origin = beforeOrigin;
    if (beforeOrigin === "manual") mergeManual.set(line.key, before);
    else mergeManual.delete(line.key);
  } else if (options.commit && element.value !== before) {
    line.text = element.value;
    line.origin = "manual";
    mergeManual.set(line.key, element.value);
  } else {
    line.text = before;
    line.origin = beforeOrigin;
  }
  element.remove();
  row.classList.remove("editing");
  styleMergePreviewRow(row, line.origin);
  const text = row.querySelector(".merge-line-text");
  if (text) { text.hidden = false; text.textContent = line.text; }
  updateMergeProvenanceCounts();
}
async function compareThreeWay(csvMode) {
  if (!$("base").value.trim() || !$("old").value.trim() || !$("new").value.trim()) { setStatus(t("enterPaths"), "error"); return false; }
  let body;
  if (csvMode) { if (!csvInspection && !(await inspectCSV())) return false; body = { ...csvRequestBody(), base: $("base").value.trim() }; }
  else body = threeWayRequestBody();
  // Same busy contract as the other compare paths: abortable, with an elapsed
  // counter and a working Cancel. This path had none of the three (#128).
  const ac = new AbortController();
  currentAbort = ac;
  $("cancel").hidden = false;
  const generation = beginRequest();
  const started = Date.now();
  const tick = () => setStatus(t("comparing") + " " + ((Date.now() - started) / 1000).toFixed(1) + "s", "busy");
  tick();
  const timer = setInterval(tick, 100);
  try {
    const response = await apiFetch(`/api/three-way/${csvMode ? "csv" : "text"}`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), signal: ac.signal });
    const data = await response.json(); if (!response.ok) throw apiError(data, response);
    if (!isCurrentRequest(generation)) return false;
    threeWayData = null; resetMergeSelection(csvMode ? "threeway-csv" : "threeway"); mergeUndo = []; mergeRedo = [];
    resetMergePreview();
    if (!$("mergeOutput").value) { const source = $("base").value.trim(); $("mergeOutput").value = source ? source.replace(/(\.[^./\\]+)?$/, ".merged$1") : (csvMode ? "merged.csv" : "merged.txt"); }
    clearInterval(timer);
    const rendered = await renderThreeWay(data, csvMode);
    if (!rendered && renderGate.cancelled) { setStatus(t("cancelled"), ""); return false; }
    return true;
  } catch (err) {
    if (err.name === "AbortError") setStatus(t("cancelled"), "");
    else setStatus(String(err.message || err), "error");
    return false;
  } finally {
    clearInterval(timer);
    $("cancel").hidden = true;
    currentAbort = null;
  }
}
async function saveThreeWayMerge(previewConfirmed) {
  const output = $("mergeOutput").value.trim(); if (!output) { setStatus(t("requiredField", { field: t("outputPath") }), "error"); return; }
  const target = currentUnresolvedTarget();
  const unresolved = mergeUnresolvedState().count;
  const allowUnresolved = unresolved > 0 && await askConfirm(t("unresolvedWarning", { count: unresolved, target: mergeTargetLabel(target) })); if (unresolved > 0 && !allowUnresolved) return;
  const overwrite = $("mergeOverwrite").checked;
  const impact = buildMergeImpact(threeWayData.csvMode ? "threeway-csv" : "threeway", unresolved);
  const confirmOverwrite = previewConfirmed || !overwrite || await askConfirm(mergeImpactPrompt(impact), mergeImpactRows(impact)); if (!confirmOverwrite) return;
  const base = threeWayData.csvMode ? { ...csvRequestBody(), base: $("base").value.trim() } : threeWayRequestBody();
  const body = { ...base, output, choices: mergeSelection.toWire(), allowUnresolved, unresolvedTarget: allowUnresolved ? target : "", overwrite, confirmOverwrite };
  // Typed result lines travel with the save so the file and the report agree
  // on what was adopted and what was entered by hand (#257).
  if (!threeWayData.csvMode && mergeManual.size) body.manual = Object.fromEntries(mergeManual);
  $("saveMerge").disabled = true;
  try {
    const response = await apiFetch(`/api/merge/three-way/${threeWayData.csvMode ? "csv" : "text"}`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const data = await response.json(); if (!response.ok) throw apiError(data, response);
    if (!threeWayData.csvMode && data.provenance) {
      const counts = provenanceBreakdown(data.provenance);
      setStatus(t("mergeSavedProvenance", { path: data.output, adopted: counts.adopted.toLocaleString(), manual: counts.manual.toLocaleString() }), "success");
      if (mergePreview) { mergePreview.provenance = data.provenance; updateMergeProvenanceCounts(); }
    } else reportMergeSaved(data, target, data.output);
  }
  catch (err) { setStatus(String(err.message || err), "error"); } finally { $("saveMerge").disabled = false; }
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
  // Explicit confirmation is offered only for a rendered text diff: a three-way
  // or folder result has no signature table, so those controls stay out of the
  // way. The counter and the unconfirmed-only stepping are separate from the
  // first/prev/next/last buttons, which keep walking every hunk as before.
  const confirmable = confirmedSignatures.length > 0;
  const unconfirmed = confirmable ? unconfirmedHunkIndexes().length : 0;
  $("confirmCounter").hidden = !confirmable;
  $("prevUnconfirmed").hidden = !confirmable;
  $("nextUnconfirmed").hidden = !confirmable;
  if (confirmable) $("confirmCounter").textContent = t("confirmCounter", confirmProgress(confirmedSignatures, confirmedHunks, active));
  for (const button of [$("prevUnconfirmed"), $("nextUnconfirmed")]) button.disabled = unconfirmed === 0;
}

function activeHunkIndexes() {
  const eventKinds = threeWayData ? threeWayData.events.map((event) => event.kind) : null;
  return navigableIndexes(lastData?.hunks || [], ignoredHunks, eventKinds);
}

// activeConflictIndexes is the conflict-only subset of a three-way result. The
// two lists are deliberately different: "next difference" and "next conflict" are
// different tasks once a merge has several kinds of event.
function activeConflictIndexes() {
  if (!threeWayData) return [];
  return (lastData?.hunks || []).map((_, index) => index)
    .filter((index) => !ignoredHunks.has(index) && threeWayData.events[index]?.kind === "conflict");
}

function isConfirmed(index) {
  return confirmedSignatures.length > 0 && confirmedHunks.has(confirmedSignatures[index]);
}

function unconfirmedHunkIndexes() {
  return activeHunkIndexes().filter((index) => !isConfirmed(index));
}

// Steps to the nearest unconfirmed hunk in the given direction, wrapping at the
// ends. A hunk that is currently selected but already confirmed is skipped: the
// search starts strictly beyond it, not from its position in the pending list.
function stepUnconfirmed(delta) {
  const pending = unconfirmedHunkIndexes();
  if (!pending.length) return;
  const ordered = delta < 0 ? [...pending].reverse() : pending;
  const target = ordered.find((index) => (delta < 0 ? index < currentHunk : index > currentHunk));
  jumpToHunk(target == null ? (delta < 0 ? pending[pending.length - 1] : pending[0]) : target);
}

function toggleConfirmedHunk(index) {
  const signature = confirmedSignatures[index];
  if (!signature) return;
  const next = !confirmedHunks.has(signature);
  if (next) confirmedHunks.add(signature); else confirmedHunks.delete(signature);
  const box = $(`hunk-${index}`);
  box?.classList.toggle("confirmed", next);
  const button = box?.querySelector(".hunk-confirm");
  if (button) {
    const label = t(next ? "unconfirmHunk" : "confirmHunk");
    button.setAttribute("aria-pressed", next ? "true" : "false");
    button.textContent = t(next ? "confirmed" : "confirmHunk");
    button.title = label;
    button.setAttribute("aria-label", label);
  }
  document.querySelectorAll(`.minimap-marker[data-hunk="${index}"]`).forEach((marker) => marker.classList.toggle("confirmed", next));
  $("sidebarList").querySelector(`.sidebar-item[data-hunk="${index}"]`)?.classList.toggle("confirmed", next);
  persistConfirmed();
  updateCounter();
}

// ---- Confirmed-mark persistence (#288) ----
// The signatures are content-derived, so restoring intersects them with the
// hunks actually being shown: a hunk whose lines changed has a different
// signature and its confirmation is dropped rather than inherited.
function readConfirmedStore() {
  try {
    const value = JSON.parse(localStorage.getItem(CONFIRMED_STORAGE_KEY) || "{}");
    return value && typeof value === "object" && !Array.isArray(value) ? value : {};
  } catch (_) { return {}; }
}

function writeConfirmedStore(store) {
  try { localStorage.setItem(CONFIRMED_STORAGE_KEY, JSON.stringify(store)); } catch (_) { /* storage full or blocked */ }
}

function persistConfirmed() {
  if (!confirmedComparison) return;
  const store = readConfirmedStore();
  const kept = confirmedSignatures.filter((signature) => confirmedHunks.has(signature));
  if (kept.length) store[confirmedComparison] = kept;
  else delete store[confirmedComparison];
  const comparisons = Object.keys(store);
  if (comparisons.length > CONFIRMED_COMPARISONS_MAX) {
    for (const key of comparisons.slice(0, comparisons.length - CONFIRMED_COMPARISONS_MAX)) delete store[key];
  }
  writeConfirmedStore(store);
}

function currentComparisonIdentity() {
  const body = requestBody();
  return comparisonIdentity({
    mode: body.mode,
    inline: body.inline,
    old: body.old,
    new: body.new,
    base: $("base")?.value.trim() || "",
    oldAbsent: body.oldAbsent,
    newAbsent: body.newAbsent,
    oldText: body.oldText,
    newText: body.newText,
  });
}

function prepareConfirmations(hunks) {
  confirmedComparison = currentComparisonIdentity();
  confirmedSignatures = hunkSignatures(hunks);
  const stored = readConfirmedStore()[confirmedComparison];
  confirmedHunks = new Set(Array.isArray(stored) ? restoreSignatures(confirmedSignatures, stored) : []);
}

function resetConfirmations() {
  confirmedComparison = "";
  confirmedSignatures = [];
  confirmedHunks = new Set();
}

function stepHunk(delta) {
  const active = activeHunkIndexes();
  if (!active.length) return;
  const position = active.indexOf(currentHunk);
  const next = position < 0 ? (delta < 0 ? active.length - 1 : 0) : Math.max(0, Math.min(active.length - 1, position + delta));
  jumpToHunk(active[next]);
}

function stepConflict(delta) {
  const active = activeConflictIndexes();
  if (!active.length) return;
  const position = active.indexOf(currentHunk);
  const next = position < 0 ? (delta < 0 ? active.length - 1 : 0) : Math.max(0, Math.min(active.length - 1, position + delta));
  jumpToHunk(active[next]);
}

// scrollToHunk moves a hunk into view without making it the current navigation
// target. Used for dismissed differences, which are visible but not differences
// (#269).
function scrollToHunk(index) {
  $(`hunk-${index}`)?.scrollIntoView({ block: "center" });
}

function jumpToHunk(index) {
  const total = lastData?.hunks?.length || 0;
  if (!total || ignoredHunks.has(index)) return;
  index = Math.max(0, Math.min(total - 1, index));
  if (isDowngraded(lastData.hunks[index])) {
    scrollToHunk(index);
    return;
  }
  document.querySelector(".hunk.current")?.classList.remove("current");
  document.querySelectorAll(".minimap-marker.current").forEach((marker) => marker.classList.remove("current"));
  currentHunk = index;
  readHunks.add(index);
  markSidebarCurrent();
  const hunk = $(`hunk-${index}`);
  hunk.classList.add("current", "read");
  hunk.focus({ preventScroll: true });
  // Instant, not smooth. The hunks carry content-visibility:auto, so their
  // heights change as a scroll animation realizes content, which invalidates
  // the target mid-flight — smooth scrolling silently did nothing and jumping
  // to a difference simply failed. Measured: behavior "auto" reaches 1807,
  // "smooth" leaves scrollTop at 0.
  hunk.scrollIntoView({ block: "center" });
  document.querySelectorAll(`.minimap-marker[data-hunk="${index}"]`).forEach((marker) => marker.classList.add("current", "read"));
  updateCounter();
  announce($("diffCounter").textContent);
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
    const ignore = box.querySelector(".hunk-ignore");
    if (ignore) {
      const label = t(restore ? "ignoreHunk" : "restoreHunk");
      ignore.textContent = label;
      ignore.title = label;
      ignore.setAttribute("aria-label", label);
    }
  }
  buildMinimap(lastData);
  updateMinimapViewport();
  if (ignoredHunks.has(currentHunk)) currentHunk = -1;
  renderSummary(lastData);
  updateCounter();
}

function buildMinimap(data) {
  const map = $("minimap");
  map.querySelectorAll(".minimap-marker").forEach((el) => el.remove());
  if (!data?.hunks?.length) return;

  // Measure the actual track even when the previous result hid it. Segment
  // generation is bounded by this pixel height, so a dense three-way result
  // cannot create an unbounded number of buttons.
  const wasHidden = map.hidden;
  map.hidden = false;
  const trackPixels = Math.max(1, Math.floor(map.getBoundingClientRect().height || 1));
  map.hidden = wasHidden;
  const segments = calculateMinimapSegments(data.hunks.map((h, index) => ({
    index,
    kind: h.minimap_kind || h.kind,
    moved: Boolean(h.move_id),
    ignored: ignoredHunks.has(index),
    downgraded: isDowngraded(h),
    displayLength: Math.max(h.display_len || 0, h.old_len || 0, h.new_len || 0, 1),
  })), trackPixels);

  for (const segment of segments) {
    const marker = document.createElement("button");
    marker.type = "button";
    marker.className = `minimap-marker ${segment.kind}${segment.moved ? " moved" : ""}${segment.ignored ? " ignored" : ""}${segment.downgraded ? " downgraded" : ""}`;
    if (readHunks.has(segment.index)) marker.classList.add("read");
    if (isConfirmed(segment.index)) marker.classList.add("confirmed");
    if (currentHunk === segment.index) marker.classList.add("current");
    marker.dataset.hunk = String(segment.index);
    marker.dataset.priority = String(segment.priority);
    marker.title = `${segment.index + 1}: ${segment.kind}`;
    marker.style.top = `${segment.top * 100}%`;
    marker.style.height = `${segment.height * 100}%`;
    marker.addEventListener("click", () => jumpToHunk(segment.index));
    map.append(marker);
  }
}

// minimapHasMarkers records whether buildMinimap produced anything, so the
// visibility check does not have to re-inspect the DOM on every scroll frame.
let minimapHasMarkers = false;

// updateMinimapViewport decides whether the minimap is worth showing and, when
// it is, positions the indicator over the currently visible slice.
//
// Visibility is decided here rather than at build time for two reasons: it
// needs the populated DOM (setupNavigation runs before the hunks are appended),
// and it has to be revisited whenever the result pane or the window resizes. Both
// render paths, the scroll handler, and the resize handler already call this.
function updateMinimapViewport() {
  const result = $("result");
  const map = $("minimap");
  const metrics = calculateMinimapViewport({
    scrollTop: result.scrollTop,
    scrollHeight: result.scrollHeight,
    clientHeight: result.clientHeight,
  });
  map.hidden = !(minimapHasMarkers && metrics.scrollable);
  if (map.hidden) return;
  const viewport = $("minimapViewport");
  viewport.style.top = `${metrics.top * 100}%`;
  viewport.style.height = `${metrics.height * 100}%`;
  viewport.setAttribute(
    "aria-valuenow",
    String(Math.round((result.scrollTop / metrics.maxScrollTop) * 100)),
  );
}

function setupNavigation(data) {
  navObserver?.disconnect();
  navObserver = null;
  currentHunk = -1;
  readHunks = new Set();
  resetConfirmations();
  const hasHunks = data.hunks.length > 0;
  $("diffNav").hidden = !hasHunks;
  $("dirStatusWrap").hidden = true;
  $("dirSearchWrap").hidden = true;
  $("dirFlat").hidden = true;
  for (const id of ["firstDiff", "prevDiff", "nextDiff", "lastDiff", "diffCounter"]) {
    const node = $(id);
    if (node) node.hidden = false;
  }
  $("addSync").hidden = false;
  renderSyncPoints();
  buildSidebar(data);
  minimapHasMarkers = hasHunks;
  $("minimap").querySelectorAll(".minimap-marker").forEach((marker) => marker.remove());
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
      // Read tracking is for differences; a dismissed hunk is not one (#269).
      if (isDowngraded(lastData?.hunks?.[index])) continue;
      if (!readHunks.has(index)) {
        readHunks.add(index);
        entry.target.classList.add("read");
        document.querySelectorAll(`.minimap-marker[data-hunk="${index}"]`).forEach((marker) => marker.classList.add("read"));
        changed = true;
      }
    }
    if (changed) updateCounter();
  }, { threshold: [0.55] });
  document.querySelectorAll(".hunk").forEach((hunk) => navObserver.observe(hunk));
}

function selectSyncLine(cell) {
  const side = cell.dataset.side;
  const previous = document.querySelector(`.cell.sync-selected[data-side="${side}"]`);
  previous?.classList.remove("sync-selected");
  previous?.setAttribute("aria-pressed", "false");
  syncSelection[side] = Number(cell.dataset.line);
  cell.classList.add("sync-selected");
  cell.setAttribute("aria-pressed", "true");
  $("addSync").disabled = syncSelection.old == null || syncSelection.new == null;
  if ($("addSync").disabled) setStatus(t("syncSelect"), "");
}

function resetSyncSelection() {
  syncSelection = { old: null, new: null };
  document.querySelectorAll(".cell.sync-selected").forEach((cell) => {
    cell.classList.remove("sync-selected");
    cell.setAttribute("aria-pressed", "false");
  });
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

// Progress and results own separate lanes (#97). A running operation writes
// progress; its outcome ends that progress and joins the message lane, where
// success withdraws itself and a failure waits to be dismissed. setStatus keeps
// its old signature so every call site reads the same as before.
function setStatus(msg, cls) {
  if (cls === "busy") { setProgress(msg); return; }
  setProgress("");
  if (msg) messageLog.post(msg, cls || "info");
}

function setProgress(msg) {
  const el = $("status");
  if (!msg) { el.textContent = ""; el.hidden = true; return; }
  el.className = "status busy";
  el.textContent = msg;
  el.hidden = false;
}

// ---- staged progress for long comparisons (#297) ----
// The status lane reports the phase the server is actually in, its own elapsed
// time, and a real percentage for the phases that have one. The stage names and
// the percentage come from progress.js, which node --test exercises directly.
const STAGE_LABEL_KEYS = {
  read: "stageRead",
  compare: "stageCompare",
  moves: "stageMoves",
  result: "stageResult",
  render: "stageRender",
};

function stageLabel(key) {
  return t(STAGE_LABEL_KEYS[key] || "comparing");
}

function renderProgress(snapshot) {
  if (!snapshot || !snapshot.stage) { setProgress(""); return; }
  // Show every phase reached so far with its own time, so the slow phase stays
  // visible after it finishes rather than only while it is running (#297).
  const parts = [];
  for (const stage of snapshot.stages) {
    if (stage.state === "pending") continue;
    const label = stageLabel(stage.key);
    if (stage.key === snapshot.stage && snapshot.determinate) {
      parts.push(t("progressDeterminate", {
        stage: label, elapsed: formatElapsed(snapshot.stageElapsedMs), done: snapshot.percent,
      }));
    } else if (stage.key === snapshot.stage) {
      parts.push(t("progressStage", { stage: label, elapsed: formatElapsed(snapshot.stageElapsedMs) }));
    } else if (stage.state === "done") {
      parts.push(t("progressStageDone", { stage: label, elapsed: formatElapsed(stage.elapsedMs) }));
    }
  }
  setProgress(parts.join(" · "));
}

// announce writes the one polite live region (#298). Without a central channel
// every counter was its own live region, so one keypress could queue several
// announcements. Failures still reach assistive technology through the message
// lane's assertive alert; this is for non-urgent feedback such as the position
// after a navigation step. Clearing first makes a repeated message (pressing
// Home twice) a fresh mutation, so it is spoken again rather than swallowed as
// "no change".
function announce(message) {
  const el = $("a11yAnnouncer");
  if (!el || !message) return;
  el.textContent = "";
  queueMicrotask(() => { el.textContent = message; });
}

// A failed API call becomes a sentence the user can act on (#94): the server's
// stable code picks the explanation and its remedy, and the path it names is
// appended rather than parsed out of the message. An unrecognized failure keeps
// the server's own words instead of a wrong guess.
function apiError(body, response) {
  const key = apiErrorKey(body?.code, response?.status);
  const fallback = body?.error || t("errHTTP", { status: response?.status ?? 0 });
  const error = new Error(key ? t(key) + apiErrorLocation(body) : fallback);
  error.code = body?.code || "";
  error.status = response?.status ?? 0;
  error.serverMessage = body?.error || "";
  return error;
}

function apiErrorLocation(body) {
  if (!body?.path) return "";
  const side = body.side === "left" ? t("left") : body.side === "right" ? t("right") : "";
  // The separator belongs to the language: Japanese sets the full-width
  // parentheses against the sentence, English wants a space first.
  return t("errorAt", { side, path: body.path });
}

function messageText(entry) {
  return entry.count > 1 ? t("messageRepeated", { message: entry.message, count: entry.count }) : entry.message;
}

function renderMessages(entries) {
  const lane = $("messages");
  const focusedID = document.activeElement?.closest?.(".message")?.dataset.messageId;
  lane.textContent = "";
  lane.hidden = entries.length === 0;
  for (const entry of entries) {
    const error = entry.tone === "error";
    const item = document.createElement("div");
    item.className = "message " + entry.tone;
    item.dataset.messageId = String(entry.id);
    // Only a failure interrupts a screen reader; the rest wait their turn.
    item.setAttribute("role", error ? "alert" : "status");
    item.setAttribute("aria-live", error ? "assertive" : "polite");

    const text = document.createElement("span");
    text.className = "message-text";
    text.textContent = messageText(entry);

    const stamp = new Date(entry.at);
    const time = document.createElement("time");
    time.className = "message-time";
    time.dateTime = stamp.toISOString();
    time.textContent = stamp.toLocaleTimeString(localeTag(lang));

    const dismiss = document.createElement("button");
    dismiss.type = "button";
    dismiss.className = "message-dismiss";
    dismiss.title = t("dismissMessage");
    dismiss.setAttribute("aria-label", t("dismissMessage"));
    dismiss.textContent = "\u00d7";
    dismiss.addEventListener("click", () => dismissMessage(entry.id));

    item.append(text, time, dismiss);
    lane.append(item);
  }
  if (focusedID) focusMessage(Number(focusedID));
}

function focusMessage(id) {
  const button = $("messages").querySelector(`.message[data-message-id="${id}"] .message-dismiss`);
  if (button) button.focus();
  return Boolean(button);
}

// Dismissing keeps the keyboard somewhere useful: the next message if the lane
// still holds one, otherwise whichever of Re-compare / Compare the current
// layout actually shows, which is where the work continues.
function dismissMessage(id) {
  const remaining = messageLog.entries().filter((entry) => entry.id !== id);
  messageLog.dismiss(id);
  const next = remaining[0];
  if (next) {
    focusMessage(next.id);
    return;
  }
  for (const controlID of ["setupRecompare", "compare"]) {
    const control = $(controlID);
    if (control && !control.hidden && control.offsetParent) {
      control.focus();
      return;
    }
  }
}


function splitList(value) {
  return String(value || "").split(",").map((item) => item.trim()).filter(Boolean);
}

function selectedCSVColumns() {
  return [...document.querySelectorAll("#columnList input:checked")].map((input) => ({ name: input.dataset.name, index: Number(input.dataset.index) }));
}

// ---- Visual filter builder (#129) ----
// The DOM is rendered from filterState and read back on every edit, so there is
// one source of truth and no two-way binding to drift. The model itself lives
// in rowfilter.js; these functions only build and read controls.
const FILTER_OP_KEYS = {
  eq: "filterOpEq", ne: "filterOpNe", contains: "filterOpContains",
  starts: "filterOpStarts", ends: "filterOpEnds", gt: "filterOpGt",
  ge: "filterOpGe", lt: "filterOpLt", le: "filterOpLe",
  between: "filterOpBetween", empty: "filterOpEmpty",
  not_empty: "filterOpNotEmpty", regex: "filterOpRegex",
};

function filterHasHeader() { return $("hasHeader").checked; }
function filterColumnNames() {
  return csvInspection && Array.isArray(csvInspection.header) ? csvInspection.header : [];
}

function renderFilterBuilders() {
  for (const builder of document.querySelectorAll(".filter-builder[data-filter-target]")) {
    const target = builder.dataset.filterTarget;
    renderFilterGroup(builder, filterState[target], target === "columns", target, "");
  }
  updateFilterBadge();
}

function renderFilterGroup(container, group, nameMode, target, path) {
  container.innerHTML = "";
  container.dataset.filterPath = path;
  const head = document.createElement("div");
  head.className = "filter-head";
  const label = document.createElement("strong");
  label.textContent = t(nameMode ? "filterColumns" : "filterRows");
  head.append(label);
  head.append(filterSelect(t("filterMatch"), [["all", t("filterMatchAll")], ["any", t("filterMatchAny")]], group.match, "filterMatch"));
  head.append(filterCheck(t("filterIgnoreCase"), group.ignoreCase, "filterIgnorecase"));
  head.append(filterCheck(t("filterNot"), group.not, "filterNot"));
  if (path !== "") {
    const remove = document.createElement("button");
    remove.type = "button"; remove.className = "filter-remove"; remove.dataset.filterRemove = "1";
    remove.textContent = "×"; remove.title = t("filterRemove"); remove.setAttribute("aria-label", t("filterRemove"));
    head.append(remove);
  }
  container.append(head);

  const tree = document.createElement("div");
  tree.className = "filter-tree";
  tree.dataset.filterTree = "1";
  group.conditions.forEach((condition) => tree.append(renderFilterCondition(condition, nameMode)));
  group.groups.forEach((child, index) => {
    const childEl = document.createElement("div");
    childEl.className = "filter-group";
    renderFilterGroup(childEl, child, nameMode, target, path === "" ? String(index) : `${path}.${index}`);
    tree.append(childEl);
  });
  container.append(tree);

  const actions = document.createElement("div");
  actions.className = "csv-actions filter-actions";
  const addCondition = document.createElement("button");
  addCondition.type = "button"; addCondition.dataset.filterAddCondition = "1"; addCondition.textContent = t("filterAddCondition");
  const addGroup = document.createElement("button");
  addGroup.type = "button"; addGroup.dataset.filterAddGroup = "1"; addGroup.textContent = t("filterAddGroup");
  actions.append(addCondition, addGroup);
  container.append(actions);
}

function filterSelect(labelText, options, value, dataName) {
  const label = document.createElement("label");
  label.className = "opt";
  const text = document.createElement("span");
  text.textContent = labelText;
  const select = document.createElement("select");
  select.dataset[dataName] = "1";
  for (const [optionValue, optionText] of options) {
    const option = document.createElement("option");
    option.value = optionValue; option.textContent = optionText;
    if (value === optionValue) option.selected = true;
    select.append(option);
  }
  label.append(text, select);
  return label;
}

function filterCheck(labelText, checked, dataName) {
  const label = document.createElement("label");
  label.className = "opt check";
  const input = document.createElement("input");
  input.type = "checkbox"; input.dataset[dataName] = "1"; input.checked = Boolean(checked);
  const text = document.createElement("span");
  text.textContent = labelText;
  label.append(input, text);
  return label;
}

function renderFilterCondition(condition, nameMode) {
  const row = document.createElement("div");
  row.className = "filter-condition";
  if (!nameMode) {
    const columns = filterColumnNames();
    if (filterHasHeader() && columns.length) {
      const select = document.createElement("select");
      select.className = "filter-column";
      const empty = document.createElement("option");
      empty.value = ""; empty.textContent = "—";
      select.append(empty);
      for (const name of columns) {
        const option = document.createElement("option");
        option.value = name; option.textContent = name;
        if (condition.column === name) option.selected = true;
        select.append(option);
      }
      row.append(select);
    } else if (!filterHasHeader()) {
      const input = document.createElement("input");
      input.className = "filter-index"; input.type = "number"; input.min = "0";
      input.value = String(condition.index);
      row.append(input);
    } else {
      const input = document.createElement("input");
      input.className = "filter-column"; input.value = condition.column;
      input.placeholder = t("filterColumn");
      row.append(input);
    }
  }
  const op = document.createElement("select");
  op.className = "filter-op";
  for (const entry of ROW_FILTER_OPERATORS) {
    const option = document.createElement("option");
    option.value = entry.op; option.textContent = t(FILTER_OP_KEYS[entry.op] || "filterOpEq");
    if (condition.op === entry.op) option.selected = true;
    op.append(option);
  }
  row.append(op);
  const info = rowFilterOperatorInfo(condition.op);
  const value = document.createElement("input");
  value.className = "filter-value"; value.value = condition.value;
  value.placeholder = t("filterValue"); value.hidden = info.values === 0;
  const value2 = document.createElement("input");
  value2.className = "filter-value2"; value2.value = condition.value2;
  value2.placeholder = t("filterValue2"); value2.hidden = info.values < 2;
  row.append(value, value2);
  row.append(filterCheck(t("filterNot"), condition.not, "filterNot"));
  const remove = document.createElement("button");
  remove.type = "button"; remove.className = "filter-remove"; remove.dataset.filterRemove = "1";
  remove.textContent = "×"; remove.title = t("filterRemove"); remove.setAttribute("aria-label", t("filterRemove"));
  row.append(remove);
  return row;
}

function readFilterGroup(container) {
  const group = newRowFilterGroup();
  const match = container.querySelector("[data-filter-match]");
  group.match = match && match.value === "any" ? "any" : "all";
  group.ignoreCase = Boolean(container.querySelector("[data-filter-ignorecase]")?.checked);
  group.not = Boolean(container.querySelector("[data-filter-not]")?.checked);
  const tree = container.querySelector("[data-filter-tree]");
  if (tree) {
    for (const child of tree.children) {
      if (child.classList.contains("filter-condition")) group.conditions.push(readFilterCondition(child));
      else if (child.classList.contains("filter-group")) group.groups.push(readFilterGroup(child));
    }
  }
  return group;
}

function readFilterCondition(element) {
  const condition = newRowFilterCondition();
  const indexInput = element.querySelector(".filter-index");
  const columnInput = element.querySelector(".filter-column");
  if (indexInput) {
    condition.byIndex = true;
    condition.index = Math.max(0, Number(indexInput.value) || 0);
  } else if (columnInput) {
    condition.column = columnInput.value;
  }
  condition.op = element.querySelector(".filter-op")?.value || "eq";
  condition.value = element.querySelector(".filter-value")?.value ?? "";
  condition.value2 = element.querySelector(".filter-value2")?.value ?? "";
  condition.not = Boolean(element.querySelector("[data-filter-not]")?.checked);
  return condition;
}

function filterModelAt(root, path) {
  let group = root;
  for (const part of String(path).split(".").filter((item) => item !== "")) {
    group = group?.groups?.[Number(part)];
    if (!group) return null;
  }
  return group;
}

function syncFilterState(event) {
  const builder = event.target.closest?.(".filter-builder[data-filter-target]");
  if (!builder) return;
  const target = builder.dataset.filterTarget;
  filterState[target] = readFilterGroup(builder);
  const op = event.target.closest(".filter-op");
  if (op) {
    const info = rowFilterOperatorInfo(op.value);
    const row = op.closest(".filter-condition");
    row.querySelector(".filter-value").hidden = info.values === 0;
    row.querySelector(".filter-value2").hidden = info.values < 2;
  }
  $("filterPreviewResult").textContent = "";
  updateFilterBadge();
}

function onFilterBuilderClick(event) {
  const builder = event.target.closest(".filter-builder[data-filter-target]");
  if (!builder) return;
  const target = builder.dataset.filterTarget;
  const addCondition = event.target.closest("[data-filter-add-condition]");
  const addGroup = event.target.closest("[data-filter-add-group]");
  const remove = event.target.closest("[data-filter-remove]");
  if (!addCondition && !addGroup && !remove) return;
  filterState[target] = readFilterGroup(builder);
  const model = filterState[target];
  if (addCondition || addGroup) {
    const groupEl = (addCondition || addGroup).closest(".filter-builder, .filter-group");
    const group = filterModelAt(model, groupEl.dataset.filterPath || "");
    if (!group) return;
    if (addCondition) group.conditions.push(newRowFilterCondition());
    else group.groups.push(newRowFilterGroup());
  } else {
    const item = remove.closest(".filter-condition, .filter-group");
    const tree = item.parentElement;
    const groupEl = tree.closest(".filter-builder, .filter-group");
    const group = filterModelAt(model, groupEl.dataset.filterPath || "");
    if (!group) return;
    const index = [...tree.children].indexOf(item);
    if (item.classList.contains("filter-condition")) group.conditions.splice(index, 1);
    else group.groups.splice(index, 1);
  }
  renderFilterBuilders();
}

function updateFilterBadge() {
  const badge = $("filterBuilderBadge");
  if (!badge) return;
  const count = rowFilterCount(filterState.rows) + rowFilterCount(filterState.columns);
  badge.textContent = count ? t("changedSettings", { count }) : "";
  badge.hidden = count === 0;
}

function compileFilterState() {
  const options = { hasHeader: filterHasHeader(), columns: filterColumnNames() };
  return {
    rowFilter: compileRowFilter(filterState.rows, options),
    columnFilter: compileRowFilter(filterState.columns, { ...options, nameMode: true }),
  };
}

function filterStateErrors() {
  const options = { hasHeader: filterHasHeader(), columns: filterColumnNames() };
  const errors = [
    ...validateRowFilter(filterState.rows, options),
    ...validateRowFilter(filterState.columns, { ...options, nameMode: true }),
  ];
  return errors.filter((error) => error.code === "unknown_column" || error.code === "invalid_regex" || error.code === "too_deep");
}

async function runFilterPreview() {
  const body = csvRequestBody();
  if (!validateInputs(body, false)) return;
  $("filterPreview").disabled = true;
  try {
    const resp = await apiFetch("/api/csv/preview", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const data = await resp.json();
    if (!resp.ok) throw apiError(data, resp);
    $("filterPreviewResult").textContent = t("csvFilterPreviewResult", data);
  } catch (err) {
    $("filterPreviewResult").textContent = "";
    setStatus(String(err.message || err), "error");
  } finally {
    $("filterPreview").disabled = false;
  }
}

// ---- Manual column mapping (#119) ----
// The result table already aligns the right side to the left header through the
// engine, but only by column name. When names disagree, or a column exists on
// one side only, this editor lets the user state the pairing. It starts from
// the engine's own name alignment when that succeeds and from positions
// otherwise; #116's content-based estimate is not part of this build, so the
// source line says so rather than pretending otherwise. Nothing is sent until
// the user edits or applies the map.
let columnMap = [];
let columnMapSource = "position";
let columnMapDirty = false;

function columnMapHeaders() {
  return { left: csvInspection?.left_header || [], right: csvInspection?.right_header || [] };
}

function fillColumnMapSelect(select, names, selected, noneLabel) {
  const none = document.createElement("option");
  none.value = String(COLUMN_ABSENT);
  none.textContent = noneLabel;
  select.append(none);
  names.forEach((name, index) => {
    const option = document.createElement("option");
    option.value = String(index);
    option.textContent = `${index}: ${name}`;
    select.append(option);
  });
  select.value = names[selected] == null ? String(COLUMN_ABSENT) : String(selected);
}

function columnMapValidation() {
  const { left, right } = columnMapHeaders();
  return validateMapping(columnMap, left.length, right.length);
}

function renderColumnMap() {
  const host = $("columnMapRows");
  if (!host) return;
  const source = $("columnMapSource");
  if (source) source.textContent = t(columnMapSource === "name" ? "columnMapFromNames" : "columnMapFromPositions");
  host.textContent = "";
  if (!csvInspection) {
    const note = document.createElement("p");
    note.className = "details-hint";
    note.textContent = t("columnMapInspectFirst");
    host.append(note);
    return;
  }
  const { left, right } = columnMapHeaders();
  const check = columnMapValidation();
  const invalid = new Set(check.errors.map((entry) => entry.position));
  columnMap.forEach((pair, position) => {
    const row = document.createElement("div");
    row.className = "column-map-row";
    const leftSelect = document.createElement("select");
    leftSelect.setAttribute("aria-label", t("columnMapLeftColumn"));
    fillColumnMapSelect(leftSelect, left, pair.left, t("columnMapNone"));
    const arrow = document.createElement("span");
    arrow.className = "column-map-arrow";
    arrow.textContent = "↔";
    const rightSelect = document.createElement("select");
    rightSelect.setAttribute("aria-label", t("columnMapRightColumn"));
    fillColumnMapSelect(rightSelect, right, pair.right, t("columnMapNone"));
    const ignore = document.createElement("input");
    ignore.type = "checkbox";
    ignore.checked = Boolean(pair.ignore);
    const ignoreLabel = document.createElement("label");
    ignoreLabel.className = "opt check";
    ignoreLabel.append(ignore, t("columnMapIgnore"));
    const remove = document.createElement("button");
    remove.type = "button";
    remove.textContent = "\u00d7";
    remove.title = t("columnMapRemove");
    remove.setAttribute("aria-label", t("columnMapRemove"));
    if (invalid.has(position)) row.classList.add("column-map-invalid");
    const update = () => {
      columnMap[position] = { left: Number(leftSelect.value), right: Number(rightSelect.value), ignore: ignore.checked };
      columnMapDirty = true;
      updateColumnMapError();
      updateCSVReview();
      scheduleComparisonURLReplace();
    };
    leftSelect.addEventListener("change", update);
    rightSelect.addEventListener("change", update);
    ignore.addEventListener("change", update);
    remove.addEventListener("click", () => {
      columnMap.splice(position, 1);
      columnMapDirty = true;
      renderColumnMap();
      updateCSVReview();
      scheduleComparisonURLReplace();
    });
    row.append(leftSelect, arrow, rightSelect, ignoreLabel, remove);
    host.append(row);
  });
  if (!columnMap.length) {
    const note = document.createElement("p");
    note.className = "details-hint";
    note.textContent = t("columnMapEmpty");
    host.append(note);
  }
  updateColumnMapError();
}

function updateColumnMapError() {
  const error = $("columnMapError");
  if (!error) return;
  const check = columnMapValidation();
  const invalid = columnMapDirty && !check.valid;
  error.hidden = !invalid;
  error.textContent = invalid ? t("columnMapInvalid") : "";
}

function setColumnMap(mapping, source, dirty) {
  columnMap = mappingFromRequest(mapping);
  columnMapSource = source || "position";
  columnMapDirty = Boolean(dirty);
  renderColumnMap();
}

function buildColumnMap() {
  const { left, right } = columnMapHeaders();
  const built = initialMapping(left, right, { alignByName: $("alignColumns").checked });
  setColumnMap(built.mapping, built.source, true);
  updateCSVReview();
}

async function applyColumnMap() {
  const check = columnMapValidation();
  if (columnMapDirty && !check.valid) { updateColumnMapError(); return false; }
  columnMapDirty = true;
  renderColumnMap();
  return compareCSV();
}

function csvRequestBody() {
  const hasHeader = $("hasHeader").checked;
  const selected = selectedCSVColumns();
  const keyMode = $("keyMode").value;
  const body = {
    old: $("old").value.trim(), new: $("new").value.trim(), hasHeader,
    alignColumnsByName: $("alignColumns").checked,
    keyNames: [], keyIndexes: [], excludeKeyNames: [], excludeKeyIndexes: [], indexBase: 0, keyMode,
    leftFormat: $("leftFormat").value, rightFormat: $("rightFormat").value,
    leftParser: $("leftParser").value, rightParser: $("rightParser").value,
    leftDelimiter: $("leftDelimiter").value, rightDelimiter: $("rightDelimiter").value,
    lazyQuotes: $("lazyQuotes").checked, trimLeadingSpace: $("trimLeadingSpace").checked,
    ignoreCase: $("ignoreCase").checked, whitespace: whitespaceScaleMode($("whitespaceScale").value),
    lineFilters: $("lineFilters").value.split(/\r?\n/).map((value) => value.trim()).filter(Boolean),
    ignoreColumnNames: [], ignoreColumnIndexes: [], tolerance: $("tolerance").value === "" ? null : Number($("tolerance").value),
    columnTolerances: [], partitions: Number($("partitions").value), parseWorkers: Number($("parseWorkers").value),
    workers: Number($("workers").value), memory: $("memory").value.trim(), tempDir: $("tempDir").value.trim(),
    partitionBuffer: $("partitionBuffer").value.trim(), mergeFanIn: Number($("mergeFanIn").value),
    maxRecordBytes: $("maxRecordBytes").value.trim(), keepTemp: $("keepTemp").checked,
    maxRows: Number($("csvMaxRows").value), output: $("csvOutput").value.trim(),
    outputFormat: $("csvOutputFormat").value, outputHeader: $("outputHeader").checked,
  };
  // A manual map is opt-in: the default comparison keeps using the engine's
  // name alignment, and the editor only supplies a pairing once touched (#119).
  body.columnMap = columnMapDirty ? mappingToRequest(columnMap) : [];
  if (keyMode === "include") body[hasHeader ? "keyNames" : "keyIndexes"] = selected.map((item) => hasHeader ? item.name : item.index);
  if (keyMode === "exclude") body[hasHeader ? "excludeKeyNames" : "excludeKeyIndexes"] = selected.map((item) => hasHeader ? item.name : item.index);
  const ignored = splitList($("ignoreColumns").value);
  if (hasHeader) body.ignoreColumnNames = ignored;
  else {
	body.ignoreColumnIndexes = ignored.map(Number);
	if (body.ignoreColumnIndexes.some((value) => !Number.isInteger(value) || value < 0)) body._validationError = t("invalidIndex", { field: t("ignoreColumns") });
  }
  for (const spec of splitList($("columnTolerances").value)) {
    const pos = spec.lastIndexOf("=");
	if (pos < 1) { body._validationError = `${t("columnTolerances")}: ${spec}`; continue; }
    const selector = spec.slice(0, pos).trim(), value = Number(spec.slice(pos + 1));
	if (!Number.isFinite(value) || value < 0 || (!hasHeader && (!Number.isInteger(Number(selector)) || Number(selector) < 0))) { body._validationError = `${t("columnTolerances")}: ${spec}`; continue; }
    body.columnTolerances.push(hasHeader ? { name: selector, value } : { index: Number(selector), by_index: true, value });
  }
  const filters = compileFilterState();
  if (filters.rowFilter) body.rowFilter = filters.rowFilter;
  if (filters.columnFilter) body.columnFilter = filters.columnFilter;
  if (filterStateErrors().length) body._validationError = t("filterInvalid");
  if (columnMapDirty && !columnMapValidation().valid) body._validationError = t("columnMapInvalid");
  return body;
}

function updateCSVReview() {
  const body = csvRequestBody();
  const mode = $("keyMode").value;
  const keys = mode === "all" ? t("allColumns") : selectedCSVColumns().map((item) => item.name).join(", ") || "—";
  $("reviewText").textContent = [
    `${t("sideLeft")}: ${body.old || "—"}`, `${t("sideRight")}: ${body.new || "—"}`,
    `${body.leftFormat}/${body.leftParser} ↔ ${body.rightFormat}/${body.rightParser}`,
    `${t("keyMode")}: ${mode} (${keys})`,
    `${t("memory")}: ${body.memory}; ${t("partitions")}: ${body.partitions}; ${t("workers")}: ${body.workers}`,
    `${t("outputPath")}: ${body.output || "browser result"}`,
  ].join("\n");
}

function renderColumnSelection(inspection) {
  const list = $("columnList");
  list.innerHTML = "";
  inspection.header.forEach((name, index) => {
    const label = document.createElement("label");
    label.className = "column-choice";
    const input = document.createElement("input");
    input.type = "checkbox"; input.dataset.name = name; input.dataset.index = String(index);
    input.addEventListener("change", updateCSVReview);
    input.addEventListener("change", scheduleComparisonURLReplace);
    const text = document.createElement("span");
    text.textContent = `${index}: ${name}`;
    label.append(input, text); list.append(label);
  });
  $("keySetup").hidden = false;
  // Rebuild the filter cache alongside the list it describes.
  buildColumnFilterIndex();
  applyColumnFilter();
  syncKeyMode();
  renderFilterBuilders();
}

async function inspectCSV() {
  const body = csvRequestBody();
  if (!validateInputs(body, false)) return false;
  $("inspectCSV").disabled = true;
  setStatus(t("comparing"), "busy");
  try {
    const resp = await apiFetch("/api/csv/inspect", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const data = await resp.json();
    if (!resp.ok) throw apiError(data, resp);
    csvInspection = data;
    $("inspection").textContent = t("inspectionDone", data);
    renderColumnSelection(data);
    // Offer the name alignment the engine would use as the editor's starting
    // value; #116's content-based estimate is not available here (#119).
    if (!columnMap.length) {
      const built = initialMapping(data.left_header, data.right_header, { alignByName: $("alignColumns").checked });
      setColumnMap(built.mapping, built.source, false);
    } else {
      renderColumnMap();
    }
    setStatus("");
    updateCSVReview();
    return true;
  } catch (err) { setStatus(String(err.message || err), "error"); return false; }
  finally { $("inspectCSV").disabled = false; }
}

// ---- "How do I make it match?" suggestions (#121) ----
//
// The reasoning that decides *what* to propose lives in suggest.js, which is
// pure and node-tested. This half only draws the cards and, on Apply, writes the
// proposed options into the setup form and re-runs the comparison. The server
// runs each candidate once (bounded) so the card can show the real residual.
let suggestionToken = 0;

function hideSuggestions() {
  const panel = $("suggestions");
  if (!panel) return;
  suggestionToken++;
  panel.hidden = true;
  const cards = $("suggestionCards");
  if (cards) cards.textContent = "";
}

function suggestionCauseKey(cause) {
  return "cause" + cause.charAt(0).toUpperCase() + cause.slice(1);
}

function suggestionActionLabel(proposal) {
  if (proposal.cause === "columnOrder") return t("suggestAlignByName");
  if (proposal.id === "whitespace") return t("suggestWhitespace", { count: proposal.count });
  if (proposal.id === "ignoreCase") return t("suggestIgnoreCase", { count: proposal.count });
  if (proposal.id === "tolerance:*") return t("suggestGlobalTolerance", { value: proposal.value });
  if (proposal.id.startsWith("tolerance:")) return t("suggestTolerance", { column: proposal.column?.name ?? proposal.column?.index, value: proposal.value });
  if (proposal.id.startsWith("ignore:")) return t("suggestIgnoreColumn", { column: proposal.column?.name ?? proposal.column?.index });
  return proposal.id;
}

function activeNormalizations(body) {
  const items = [];
  if (body.whitespace && body.whitespace !== "none") items.push(`${t("whitespace")}: ${body.whitespace}`);
  if (body.ignoreCase) items.push(t("ignoreCase"));
  if (body.tolerance != null) items.push(`${t("tolerance")}: ${body.tolerance}`);
  for (const item of body.columnTolerances || []) items.push(`${item.name ?? item.index}=${item.value}`);
  for (const name of body.ignoreColumnNames || []) items.push(`${t("ignoreColumns")}: ${name}`);
  for (const index of body.ignoreColumnIndexes || []) items.push(`${t("ignoreColumns")}: ${index}`);
  return items;
}

function renderSuggestionCards(proposals, baseCount, residuals, loading) {
  const cards = $("suggestionCards");
  if (!cards) return;
  for (const existing of cards.querySelectorAll(".suggestion-card")) existing.remove();
  for (const proposal of proposals) {
    const card = document.createElement("div"); card.className = "suggestion-card";
    const cause = document.createElement("div"); cause.className = "suggestion-cause"; cause.textContent = t(suggestionCauseKey(proposal.cause));
    const action = document.createElement("div"); action.className = "suggestion-action"; action.textContent = suggestionActionLabel(proposal);
    const residual = document.createElement("div"); residual.className = "suggestion-residual";
    const button = document.createElement("button"); button.type = "button"; button.className = "suggestion-apply"; button.textContent = t("suggestApply");
    const result = residuals.get(proposal.id);
    if (loading) {
      residual.textContent = t("suggestionsChecking");
      button.disabled = true;
    } else if (!result || result.error) {
      residual.textContent = t("suggestUnavailable");
      button.disabled = true;
    } else if (result.difference_count === 0) {
      residual.textContent = t("suggestResolves");
      residual.classList.add("suggestion-resolves");
    } else {
      residual.textContent = t("suggestLeaves", { residual: result.difference_count, base: baseCount });
    }
    button.onclick = () => { void applySuggestion(proposal); };
    card.append(cause, action, residual, button);
    cards.append(card);
  }
}

async function refreshSuggestions() {
  const panel = $("suggestions");
  if (!panel) return;
  if ($("mode").value !== "csv" || !csvData) { hideSuggestions(); return; }
  const base = csvRequestBody();
  if (base._validationError || !base.old || !base.new) { hideSuggestions(); return; }
  const baseCount = differenceCount(csvData.summary);
  const causes = estimateCauses(csvData, base);
  const proposals = buildProposals(csvData, base);
  const active = activeNormalizations(base);
  panel.hidden = false;
  $("suggestionsIntro").textContent = t("suggestionsBase", { count: baseCount });
  const cards = $("suggestionCards");
  cards.textContent = "";
  if (causes.length) {
    const note = document.createElement("p"); note.className = "suggestion-causes";
    note.textContent = t("suggestDetected", { list: causes.map((cause) => `${t(suggestionCauseKey(cause.cause))}×${cause.count}`).join(", ") });
    cards.append(note);
  }
  if (active.length) {
    const note = document.createElement("p"); note.className = "suggestion-active";
    note.textContent = t("suggestActive", { list: active.join(", ") });
    cards.append(note);
  }
  if (!proposals.length) {
    const note = document.createElement("p"); note.className = "suggestion-none";
    note.textContent = t("suggestionsNone");
    cards.append(note);
    return;
  }
  const token = ++suggestionToken;
  renderSuggestionCards(proposals, baseCount, new Map(), true);
  const candidates = proposals.slice(0, MAX_SUGGEST_CANDIDATES).map((proposal) => ({ id: proposal.id, request: applyProposals(base, [proposal]) }));
  try {
    const resp = await apiFetch("/api/csv/suggest", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ candidates }) });
    const data = await resp.json();
    if (!resp.ok) throw apiError(data, resp);
    if (token !== suggestionToken) return;
    renderSuggestionCards(proposals, baseCount, new Map((data.results || []).map((item) => [item.id, item])), false);
  } catch (err) {
    if (token !== suggestionToken) return;
    $("suggestionsIntro").textContent = t("suggestFailed", { message: String(err.message || err) });
  }
}

// applySuggestionPatch writes a proposal's options into the setup controls.
// The next Save project then persists exactly this normalization set (#121).
function applySuggestionPatch(patch) {
  if (patch.alignColumnsByName) $("alignColumns").checked = true;
  if (patch.whitespace) $("whitespace").value = patch.whitespace;
  if (patch.ignoreCase) $("ignoreCase").checked = true;
  if (patch.tolerance != null) $("tolerance").value = String(patch.tolerance);
  if (patch.columnTolerances) {
    const specs = splitList($("columnTolerances").value);
    for (const item of patch.columnTolerances) {
      const spec = `${item.name != null ? item.name : item.index}=${item.value}`;
      if (!specs.includes(spec)) specs.push(spec);
    }
    $("columnTolerances").value = specs.join(", ");
  }
  for (const [key, target] of [["ignoreColumnNames", "ignoreColumns"], ["ignoreColumnIndexes", "ignoreColumns"]]) {
    if (!patch[key]) continue;
    const values = splitList($(target).value);
    for (const value of patch[key]) if (!values.includes(String(value))) values.push(String(value));
    $(target).value = values.join(", ");
  }
}

async function applySuggestion(proposal) {
  applySuggestionPatch(proposal.patch);
  setStatus(t("suggestApplied"), "success");
  await compare();
}

// The key span of the differences that made it into the response. The CSV table
// shows differences only and never folds unchanged rows, so there is no collapsed
// bar to label; when the display cap omits part of the result, naming the key
// range the shown rows cover is the closest honest equivalent (#268). Only the
// explicit key columns are used — with "all columns" keys there is nothing a
// reader would call a key.
function csvRowsKeyRange(rows) {
  if (!csvKeyColumns.length) return "";
  const list = (rows || []).map((diff) => (diff.new?.length ? diff.new : diff.old));
  return formatKeyRange(keyRange(list, csvKeyColumns));
}

function csvShownKeyRange(data) {
  return csvRowsKeyRange(data?.differences);
}

function renderCSVSummary(data) {
  const summary = data.summary, el = $("summary");
  el.innerHTML = "";
  const add = (label, value, cls = "") => { const item = document.createElement("span"); item.className = `stat ${cls}`; const b = document.createElement("b"); b.textContent = fmt(Number(value || 0)); item.append(b, ` ${label}`); el.append(item); };
  add(t("leftOnly"), summary.left_only, "del"); add(t("rightOnly"), summary.right_only, "add");
  add(t("changed"), Math.max(summary.changed_left || 0, summary.changed_right || 0), "chg"); add(t("equalRows"), summary.equal_rows);
  const memory = memoryStatus(data.memory);
  if (memory) {
    const note = document.createElement("span");
    note.className = "note";
    note.textContent = t("memoryBudget", memory);
    if (memory.spilled) note.textContent += " " + t("memorySpilled", { dir: memory.dir || t("systemTemp") });
    el.append(note);
  }
  // The counts alone leave the reader to decide whether anything is really
  // different. State the verdict in one line: real data differences, or none
  // apart from column/row order (#116).
  const verdict = data.verdict;
  if (verdict && !verdict.substantively_equal) {
    const note = document.createElement("span"); note.className = "note csv-verdict-differences";
    note.textContent = t("csvDataDifferenceVerdict", { count: Number(verdict.differences || 0).toLocaleString() });
    el.append(note);
  }
  if (data.truncated) {
    const note = document.createElement("span"); note.className = "note"; note.textContent = t("csvTruncated"); el.append(note);
    const range = csvShownKeyRange(data);
    if (range) {
      const keys = document.createElement("span"); keys.className = "note"; keys.textContent = t("csvShownKeyRange", { range }); el.append(keys);
    }
  }
  el.hidden = false;
}

// A reordered column set is not an error when both sides name the same
// columns: show the verdict and a one-click way to align and compare (#116).
function csvAlignProposalCard(data) {
  const card = resultStateCard(t("columnsReorderedTitle"), t("columnsReorderedScope"), "partial");
  const button = document.createElement("button");
  button.type = "button"; button.className = "csv-align-columns"; button.textContent = t("alignAndCompare");
  button.onclick = () => { $("alignColumns").checked = true; compareCSV(); };
  card.append(button);
  return card;
}

// csvView holds the parts of the table that survive a page turn. Paging used to
// rebuild the entire result — pane headers, summary, column headers, pager and
// all — to change a hundred rows; now it replaces the rows and moves the pager
// (#154).
let csvView = null;

// CSV results used to hide the whole result toolbar. The reading axis lives on
// that toolbar now, so the bar stays and only the controls that describe a line
// diff are tucked away, leaving the reading (and the copy-link) reachable
// (#263).
function showCSVToolbar() {
  $("diffNav").hidden = false;
  for (const id of [
    "firstDiff", "prevDiff", "diffCounter", "nextDiff", "lastDiff",
    "viewModeWrap", "contextToggle", "dirStatusWrap", "dirSearchWrap",
    "dirContinuous", "backToFolder", "sidebarToggle", "addSync", "clearSync",
    "editMode", "mergeMode",
  ]) {
    const node = $(id); if (node) node.hidden = true;
  }
}

// renderCSV builds one page of the table (CSV_PAGE_SIZE rows), which is bounded
// by construction, so unlike the text, three-way, folder and continuous paths it
// does not slice: the remaining rows arrive one page at a time through
// renderCSVRows, and nothing here blocks on the size of the whole difference.
function renderCSV(data) {
  csvData = data;
  lastData = null;
  lastComparedRequest = null;
  clearUnchangedContext();
  void refreshSuggestions();
  csvView = null;
  syncExportPatchVisibility();
  minimapHasMarkers = false; showCSVToolbar(); $("syncPanel").hidden = true; $("minimap").hidden = true;
  // The rows this index pointed at are about to be replaced. Updating the merge
  // UI here would only toggle classes on nodes that are discarded below; the
  // call after the table is built is the one that matters (#154).
  resetMergeRowIndex();
  const result = $("result"); result.innerHTML = "";
  result.append(paneHeads(data));
  if (alignmentProposal(data.verdict)) {
    $("summary").hidden = true;
    result.append(csvAlignProposalCard(data));
    return;
  }
  renderCSVSummary(data);
  result.append(statsPanel("csv", data));
  if (!data.differences.length) {
    if (data.truncated) result.append(resultStateCard(t("matchNotVerified"), t("csvTruncated"), "partial"));
    else {
      const scope = t("csvMatchScope", { rows: fmt(Number(data.summary.equal_rows || 0)), columns: fmt(data.header.length) });
      result.append(resultStateCard(t(equivalenceTitleKey(data.verdict, comparisonUsesRules(true))), scope));
    }
    return;
  }

  const pageCount = csvPageCount(data.differences.length, CSV_PAGE_SIZE);
  csvPage = clampPage(csvPage, pageCount);

  const controls = document.createElement("div"); controls.className = "csv-pages";
  const prev = document.createElement("button"); prev.type = "button"; prev.className = "csv-page-prev"; prev.textContent = "←"; prev.setAttribute("aria-label", t("previousPage")); prev.title = t("previousPage");
  const pageInput = document.createElement("input"); pageInput.type = "number"; pageInput.className = "csv-page-input"; pageInput.min = "1"; pageInput.max = String(pageCount); pageInput.step = "1"; pageInput.setAttribute("aria-label", t("pageInput", { total: pageCount }));
  const total = document.createElement("span"); total.textContent = t("pageTotal", { total: pageCount });
  // The key span of the rows on this page (#268). The CSV view lists differences
  // only, so the key range is the one thing a reader checking data by key can
  // hold onto when the display cap hides the rest.
  const keyInfo = document.createElement("span"); keyInfo.className = "csv-key-range"; keyInfo.hidden = true;
  const next = document.createElement("button"); next.type = "button"; next.className = "csv-page-next"; next.textContent = "→"; next.setAttribute("aria-label", t("nextPage")); next.title = t("nextPage");
  // The controls stay put across a page turn, so the button that was clicked
  // keeps its focus without having to be found again afterwards.
  prev.onclick = () => showCSVPage(csvPage - 1);
  next.onclick = () => showCSVPage(csvPage + 1);
  const jumpToPage = () => {
    const requested = Number(pageInput.value);
    if (!Number.isInteger(requested) || requested < 1 || requested > csvView.pageCount) {
      pageInput.value = String(csvPage + 1);
      return;
    }
    showCSVPage(requested - 1);
  };
  pageInput.onchange = jumpToPage;
  pageInput.onkeydown = (event) => { if (event.key === "Enter") { event.preventDefault(); jumpToPage(); } };
  controls.append(prev, pageInput, total, keyInfo, next); result.append(controls);

  const wrap = document.createElement("div"); wrap.className = "csv-table-wrap";
  const table = document.createElement("table"); table.className = "csv-table";
  const head = document.createElement("thead");
  const tbody = document.createElement("tbody");
  table.append(head, tbody); wrap.append(table); result.append(wrap);

  csvView = { data, table, head, tbody, controls: { prev, next, pageInput, keyInfo }, pageCount, columns: [] };
  renderCSVColumns();
}

// renderCSVColumns rebuilds the column headers and the rows under them. The
// column set only changes when "changed columns only" is toggled, so paging
// does not come through here.
function renderCSVColumns() {
  if (!csvView) return;
  const { data } = csvView;
  const changedSet = (data.summary.column_changes || []).map((column) => column.index);
  csvView.columns = visibleColumns(data.header.length, changedSet, $("changedColumnsOnly").checked);
  renderCSVHeader();
  renderCSVRows();
}

// renderCSVHeader builds the column headers, each draggable so a wide result
// can be arranged to read left-to-right by hand (#119). Reordering is
// presentation only: csvView.columns holds canonical indexes, and the rows
// below read the same indexes in the new order.
function renderCSVHeader() {
  if (!csvView) return;
  const { data, head } = csvView;
  head.textContent = "";
  const headerRow = document.createElement("tr");
  const sideHead = document.createElement("th"); sideHead.textContent = "_side"; headerRow.append(sideHead);
  const counts = new Map((data.summary.column_changes || []).map((column) => [column.index, column.count]));
  csvView.columns.forEach((index, position) => {
    const th = document.createElement("th");
    th.textContent = data.header[index];
    if (counts.has(index)) { const badge = document.createElement("b"); badge.textContent = counts.get(index); th.append(badge); }
    th.draggable = true;
    th.title = t("columnMapDragHint");
    th.addEventListener("dragstart", (event) => {
      event.dataTransfer.setData("text/plain", String(position));
      event.dataTransfer.effectAllowed = "move";
      th.classList.add("csv-column-dragging");
    });
    th.addEventListener("dragend", () => th.classList.remove("csv-column-dragging"));
    th.addEventListener("dragover", (event) => event.preventDefault());
    th.addEventListener("drop", (event) => {
      event.preventDefault();
      const from = Number(event.dataTransfer.getData("text/plain"));
      csvView.columns = reorderColumns(csvView.columns, from, position);
      renderCSVHeader();
      renderCSVRows();
    });
    headerRow.append(th);
  });
  head.append(headerRow);
}

// showCSVPage moves to a page by replacing the rows, which is the only part of
// the table a page turn changes.
function showCSVPage(page) {
  if (!csvView) return;
  const next = clampPage(page, csvView.pageCount);
  if (next === csvPage && csvView.tbody.childElementCount) return;
  csvPage = next;
  renderCSVRows();
}

function renderCSVRows() {
  if (!csvView) return;
  const { data, table, tbody, columns } = csvView;
  // The rows carrying these ids are being replaced, so the index has to go
  // with them rather than point at detached nodes (#154).
  resetMergeRowIndex();
  tbody.textContent = "";

  const appendRow = (values, side, kind, changed) => {
    if (!values?.length) return;
    const tr = document.createElement("tr"); tr.className = `csv-${kind.toLowerCase()} csv-${side}`;
    const sideCell = document.createElement("th"); sideCell.textContent = side; tr.append(sideCell);
    for (const index of columns) {
      const td = document.createElement("td");
      const value = values[index] ?? "";
      td.textContent = value;
      td.title = value;
      if (changed.has(index)) td.classList.add("csv-cell-changed");
      // A value that fits needs no affordance; one that is cut off gets a
      // click-to-expand and a copy button, since a tooltip is unreadable on a
      // touch screen and hides any change past the ellipsis (#105).
      if (value.length > CSV_CELL_PREVIEW_CHARS) makeCellExpandable(td, value);
      tr.append(td);
    }
    tbody.append(tr);
  };

  const { start, rows } = pageSlice(data.differences, csvPage, CSV_PAGE_SIZE);
  const keyInfo = csvView.controls.keyInfo;
  if (keyInfo) {
    const range = csvRowsKeyRange(rows);
    keyInfo.textContent = range ? t("csvPageKeyRange", { range }) : "";
    keyInfo.hidden = !range;
  }
  for (const [pageIndex, diff] of rows.entries()) {
    const action = document.createElement("tr"); action.className = "csv-merge-choice"; action.dataset.mergeId = diff.id;
    action.dataset.scrollAnchor = "csv";
    action.dataset.scrollKey = String(diff.id);
    action.dataset.scrollOrder = String(start + pageIndex);
    indexMergeRow(diff.id, action);
    const actionCell = document.createElement("th"); actionCell.colSpan = columns.length + 1;
    const label = document.createElement("span"); label.textContent = `${diff.kind} · ${diff.id.slice(0, 8)}`;
    actionCell.append(label);
    // Both toggles are offered even for a one-sided difference: the present
    // side keeps the row and the absent side drops it, mirroring the engine's
    // keep/drop decision. On a CHANGED pair both sides adopt real rows (#271).
    for (const [side, text] of [["left", t("chooseLeft")], ["right", t("chooseRight")]]) {
      const button = document.createElement("button"); button.type = "button"; button.className = `choose-${side}`; button.textContent = text;
      button.setAttribute("aria-pressed", "false"); button.title = t("mergeToggleHint");
      button.onclick = () => chooseMerge(diff.id, side); actionCell.append(button);
    }
    action.append(actionCell); tbody.append(action);
    const changed = new Set((diff.changed_columns || []).map((column) => column.index));
    appendRow(diff.old, "left", diff.kind, changed); appendRow(diff.new, "right", diff.kind, changed);
  }

  table.classList.toggle("wrap-cells", $("wrap").checked);
  syncCSVPager();
  updateCSVMergeUI();
}

function syncCSVPager() {
  if (!csvView) return;
  const { prev, next, pageInput } = csvView.controls;
  const state = pagerState(csvPage, csvView.pageCount);
  prev.disabled = state.atFirst;
  next.disabled = state.atLast;
  if (document.activeElement !== pageInput) pageInput.value = state.value;
}

async function compareCSV() {
	let body = csvRequestBody();
	if (!csvInspection) {
	  if (!validateInputs(body, false) || !(await inspectCSV())) return false;
	  body = csvRequestBody();
	}
	if (!validateInputs(body)) return false;
  const ac = new AbortController(); currentAbort = ac; $("cancel").hidden = false;
  const generation = beginRequest();
  const started = Date.now(), tick = () => setStatus(t("comparing") + " " + ((Date.now() - started) / 1000).toFixed(1) + "s", "busy"); tick();
  const timer = setInterval(tick, 100);
  try {
    const resp = await apiFetch("/api/csv/diff", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), signal: ac.signal });
    const data = await resp.json(); if (!resp.ok) throw apiError(data, resp);
    if (!isCurrentRequest(generation)) return false;
    threeWayData = null; resetMergeSelection("csv"); mergeUndo = []; mergeRedo = [];
    resetMergePreview();
    if (!$("mergeOutput").value) {
      const source = $("old").value.trim(); $("mergeOutput").value = source ? source.replace(/(\.[^./\\]+)?$/, ".merged$1") : "merged.csv";
    }
    csvKeyColumns = body.keyMode === "include" ? hiddenKeyIndexes(data.header, body.keyNames, body.keyIndexes) : [];
    csvPage = 0; renderCSV(data); rememberComparison(body); setStatus(""); return true;
  } catch (err) { if (err.name === "AbortError") setStatus(t("cancelled"), ""); else setStatus(String(err.message || err), "error"); return false; }
  finally { clearInterval(timer); $("cancel").hidden = true; currentAbort = null; }
}

async function runExportCSV() {
	let body = csvRequestBody();
	if (!csvInspection) { if (!validateInputs(body, false) || !(await inspectCSV())) return; body = csvRequestBody(); }
	if (!validateInputs(body)) return;
  if (!body.output) { setStatus(t("requiredField", { field: t("outputPath") }), "error"); return; }
  $("exportCSV").disabled = true; setStatus(t("comparing"), "busy");
  try { const resp = await apiFetch("/api/csv/export", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await resp.json(); if (!resp.ok) throw apiError(data, resp); setStatus(t("exportedCSV", data.output), "success"); }
  catch (err) { setStatus(String(err.message || err), "error"); } finally { $("exportCSV").disabled = false; }
}

function recentComparisons() {
  try { const value = JSON.parse(localStorage.getItem("ayame-recent-csv") || "[]"); return Array.isArray(value) ? value : []; }
  catch (_) { return []; }
}

function renderRecentComparisons() {
  const select = $("recentProjects"); select.innerHTML = "";
  const first = document.createElement("option"); first.value = ""; first.textContent = t("recent"); select.append(first);
  recentComparisons().forEach((body, index) => { const option = document.createElement("option"); option.value = String(index); option.textContent = `${body.old || "?"} ↔ ${body.new || "?"}`; select.append(option); });
}

function rememberComparison(body) {
  const clean = { ...body }; delete clean._validationError;
  const items = recentComparisons().filter((item) => item.old !== clean.old || item.new !== clean.new);
  items.unshift(clean); localStorage.setItem("ayame-recent-csv", JSON.stringify(items.slice(0, 10))); renderRecentComparisons();
}

async function applyCSVProject(body) {
  $("old").value = body.old || ""; $("new").value = body.new || "";
  for (const id of ["leftFormat", "rightFormat", "leftParser", "rightParser", "leftDelimiter", "rightDelimiter", "memory", "tempDir", "partitionBuffer", "maxRecordBytes"]) if (body[id] != null) $(id).value = body[id];
  $("whitespaceScale").value = whitespaceScaleLevel(body.whitespace);
  for (const id of ["hasHeader", "alignColumnsByName", "lazyQuotes", "trimLeadingSpace", "ignoreCase", "keepTemp", "outputHeader"]) {
    const target = id === "alignColumnsByName" ? "alignColumns" : id; if (body[id] != null) $(target).checked = Boolean(body[id]);
  }
  for (const id of ["partitions", "parseWorkers", "workers", "mergeFanIn", "maxRows"]) { const target = id === "maxRows" ? "csvMaxRows" : id; if (body[id]) $(target).value = body[id]; }
  $("lineFilters").value = (body.lineFilters || []).join("\n");
  filterState.rows = normalizeRowFilter(body.rowFilter);
  filterState.columns = normalizeRowFilter(body.columnFilter);
  renderFilterBuilders();
  $("ignoreColumns").value = (body.ignoreColumnNames?.length ? body.ignoreColumnNames : body.ignoreColumnIndexes || []).join(", ");
  $("tolerance").value = body.tolerance == null ? "" : body.tolerance;
  $("columnTolerances").value = (body.columnTolerances || []).map((item) => `${item.name ?? item.index}=${item.value}`).join(", ");
  $("csvOutput").value = body.output || ""; $("csvOutputFormat").value = body.outputFormat || "tsv";
  if (body.projectPath) $("projectPath").value = body.projectPath;
  csvInspection = null; $("keyMode").value = "all";
  if (!(await inspectCSV())) return;
  $("keyMode").value = body.keyMode || ((body.keyNames?.length || body.keyIndexes?.length) ? "include" : ((body.excludeKeyNames?.length || body.excludeKeyIndexes?.length) ? "exclude" : "all"));
  const names = new Set([...(body.keyNames || []), ...(body.excludeKeyNames || [])]);
  const indexes = new Set([...(body.keyIndexes || []), ...(body.excludeKeyIndexes || [])]);
  document.querySelectorAll("#columnList input").forEach((input) => { input.checked = names.has(input.dataset.name) || indexes.has(Number(input.dataset.index)); });
  syncKeyMode();
  if (body.columnMap?.length) setColumnMap(body.columnMap, "position", true);
  updateCSVReview();
  // A loaded project is a complete, deliberate state (#260); a later Compare
  // must not replace it with whatever this identity was last remembered as.
  adoptCurrentComparison();
  // A project is a session state, not the default: applying it updates the
  // scope indicator without touching what was saved as the default (#262).
  updateConditionScope();
}

async function runSaveProject() {
	let body = csvRequestBody();
	if (!csvInspection) { if (!validateInputs(body, false) || !(await inspectCSV())) return; body = csvRequestBody(); }
	body.projectPath = $("projectPath").value.trim();
  if (!validateInputs(body)) return;
  const missing = [];
  if (!body.projectPath) missing.push(t("projectPath"));
  if (!body.output) missing.push(t("outputPath"));
  if (missing.length) { setStatus(t("requiredFields", { fields: missing }), "error"); return; }
  try { const resp = await apiFetch("/api/project/save", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await resp.json(); if (!resp.ok) throw apiError(data, resp); rememberComparison(body); setStatus(t("projectSaved"), "success"); }
  catch (err) { setStatus(String(err.message || err), "error"); }
}

async function runLoadProject() {
  const path = $("projectPath").value.trim(); if (!path) { setStatus(t("requiredField", { field: t("projectPath") }), "error"); return; }
  try { const resp = await apiFetch("/api/project/load", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path }) }); const data = await resp.json(); if (!resp.ok) throw apiError(data, resp); if (data.mode === "dir") applyDirectoryProject(data); else await applyCSVProject(data); rememberComparison(data); setStatus(""); }
  catch (err) { setStatus(String(err.message || err), "error"); }
}

function dirRequestBody() { return { mode: "dir", old: $("old").value.trim(), new: $("new").value.trim(), includes: splitList($("dirIncludes").value), excludes: splitList($("dirExcludes").value), filter: $("dirFilter").value.trim(), filterFile: $("dirFilterFile").value.trim(), filterSets: splitList($("dirFilterSet").value), compareBy: $("dirCompareBy").value, hidden: $("dirHidden").checked, workers: Number($("dirWorkers").value) || 8 }; }

async function runPreviewDirectoryFilter() {
  const body = dirRequestBody(); if (!validateInputs(body)) return;
  $("dirPreview").disabled = true; $("dirPreviewResult").textContent = t("comparing");
  try { const resp = await apiFetch("/api/dir/preview", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await resp.json(); if (!resp.ok) throw apiError(data, resp); $("dirPreviewResult").textContent = t("filterPreviewResult", data); $("dirPreviewResult").title = (data.sample || []).join("\n"); }
  catch (err) { $("dirPreviewResult").textContent = String(err.message || err); }
  finally { $("dirPreview").disabled = false; }
}

function applyDirectoryProject(body) {
  $("mode").value = "dir"; syncModeOpts(); $("old").value = body.old || ""; $("new").value = body.new || "";
  $("dirIncludes").value = (body.includes || []).join(", "); $("dirExcludes").value = (body.excludes || []).join(", ");
  $("dirFilter").value = body.filter || ""; $("dirFilterSet").value = (body.filterSets || []).join(", ");
  $("dirCompareBy").value = body.compareBy || "contents"; $("dirHidden").checked = Boolean(body.hidden); $("dirWorkers").value = body.workers || 8;
  if (body.projectPath) $("dirProjectPath").value = body.projectPath;
  adoptCurrentComparison();
}

async function runSaveDirectoryProject() {
  const body = dirRequestBody(); body.projectPath = $("dirProjectPath").value.trim();
  if (!validateInputs(body) || !body.projectPath) { if (!body.projectPath) setStatus(t("requiredField", { field: t("projectPath") }), "error"); return; }
  try { const resp = await apiFetch("/api/project/save", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await resp.json(); if (!resp.ok) throw apiError(data, resp); rememberComparison(body); setStatus(t("projectSaved"), "success"); }
  catch (err) { setStatus(String(err.message || err), "error"); }
}

async function runLoadDirectoryProject() {
  const path = $("dirProjectPath").value.trim(); if (!path) { setStatus(t("requiredField", { field: t("projectPath") }), "error"); return; }
  try { const resp = await apiFetch("/api/project/load", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path }) }); const data = await resp.json(); if (!resp.ok) throw apiError(data, resp); if (data.mode !== "dir") throw new Error("not a folder project"); applyDirectoryProject(data); rememberComparison(data); setStatus(""); }
  catch (err) { setStatus(String(err.message || err), "error"); }
}

// Opening a file from a folder result used to overwrite the mode and both paths
// and re-run, which threw the folder comparison away. Checking a hundred files
// meant re-scanning the tree a hundred times. The result is still in memory, so
// going back is a re-render, not another scan.
let folderReturn = null;

function captureFolderTreeState(selectedPath = "") {
  const expanded = new Map();
  for (const row of document.querySelectorAll(".dir-folder[data-dir-path]")) {
    expanded.set(row.dataset.dirPath, row.getAttribute("aria-expanded") === "true");
  }
  return {
    anchor: captureResultScrollAnchor(),
    expanded,
    selectedPath: selectedPath || document.activeElement?.dataset.dirPath || document.querySelector(".dir-entry.selected")?.dataset.dirPath || "",
  };
}

async function openFromFolder(entry, body) {
  teardownContinuousView();
  folderReturn = {
    data: directoryData,
    body: directoryBody,
    ...captureFolderTreeState(entry.path),
  };
  const view = directoryEntryRequest(entry, body.old, body.new);
  directoryEntryView = entry.status === "added" || entry.status === "removed" ? view : null;
  $("mode").value = "text"; syncModeOpts();
  $("old").value = view.old;
  $("new").value = view.new;
  await compare();
  syncFolderReturn();
}

function syncFolderReturn() {
  const button = $("backToFolder");
  if (button) button.hidden = !folderReturn;
}

async function returnToFolder() {
  if (!folderReturn) return;
  const { data, body, anchor, expanded, selectedPath } = folderReturn;
  folderReturn = null;
  directoryEntryView = null;
  $("mode").value = "dir"; syncModeOpts();
  $("old").value = body.old; $("new").value = body.new;
  await renderDirectory(data, body, { expanded, selectedPath });
  // The list may have changed height, so restore its logical path rather than
  // the old pixel offset.
  restoreResultScrollAnchor(anchor, true);
  syncFolderReturn();
}

// ---- Folder result tree (#104) ----
// The result was a flat list of buttons, one per file, each repeating its whole
// path and faking depth with a left padding. Nothing could be folded away, so a
// large comparison could only be scrolled, and the size and timestamp the server
// already sends were reachable only as a hover tooltip.

// Sizes and times are on the wire already but were only ever in a title. Bytes
// arrive raw and timestamps as RFC3339Nano UTC, neither of which is readable in
// a list.
// A size sits in a narrow column beside a name and a timestamp, so it is shown
// with the symbol rather than the word: "7 B → 15 B" instead of spelling out
// the unit twice. The row's tooltip carries the translated long form.
function describeBytes(entry) {
  const sizes = entry.status === "removed" ? [entry.old_size] : entry.status === "added" ? [entry.new_size] : [entry.old_size, entry.new_size];
  return `${sizes.join(" → ")} ${t("bytes")}`;
}

async function renderDirectory(data, body, state = {}) {
  directoryData = data; directoryBody = body;
  folderReturn = null; syncFolderReturn();
  directoryEntryView = null;
  hideSuggestions();
  teardownContinuousView();
  syncContinuousControls();
  csvData = null; lastData = null; lastComparedRequest = null; minimapHasMarkers = false; $("minimap").hidden = true; $("syncPanel").hidden = true;
  clearUnchangedContext();
  // The toolbar used to be hidden outright for a folder result, which took the
  // theme and colour choices with it and left the status filter stranded in the
  // setup form, reachable only by scrolling back up (#104). Keep the bar and
  // show the parts that apply to a folder.
  $("diffNav").hidden = false;
  $("dirStatusWrap").hidden = false;
  $("dirSearchWrap").hidden = false;
  syncDirFlatControls();
  const flat = dirFlatActive();
  for (const id of ["firstDiff", "prevDiff", "nextDiff", "lastDiff", "addSync", "clearSync", "diffCounter", "viewModeWrap", "sidebarToggle", "confirmCounter", "prevUnconfirmed", "nextUnconfirmed"]) {
    const node = $(id); if (node) node.hidden = true;
  }
  syncExportPatchVisibility();
  // A folder has no reading axis; make sure the control a previous file result
  // may have shown is gone (#263).
  syncInterpretationVisibility();
  $("mergePanel").hidden = true;
  const summary = $("summary"); summary.innerHTML = "";
  // The rows are marked + − ~ =, which nothing explained. Putting each marker on
  // the count it belongs to makes the summary the legend, rather than adding a
  // second thing to read.
  for (const [name, cls] of [["added", "add"], ["removed", "del"], ["changed", "chg"], ["same", ""]]) {
    const item = document.createElement("span"); item.className = `stat ${cls}`;
    const mark = document.createElement("span"); mark.className = "stat-marker"; mark.textContent = DIR_MARKERS[name];
    mark.setAttribute("aria-hidden", "true");
    const b = document.createElement("b"); b.textContent = fmt(data[name]);
    item.append(mark, " ", b, ` ${t(name)}`); summary.append(item);
  }
  summary.hidden = false;
  const result = $("result"); result.innerHTML = "";
  result.append(paneHeads(data));
  const tree = document.createElement("div"); tree.className = "dir-tree";
  tree.classList.toggle("flat", flat);
  tree.setAttribute("role", "tree");
  tree.setAttribute("aria-label", t("folderSetup"));
  const header = document.createElement("div"); header.className = "dir-entry dir-header";
  header.setAttribute("role", "presentation");
  const headerColumns = [["dir-marker", ""], ["dir-name", t("folderName")]];
  if (flat) headerColumns.push(["dir-location", t("folderLocation")]);
  headerColumns.push(["dir-size", t("folderSize")], ["dir-stamp", t("folderModified")]);
  for (const [cls, label] of headerColumns) {
    const cell = document.createElement("span"); cell.className = cls; cell.textContent = label; header.append(cell);
  }
  tree.append(header);
  const filter = $("dirStatus").value;
  // A folder comparison carries one entry per file with no cap, so a large
  // tree is exactly the case that must not arrive as one blocking loop (#127).
  const visible = filterDirectoryEntries(data.entries, filter, $("dirSearch").value);

  const root = buildDirTree(visible);
  const expandByDefault = visible.length <= DIR_AUTO_EXPAND_LIMIT;
  let directoryRowOrder = 0;

  const fileRow = (entry, depth) => {
    const row = document.createElement("button");
    row.type = "button"; row.className = `dir-entry ${entry.status}`;
    row.setAttribute("role", "treeitem");
    row.tabIndex = -1;
    row.style.setProperty("--dir-depth", String(depth));
    row.dataset.dirPath = entry.path;
    row.dataset.scrollAnchor = "directory";
    row.dataset.scrollKey = entry.path;
    row.dataset.scrollOrder = String(directoryRowOrder++);
    row.classList.toggle("selected", state.selectedPath === entry.path);
    const marker = document.createElement("span"); marker.className = "dir-marker"; marker.textContent = DIR_MARKERS[entry.status];
    marker.setAttribute("aria-hidden", "true");
    // The name alone, not the whole path: the path is what the nesting says.
    const name = document.createElement("span"); name.className = "dir-name"; name.textContent = entry.name;
    const size = document.createElement("span"); size.className = "dir-size"; size.textContent = dirEntrySize(entry);
    const stamp = document.createElement("span"); stamp.className = "dir-stamp"; stamp.textContent = dirEntryStamp(entry);
    // The marker carries meaning that colour alone would not convey.
    if (flat) {
      const location = document.createElement("span");
      location.className = "dir-location";
      location.textContent = locationOf(entry.path);
      location.title = entry.path;
      row.append(marker, name, location, size, stamp);
    } else {
      row.append(marker, name, size, stamp);
    }
    row.setAttribute("aria-label", `${t(entry.status)} ${entry.path}`);
    row.title = `${entry.path}\n${describeBytes(entry)}`;
    if (entry.status !== "same") row.addEventListener("click", () => openFromFolder(entry, body));
    else {
      row.classList.add("inert");
      row.setAttribute("aria-disabled", "true");
    }
    return row;
  };

  // A folder's children are built the first time it opens. Expanding everything
  // up front is exactly what made a large result expensive, and a folded folder
  // that has never been opened should cost nothing but its own row.
  const folderRow = (node, depth) => {
    const group = document.createElement("div"); group.className = "dir-group";
    const row = document.createElement("button");
    row.type = "button"; row.className = "dir-entry dir-folder";
    row.setAttribute("role", "treeitem");
    row.tabIndex = -1;
    row.style.setProperty("--dir-depth", String(depth));
    const twisty = document.createElement("span"); twisty.className = "dir-twisty"; twisty.setAttribute("aria-hidden", "true");
    const name = document.createElement("span"); name.className = "dir-name"; name.textContent = node.name;
    const badges = document.createElement("span"); badges.className = "dir-badges";
    const total = document.createElement("span");
    total.className = "dir-badge total";
    total.textContent = fmt(node.total);
    total.title = t("folderFileCount", { count: fmt(node.total) });
    badges.append(total);
    for (const [status, cls] of [["added", "add"], ["removed", "del"], ["changed", "chg"]]) {
      if (!node.counts[status]) continue;
      const badge = document.createElement("span");
      badge.className = `dir-badge ${cls}`;
      badge.textContent = `${DIR_MARKERS[status]}${node.counts[status]}`;
      badge.title = `${node.counts[status]} ${t(status)}`;
      badges.append(badge);
    }
    row.append(twisty, name, badges);
    const children = document.createElement("div"); children.className = "dir-children";
    children.setAttribute("role", "group");
    let built = false;
    const setOpen = (open) => {
      row.setAttribute("aria-expanded", String(open));
      children.hidden = !open;
      if (open && !built) { built = true; fillChildren(children, node, depth + 1); }
    };
    row.addEventListener("click", () => setOpen(row.getAttribute("aria-expanded") !== "true"));
    row.dataset.dirPath = node.path;
    row._setOpen = setOpen;
    const restoreOpen = state.expanded instanceof Map && state.expanded.has(node.path)
      ? state.expanded.get(node.path)
      : expandByDefault;
    setOpen(restoreOpen);
    group.append(row, children);
    return group;
  };

  function fillChildren(target, node, depth) {
    for (const child of [...node.dirs.values()].sort((a, b) => a.name.localeCompare(b.name))) {
      target.append(folderRow(child, depth));
    }
    for (const file of node.files.sort((a, b) => a.name.localeCompare(b.name))) {
      target.append(fileRow(file, depth));
    }
  }

  result.append(tree);
  if (flat) {
    // No hierarchy: every filtered file in path order, with its relative
    // location beside the name (#275).
    const items = sortDirectoryEntries(visible).map((entry) => ({ file: entry }));
    if (!(await renderInSlices(tree, items, (item) => fileRow(item.file, 0)))) return;
    initDirKeyboard(tree, state.selectedPath);
    setStatus("");
    return;
  }
  if (visible.length) {
    // Slice the top level so a wide root still yields to the browser (#127).
    const top = [
      ...[...root.dirs.values()].sort((a, b) => a.name.localeCompare(b.name)).map((n) => ({ dir: n })),
      ...root.files.sort((a, b) => a.name.localeCompare(b.name)).map((f) => ({ file: f })),
    ];
    if (!(await renderInSlices(tree, top, (item) => (item.dir ? folderRow(item.dir, 0) : fileRow(item.file, 0))))) return false;
    initDirKeyboard(tree, state.selectedPath);
  }
  setStatus("");
  return true;
}

// initDirKeyboard gives the tree the one-tab-stop, arrow-driven model a tree is
// expected to have. Every row used to be a separate tab stop, and rows that
// could not be opened were disabled, which dropped them out of the tab order
// entirely — so on a large result the keyboard was no way through it at all.
function initDirKeyboard(tree, selectedPath = "") {
  const rows = () => [...tree.querySelectorAll('[role="treeitem"]')].filter((row) => row.offsetParent !== null);
  const focus = (row) => {
    if (!row) return;
    for (const other of tree.querySelectorAll('[role="treeitem"]')) other.tabIndex = -1;
    row.tabIndex = 0; row.focus();
  };
  const first = selectedPath
    ? [...tree.querySelectorAll('[role="treeitem"]')].find((row) => row.dataset.dirPath === selectedPath)
    : tree.querySelector('[role="treeitem"]');
  if (first) first.tabIndex = 0;
  if (selectedPath && first) first.focus({ preventScroll: true });
  tree.addEventListener("keydown", (event) => {
    const current = event.target.closest('[role="treeitem"]');
    if (!current) return;
    const list = rows();
    const at = list.indexOf(current);
    const open = current.getAttribute("aria-expanded");
    switch (event.key) {
      case "ArrowDown": focus(list[at + 1]); break;
      case "ArrowUp": focus(list[at - 1]); break;
      case "ArrowRight":
        if (open === "false") current._setOpen(true);
        else if (open === "true") focus(list[at + 1]);
        else return;
        break;
      case "ArrowLeft":
        if (open === "true") current._setOpen(false);
        else {
          // Leaving a leaf or a closed folder means going up to the parent row.
          const parent = current.closest(".dir-children")?.parentElement?.querySelector('[role="treeitem"]');
          focus(parent);
        }
        break;
      case "Home": focus(list[0]); break;
      case "End": focus(list[list.length - 1]); break;
      default: return;
    }
    event.preventDefault();
  });
}

async function compareDirectory() {
  const body = dirRequestBody(); if (!validateInputs(body)) return false;
  const ac = new AbortController(); currentAbort = ac; $("cancel").hidden = false;
  const generation = beginRequest();
  const started = Date.now();
  const tick = () => setStatus(t("comparing") + " " + ((Date.now() - started) / 1000).toFixed(1) + "s", "busy");
  tick();
  const timer = setInterval(tick, 100);
  try {
    const resp = await apiFetch("/api/dir/diff", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), signal: ac.signal });
    const data = await resp.json(); if (!resp.ok) throw apiError(data, resp);
    if (!isCurrentRequest(generation)) return false;
    clearInterval(timer);
    const rendered = await renderDirectory(data, body);
    if (!rendered && renderGate.cancelled) { setStatus(t("cancelled"), ""); return false; }
    return true;
  }
  catch (err) { if (err.name === "AbortError") setStatus(t("cancelled"), ""); else setStatus(String(err.message || err), "error"); return false; }
  finally { clearInterval(timer); $("cancel").hidden = true; currentAbort = null; }
}

// ---- streaming a text comparison (#297) ----
// The comparison is long before it is whole, so /api/diff/stream reports each
// phase as it happens and sends the hunks in bounded pages. The first page is
// rendered as soon as it lands; later pages are appended; the unchanged-context
// regions, which need the full hunk geometry, are interleaved once the stream
// ends. A small diff arrives as a single page and takes the same path.

// readDiffStream consumes the NDJSON reply, calling onEvent for each line and
// returning the assembled diff response. Rendering is awaited inside the loop,
// so pages paint in order.
async function readDiffStream(response, onEvent) {
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  const data = { hunks: [] };
  let failure = null;

  const handleLine = async (line) => {
    const trimmed = line.trim();
    if (!trimmed) return;
    const event = JSON.parse(trimmed);
    await onEvent(event, data);
    if (event.type === "error") failure = event;
  };

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let newline;
    while ((newline = buffer.indexOf("\n")) >= 0) {
      const line = buffer.slice(0, newline);
      buffer = buffer.slice(newline + 1);
      await handleLine(line);
    }
  }
  if (buffer) await handleLine(buffer);

  if (failure) {
    throw apiError(
      { code: failure.code, error: failure.error, path: failure.path, side: failure.side },
      { status: 0 },
    );
  }
  return data;
}

// beginStreamRender paints the frame a large diff needs before its hunks: the
// summary, the pane headers, and an empty result that the pages fill.
function beginStreamRender(data) {
  applyDisplayPreferences();
  renderSummary(data);
  const result = $("result");
  result.innerHTML = "";
  setupNavigation(data);
  syncExportPatchVisibility();
  result.append(paneHeads(data));
  clearUnchangedContext();
}

// appendStreamHunks renders one page onto the result. start is the global hunk
// index, so every hunk keeps the id and merge coordinates the finished diff
// would have given it.
async function appendStreamHunks(hunks, stream) {
  if (!hunks.length) return;
  const start = stream.rendered;
  const complete = await renderInSlices($("result"), hunks, (hunk, index) => renderHunk(hunk, start + index));
  if (!complete) return;
  stream.rendered += hunks.length;
  // renderInSlices reports its own page-local count; the tracker knows the
  // whole streamed result, so let it own the lane again after a page.
  renderProgress(stream.tracker.snapshot());
}

// applyDiffStreamEvent routes one progress event: stage transitions drive the
// tracker, a forecast posts to the message lane before the reader meets the
// truncation, and result pages paint as they arrive.
async function applyDiffStreamEvent(event, stream, generation) {
  if (!isCurrentRequest(generation)) return;
  if (event.type === "stage") {
    if (event.state === "active") stream.tracker.begin(event.stage);
    else stream.tracker.finish(event.stage, event.elapsed_ms);
    return;
  }
  if (event.type === "forecast") {
    stream.tracker.setForecast(event);
    const notice = degradationNotice(event);
    if (notice) messageLog.post(t(notice.key, notice.params), "warning");
    return;
  }
  if (event.type === "result" && event.result) {
    Object.assign(stream.data, event.result);
    stream.data.hunks = Array.isArray(event.result.hunks) ? [...event.result.hunks] : [];
    if (event.page_count) stream.tracker.progress(1, event.page_count);
    if (!stream.started) {
      stream.started = true;
      beginStreamRender(stream.data);
    }
    await appendStreamHunks(stream.data.hunks, stream);
    return;
  }
  if (event.type === "result_page" && Array.isArray(event.hunks)) {
    stream.data.hunks.push(...event.hunks);
    if (event.page_count) stream.tracker.progress((event.page || 0) + 1, event.page_count);
    await appendStreamHunks(event.hunks, stream);
  }
}

// finishStreamRender interleaves the unchanged-context regions now that every
// hunk is on screen, then runs the same finishing steps as a one-shot render.
async function finishStreamRender(stream) {
  const data = stream.data;
  const result = $("result");
  stream.tracker.begin("render");
  if (!data.hunks.length) {
    const scope = t("textMatchScope", {
      old: Number(data.old_lines || 0).toLocaleString(),
      new: Number(data.new_lines || 0).toLocaleString(),
    });
    result.append(resultStateCard(t(comparisonUsesRules() ? "filteredMatch" : "completeMatch"), scope));
    stream.tracker.finish("render");
    return;
  }
  prepareUnchangedContext(data);
  for (let index = 0; index < data.hunks.length; index++) {
    const node = document.getElementById(`hunk-${index}`);
    if (node && unchangedRegions[index]) node.before(unchangedRegions[index].node);
  }
  if (unchangedRegions[data.hunks.length]) result.append(unchangedRegions[data.hunks.length].node);
  syncContextVisibility();
  const contextComplete = await loadInitialContext({ announce: true });
  if (contextComplete) setStatus("");
  if (searchOpen()) runSearch();
  updateMergeUI();
  observeHunks();
  buildMinimap(data);
  updateMinimapViewport();
  stream.tracker.finish("render");
}

async function runCompare() {
  // A provisional line render only exists in a text edit session; anything that
  // leaves text mode must drop it rather than strand the banner (#258).
  if ($("mode").value === "threeway") { clearProvisional(); return compareThreeWay(false); }
  if ($("mode").value === "threeway-csv") { clearProvisional(); return compareThreeWay(true); }
  if ($("mode").value === "csv") { clearProvisional(); return compareCSV(); }
	if ($("mode").value === "dir") { clearProvisional(); return compareDirectory(); }
  const body = requestBody();
  if (!validateInputs(body)) return false;
  ignoredHunks = new Set();
  resetSyncSelection();
  const ac = new AbortController();
  currentAbort = ac;
  const generation = beginRequest();
  $("cancel").hidden = false;
  // While a provisional line edit is on screen, hold it and let the status line
  // carry progress: the whole result is replaced when this comparison finishes
  // and `compare` restores the scroll anchor, so the reader sees neither a blank
  // skeleton nor a jump (#258, #127). Comparison-condition changes have no
  // honest approximation, so they keep the skeleton-and-wait behaviour.
  const holdProvisional = provisionalActive && provisionalRenderingPossible();
  if (!holdProvisional) {
    clearProvisional();
    lastData = null;
    lastComparedRequest = null;
    clearUnchangedContext();
    syncExportPatchVisibility();
    $("summary").hidden = true;
    showResultSkeleton();
  }
  // The tracker renders the phases the server reports; there is no client-side
  // elapsed timer pretending to know more than the pipeline does (#297).
  const tracker = createProgressTracker({ onChange: renderProgress });
  tracker.start(comparisonStagePlan({ detectMoves: body.detectMoves }));
  const stream = { data: { hunks: [] }, tracker, rendered: 0, started: false };
  try {
    const response = await apiFetch("/api/diff/stream", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal: ac.signal,
    });
    if (!response.ok) {
      const failure = await response.json().catch(() => ({}));
      throw apiError(failure, response);
    }
    const data = await readDiffStream(response, (event) => applyDiffStreamEvent(event, stream, generation));
    // Drag and drop, folder-entry clicks, and sync-point edits all call
    // compare() directly, so a newer comparison can already be running (#128).
    if (!isCurrentRequest(generation)) return false;
    lastData = data;
    lastComparedRequest = JSON.stringify(body);
    threeWayData = null;
    resetMergeSelection("text"); mergeUndo = []; mergeRedo = [];
    resetMergePreview();
    setMergeMode(false); // every fresh diff opens in reading mode (#100)
    if (!$("mergeOutput").value) {
      const source = $("old").value.trim();
      $("mergeOutput").value = source ? source.replace(/(\.[^./\\]+)?$/, ".merged$1") : "merged.txt";
    }
    await finishStreamRender(stream);
    tracker.stop();
    // A render the user stopped is not a finished comparison: report the
    // cancellation and leave the form open, like an aborted request (#128).
    if (renderGate.cancelled) {
      setStatus(t("cancelled"), "");
      return false;
    }
    return true;
  } catch (err) {
    // A superseded request must not clear a provisional render that the newer
    // comparison is holding on to.
    if (isCurrentRequest(generation)) clearProvisional();
    if (err.name === "AbortError") setStatus(t("cancelled"), "");
    else setStatus(String(err.message || err), "error");
    return false;
  } finally {
    tracker.stop();
    $("cancel").hidden = true;
    currentAbort = null;
  }
}

// Public entry points take the exclusion lock; the run* functions hold the
// actual work. Wrapping here rather than at each button covers the callers
// that never touch a button — drag and drop, folder-entry clicks, sync-point
// edits, and the Enter key (#128).
async function compare(options = {}) {
  if (busyOperation) return false;
  // Resolve the conditions before the request body is built (#260): a
  // comparison starts from its own remembered state, not the last one's.
  const conditionKey = await scopeConditionsForRun(Boolean(options.keepConditions));
  const preparedWatch = options.watch || await prepareFileWatch();
  // A tab switch supplies its own anchor; otherwise keep the current position.
  const scrollAnchor = Object.prototype.hasOwnProperty.call(options, "scrollAnchor")
    ? options.scrollAnchor
    : captureResultScrollAnchor();
  const result = await runExclusive("compare", runCompare);
  if (result) restoreResultScrollAnchor(scrollAnchor, true);
  // Only fold once something is actually on screen. A failed or cancelled run
  // leaves the form open, because the inputs are then what needs attention.
  if (result && $("result").children.length) {
    collapseSetupAfterCompare();
    rememberPaths();
    storeComparisonState(conditionKey, captureComparisonState());
    const historyAction = options.urlHistory || (options.external ? "replace" : "push");
    updateComparisonURL(historyAction);
  }
  if (preparedWatch && $("autoReload").checked &&
      sameWatchPaths(preparedWatch.paths, currentWatchPaths())) {
    fileWatcher.start(preparedWatch.paths, preparedWatch.snapshot);
  }
  if (options.external && result) setStatus(t("externalReloaded"), "success");
  await resumeEditingAfterReload();
  return result;
}
async function exportPatch() { return runExclusive("exportPatch", runExportPatch); }
async function exportReport() { return runExclusive("exportReport", runExportReport); }
async function exportCSV() { return runExclusive("exportCSV", runExportCSV); }
async function saveProject() { return runExclusive("saveProject", runSaveProject); }
async function loadProject() { return runExclusive("loadProject", runLoadProject); }
async function previewDirectoryFilter() { return runExclusive("previewDirectoryFilter", runPreviewDirectoryFilter); }
async function saveDirectoryProject() { return runExclusive("saveDirectoryProject", runSaveDirectoryProject); }
async function loadDirectoryProject() { return runExclusive("loadDirectoryProject", runLoadDirectoryProject); }

// ---- In-result search (#118) ----
// The browser's own find-in-page is a poor fit here: a side-by-side diff wraps
// and scrolls horizontally, matches inside a collapsed off-screen hunk behave
// unpredictably, and it reports no count. This searches the rendered result,
// marks every hit, and steps between them.
//
// Matches are wrapped in place with CSS Highlight-free spans so nothing about
// the diff's own markup changes: only text nodes are touched, and clearing
// restores them by normalising the parent back together.

let searchHits = [];
let searchIndex = -1;
let searchTimer = 0;

function searchOpen() { return !$("resultSearch").hidden; }

function openSearch() {
  if (!$("result").children.length) return;
  $("resultSearch").hidden = false;
  $("searchInput").focus();
  $("searchInput").select();
  if ($("searchInput").value) runSearch();
}

function closeSearch() {
  $("resultSearch").hidden = true;
  clearSearchHits();
  setSearchCounter(0, 0);
}

// clearSearchHits unwraps every marker and rejoins the split text nodes, so a
// repeated search never sees text fragmented by the previous one.
function clearSearchHits() {
  for (const hit of document.querySelectorAll("#result .search-hit")) {
    const parent = hit.parentNode;
    if (!parent) continue;
    parent.replaceChild(document.createTextNode(hit.textContent), hit);
    parent.normalize();
  }
  searchHits = [];
  searchIndex = -1;
}

function searchPattern() {
  const raw = $("searchInput").value;
  if (!raw) return null;
  const flags = $("searchCase").checked ? "g" : "gi";
  if ($("searchRegex").checked) {
    try {
      return new RegExp(raw, flags);
    } catch {
      return undefined; // signals an invalid pattern, distinct from "empty"
    }
  }
  return new RegExp(raw.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), flags);
}

// searchableCells yields the text containers to scan, honoring the changed-only
// scope.
//
// It targets the .tx span rather than the whole .cell: a cell also holds the
// line-number gutter, so searching the cell made a query like "12" light up
// line numbers instead of content.
function searchableCells() {
  const selector = $("searchChangedOnly").checked
    ? "#result .cell.add .tx, #result .cell.del .tx, #result .cell.chg .tx, #result .three-line, #result td"
    : "#result .cell .tx, #result .three-line, #result td";
  const cells = [...document.querySelectorAll(selector)];
  // Unified context displays one unchanged line rather than duplicate left and
  // right copies. Do not count or jump to the hidden right-hand copy.
  if ($("result").classList.contains("unified")) {
    return cells.filter((node) => !node.closest(".context-region .cell.context-duplicate"));
  }
  return cells;
}

function runSearch() {
  clearSearchHits();
  const pattern = searchPattern();
  const input = $("searchInput");
  input.classList.toggle("no-match", pattern === undefined);
  if (!pattern) {
    setSearchCounter(0, 0);
    return;
  }
  for (const cell of searchableCells()) {
    markMatchesIn(cell, pattern);
    // A pathological pattern on a huge diff would otherwise lock the page; the
    // count tells the user the result is partial.
    if (searchHits.length >= SEARCH_MAX_HITS) break;
  }
  input.classList.toggle("no-match", searchHits.length === 0 && Boolean($("searchInput").value));
  if (searchHits.length) {
    searchIndex = 0;
    focusSearchHit();
  } else {
    setSearchCounter(0, 0);
  }
}

// SEARCH_MAX_HITS bounds how many markers are inserted. Each one is a DOM node
// inside the diff, so an unbounded count would undo the render budget #127 set.
const SEARCH_MAX_HITS = 2000;

// markMatchesIn wraps every match inside one cell.
//
// Matching runs against the cell's whole text, not against individual text
// nodes: syntax highlighting and word-diff markup split a line into many nodes,
// so a per-node scan silently failed on anchors and on anything spanning a
// boundary — a regex like `payload_1[0-9]$` found nothing at all. The match
// ranges are then mapped back onto the nodes they cover, which is also what
// lets a single match straddle several spans.
function markMatchesIn(cell, pattern) {
  const full = cell.textContent;
  if (!full) return;
  pattern.lastIndex = 0;
  const ranges = [];
  for (let match = pattern.exec(full); match; match = pattern.exec(full)) {
    if (match[0] === "") {
      // A zero-length match would loop forever; step past it.
      pattern.lastIndex++;
      continue;
    }
    ranges.push([match.index, match.index + match[0].length]);
    if (searchHits.length + ranges.length >= SEARCH_MAX_HITS) break;
  }
  if (!ranges.length) return;

  const walker = document.createTreeWalker(cell, NodeFilter.SHOW_TEXT);
  const nodes = [];
  for (let node = walker.nextNode(); node; node = walker.nextNode()) nodes.push(node);

  let offset = 0;
  for (const node of nodes) {
    const text = node.nodeValue || "";
    const nodeStart = offset;
    const nodeEnd = offset + text.length;
    offset = nodeEnd;
    // Every range overlapping this node contributes a marked slice.
    const overlapping = ranges.filter(([from, to]) => from < nodeEnd && to > nodeStart);
    if (!overlapping.length) continue;
    const fragment = document.createDocumentFragment();
    let cursor = 0;
    for (const [from, to] of overlapping) {
      const localFrom = Math.max(0, from - nodeStart);
      const localTo = Math.min(text.length, to - nodeStart);
      if (localFrom > cursor) fragment.append(text.slice(cursor, localFrom));
      const mark = document.createElement("span");
      mark.className = "search-hit";
      mark.textContent = text.slice(localFrom, localTo);
      // A match split across nodes yields one marker per node; only the piece
      // that starts the match counts, so the tally matches what was found.
      if (from >= nodeStart) searchHits.push(mark);
      else mark.dataset.continuation = "1";
      fragment.append(mark);
      cursor = localTo;
    }
    if (cursor < text.length) fragment.append(text.slice(cursor));
    node.parentNode?.replaceChild(fragment, node);
  }
}

function setSearchCounter(current, total) {
  $("searchCounter").textContent = total
    ? t("searchCounter", { current, total, capped: total >= SEARCH_MAX_HITS ? "+" : "" })
    : ($("searchInput").value ? t("searchNoMatches") : "");
}

function focusSearchHit() {
  document.querySelector("#result .search-hit.current")?.classList.remove("current");
  const hit = searchHits[searchIndex];
  if (!hit) return;
  hit.classList.add("current");
  hit.scrollIntoView({ block: "center" });  // see jumpToHunk: smooth fails under content-visibility
  setSearchCounter(searchIndex + 1, searchHits.length);
}

function stepSearch(delta) {
  if (!searchHits.length) return;
  searchIndex = (searchIndex + delta + searchHits.length) % searchHits.length;
  focusSearchHit();
  announce($("searchCounter").textContent);
}

// Typing re-runs the whole scan, so it is debounced like the column filter.
function scheduleSearch() {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(runSearch, 120);
}

// ---- Progressive disclosure for the CSV setup (#93) ----
// The setup showed about 30 controls at once, seventeen of them engine tuning
// that a first comparison never needs. The rarely-used groups are collapsed;
// so that collapsing does not hide a setting someone actually changed, each
// group's header carries a count of the controls differing from their default.

// defaultControlValues snapshots every control's initial state, taken before
// anything can modify it, so "changed" means changed by the user rather than
// hardcoded per control.
const defaultControlValues = new Map();

function captureControlDefaults(root) {
  // textarea is included because the comparison-condition group has the
  // line-filter box; without it a filter typed there would never light the
  // group's badge (#262).
  for (const control of root.querySelectorAll("input, select, textarea")) {
    if (!control.id) continue;
    defaultControlValues.set(control.id, control.type === "checkbox" ? control.checked : control.value);
  }
}

function changedControlCount(root) {
  let changed = 0;
  for (const control of root.querySelectorAll("input, select, textarea")) {
    if (!control.id || !defaultControlValues.has(control.id)) continue;
    const initial = defaultControlValues.get(control.id);
    const now = control.type === "checkbox" ? control.checked : control.value;
    if (now !== initial) changed++;
  }
  return changed;
}

// updateDetailsBadges keeps each collapsed group honest about what it hides.
// The condition scope rides along because the badge and the scope chip both
// describe "differs from the default" and must never disagree.
function updateDetailsBadges() {
  for (const [groupID, badgeID] of [["csvAdvanced", "csvAdvancedBadge"], ["csvParsing", "csvParsingBadge"], ["engineTuning", "engineTuningBadge"], ["compareConditions", "conditionsBadge"], ["resultDisplay", "resultDisplayBadge"]]) {
    const group = $(groupID), badge = $(badgeID);
    if (!group || !badge) continue;
    const changed = changedControlCount(group);
    badge.textContent = changed ? t("changedSettings", { count: changed }) : "";
    badge.hidden = changed === 0;
  }
  updateConditionScope();
  syncConditionToolbar();
}

// ---- Comparison conditions: session override vs. saved default (#262) ----
// The "Comparison conditions" controls start from a default stored on its own.
// A change while comparing applies to that comparison only; nothing is written
// to storage until "Make default" is pressed, so a one-off tweak can never
// become the next visit's default by accident. This is Beyond Compare's
// session-scope dropdown reduced to the two scopes a single-page tool needs;
// URL and project state are just another session override, which is why loading
// one leaves the saved default alone.
function readConditionControlValue(id) {
  const node = $(id);
  if (!node) return "";
  return node.type === "checkbox" ? node.checked : node.value;
}

function writeConditionControlValue(control, value) {
  const node = $(control.id);
  if (!node) return;
  if (control.type === "checkbox") node.checked = Boolean(value);
  else node.value = String(value);
}

function currentConditions() {
  return captureConditionValues(readConditionControlValue);
}

// Snapshot the factory defaults first, then resolve the saved default against
// them and put the result into the controls before anything captures a
// baseline.
const factoryConditionDefaults = currentConditions();
let conditionDefaults = resolveConditionDefaults(
  factoryConditionDefaults,
  loadSavedConditionDefaults(),
  null,
);

function loadSavedConditionDefaults() {
  let text = null;
  try { text = localStorage.getItem(CONDITION_DEFAULTS_KEY); } catch (_) { text = null; }
  return parseConditionDefaults(text, factoryConditionDefaults);
}

// updateConditionScope names the scope in effect and keeps "Make default" and
// "Reset to default" enabled only while they would do something.
function updateConditionScope() {
  const current = currentConditions();
  const changed = changedConditionIds(current, conditionDefaults);
  const chip = $("conditionScopeState");
  if (chip) {
    chip.textContent = changed.length
      ? t("conditionScopeSession", { count: changed.length })
      : t("conditionScopeDefault");
    chip.classList.toggle("customized", changed.length > 0);
  }
  $("compareConditions")?.classList.toggle("session-customized", changed.length > 0);
  const atDefault = conditionsAreDefault(current, conditionDefaults);
  const make = $("makeConditionsDefault");
  const reset = $("resetConditionsDefault");
  if (make) make.disabled = atDefault;
  if (reset) reset.disabled = atDefault;
}

// syncConditionBaseline moves the "changed" baseline onto the new default, so
// promoting one clears the badge instead of leaving it lit. It also refreshes
// the per-comparison baseline (#260), so a comparison with no memory resets to
// the promoted default rather than the values the page loaded with.
function syncConditionBaseline() {
  captureControlDefaults($("compareConditions"));
  captureConditionDefaults();
  updateDetailsBadges();
}

function makeConditionsDefault() {
  conditionDefaults = promoteConditionDefaults(currentConditions(), conditionDefaults);
  try {
    localStorage.setItem(CONDITION_DEFAULTS_KEY, serializeConditionDefaults(conditionDefaults));
  } catch (_) {
    setStatus(t("conditionsDefaultUnavailable"), "error");
    return false;
  }
  syncConditionBaseline();
  setStatus(t("conditionsDefaultSaved"), "success");
  return true;
}

function resetConditionsToDefault() {
  applyConditionValues(conditionDefaults, writeConditionControlValue);
  syncConditionBaseline();
  setStatus(t("conditionsDefaultReset"), "");
}

applyConditionValues(conditionDefaults, writeConditionControlValue);

// swapSides exchanges the two inputs (#90). Every comparison tool has this and
// ayame-diff did not: reversing a comparison meant retyping both paths.
//
// It swaps whichever pair is actually in use — the paths, or the pasted text
// when scratch mode is on — and re-runs only if a result is already showing, so
// the button never starts work the user did not ask for.
function swapSides() {
  const activeDirectoryEntry = directoryEntryView
    && $("old").value.trim() === directoryEntryView.old
    && $("new").value.trim() === directoryEntryView.new;
  const pairs = $("scratch").checked
    ? [["oldText", "newText"]]
    : [["old", "new"]];
  for (const [left, right] of pairs) {
    const a = $(left), b = $(right);
    [a.value, b.value] = [b.value, a.value];
  }
  if (activeDirectoryEntry) {
    directoryEntryView = {
      old: directoryEntryView.new,
      new: directoryEntryView.old,
      oldAbsent: directoryEntryView.newAbsent,
      newAbsent: directoryEntryView.oldAbsent,
    };
  }
  // The inspection describes the previous pairing.
  csvInspection = null;
  $("inspection").textContent = "";
  $("keySetup").hidden = true;
  // Same comparison, reversed (#90): the conditions should follow the sides
  // rather than reset as though this were a different pair (#260).
  if (lastData || csvData || threeWayData || directoryData) compare({ keepConditions: true });
}

// ---- In-app dialogs (#98, #99) ----
// window.confirm and window.alert cannot be styled or read alongside the UI
// they describe, and the merge path used confirm to ask about writing a file —
// blocking the page on a native prompt at the moment the user most needs to see
// what they are confirming.

// askConfirm resolves true when the user proceeds. It mirrors confirm()'s
// shape so the call sites stay a single awaited expression. An optional list of
// impact rows (#273) is rendered inside the dialog so a destructive step shows
// what it will do before Proceed is available.
function askConfirm(message, details) {
  const dialog = $("confirmDialog");
  // showModal throws on an already-open dialog, which would reject into a
  // caller that has no answer to give. A second question while one is on
  // screen is declined instead, leaving the first one to be answered.
  if (dialog.open) return Promise.resolve(false);
  $("confirmMessage").textContent = message;
  const list = $("confirmDetails");
  if (list) {
    list.textContent = "";
    for (const item of details || []) {
      const row = document.createElement("li");
      row.textContent = typeof item === "string" ? item : item.text;
      row.className = "impact-" + (item && item.kind ? item.kind : "info");
      list.append(row);
    }
    list.hidden = list.childElementCount === 0;
  }
  const opener = document.activeElement;
  return new Promise((resolve) => {
    dialog.addEventListener("close", () => {
      // Focus returns to whatever opened the dialog, which Escape would
      // otherwise strand.
      if (opener && typeof opener.focus === "function") opener.focus();
      resolve(dialog.returnValue === "ok");
    }, { once: true });
    dialog.showModal();
    $("confirmOk").focus();
  });
}

// The merge result is on screen as a pane, so its destination is a save-time
// decision, not something to set before anything can be read (#282). This asks
// for the path only when Save is pressed, and closes first so the overwrite
// confirmation that follows is not a second dialog opened underneath this one.
function defaultMergeOutput() {
  const csv = $("mode").value === "threeway-csv" || $("mode").value === "csv";
  const source = ($("base").value || $("old").value || "").trim();
  if (!source) return csv ? "merged.csv" : "merged.txt";
  return source.replace(/(\.[^./\\]+)?$/, ".merged$1");
}

function askMergeOutput() {
  const dialog = $("mergeSaveDialog");
  if (dialog.open) return Promise.resolve(false);
  if (!$("mergeOutput").value.trim()) $("mergeOutput").value = defaultMergeOutput();
  const opener = document.activeElement;
  return new Promise((resolve) => {
    dialog.addEventListener("close", () => {
      if (opener && typeof opener.focus === "function") opener.focus();
      resolve(dialog.returnValue === "ok" && Boolean($("mergeOutput").value.trim()));
    }, { once: true });
    dialog.showModal();
    $("mergeOutput").focus();
  });
}

async function stopServer() {
  if (!await askConfirm(t("stopServerConfirm"))) return;
  const button = $("stopServer");
  button.disabled = true;
  setStatus(t("stoppingServer"), "busy");
  try {
    const response = await apiFetch("/api/shutdown", { method: "POST" });
    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      throw apiError(body, response);
    }
    stopBrowserHeartbeat();
    setStatus(t("stoppedServer"), "success");
  } catch (error) {
    button.disabled = false;
    setStatus(t("shutdownFailed", { message: error.message || String(error) }), "error");
  }
}

// ---- Keyboard shortcuts (#285, #277) ----
// AyameKeymap owns the chord syntax, the presets and conflict detection; this
// table says what each action's label is. Both the help dialog and the global
// handlers read the resolved bindings, so a remap cannot leave the help
// describing keys that no longer fire. The merge-flow actions (#277) are part of
// the same table, so conflict navigation, side adoption, auto-advance and save
// are remappable like everything else.
const SHORTCUT_LABELS = {
  navigateNext: "shortcutNavigateNext",
  navigatePrev: "shortcutNavigatePrev",
  firstDiff: "shortcutFirst",
  lastDiff: "shortcutLast",
  nextConflict: "shortcutNavigateConflictNext",
  prevConflict: "shortcutNavigateConflictPrev",
  chooseLeft: "shortcutChooseLeft",
  chooseRight: "shortcutChooseRight",
  chooseBase: "shortcutChooseBase",
  chooseBoth: "shortcutChooseBoth",
  toggleAutoAdvance: "shortcutAutoAdvance",
  saveMerge: "shortcutSaveMerge",
  search: "shortcutSearch",
  searchNext: "shortcutSearchNext",
  searchPrev: "shortcutSearchPrev",
  close: "shortcutClose",
  compare: "shortcutCompare",
  revertLine: "shortcutRevertLine",
};
const SHORTCUT_ACTIONS = KEYMAP_ACTION_IDS.map((id) => ({ id, labelKey: SHORTCUT_LABELS[id] }));
const KEYMAP_STORAGE = "ayame-keybindings";
const KEYMAP_DEFAULT_PRESET = "default";

function presetBindings(name) {
  return Object.prototype.hasOwnProperty.call(KEYMAP_PRESETS, name) ? KEYMAP_PRESETS[name] : DEFAULT_BINDINGS;
}

// A stored choice is user data, so a damaged or unknown entry falls back to the
// default keymap instead of taking the whole page down at start-up.
function readKeymapState() {
  try {
    const stored = JSON.parse(localStorage.getItem(KEYMAP_STORAGE) || "null");
    if (stored && typeof stored === "object") {
      return {
        preset: typeof stored.preset === "string" && Object.prototype.hasOwnProperty.call(KEYMAP_PRESETS, stored.preset) ? stored.preset : KEYMAP_DEFAULT_PRESET,
        overrides: stored.overrides && typeof stored.overrides === "object" ? stored.overrides : {},
      };
    }
  } catch (error) { /* fall back below */ }
  return { preset: KEYMAP_DEFAULT_PRESET, overrides: {} };
}

let keymapState = readKeymapState();
let keyBindings = mergeBindings(SHORTCUT_ACTIONS, presetBindings(keymapState.preset), keymapState.overrides);
let capturingAction = null;

function applyKeymapState(state) {
  keymapState = state;
  keyBindings = mergeBindings(SHORTCUT_ACTIONS, presetBindings(keymapState.preset), keymapState.overrides);
  try { localStorage.setItem(KEYMAP_STORAGE, JSON.stringify(keymapState)); }
  catch (error) { /* private mode: the choice still applies for this page */ }
}

// matchesShortcut lets the handlers ask for an action by name instead of
// spelling out a key, which is what keeps the help and the handlers in step.
function matchesShortcut(event, id) {
  if (!event || event.isComposing) return false;
  return eventMatchesChord(event, keyBindings[id]);
}

const CHORD_GLYPHS = { ArrowUp: "↑", ArrowDown: "↓", ArrowLeft: "←", ArrowRight: "→", Enter: "↵", Escape: "Esc", Space: "Space", Tab: "Tab" };
function displayChord(binding) {
  const chord = parseChord(binding);
  if (!chord) return "";
  const parts = [];
  if (chord.ctrl) parts.push("Ctrl");
  if (chord.alt) parts.push("Alt");
  if (chord.shift) parts.push("Shift");
  parts.push(CHORD_GLYPHS[chord.key] || chord.key);
  return parts.join("+");
}

function actionLabel(id) {
  const action = SHORTCUT_ACTIONS.find((entry) => entry.id === id);
  return action ? t(action.labelKey) : id;
}

function showShortcuts() {
  const list = $("shortcutsList");
  list.innerHTML = "";
  for (const action of SHORTCUT_ACTIONS) {
    const term = document.createElement("dt");
    term.textContent = displayChord(keyBindings[action.id]) || t("shortcutUnbound");
    const description = document.createElement("dd");
    description.textContent = t(action.labelKey);
    list.append(term, description);
  }
  const opener = document.activeElement;
  $("shortcutsDialog").addEventListener("close", () => {
    if (opener && typeof opener.focus === "function") opener.focus();
  }, { once: true });
  $("shortcutsDialog").showModal();
}

function keymapStatus(text, isWarning) {
  const status = $("keymapStatus");
  status.textContent = text;
  status.classList.toggle("warning", !!isWarning);
}

function renderKeymapStatus() {
  const conflicts = findConflicts(keyBindings);
  if (!conflicts.length) { keymapStatus(t("shortcutConflictNone"), false); return; }
  keymapStatus(conflicts.map((conflict) => t("shortcutConflict", {
    chord: displayChord(conflict.chord),
    actions: conflict.actions.map(actionLabel).join(", "),
  })).join(" "), true);
}

function renderKeymapEditor() {
  const list = $("keymapList");
  list.innerHTML = "";
  for (const action of SHORTCUT_ACTIONS) {
    const row = document.createElement("div");
    row.className = "keymap-row" + (capturingAction === action.id ? " capturing" : "");
    const label = document.createElement("span");
    label.className = "keymap-label";
    label.textContent = t(action.labelKey);
    const chordButton = document.createElement("button");
    chordButton.type = "button";
    chordButton.className = "keymap-chord";
    const chordText = displayChord(keyBindings[action.id]) || t("shortcutUnbound");
    chordButton.textContent = capturingAction === action.id ? t("shortcutPressKeys") : chordText;
    chordButton.setAttribute("aria-label", `${t(action.labelKey)}: ${chordText}`);
    chordButton.addEventListener("click", () => { capturingAction = action.id; renderKeymapEditor(); });
    const clearButton = document.createElement("button");
    clearButton.type = "button";
    clearButton.className = "keymap-clear";
    clearButton.textContent = t("shortcutClear");
    clearButton.disabled = !keyBindings[action.id];
    clearButton.addEventListener("click", () => setKeyBinding(action.id, null));
    row.append(label, chordButton, clearButton);
    list.append(row);
  }
  renderKeymapStatus();
  $("keymapPreset").value = keymapState.preset;
}

function setKeyBinding(id, chord) {
  if (chord && isReservedChord(chord)) {
    capturingAction = null;
    renderKeymapEditor();
    keymapStatus(t("shortcutReserved", { chord: displayChord(chord) }), true);
    return;
  }
  const overrides = { ...keymapState.overrides, [id]: chord == null || chord === "" ? null : chord };
  applyKeymapState({ preset: keymapState.preset, overrides });
  capturingAction = null;
  renderKeymapEditor();
}

// Capture runs in the capture phase so the key being assigned never reaches the
// handler it would otherwise trigger. Escape and Tab always abandon the capture,
// which is why they cannot be bound from the keyboard.
document.addEventListener("keydown", (event) => {
  if (!capturingAction) return;
  event.preventDefault();
  event.stopPropagation();
  if (event.key === "Escape" || event.key === "Tab") { capturingAction = null; renderKeymapEditor(); return; }
  if (event.isComposing || event.keyCode === 229) return;
  if (["Control", "Alt", "Shift", "Meta", "OS"].includes(event.key)) return;
  const chord = formatChord(chordFromEvent(event));
  if (!chord) return;
  setKeyBinding(capturingAction, chord);
}, { capture: true });

$("shortcutCustomize").addEventListener("click", () => {
  capturingAction = null;
  renderKeymapEditor();
  const opener = document.activeElement;
  $("keymapDialog").addEventListener("close", () => {
    if (opener && typeof opener.focus === "function") opener.focus();
  }, { once: true });
  $("keymapDialog").showModal();
});
$("keymapPreset").addEventListener("change", () => {
  capturingAction = null;
  applyKeymapState({ preset: $("keymapPreset").value, overrides: {} });
  renderKeymapEditor();
});
$("keymapReset").addEventListener("click", () => {
  capturingAction = null;
  applyKeymapState({ preset: keymapState.preset, overrides: {} });
  renderKeymapEditor();
});
$("keymapExport").addEventListener("click", () => {
  const blob = new Blob([serializeBindings(keymapState)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = "ayame-shortcuts.json";
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
  keymapStatus(t("shortcutExportedJSON"), false);
});

// jumpToKind moves to the next hunk of one kind, cycling through them so
// repeated clicks walk the group rather than sticking on the first (#110).
let lastJumpKind = "";
let lastJumpIndex = -1;

function jumpToKind(kind) {
  const hunks = lastData?.hunks || [];
  const matches = [];
  for (let i = 0; i < hunks.length; i++) {
    if (hunks[i].kind === kind && !ignoredHunks.has(i) && !isDowngraded(hunks[i])) matches.push(i);
  }
  if (!matches.length) return;
  if (kind !== lastJumpKind) lastJumpIndex = -1;
  lastJumpKind = kind;
  // Continue from wherever this kind was left, wrapping at the end.
  const next = matches.find((index) => index > lastJumpIndex);
  lastJumpIndex = next === undefined ? matches[0] : next;
  jumpToHunk(lastJumpIndex);
}

// ---- Theme (#106) ----
// Dark mode followed the OS only, so a user whose system is light could not
// read a diff in a dark room, and the choice could not be made per app.
const THEMES = ["system", "light", "dark"];

function applyTheme(value) {
  const theme = THEMES.includes(value) ? value : "system";
  // "system" removes the attribute so the prefers-color-scheme rules apply.
  if (theme === "system") document.documentElement.removeAttribute("data-theme");
  else document.documentElement.setAttribute("data-theme", theme);
  localStorage.setItem("ayame-theme", theme);
  $("theme").value = theme;
}

// ---- CSV cell expansion (#105) ----
// CSV_CELL_PREVIEW_CHARS is the length past which a value is assumed to be cut
// off by the column's max-width. It is deliberately generous: an unnecessary
// affordance costs little, an unreadable value costs a lot.
const CSV_CELL_PREVIEW_CHARS = 60;

function makeCellExpandable(td, value) {
  td.classList.add("expandable");
  td.addEventListener("click", (event) => {
    if (event.target.closest(".csv-cell-copy")) return;
    td.classList.toggle("expanded");
  });
  const copy = document.createElement("button");
  copy.type = "button";
  copy.className = "csv-cell-copy";
  copy.textContent = "⧉";
  copy.title = t("copyValue");
  copy.addEventListener("click", async (event) => {
    // The button lives inside the cell, so without this the copy would also
    // toggle the cell it sits in.
    event.stopPropagation();
    try {
      await navigator.clipboard.writeText(value);
      setStatus(t("copied"), "success");
    } catch {
      // Clipboard access can be refused; selecting the text keeps the value
      // obtainable rather than leaving the button dead.
      const range = document.createRange();
      range.selectNodeContents(td);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      setStatus(t("copyFailed"), "error");
    }
  });
  td.append(copy);
}

// requestAlignWhitespace is the whitespace policy used only for line
// correspondence (#270). The pure web/alignment.js module owns the mapping; the
// fallback keeps an older cached page (without the script) comparing as before.
function requestAlignWhitespace() {
  return globalThis.AyameAlignment?.alignmentWhitespace(whitespaceScaleMode($("whitespaceScale").value), $("alignWhitespace").checked) || "none";
}

function requestBody() {
  const scratch = $("scratch").checked;
  const old = $("old").value.trim();
  const newPath = $("new").value.trim();
  const directoryView = !scratch && $("mode").value === "text"
    && directoryEntryView?.old === old && directoryEntryView?.new === newPath
    ? directoryEntryView
    : null;
  // While editing, the comparison runs on what is in the panes rather than what
  // is on disk — that is what makes typing show up in the diff (#255).
  const editing = editingEnabled();
  return {
    inline: scratch || editing,
    old,
    new: newPath,
    oldAbsent: Boolean(directoryView?.oldAbsent),
    newAbsent: Boolean(directoryView?.newAbsent),
    oldText: editing ? editBufferFor("old").text() : $("oldText").value,
    newText: editing ? editBufferFor("new").text() : $("newText").value,
    mode: $("mode").value,
    encoding: $("encoding").value,
    // A per-side correction made from the result header (#278). Scratch text
    // and the in-pane edit buffer are already Unicode, so it does not apply.
    oldEncoding: scratch || editing ? "" : encodingOverrideFor("old", old),
    newEncoding: scratch || editing ? "" : encodingOverrideFor("new", newPath),
    window: Number($("window").value) || 128,
    maxHunks: Number($("maxHunks").value) || 200,
    maxLines: Number($("maxLines").value) || 200,
    numeric: $("numeric").checked,
    reverse: $("reverse").checked,
    ignoreCase: $("ignoreCase").checked,
	...whitespaceRequestFields($("whitespaceScale").value),
	lineFilters: $("lineFilters").value.split(/\r?\n/).map((value) => value.trim()).filter(Boolean),
    alignWhitespace: requestAlignWhitespace(),
    detectMoves: $("detectMoves").checked,
    moveMinLines: Math.max(1, Number($("moveMinLines").value) || 2),
    syncPoints: syncPoints.map((point) => ({ ...point })),
  };
}

function activeFilters() {
	const filters = [];
	if ($("ignoreCase").checked) filters.push(t("ignoreCase"));
	const whitespaceLevel = $("whitespaceScale").value;
	if (whitespaceLevel !== "strict") filters.push(`${t("whitespaceScale")}: ${t(whitespaceScaleLabelKey(whitespaceLevel))}`);
	if (requestAlignWhitespace() !== "none") filters.push(t("absorbReindent"));
	for (const pattern of $("lineFilters").value.split(/\r?\n/).map((value) => value.trim()).filter(Boolean))
	  filters.push(`/${pattern}/`);
	return filters;
}

// ---- Condition toolbar (#264) ----
// JetBrains keeps the settings that change how a diff is *read* in the viewer's
// toolbar, so the applied policy stays visible while the result is on screen.
// These controls mirror the settings dialog rather than owning a second copy of
// the state: each row derives its label from the same controls the request
// reads, so the bar doubles as a status display. Only the toggles are
// represented here; move min lines, context lines, and the filter definitions
// stay in Settings, which keeps the row count fixed.
function currentConditionPolicy() {
  return readConditionPolicy({
    whitespaceScale: $("whitespaceScale")?.value,
    ignoreCase: $("ignoreCase")?.checked,
    lineFilters: $("lineFilters")?.value,
    detectMoves: $("detectMoves")?.checked,
  });
}

// A condition is hidden when the active mode ignores it (#124). Rather than
// repeat that policy, ask the settings control the form already hid, so the
// toolbar and the dialog can never disagree about what is live.
const CONDITION_SETTING = {
  whitespace: "whitespaceScale", case: "ignoreCase", eol: "whitespaceScale",
  filters: "lineFilters", moves: "detectMoves",
};

function conditionSettingHidden(id) {
  const node = $(CONDITION_SETTING[id]);
  const holder = node?.closest("label") || node;
  return Boolean(holder?.hidden);
}

function syncConditionToolbar() {
  const bar = $("conditionBar");
  if (!bar) return;
  let visible = false;
  for (const row of describeConditions(currentConditionPolicy(), t)) {
    const control = $(row.element);
    if (!control) continue;
    const wrap = control.closest(".condition") || control;
    const dead = conditionSettingHidden(row.id);
    wrap.hidden = dead;
    if (!dead) visible = true;
    if (control.tagName === "SELECT") control.value = row.value;
    if (row.id === "filters") $("tbFiltersValue").textContent = row.text;
    const label = `${row.name}: ${row.text}`;
    wrap.dataset.active = row.active ? "true" : "false";
    control.setAttribute("aria-label", label);
    control.title = label;
  }
  bar.hidden = !visible;
}

// A toolbar change writes the settings control it mirrors and re-runs, replacing
// the current history entry: the same inputs under a different policy are a
// different result, not a new comparison (#254).
function applyConditionToolbar(control, value) {
  const patch = writeConditionPolicy(control, value, currentConditionPolicy());
  for (const [id, next] of Object.entries(patch)) {
    const node = $(id);
    if (!node) continue;
    if (node.type === "checkbox") node.checked = Boolean(next);
    else node.value = String(next);
  }
  if (Object.prototype.hasOwnProperty.call(patch, "detectMoves")) syncMoveMinLines();
  updateDetailsBadges();
  if (hasComparisonResult()) void compare({ urlHistory: "replace" });
}

function validateInputs(body, validateKeys = true) {
	if (body._validationError) { setStatus(body._validationError, "error"); return false; }
	if (validateKeys && body.keyMode === "include" && !(body.keyNames?.length || body.keyIndexes?.length)) { setStatus(t("selectKey"), "error"); return false; }
  if (!body.inline && (!body.old || !body.new)) {
    setStatus(t("enterPaths"), "error");
    return false;
  }
  return true;
}

async function runExportPatch() {
  const body = requestBody();
  if (!validateInputs(body)) return;
  body.patchFormat = $("patchFormat").value;
  body.context = Math.max(0, Number($("patchContext").value) || 0);
  body.ignoredHunks = essentialIndexes(lastData?.hunks || [], [...ignoredHunks].sort((a, b) => a - b));
  $("exportPatch").disabled = true;
  setStatus(t("exporting"), "busy");
  try {
    const resp = await apiFetch("/api/patch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!resp.ok) {
      const data = await resp.json().catch(() => ({}));
      throw apiError(data, resp);
    }
    downloadBlob(await resp.blob(), "ayame.patch");
    setStatus(t("exported"), "success");
  } catch (err) {
    setStatus(String(err.message || err), "error");
  } finally {
    $("exportPatch").disabled = false;
  }
}

// ---- two axes behind one mode (#263) ----
// `#mode` is the flat value the API still takes. The visible controls are
// projections of it: the input shape (what is compared) on the setup side, and
// the reading (how it is read) on the result toolbar. Every path that sets
// `#mode` runs through syncModeOpts, which reflects it back into both controls,
// so the two axes can never drift from the value a request will send.
function readInputShape() {
  return document.querySelector('input[name="inputShape"]:checked')?.value || "two";
}

// readInterpretation clamps the toolbar value to what the shape supports, so a
// reading left over from another shape can never be composed into a mode.
function readInterpretation() {
  const shape = readInputShape();
  const value = $("interpretation").value;
  return interpretationsFor(shape).includes(value) ? value : (interpretationsFor(shape)[0] || "");
}

// syncModeAxes reflects the canonical mode into the two visible controls.
function syncModeAxes() {
  const { shape, interpretation } = decomposeMode($("mode").value);
  const shapeInput = document.querySelector(`input[name="inputShape"][value="${shape}"]`);
  if (shapeInput) shapeInput.checked = true;
  const select = $("interpretation");
  if (!select) return;
  const supported = new Set(interpretationsFor(shape));
  for (const option of select.options) option.disabled = !supported.has(option.value);
  select.value = supported.has(interpretation) ? interpretation : (interpretationsFor(shape)[0] || "text");
  select.disabled = supported.size < 2;
}

// setMode writes the canonical value and keeps the visible axes in step.
function setMode(mode) {
  $("mode").value = mode;
  syncModeAxes();
}

// syncInterpretationVisibility shows the reading control only where a reading
// exists: a folder result compares names and bytes, not a file to interpret.
function syncInterpretationVisibility() {
  const { shape } = decomposeMode($("mode").value);
  const wrap = $("interpretWrap");
  if (wrap) wrap.hidden = interpretationsFor(shape).length === 0;
  syncModeAxes();
}

// changeInputShape recomposes the mode after the user picks a different shape.
// Like the old mode dropdown it only reconfigures the form; the user runs the
// comparison with **Compare**, so picking a shape whose paths are not ready
// does not fire a request that can only fail.
async function changeInputShape() {
  if (editingEnabled() && !(await guardUnsavedEdits())) {
    syncModeAxes();
    await compare();
    return;
  }
  setMode(composeMode(readInputShape(), readInterpretation()));
  syncModeOpts();
  stopFileWatch();
}

// changeInterpretation is the reading axis: it re-runs the comparison with the
// same inputs so a new reading can be tried and reverted after a result exists
// (#263). It is the whole point of moving the axis to the result toolbar.
async function changeInterpretation() {
  if (editingEnabled() && !(await guardUnsavedEdits())) {
    syncModeAxes();
    await compare();
    return;
  }
  setMode(composeMode(readInputShape(), $("interpretation").value));
  syncModeOpts();
  stopFileWatch();
  if (hasComparisonResult()) await compare({ urlHistory: "replace" });
  else scheduleComparisonURLReplace();
}

// downloadBlob hands a server-built attachment to the browser without leaving
// an object URL behind.
function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.append(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

// The confirmation report records what was compared, under which conditions,
// what was ignored or read, and the state needed to rerun it (#296). Confirmed
// hunks (#288) are not part of this build; the server says so in the report
// rather than guessing, so the UI sends only the state it actually has.
async function runExportReport() {
  const body = requestBody();
  if (!validateInputs(body)) return;
  const format = $("reportFormat").value;
  body.format = format;
  body.includeContent = $("reportIncludeContent").checked;
  body.ignoredHunks = [...ignoredHunks].sort((a, b) => a - b);
  body.readHunks = [...readHunks].sort((a, b) => a - b);
  const state = captureComparisonState();
  if (state) {
    body.comparisonState = state;
    try { body.reproduceURL = buildShareURL(location.href, state); } catch (_) { /* URL state too large for a link; the state still travels */ }
  }
  const extensions = { html: "html", markdown: "md", json: "json" };
  $("exportReport").disabled = true;
  setStatus(t("reportGenerating"), "busy");
  try {
    const resp = await apiFetch("/api/report", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!resp.ok) {
      const data = await resp.json().catch(() => ({}));
      throw apiError(data, resp);
    }
    downloadBlob(await resp.blob(), `ayame-report.${extensions[format] || "json"}`);
    setStatus(t("reportExported"), "success");
  } catch (err) {
    setStatus(String(err.message || err), "error");
  } finally {
    $("exportReport").disabled = false;
  }
}

function syncModeOpts() {
  const sorted = $("mode").value === "sorted";
	const csv = $("mode").value === "csv" || $("mode").value === "threeway-csv";
	const threeway = $("mode").value === "threeway" || $("mode").value === "threeway-csv";
	const directory = $("mode").value === "dir";
	const structured = csv || directory;
  $("resultDisplay").hidden = structured || threeway;
  $("numericWrap").hidden = !sorted;
  $("reverseWrap").hidden = !sorted;
	$("csvOptions").hidden = !csv;
	$("exportCSV").hidden = $("mode").value === "threeway-csv";
	$("projectPath").closest(".project-actions").hidden = $("mode").value === "threeway-csv";
	$("basePathRow").hidden = !threeway;
	$("dirOptions").hidden = !directory;
	$("scratch").closest("label").hidden = structured;
	if (structured && $("scratch").checked) { $("scratch").checked = false; applyScratch(); }
	// Pasting text is a two-file text comparison with no shape to choose.
	$("inputShapeGroup").hidden = $("scratch").checked;
	for (const id of ["encoding", "window", "maxHunks", "maxLines", "word", "detectMoves", "moveMinLines", "patchFormat", "patchContext", "wrap", "syntax", "showWs"]) {
		const node = $(id), holder = node?.closest("label") || node;
		if (holder) holder.hidden = structured;
	}
	for (const id of ["patchFormat", "patchContext", "detectMoves", "moveMinLines", "word"]) { const node = $(id), holder = node?.closest("label") || node; if (holder && threeway) holder.hidden = true; }
	// Hide comparison-condition controls the active mode never reads, so a
	// visible toggle always affects the result (#124). Recomputed every call, so
	// switching back to a mode that honors a condition restores its control.
	const dead = new Set(globalThis.AyameModes?.deadCompareConditions($("mode").value) || []);
	for (const id of globalThis.AyameModes?.COMPARE_CONDITIONS || []) {
		const node = $(id), holder = node?.closest("label") || node;
		if (holder) holder.hidden = dead.has(id);
	}
	syncMoveMinLines();
	syncExportPatchVisibility();
	syncInterpretationVisibility();
	syncConditionToolbar();
	if (csv) updateCSVReview();
}

// syncMoveMinLines disables the "move min lines" input while move detection is
// off: the value is meaningless — and ignored by the server — unless
// detectMoves is checked, so a live-looking control there is a false affordance
// (#124).
function syncMoveMinLines() {
	const node = $("moveMinLines");
	if (node) node.disabled = !$("detectMoves").checked;
}

// The patch and report controls live inside the Export menu, so the whole menu
// disappears when neither export is available rather than leaving an empty
// menu title in the bar.
function syncPatchSettingsVisibility() {
  const patchVisible = !$("exportPatch").hidden;
  // Patch format/context only mean something next to Export patch; with the
  // report sharing the menu they must not linger for sorted mode (#296).
  for (const id of ["patchFormat", "patchContext"]) {
    const holder = $(id)?.closest("label");
    if (holder) holder.hidden = !patchVisible;
  }
  const reportElement = $("exportReport");
  const reportVisible = Boolean(reportElement && !reportElement.hidden);
  const menuExport = $("menuExport");
  if (menuExport) {
    menuExport.hidden = !(patchVisible || reportVisible);
    if (menuExport.hidden) menuExport.open = false;
  }
}

// Unified only means anything for the two-way text diff: the three-way view is
// three panes and CSV is a table, neither built from .row. Offering the switch
// there would be a control that visibly does nothing (#124). This rides the
// post-render hook below because every render path already calls it.
// Probing the DOM for a .row would be wrong here: this runs before the hunks
// are appended, and they arrive asynchronously in slices. Go by the same state
// updateMergeUI uses to decide a result is a two-way text diff.
function syncViewModeVisibility() {
  const wrap = $("viewModeWrap");
  if (wrap) wrap.hidden = !(Boolean(lastData?.hunks?.length) && $("mode").value === "text");
}

function syncExportPatchVisibility() {
  // A patch applies only to text mode, but the confirmation report also covers
  // sorted mode, so they have separate visibility over the same freshness check.
  const mode = $("mode").value;
  const comparable = mode === "text" || mode === "sorted";
  const currentRequest = comparable ? JSON.stringify(requestBody()) : null;
  const fresh = Boolean(lastData && lastComparedRequest && currentRequest === lastComparedRequest);
  $("exportPatch").hidden = !(fresh && mode === "text");
  const report = $("exportReport");
  if (report) report.hidden = !fresh;
  syncPatchSettingsVisibility();
  syncViewModeVisibility();
  syncContextVisibility();
  syncEditControls();
}

function syncKeyMode() {
  const disabled = $("keyMode").value === "all";
  document.querySelectorAll("#columnList input").forEach((input) => { input.disabled = disabled; });
  $("selectAllColumns").disabled = disabled; $("invertColumns").disabled = disabled;
  updateCSVReview();
}

// columnFilterIndex caches the column labels and their lowercased text, built
// once per column list rather than re-derived on every keystroke. Reading
// textContent concatenates the subtree and toLocaleLowerCase goes through ICU,
// so the old version allocated two strings per column per keystroke (#154).
let columnFilterIndex = [];

function buildColumnFilterIndex() {
  columnFilterIndex = [...document.querySelectorAll("#columnList .column-choice")]
    .map((label) => ({ label, text: label.textContent.toLocaleLowerCase() }));
}

let columnFilterTimer = 0;

// filterColumns is debounced because each run writes hidden on every label,
// which dirties layout for the whole list.
function filterColumns() {
  clearTimeout(columnFilterTimer);
  columnFilterTimer = setTimeout(applyColumnFilter, 80);
}

function applyColumnFilter() {
  const needle = $("columnSearch").value.toLocaleLowerCase();
  for (const entry of columnFilterIndex) {
    const hidden = !entry.text.includes(needle);
    // Only touch the DOM when the state actually changes.
    if (entry.label.hidden !== hidden) entry.label.hidden = hidden;
  }
}

async function loadBrowser(path) {
  const resp = await apiFetch(`/api/files?path=${encodeURIComponent(path || "")}`), data = await resp.json();
  if (!resp.ok) throw apiError(data, resp);
  const current = data.Path || data.path;
  $("browserPath").value = current; $("browserUp").dataset.path = data.Parent || data.parent;
  browserEntries = (data.Entries || data.entries || []).slice();
  $("browserFilter").value = "";
  const places = rememberPlace(readBrowserPlaces(), current);
  writeBrowserPlaces(places);
  renderBrowserRecent(places);
  renderBrowserEntries();
}

// ---- File browser: keyboard traversal, filtering, recent places (#103) ----
//
// The browser is the main route for someone who does not type paths, so it has
// to be usable without a mouse and without clicking "up" repeatedly. Entries are
// a roving-tabindex list: one tab stop, arrows to move, Enter to open, Backspace
// to go to the parent. Home, the filesystem root and recently opened places are
// one click away.
let browserEntries = [];
let browserFocus = -1;

const BROWSER_PLACES_KEY = "ayame-browser-places";

function readBrowserPlaces() {
  try {
    const parsed = JSON.parse(localStorage.getItem(BROWSER_PLACES_KEY) || "[]");
    return Array.isArray(parsed) ? parsed.filter((item) => typeof item === "string") : [];
  } catch (err) {
    return [];
  }
}

function writeBrowserPlaces(places) {
  try { localStorage.setItem(BROWSER_PLACES_KEY, JSON.stringify(places)); } catch (err) { /* storage may be unavailable */ }
}

function renderBrowserRecent(places) {
  const select = $("browserRecent");
  if (!select) return;
  select.textContent = "";
  const first = document.createElement("option");
  first.value = "";
  first.textContent = places.length ? t("browserRecent") : t("browserRecentEmpty");
  select.append(first);
  for (const path of places) {
    const option = document.createElement("option");
    option.value = path;
    option.textContent = path;
    select.append(option);
  }
}

function browserVisibleEntries() {
  const needle = $("browserFilter").value.trim().toLocaleLowerCase();
  if (!needle) return browserEntries;
  return browserEntries.filter((item) => String(item.Name || item.name || "").toLocaleLowerCase().includes(needle));
}

function renderBrowserEntries() {
  const entries = $("browserEntries"); entries.innerHTML = "";
  const items = browserVisibleEntries();
  for (const item of items) {
    const button = document.createElement("button"); button.type = "button"; button.className = item.directory ? "directory" : "file";
    button.setAttribute("role", "option");
    const itemPath = item.Path || item.path;
    const label = document.createElement("span"); label.className = "browser-entry-name";
    label.textContent = `${item.directory ? "📁" : "📄"} ${item.Name || item.name}`;
    button.append(label);
    if (!item.directory) {
      const meta = document.createElement("span"); meta.className = "browser-entry-meta";
      const size = formatBytes(Number(item.Size ?? item.size));
      const stamp = formatEpochNanos(item.Modified || item.modified);
      meta.textContent = [size, stamp].filter(Boolean).join(" · ");
      button.append(meta);
    }
    button.dataset.browserPath = itemPath;
    button.addEventListener("click", async () => {
      if (item.directory) await loadBrowser(itemPath);
      else await selectBrowserPath(itemPath);
    });
    entries.append(button);
  }
  browserFocus = items.length ? 0 : -1;
  syncBrowserFocus(false);
}

// A roving tabindex keeps one tab stop in the list while the arrows move a
// visible focus, so Tab leaves the browser instead of walking every entry.
function syncBrowserFocus(move = true) {
  const rows = [...$("browserEntries").children];
  rows.forEach((row, index) => { row.tabIndex = index === browserFocus ? 0 : -1; });
  if (move && rows[browserFocus]) rows[browserFocus].focus();
}

function stepBrowserFocus(delta) {
  const rows = [...$("browserEntries").children];
  if (!rows.length) return;
  browserFocus = Math.max(0, Math.min(rows.length - 1, browserFocus + delta));
  syncBrowserFocus();
}

async function browserGoParent() {
  const parent = $("browserUp").dataset.path;
  if (parent) await loadBrowser(parent);
}


async function selectBrowserPath(path) {
  if (!browserTarget) return;
  const target = browserTarget;
  const afterSelect = browserAfterSelect;
  browserTarget = null;
  browserAfterSelect = null;
  $(target).value = path;
  csvInspection = null;
  $("fileBrowser").close();
  updateCSVReview();
  syncCompareReady();
  updateSetupSummary();
  if (afterSelect) await afterSelect(path);
}

async function openBrowser(target, afterSelect = null) {
  browserTarget = target;
  browserAfterSelect = afterSelect;
  $("fileBrowser").showModal();
	$("chooseFolder").hidden = $("mode").value !== "dir";
  try { await loadBrowser($(target).value ? $(target).value.replace(/[\\/][^\\/]*$/, "") : ""); }
  catch (err) { setStatus(String(err.message || err), "error"); }
}
function syncPatchOpts() {
  $("patchContextWrap").hidden = $("patchFormat").value === "normal";
}

// ---- Setup collapse ----
// Expanded, the form filled the window and left the diff below the fold: the
// tool was unusable for the thing it exists for. A comparison means the inputs
// have done their job, so the form folds to one line carrying the two names and
// a re-run. It never folds on a failure, where the inputs are what needs fixing.
function setSetupCompact(compact) {
  const setup = $("setup");
  setup.classList.toggle("compact", compact);
  $("setupToggle").setAttribute("aria-expanded", String(!compact));
}
function baseName(path) {
  const cleaned = String(path || "").replace(/[\\/]+$/, "");
  const cut = Math.max(cleaned.lastIndexOf("/"), cleaned.lastIndexOf("\\"));
  return cut >= 0 ? cleaned.slice(cut + 1) : cleaned;
}
function updateSetupSummary() {
  const summary = $("setupSummary");
  const scratch = $("scratch").checked;
  const left = scratch ? t("scratch") : baseName($("old").value);
  const right = scratch ? t("scratch") : baseName($("new").value);
  if (!left && !right) { summary.hidden = true; return; }
  summary.innerHTML = "";
  const a = document.createElement("span"); a.className = "side"; a.textContent = left;
  const b = document.createElement("span"); b.className = "side"; b.textContent = right;
  summary.append(a, " ⇄ ", b);
  summary.title = `${$("old").value}\n${$("new").value}`;
  summary.hidden = false;
}
// ---- Difference index (sidebar) ----
// Reaching a particular difference among thirty meant stepping through them or
// scrolling. This lists them by kind and line and jumps on click, and marks the
// current one so the list doubles as a position indicator.
function buildSidebar(data) {
  const list = $("sidebarList");
  list.innerHTML = "";
  const hunks = data?.hunks || [];
  $("sidebarToggle").hidden = hunks.length === 0;
  if (!hunks.length) { $("sidebar").hidden = true; return; }
  hunks.forEach((hunk, index) => {
    const item = document.createElement("li");
    const button = document.createElement("button");
    button.type = "button";
    button.className = `sidebar-item ${hunk.kind}`;
    button.dataset.hunk = String(index);
    if (isDowngraded(hunk)) {
      button.classList.add("downgraded");
      button.title = t("downgradedHint");
    }
    const mark = document.createElement("span");
    mark.className = "sidebar-mark"; mark.textContent = isDowngraded(hunk) ? "≈" : (hunkMarker(hunk.kind) || "~");
    mark.setAttribute("aria-hidden", "true");
    const line = document.createElement("span");
    line.className = "sidebar-line"; line.textContent = String(hunk.new_start + 1);
    button.append(mark, line);
    button.setAttribute("aria-label", `${t(hunkKindKey(hunk.kind))} ${hunk.new_start + 1}`);
    button.addEventListener("click", () => jumpToHunk(index));
    item.append(button);
    list.append(item);
  });
  markSidebarCurrent();
}
function markSidebarCurrent() {
  for (const button of $("sidebarList").querySelectorAll(".sidebar-item")) {
    const index = Number(button.dataset.hunk);
    button.classList.toggle("current", index === currentHunk);
    button.classList.toggle("read", readHunks.has(index));
    button.classList.toggle("confirmed", isConfirmed(index));
    button.classList.toggle("ignored", ignoredHunks.has(index));
  }
}

// ---- Path history ----
// WinMerge puts an MRU dropdown on every path field, and it is the affordance
// that makes re-running a comparison cheap. The existing recent list only ever
// recorded CSV and project flows and lived inside a collapsed group, so an
// ordinary text comparison left no trace. This keeps one list per side.
const PATH_HISTORY_KEY = "ayame-recent-paths";
const PATH_HISTORY_MAX = 12;
function pathHistory() {
  try {
    const value = JSON.parse(localStorage.getItem(PATH_HISTORY_KEY) || "{}");
    return value && typeof value === "object" ? value : {};
  } catch (_) { return {}; }
}
function renderPathHistory() {
  const store = pathHistory();
  for (const side of ["old", "new", "base"]) {
    const list = $(`${side}History`);
    if (!list) continue;
    list.innerHTML = "";
    for (const path of store[side] || []) {
      const option = document.createElement("option");
      option.value = path;
      list.append(option);
    }
  }
}
function rememberPaths() {
  if ($("scratch").checked) return;  // pasted text has no path to remember
  const store = pathHistory();
  for (const side of ["old", "new", "base"]) {
    const value = $(side)?.value.trim();
    if (!value) continue;
    // Most-recent-first, and a repeat moves to the top rather than duplicating.
    const rest = (store[side] || []).filter((item) => item !== value);
    store[side] = [value, ...rest].slice(0, PATH_HISTORY_MAX);
  }
  try { localStorage.setItem(PATH_HISTORY_KEY, JSON.stringify(store)); } catch (_) { /* storage full or blocked */ }
  renderPathHistory();
}

// syncCompareReady mirrors WinMerge's Select-Files screen: Compare stays
// disabled until the inputs make sense, and a line under the form says which
// one is wrong. Refusing up front beats accepting the click and returning an
// error, because the answer is always "fix the path you already typed".
function syncCompareReady() {
  const scratch = $("scratch").checked;
  const mode = $("mode").value;
  const needsBase = mode === "threeway" || mode === "threeway-csv";
  const missing = [];
  if (scratch) {
    if (!$("oldText").value && !$("newText").value) missing.push(`${t("sideLeft")} / ${t("sideRight")}`);
  } else {
    if (!$("old").value.trim()) missing.push(t("sideLeft"));
    if (!$("new").value.trim()) missing.push(t("sideRight"));
    if (needsBase && !$("base").value.trim()) missing.push(t("sideBase"));
  }
  const ready = missing.length === 0;
  $("compare").disabled = !ready;
  const note = $("setupNote");
  if (note) {
    note.textContent = ready ? "" : t("needPaths", { fields: missing.join(" / ") });
    note.hidden = ready;
  }
  return ready;
}

// The status bar answers "what am I looking at" without spending a row of the
// result on it. It carries the two sides; the counts live beside it and stay
// clickable jump targets (#110).
function updateStatusBar() {
  const bar = $("statusbar");
  const paths = $("statusPaths");
  const scratch = $("scratch").checked;
  const left = scratch ? t("scratch") : $("old").value;
  const right = scratch ? t("scratch") : $("new").value;
  if (!left && !right) { bar.hidden = true; return; }
  paths.innerHTML = "";
  const a = document.createElement("span"); a.className = "side"; a.textContent = baseName(left) || left;
  const b = document.createElement("span"); b.className = "side"; b.textContent = baseName(right) || right;
  paths.append(a, " ⇄ ", b);
  paths.title = `${left}\n${right}`;
  bar.hidden = false;
}

function syncLaunchPathsVisibility() {
  const hasResult = Boolean(lastData || csvData || threeWayData || directoryData);
  $("paths").hidden = $("scratch").checked || hasResult;
}

function collapseSetupAfterCompare() {
  updateStatusBar();
  updateSetupSummary();
  $("setupRecompare").hidden = false;
  syncLaunchPathsVisibility();
  syncCopyComparisonURLVisibility();
  setSetupCompact(true);
}

// Display preferences (color scheme + line wrap), persisted across visits.
function applyScheme(v) {
  document.documentElement.setAttribute("data-scheme", v === "default" ? "" : v);
  localStorage.setItem("ayame-scheme", v);
  $("scheme").value = v;
  // A custom theme is one more choice beside default and colorblind: switching
  // to it paints the saved overrides, switching away clears them again (#286).
  applyActiveTheme(v === THEME_CUSTOM_SCHEME);
}

// ---- Theme customization (#286) ----
// The token catalogue, the merge rules and the contrast maths live in theme.js
// and are tested under node --test; this is only the wiring that turns them
// into a dialog and applies the result to :root.
const THEME_GROUP_KEYS = { ground: "themeGroupGround", accent: "themeGroupAccent", diff: "themeGroupDiff", type: "themeGroupType" };
let activeTheme = loadActiveTheme(localStorage);
let themeDraft = null;

// A "system" base is resolved from the OS before the contrast check, so the
// preview and the warning agree about which ground is actually behind a wash.
function resolvedThemeBase(theme) {
  if (theme.base === "system") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  return theme.base;
}

function draftEffectiveTokens(theme) {
  return effectiveTokens({ base: resolvedThemeBase(theme), tokens: theme.tokens });
}

function clearAppliedThemeTokens() {
  for (const key of TOKEN_KEYS) document.documentElement.style.removeProperty(key);
}

function applyThemeTokens(theme) {
  clearAppliedThemeTokens();
  for (const key of Object.keys(theme.tokens)) document.documentElement.style.setProperty(key, theme.tokens[key]);
}

function currentThemeBase() {
  const value = $("theme").value;
  return value === "dark" ? "dark" : value === "system" ? "system" : "light";
}

function applyActiveTheme(on) {
  if (!on) { clearAppliedThemeTokens(); return; }
  if (!activeTheme) {
    activeTheme = createTheme("Custom", currentThemeBase(), {});
    saveActiveTheme(localStorage, activeTheme);
  }
  applyTheme(activeTheme.base);
  applyThemeTokens(activeTheme);
}

function commitThemeDraft() {
  activeTheme = normalizeTheme(themeDraft);
  saveActiveTheme(localStorage, activeTheme);
  applyTheme(activeTheme.base);
  applyThemeTokens(activeTheme);
  renderThemeContrast();
}

function renderThemePresetOptions() {
  const select = $("themePreset");
  select.textContent = "";
  const none = document.createElement("option");
  none.value = "";
  none.textContent = "\u2014";
  select.append(none);
  for (const preset of presetList()) {
    const option = document.createElement("option");
    option.value = preset.id;
    option.textContent = preset.name;
    select.append(option);
  }
}

function renderThemeTokens() {
  const host = $("themeTokens");
  host.textContent = "";
  const effective = draftEffectiveTokens(themeDraft);
  for (const group of TOKEN_GROUPS) {
    const section = document.createElement("details");
    section.className = "theme-group";
    section.open = true;
    const summary = document.createElement("summary");
    summary.textContent = t(THEME_GROUP_KEYS[group.id] || group.id);
    section.append(summary);
    const grid = document.createElement("div");
    grid.className = "theme-token-grid";
    for (const token of group.tokens) {
      const row = document.createElement("label");
      row.className = "theme-token";
      const name = document.createElement("span");
      name.className = "theme-token-name";
      name.textContent = token.key;
      name.setAttribute("title", token.key);
      const text = document.createElement("input");
      text.type = "text";
      text.spellcheck = false;
      text.maxLength = 200;
      text.setAttribute("aria-label", token.key);
      text.value = themeDraft.tokens[token.key] || "";
      text.placeholder = effective[token.key] || "";
      row.append(name, text);
      let swatch = null;
      if (token.kind === "color") {
        swatch = document.createElement("input");
        swatch.type = "color";
        swatch.className = "theme-swatch";
        swatch.setAttribute("aria-label", token.key);
        swatch.value = formatColor(resolveColor(effective[token.key], effective)) || "#000000";
        swatch.addEventListener("input", () => {
          text.value = swatch.value;
          themeDraft.tokens[token.key] = swatch.value;
          commitThemeDraft();
        });
        row.append(swatch);
      }
      text.addEventListener("input", () => {
        const value = text.value.trim();
        if (value) themeDraft.tokens[token.key] = value;
        else delete themeDraft.tokens[token.key];
        if (swatch) {
          const current = draftEffectiveTokens(themeDraft);
          swatch.value = formatColor(resolveColor(current[token.key], current)) || swatch.value;
        }
        commitThemeDraft();
      });
      grid.append(row);
    }
    section.append(grid);
    host.append(section);
  }
}

function renderThemeContrast() {
  const host = $("themeContrast");
  host.textContent = "";
  const rows = checkContrast(draftEffectiveTokens(themeDraft));
  for (const row of rows) {
    // A pair the resolver cannot follow (an advanced color-mix the editor
    // allows through) is reported as unknown, not as a pass.
    const state = row.ratio === null ? "unknown" : row.aa ? "pass" : "fail";
    const line = document.createElement("div");
    line.className = `theme-contrast-row ${state}`;
    const pair = document.createElement("span");
    pair.className = "theme-contrast-pair";
    pair.textContent = `${row.fg} / ${row.bg}`;
    const value = document.createElement("span");
    value.className = "theme-contrast-value";
    value.textContent = row.ratio === null ? "\u2014" : `${row.ratio}:1`;
    const status = document.createElement("span");
    status.className = "theme-contrast-status";
    status.textContent = row.ratio === null ? t("themeContrastUnknown") : row.aa ? t("themeContrastPass") : t("themeContrastFail");
    line.append(pair, value, status);
    host.append(line);
  }
}

function renderSavedThemes() {
  const host = $("themeSavedList");
  host.textContent = "";
  const list = loadThemeList(localStorage);
  const names = Object.keys(list).sort();
  if (!names.length) {
    const empty = document.createElement("p");
    empty.className = "theme-saved-empty";
    empty.textContent = t("themeNoneSaved");
    host.append(empty);
    return;
  }
  for (const name of names) {
    const row = document.createElement("div");
    row.className = "theme-saved-row";
    const label = document.createElement("span");
    label.className = "theme-saved-name";
    label.textContent = name;
    const apply = document.createElement("button");
    apply.type = "button";
    apply.textContent = t("themeApply");
    apply.addEventListener("click", () => loadThemeIntoDraft(list[name]));
    const remove = document.createElement("button");
    remove.type = "button";
    remove.textContent = t("themeDelete");
    remove.addEventListener("click", () => {
      deleteNamedTheme(localStorage, name);
      renderSavedThemes();
      setStatus(t("themeDeleted"), "success");
    });
    row.append(label, apply, remove);
    host.append(row);
  }
}

function loadThemeIntoDraft(theme) {
  themeDraft = normalizeTheme(theme);
  $("themeName").value = themeDraft.name;
  $("themeBase").value = themeDraft.base;
  $("themePreset").value = "";
  renderThemeTokens();
  commitThemeDraft();
}

function loadThemePreset(id) {
  const preset = id ? presetById(id) : null;
  if (!preset) return;
  themeDraft = preset;
  $("themeName").value = themeDraft.name;
  $("themeBase").value = themeDraft.base;
  renderThemeTokens();
  commitThemeDraft();
}

function openThemeEditor() {
  themeDraft = activeTheme ? normalizeTheme(activeTheme) : createTheme("Custom", currentThemeBase(), {});
  activeTheme = themeDraft;
  saveActiveTheme(localStorage, activeTheme);
  // Live preview means the open diff is the sample, so selecting the custom
  // scheme is part of opening the editor rather than a separate step.
  applyScheme(THEME_CUSTOM_SCHEME);
  $("themeName").value = themeDraft.name;
  $("themeBase").value = themeDraft.base;
  renderThemePresetOptions();
  $("themePreset").value = "";
  $("themeJSON").value = "";
  renderThemeTokens();
  renderThemeContrast();
  renderSavedThemes();
  const dialog = $("themeDialog");
  if (dialog.open) dialog.close();
  dialog.showModal();
}

function saveDraftTheme() {
  const name = $("themeName").value.trim();
  if (!name) { setStatus(t("themeNameRequired"), "error"); return; }
  themeDraft.name = name;
  activeTheme = saveNamedTheme(localStorage, themeDraft);
  saveActiveTheme(localStorage, activeTheme);
  renderSavedThemes();
  setStatus(t("themeSaved"), "success");
}

function resetThemeToPreset() {
  const id = $("themePreset").value || (resolvedThemeBase(themeDraft) === "dark" ? "ayame-dark" : "ayame-light");
  $("themePreset").value = id;
  loadThemePreset(id);
}

async function exportThemeDraft() {
  const text = serializeTheme(themeDraft);
  $("themeJSON").value = text;
  try {
    await navigator.clipboard.writeText(text);
    setStatus(t("themeExported"), "success");
  } catch {
    setStatus(t("themeExportFailed"), "warning");
  }
}

function importThemeDraft() {
  const result = parseTheme($("themeJSON").value);
  if (!result.ok) { setStatus(t("themeImportFailed"), "error"); return; }
  loadThemeIntoDraft(result.theme);
  setStatus(t("themeImported"), "success");
}

function applyWrap(on) {
  const scrollAnchor = captureResultScrollAnchor();
  $("result").classList.toggle("nowrap", !on);
  localStorage.setItem("ayame-wrap", on ? "1" : "0");
  $("wrap").checked = on;
  document.querySelector(".csv-table")?.classList.toggle("wrap-cells", on);
  restoreResultScrollAnchor(scrollAnchor);
  refreshMinimapGeometry();
}
// Display width settings (#289). The tab size is a CSS token so raw tabs render
// at the chosen width; the East Asian Ambiguous choice feeds the width model
// used for whitespace markers. Both are read by the CLI's internal/textwidth
// too, so the two halves agree on what a line occupies.
function applyTabSize(size) {
  const value = normalizeTabSize(size);
  document.documentElement.style.setProperty("--tab-size", String(value));
  localStorage.setItem("ayame-tab-size", String(value));
  $("tabSize").value = String(value);
}
function applyAmbiguousWide(on) {
  localStorage.setItem("ayame-ambiguous-wide", on ? "1" : "0");
  $("ambiguousWide").checked = on;
}
// applyViewMode switches between side-by-side and the unified, git-style single
// column (#115). Nothing is re-rendered: a changed row already carries both
// cells, so the layout is entirely a CSS concern and every other feature
// (word highlight, merge selection, the minimap, navigation) keeps working.
function applyViewMode(mode) {
  const scrollAnchor = captureResultScrollAnchor();
  const unified = mode === "unified";
  $("result").classList.toggle("unified", unified);
  localStorage.setItem("ayame-view", unified ? "unified" : "side");
  $("viewMode").value = unified ? "unified" : "side";
  restoreResultScrollAnchor(scrollAnchor);
  refreshMinimapGeometry();
}
function applyDisplayPreferences() {
  const result = $("result");
  result.classList.toggle("show-whitespace", $("showWs").checked);
  result.classList.toggle("syntax-highlight", $("syntax").checked);
  result.classList.toggle("word-highlight", $("word").checked);
}

// rerenderForDisplayChange redraws the current result after a toggle that
// changes which nodes are built. Whitespace markers and word-diff spans are no
// longer created when their option is off (#127), so a class flip alone can no
// longer reveal them.
let displayRenderGeneration = 0;
async function rerenderForDisplayChange() {
  const generation = ++displayRenderGeneration;
  const scrollAnchor = captureResultScrollAnchor();
  applyDisplayPreferences();
  if (threeWayData) await renderThreeWay(threeWayData, threeWayData.csvMode);
  else if (lastData) await renderResult(lastData);
  if (generation === displayRenderGeneration) restoreResultScrollAnchor(scrollAnchor, true);
}

function droppedPaths(dataTransfer) {
  const uriList = dataTransfer.getData("text/uri-list");
  const fromURIs = uriList.split(/\r?\n/).filter((line) => line && !line.startsWith("#")).map((line) => {
    try {
      const value = new URL(line);
      if (value.protocol !== "file:") return "";
      let path = decodeURIComponent(value.pathname);
      if (/^\/[A-Za-z]:\//.test(path)) path = path.slice(1);
      return path;
    } catch (_) { return ""; }
  }).filter(Boolean);
  if (fromURIs.length) return fromURIs;
  return [...dataTransfer.files].map((file) => file.path || "").filter(Boolean);
}

async function uploadDrop(file, session, relative, directory = false) {
  const query = new URLSearchParams({ session, relative });
  if (directory) query.set("directory", "1");
  const response = await apiFetch(`/api/drop?${query}`, { method: "POST", body: directory ? new Blob([]) : file });
  const data = await response.json();
  if (!response.ok) throw apiError(data, response);
  return data.path;
}

function entryFile(entry) { return new Promise((resolve, reject) => entry.file(resolve, reject)); }
async function readDirectory(reader) {
  const all = [];
  for (;;) {
    const batch = await new Promise((resolve, reject) => reader.readEntries(resolve, reject));
    if (!batch.length) return all;
    all.push(...batch);
  }
}
async function uploadEntry(entry, session, relative) {
  if (entry.isFile) return uploadDrop(await entryFile(entry), session, relative);
  const root = await uploadDrop(null, session, relative, true);
  for (const child of await readDirectory(entry.createReader())) await uploadEntry(child, session, `${relative}/${child.name}`);
  return root;
}

async function droppedItems(dataTransfer) {
  const native = droppedPaths(dataTransfer);
  if (native.length) return native;
  const session = crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`;
  const entries = [...dataTransfer.items].map((item) => item.webkitGetAsEntry?.()).filter(Boolean).slice(0, 2);
  if (entries.length) {
    const paths = [];
    for (const entry of entries) paths.push(await uploadEntry(entry, session, entry.name));
    return paths;
  }
  const paths = [];
  for (const file of [...dataTransfer.files].slice(0, 2)) paths.push(await uploadDrop(file, session, file.name));
  return paths;
}

async function setDroppedPaths(paths) {
  if (!paths.length) return;
  if (paths.length >= 2) {
    $("old").value = paths[0]; $("new").value = paths[1];
  } else if (!$("old").value) $("old").value = paths[0];
  else $("new").value = paths[0];
  csvInspection = null;
  if ($("old").value && $("new").value) {
    try {
      const info = await Promise.all(["old", "new"].map(async (id) => {
        const response = await apiFetch(`/api/path-info?path=${encodeURIComponent($(id).value)}`);
        return response.ok ? response.json() : null;
      }));
      $("mode").value = info.every((item) => item?.directory) ? "dir" : "text";
    } catch (_) { $("mode").value = "text"; }
    syncModeOpts();
    await compare();
  }
}

let dragDepth = 0;
document.addEventListener("dragenter", (event) => { event.preventDefault(); dragDepth++; document.body.classList.add("drag-active"); });
document.addEventListener("dragover", (event) => { event.preventDefault(); event.dataTransfer.dropEffect = "copy"; });
document.addEventListener("dragleave", (event) => { event.preventDefault(); if (--dragDepth <= 0) { dragDepth = 0; document.body.classList.remove("drag-active"); } });
document.addEventListener("drop", async (event) => {
  event.preventDefault(); dragDepth = 0; document.body.classList.remove("drag-active");
  try { await setDroppedPaths((await droppedItems(event.dataTransfer)).slice(0, 2)); }
  catch (err) { setStatus(String(err.message || err), "error"); }
});

$("compare").addEventListener("click", compare);
// Changing the shared encoding retires any per-side corrections made from a
// result header, so the next run starts from the shared setting again (#278).
$("encoding").addEventListener("change", clearEncodingOverrides);
$("newTab").addEventListener("click", openTab);
$("setupToggle").addEventListener("click", () => setSetupCompact(!$("setup").classList.contains("compact")));
$("openSettings").addEventListener("click", () => $("settingsDialog").showModal());
$("makeConditionsDefault").addEventListener("click", makeConditionsDefault);
$("resetConditionsDefault").addEventListener("click", resetConditionsToDefault);
$("backToFolder").addEventListener("click", returnToFolder);
$("copyComparisonURL").addEventListener("click", copyComparisonURL);
// Every input that decides whether a comparison is possible re-checks it.
for (const id of ["old", "new", "base", "oldText", "newText", "mode", "scratch"]) {
  const node = $(id);
  if (!node) continue;
  node.addEventListener("input", syncCompareReady);
  node.addEventListener("change", syncCompareReady);
  node.addEventListener("input", stopFileWatch);
  node.addEventListener("change", stopFileWatch);
}
$("autoReload").checked = localStorage.getItem(AUTO_RELOAD_KEY) !== "0";
$("autoReload").addEventListener("change", () => {
  localStorage.setItem(AUTO_RELOAD_KEY, $("autoReload").checked ? "1" : "0");
  if ($("autoReload").checked) armFileWatchFromCurrentState();
  else stopFileWatch();
});
$("externalReload").addEventListener("click", async () => {
  const change = pendingExternalChange;
  if (!change) return;
  // Reload is an explicit discard decision. The future direct editor listens
  // for this event to clear its own model before the comparison is replaced.
  document.dispatchEvent(new CustomEvent("ayame:discard-unsaved-changes"));
  document.body.dataset.unsavedChanges = "false";
  await reloadExternalChange(change);
});
$("externalKeep").addEventListener("click", () => {
  const change = pendingExternalChange;
  pendingExternalChange = null;
  hideExternalChangeBar();
  if (change && $("autoReload").checked &&
      sameWatchPaths(change.paths, currentWatchPaths())) {
    fileWatcher.start(change.paths, change.snapshot);
  }
});
window.addEventListener("pagehide", () => {
  stopFileWatch();
  releaseBrowserSession();
});
window.addEventListener("pageshow", startBrowserSession);
syncCompareReady();
renderPathHistory();
$("sidebarToggle").addEventListener("click", () => {
  const sidebar = $("sidebar");
  sidebar.hidden = !sidebar.hidden;
  $("sidebarToggle").setAttribute("aria-pressed", String(!sidebar.hidden));
  localStorage.setItem("ayame-sidebar", sidebar.hidden ? "0" : "1");
});
$("setupRecompare").addEventListener("click", compare);
for (const id of ["old", "new", "base", "scratch"]) $(id)?.addEventListener("change", updateSetupSummary);
// A menu should close when the pointer goes elsewhere; <details> alone keeps it
// open until its own summary is clicked again.
document.addEventListener("click", (event) => {
  for (const menu of document.querySelectorAll(".menubar .menu[open]")) {
    if (!menu.contains(event.target)) menu.open = false;
  }
});
document.addEventListener("keydown", (event) => {
  if (event.key !== "Escape") return;
  for (const menu of document.querySelectorAll(".menubar .menu[open]")) menu.open = false;
});
// A hunk toolbar pinned open from its handle closes the same way a menu does:
// a click elsewhere, or Escape.
document.addEventListener("click", (event) => {
  for (const box of document.querySelectorAll(".hunk.toolbar-open")) {
    if (!box.contains(event.target)) setHunkToolbarOpen(box, false);
  }
});
document.addEventListener("keydown", (event) => {
  if (event.key !== "Escape") return;
  for (const box of document.querySelectorAll(".hunk.toolbar-open")) setHunkToolbarOpen(box, false);
});
$("exportPatch").addEventListener("click", exportPatch);
$("exportReport").addEventListener("click", exportReport);
$("inspectCSV").addEventListener("click", inspectCSV);
$("csvFilterBuilder").addEventListener("click", onFilterBuilderClick);
$("csvFilterBuilder").addEventListener("input", syncFilterState);
$("csvFilterBuilder").addEventListener("change", syncFilterState);
$("filterPreview").addEventListener("click", () => void runFilterPreview());
$("exportCSV").addEventListener("click", exportCSV);
$("saveProject").addEventListener("click", saveProject);
$("loadProject").addEventListener("click", loadProject);
$("recentProjects").addEventListener("change", async () => { if ($("recentProjects").value !== "") { const body = recentComparisons()[Number($("recentProjects").value)]; if (body.mode === "dir") applyDirectoryProject(body); else await applyCSVProject(body); } });
$("cancel").addEventListener("click", cancelCurrentOperation);
// The two axes (#263). A refused switch restores the controls from the
// canonical mode rather than from a remembered value, so the radios and the
// select always agree with what a request would send. Nothing is lost either
// way — the buffers only die when the user says so.
for (const input of document.querySelectorAll('input[name="inputShape"]')) {
  input.addEventListener("change", changeInputShape);
}
$("interpretation").addEventListener("change", changeInterpretation);
$("detectMoves").addEventListener("change", syncMoveMinLines);
$("setup").addEventListener("input", syncExportPatchVisibility);
$("setup").addEventListener("change", syncExportPatchVisibility);
$("keyMode").addEventListener("change", syncKeyMode);
$("columnSearch").addEventListener("input", filterColumns);
$("selectAllColumns").addEventListener("click", () => { document.querySelectorAll("#columnList .column-choice:not([hidden]) input").forEach((input) => { input.checked = true; }); updateCSVReview(); });
$("invertColumns").addEventListener("click", () => { document.querySelectorAll("#columnList .column-choice:not([hidden]) input").forEach((input) => { input.checked = !input.checked; }); updateCSVReview(); });
$("changedColumnsOnly").addEventListener("change", () => {
  if (!csvData || !csvView) return;
  const scrollAnchor = captureResultScrollAnchor();
  csvPage = 0;
  // Only the columns change: the summary, the pane headers and the pager are
  // the same result they were describing a moment ago (#154).
  renderCSVColumns();
  restoreResultScrollAnchor(scrollAnchor, true);
});
document.querySelectorAll(".browse").forEach((button) => button.addEventListener("click", () => openBrowser(button.dataset.target)));
$("browserGo").addEventListener("click", async () => { try { await loadBrowser($("browserPath").value); } catch (err) { setStatus(String(err.message || err), "error"); } });
$("browserUp").addEventListener("click", async () => { try { await loadBrowser($("browserUp").dataset.path); } catch (err) { setStatus(String(err.message || err), "error"); } });
$("chooseFolder").addEventListener("click", () => selectBrowserPath($("browserPath").value));
$("dirStatus").addEventListener("change", async () => {
  if (!directoryData) return;
  if (continuousActive()) {
    await renderContinuous(directoryData, directoryBody);
    return;
  }
  const state = captureFolderTreeState();
  await renderDirectory(directoryData, directoryBody, state);
  restoreResultScrollAnchor(state.anchor, true);
});
$("dirSearch").addEventListener("input", () => {
  clearTimeout(directorySearchTimer);
  directorySearchTimer = setTimeout(async () => {
    if (!directoryData || $("mode").value !== "dir") return;
    if (continuousActive()) {
      await renderContinuous(directoryData, directoryBody);
      return;
    }
    const state = captureFolderTreeState();
    await renderDirectory(directoryData, directoryBody, state);
    restoreResultScrollAnchor(state.anchor, true);
  }, 120);
});
$("dirPreview").addEventListener("click", previewDirectoryFilter);
$("saveDirProject").addEventListener("click", saveDirectoryProject);
$("loadDirProject").addEventListener("click", loadDirectoryProject);
$("browserPath").addEventListener("keydown", (event) => { if (event.key === "Enter") { event.preventDefault(); $("browserGo").click(); } });

// The browser's keyboard model (#103). Entries are reached with the arrows,
// opened with Enter, and the parent with Backspace, so a path is reachable
// without a pointer. The filter narrows the list as it is typed.
$("browserFilter").addEventListener("input", () => renderBrowserEntries());
$("browserEntries").addEventListener("keydown", (event) => {
  const rows = [...$("browserEntries").children];
  if (event.key === "ArrowDown") { stepBrowserFocus(1); event.preventDefault(); }
  else if (event.key === "ArrowUp") { stepBrowserFocus(-1); event.preventDefault(); }
  else if (event.key === "Home") { browserFocus = rows.length ? 0 : -1; syncBrowserFocus(); event.preventDefault(); }
  else if (event.key === "End") { browserFocus = rows.length - 1; syncBrowserFocus(); event.preventDefault(); }
  else if (event.key === "Backspace") {
    // Backspace on a focused entry means "up", the same as the parent button.
    browserGoParent().catch((err) => setStatus(String(err.message || err), "error"));
    event.preventDefault();
  }
});
$("browserHome").addEventListener("click", async () => { try { await loadBrowser("~"); } catch (err) { setStatus(String(err.message || err), "error"); } });
$("browserRoot").addEventListener("click", async () => { try { await loadBrowser("/"); } catch (err) { setStatus(String(err.message || err), "error"); } });
$("browserRecent").addEventListener("change", async () => {
  const path = $("browserRecent").value;
  if (!path) return;
  try { await loadBrowser(path); } catch (err) { setStatus(String(err.message || err), "error"); }
});
function compareFromKeyboard(event) {
  if (event.key !== "Enter" || event.isComposing || event.keyCode === 229) return;
  if (event.currentTarget.tagName === "TEXTAREA" && !event.ctrlKey && !event.metaKey) return;
  event.preventDefault();
  if (!$("compare").disabled) compare();
}
for (const id of ["base", "old", "new", "oldText", "newText"]) {
  $(id).addEventListener("keydown", compareFromKeyboard);
}
$("patchFormat").addEventListener("change", syncPatchOpts);
$("firstDiff").addEventListener("click", () => {
  if (continuousActive()) { void continuousStep(1, { edge: "first" }); return; }
  const active = activeHunkIndexes(); if (active.length) jumpToHunk(active[0]);
});
$("prevDiff").addEventListener("click", () => {
  if (continuousActive()) { void continuousStep(-1); return; }
  stepHunk(-1);
});
$("nextDiff").addEventListener("click", () => {
  if (continuousActive()) { void continuousStep(1); return; }
  stepHunk(1);
});
$("lastDiff").addEventListener("click", () => {
  if (continuousActive()) { void continuousStep(1, { edge: "last" }); return; }
  const active = activeHunkIndexes(); if (active.length) jumpToHunk(active[active.length - 1]);
});
$("prevUnconfirmed").addEventListener("click", () => stepUnconfirmed(-1));
$("nextUnconfirmed").addEventListener("click", () => stepUnconfirmed(1));
$("dirContinuous").addEventListener("click", () => void toggleContinuousView());
// Flattening only changes how the same result is drawn, so it re-renders the
// existing data and keeps the reader's place instead of re-comparing (#275).
$("dirFlat").addEventListener("click", async () => {
  if (!directoryData || !directoryBody || continuousActive()) return;
  const state = captureFolderTreeState();
  applyDirFlat(!dirFlatActive());
  await renderDirectory(directoryData, directoryBody, state);
});
$("addSync").addEventListener("click", addSyncPoint);
$("clearSync").addEventListener("click", clearSyncPoints);
// The units "All left / All right / All base" apply to: every hunk of the
// active comparison, conflicts only for three-way. Dismissed (downgraded) text
// hunks are not real differences, so they are not merge units (#269).
function mergeUnitIds() {
  if (threeWayData && ($("mode").value === "threeway" || $("mode").value === "threeway-csv")) return threeWayData.events.filter((item) => item.kind === "conflict").map((item) => item.id);
  if ($("mode").value === "csv" && csvData) return csvData.differences.map((item) => item.id);
  return (lastData?.hunks || []).map((_, index) => index).filter((index) => !isDowngraded(lastData.hunks[index]));
}
function chooseAllMerge(side) {
  return () => {
    if (!mergeSelection.order.includes(side)) return;
    const ids = mergeUnitIds();
    mutateMerge(() => { for (const id of ids) mergeSelection.choose(id, side); });
  };
}
$("allLeft").addEventListener("click", chooseAllMerge("left"));
$("allRight").addEventListener("click", chooseAllMerge("right"));
$("allBase").addEventListener("click", chooseAllMerge("base"));
$("mergeMode").addEventListener("click", () => { setMergeMode(!mergeMode); updateMergeUI(); });
$("mergeUndo").addEventListener("click", undoMerge);
$("mergeRedo").addEventListener("click", redoMerge);
$("toggleBase").addEventListener("click", () => setThreeWayBase(!threeWayShowBase));
$("saveMerge").addEventListener("click", async () => { if (await askMergeOutput()) await saveMergeResult(); });
$("simulateMerge").addEventListener("click", async () => { if (await askMergeOutput()) await previewMergeImpact(); });
$("mergeOutput").addEventListener("keydown", (event) => {
  if (event.key !== "Enter" || event.isComposing) return;
  event.preventDefault();
  $("mergeSaveDialog").close("ok");
});
// The preview is a demand-driven server call, so it is fetched when opened and
// refreshed on choice changes, never on load (#257).
$("mergeProvenance").addEventListener("toggle", () => { if ($("mergeProvenance").open) void refreshMergePreview(); });
// Auto-advance is a merge-flow preference (#277), so its control follows the
// resolved keymap actions rather than a fixed chord.
const autoAdvanceToggle = $("mergeAutoAdvance");
if (autoAdvanceToggle) autoAdvanceToggle.addEventListener("change", () => setMergeAutoAdvance(autoAdvanceToggle.checked));
$("navHelp").addEventListener("click", showShortcuts);
document.addEventListener("keydown", (event) => {
  if (!lastData?.hunks?.length) return;
  const active = activeHunkIndexes();
  // Save the merge result. Ctrl+S stays with the editable panes, so the merge
  // writer owns Ctrl+Shift+S; both are only useful when a merge is on screen.
  // The output path comes from the same dialog the Save button uses.
  if (matchesShortcut(event, "saveMerge") && mergePanelVisible() && !$("saveMerge").disabled) {
    event.preventDefault();
    void (async () => { if (await askMergeOutput()) await saveMergeResult(); })();
    return;
  }
  if (matchesShortcut(event, "toggleAutoAdvance") && mergePanelVisible()) {
    event.preventDefault();
    setMergeAutoAdvance(!mergeAutoAdvance);
    return;
  }
  // Conflict navigation is separate from difference navigation (#277) and only
  // means anything once a three-way result has conflicts to resolve.
  if (threeWayData && matchesShortcut(event, "nextConflict")) { event.preventDefault(); stepConflict(1); return; }
  if (threeWayData && matchesShortcut(event, "prevConflict")) { event.preventDefault(); stepConflict(-1); return; }
  const chooseLeft = matchesShortcut(event, "chooseLeft");
  const chooseRight = matchesShortcut(event, "chooseRight");
  const chooseBase = Boolean(threeWayData) && matchesShortcut(event, "chooseBase");
  const chooseBoth = Boolean(threeWayData) && matchesShortcut(event, "chooseBoth");
  if (chooseLeft || chooseRight || chooseBase || chooseBoth) {
    // A three-way result mixes conflicts with left-only/right-only/auto-merged
    // events; a side is only meaningful for a conflict, so adoption stays on
    // that subset even though difference navigation now visits every event.
    const pool = threeWayData ? activeConflictIndexes() : active;
    if (!pool.length) return;
    event.preventDefault();
    const index = pool.includes(currentHunk) ? currentHunk : pool[0];
    const key = threeWayData?.events?.[index]?.id ?? index;
    const side = chooseLeft ? "left" : chooseRight ? "right" : chooseBase ? "base" : "both";
    chooseMerge(key, side);
    return;
  }
  if (matchesShortcut(event, "navigateNext")) { event.preventDefault(); stepHunk(1); return; }
  if (matchesShortcut(event, "navigatePrev")) { event.preventDefault(); stepHunk(-1); return; }
  let target = null;
  if (matchesShortcut(event, "firstDiff")) target = active[0];
  else if (matchesShortcut(event, "lastDiff")) target = active[active.length - 1];
  if (target != null) {
    event.preventDefault();
    jumpToHunk(target);
  }
});
let viewportFrame = 0;
function scheduleMinimapViewport() {
  if (viewportFrame) return;
  viewportFrame = requestAnimationFrame(() => { viewportFrame = 0; updateMinimapViewport(); });
}
function refreshMinimapGeometry() {
  if (minimapHasMarkers && lastData) buildMinimap(lastData);
  updateMinimapViewport();
}
$("result").addEventListener("scroll", scheduleMinimapViewport, { passive: true });
$("result").addEventListener("scroll", trackContinuousScroll, { passive: true });
// Throttled like scroll above: a resize fires continuously while dragging, and
// each call forces a layout read (#155).
let minimapResizeTimer = 0;
window.addEventListener("resize", () => {
  scheduleMinimapViewport();
  clearTimeout(minimapResizeTimer);
  minimapResizeTimer = setTimeout(refreshMinimapGeometry, 120);
});

let minimapPointerId = null;
let minimapGrabOffset = 0;
function moveMinimapPointer(pointerY) {
  const mapRect = $("minimap").getBoundingClientRect();
  const viewportRect = $("minimapViewport").getBoundingClientRect();
  const result = $("result");
  result.scrollTop = scrollTopForMinimapPointer({
    pointerY,
    trackTop: mapRect.top,
    trackHeight: mapRect.height,
    viewportHeight: viewportRect.height,
    grabOffset: minimapGrabOffset,
    scrollHeight: result.scrollHeight,
    clientHeight: result.clientHeight,
  });
  updateMinimapViewport();
}
function finishMinimapDrag(event) {
  if (event.pointerId !== minimapPointerId) return;
  const map = $("minimap");
  if (map.hasPointerCapture?.(event.pointerId)) map.releasePointerCapture(event.pointerId);
  minimapPointerId = null;
  map.classList.remove("dragging");
}
$("minimap").addEventListener("pointerdown", (event) => {
  if (event.button !== 0 || event.target.closest(".minimap-marker")) return;
  const viewport = $("minimapViewport");
  const viewportRect = viewport.getBoundingClientRect();
  minimapGrabOffset = event.target.closest(".minimap-viewport")
    ? Math.max(0, Math.min(viewportRect.height, event.clientY - viewportRect.top))
    : viewportRect.height / 2;
  minimapPointerId = event.pointerId;
  $("minimap").setPointerCapture?.(event.pointerId);
  $("minimap").classList.add("dragging");
  viewport.focus({ preventScroll: true });
  event.preventDefault();
  moveMinimapPointer(event.clientY);
});
$("minimap").addEventListener("pointermove", (event) => {
  if (event.pointerId !== minimapPointerId) return;
  event.preventDefault();
  moveMinimapPointer(event.clientY);
});
$("minimap").addEventListener("pointerup", finishMinimapDrag);
$("minimap").addEventListener("pointercancel", finishMinimapDrag);
$("minimap").addEventListener("lostpointercapture", finishMinimapDrag);
$("minimapViewport").addEventListener("keydown", (event) => {
  const result = $("result");
  const maximum = Math.max(0, result.scrollHeight - result.clientHeight);
  let next = result.scrollTop;
  if (event.key === "ArrowUp") next -= Math.max(40, result.clientHeight * 0.1);
  else if (event.key === "ArrowDown") next += Math.max(40, result.clientHeight * 0.1);
  else if (event.key === "PageUp") next -= result.clientHeight * 0.9;
  else if (event.key === "PageDown") next += result.clientHeight * 0.9;
  else if (event.key === "Home") next = 0;
  else if (event.key === "End") next = maximum;
  else return;
  event.preventDefault();
  result.scrollTop = Math.max(0, Math.min(maximum, next));
  updateMinimapViewport();
});
// In-result search (#118). Ctrl+F is intercepted only when there is a result to
// search; otherwise the browser's own find is left alone. The chords come from
// the keymap so the help cannot describe a search key that no longer fires.
document.addEventListener("keydown", (event) => {
  if (matchesShortcut(event, "search") && $("result").children.length) {
    event.preventDefault();
    openSearch();
    return;
  }
  if (matchesShortcut(event, "close") && searchOpen()) {
    event.preventDefault();
    closeSearch();
    return;
  }
  if ((matchesShortcut(event, "searchNext") || matchesShortcut(event, "searchPrev")) && searchOpen() && document.activeElement === $("searchInput")) {
    event.preventDefault();
    stepSearch(matchesShortcut(event, "searchPrev") ? -1 : 1);
  }
});
// Run the comparison from anywhere with the bound chord. The setup inputs keep
// their own Enter handling (compareFromKeyboard), so they are left alone here to
// avoid running the comparison twice.
document.addEventListener("keydown", (event) => {
  if (event.isComposing || event.keyCode === 229) return;
  if (!matchesShortcut(event, "compare")) return;
  const target = event.target;
  if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA")) return;
  if ($("compare").disabled) return;
  event.preventDefault();
  compare();
});
// Escape dismisses the focused message (#97), so the lane can be cleared
// without reaching for the mouse.
$("messages").addEventListener("keydown", (event) => {
  if (event.key !== "Escape") return;
  const item = event.target.closest?.(".message");
  if (!item) return;
  event.preventDefault();
  dismissMessage(Number(item.dataset.messageId));
});
$("editMode").addEventListener("click", () => void toggleEditMode());
// Ctrl+S saves the pane being edited. With edits on both sides and no editor
// focused, the left one goes first, and a second press saves the right.
// Ctrl+Shift+S is the merge writer (#277), so Shift must not fall through here.
document.addEventListener("keydown", (event) => {
  if (!editingEnabled()) return;
  if (!(event.ctrlKey || event.metaKey) || event.shiftKey || event.key.toLowerCase() !== "s") return;
  event.preventDefault();
  const focused = lineEditor?.side || document.activeElement?.closest?.(".pane-head")?.classList.contains("new") && "new";
  const target = focused || editedSides()[0];
  if (target) void saveEditedPane(target);
});
// Unsaved lines live only in this tab, so leaving has to say so. The browser
// shows its own wording; what matters is that the prompt appears at all.
window.addEventListener("beforeunload", (event) => {
  if (!editedSides().length) return;
  event.preventDefault();
  event.returnValue = "";
});
$("searchInput").addEventListener("input", scheduleSearch);
for (const id of ["searchCase", "searchRegex", "searchChangedOnly"]) $(id).addEventListener("change", runSearch);
$("searchNext").addEventListener("click", () => stepSearch(1));
$("searchPrev").addEventListener("click", () => stepSearch(-1));
$("searchClose").addEventListener("click", closeSearch);
const savedContextValue = localStorage.getItem("ayame-context-lines");
const savedContextLines = savedContextValue === null ? NaN : Number(savedContextValue);
$("contextLines").value = String(Number.isFinite(savedContextLines)
  ? Math.max(0, Math.min(CONTEXT_MAX_LINES, Math.trunc(savedContextLines)))
  : 3);
setContextVisibility(localStorage.getItem("ayame-context-visible") !== "0", false);
$("contextToggle").addEventListener("click", () => setContextVisibility(!contextIsVisible()));
captureControlDefaults($("resultDisplay"));
$("contextLines").addEventListener("input", updateDetailsBadges);
$("contextLines").addEventListener("change", () => {
  const count = contextLineCount();
  $("contextLines").value = String(count);
  localStorage.setItem("ayame-context-lines", String(count));
  updateDetailsBadges();
  resetContextRanges();
});
captureControlDefaults($("compareConditions"));
for (const control of $("compareConditions").querySelectorAll("input, select, textarea")) {
  control.addEventListener("change", updateDetailsBadges);
  control.addEventListener("input", updateDetailsBadges);
}
// #264: the condition toolbar mirrors these controls, so a change here has to
// re-run or the policy it shows would drift from the policy the result was
// built under. Only a committed change re-runs, so typing a filter does not
// start a comparison per keystroke; `input` still only refreshes the badges.
let conditionRecompareTimer = 0;
function scheduleConditionRecompare() {
  if (!hasComparisonResult()) return;
  clearTimeout(conditionRecompareTimer);
  conditionRecompareTimer = setTimeout(() => {
    conditionRecompareTimer = 0;
    void compare({ urlHistory: "replace" });
  }, 120);
}
for (const id of ["ignoreCase", "whitespaceScale", "lineFilters", "detectMoves"]) {
  $(id).addEventListener("change", scheduleConditionRecompare);
}
captureControlDefaults($("engineTuning"));
for (const control of $("engineTuning").querySelectorAll("input, select")) {
  control.addEventListener("change", updateDetailsBadges);
  control.addEventListener("input", updateDetailsBadges);
}
captureControlDefaults($("csvOptions"));
for (const control of $("csvOptions").querySelectorAll("input, select")) {
  control.addEventListener("change", updateDetailsBadges);
  control.addEventListener("input", updateDetailsBadges);
}
// The per-comparison defaults are captured here, before any URL restore or
// autorun can touch the controls, so a comparison with no memory can reset to
// them (#260).
captureConditionDefaults();
updateDetailsBadges();
$("theme").addEventListener("change", () => {
  applyTheme($("theme").value);
  // When a custom theme is active, the theme menu is its base, so keep the
  // stored theme in step with the choice (#286).
  if ($("scheme").value === THEME_CUSTOM_SCHEME && activeTheme) {
    activeTheme.base = currentThemeBase();
    saveActiveTheme(localStorage, activeTheme);
  }
});
$("scheme").addEventListener("change", () => applyScheme($("scheme").value));
$("themeCustomize").addEventListener("click", openThemeEditor);
$("themePreset").addEventListener("change", () => loadThemePreset($("themePreset").value));
$("themeBase").addEventListener("change", () => {
  themeDraft.base = $("themeBase").value;
  renderThemeTokens();
  commitThemeDraft();
});
$("themeName").addEventListener("input", () => { themeDraft.name = $("themeName").value; });
$("themeSave").addEventListener("click", saveDraftTheme);
$("themeReset").addEventListener("click", resetThemeToPreset);
$("themeExport").addEventListener("click", exportThemeDraft);
$("themeImport").addEventListener("click", importThemeDraft);
$("wrap").addEventListener("change", () => applyWrap($("wrap").checked));
$("viewMode").addEventListener("change", () => applyViewMode($("viewMode").value));
// The condition toolbar mirrors the settings controls; changing a row writes
// the control it owns and re-runs (#264). The filter row has no state of its
// own — the definitions live in the dialog — so it opens settings instead.
for (const [element, control] of [["tbWhitespace", "whitespace"], ["tbCase", "case"], ["tbEol", "eol"], ["tbMoves", "moves"]]) {
  $(element).addEventListener("change", () => applyConditionToolbar(control, $(element).value));
}
$("tbFilters").addEventListener("click", () => {
  const dialog = $("settingsDialog");
  if (!dialog.open) dialog.showModal();
  $("compareConditions").open = true;
  $("lineFilters").focus();
});
$("showWs").addEventListener("change", () => {
  localStorage.setItem("ayame-showws", $("showWs").checked ? "1" : "0");
  rerenderForDisplayChange();
});
$("tabSize").addEventListener("change", () => {
  applyTabSize($("tabSize").value);
  rerenderForDisplayChange();
});
$("ambiguousWide").addEventListener("change", () => {
  applyAmbiguousWide($("ambiguousWide").checked);
  rerenderForDisplayChange();
});
$("syntax").addEventListener("change", () => {
  localStorage.setItem("ayame-syntax", $("syntax").checked ? "1" : "0");
  const scrollAnchor = captureResultScrollAnchor();
  applyDisplayPreferences();
  restoreResultScrollAnchor(scrollAnchor);
});
$("word").addEventListener("change", rerenderForDisplayChange);
// #260: word highlight was the one display toggle that reset on every reload.
// It joins the global display preferences, not the per-comparison conditions.
$("word").addEventListener("change", () => {
  localStorage.setItem("ayame-word", $("word").checked ? "1" : "0");
});
for (const input of document.querySelectorAll("#csvOptions input, #csvOptions select")) input.addEventListener("change", updateCSVReview);
$("buildColumnMap").addEventListener("click", buildColumnMap);
$("addColumnMapRow").addEventListener("click", () => {
  columnMap.push({ left: COLUMN_ABSENT, right: COLUMN_ABSENT, ignore: false });
  columnMapDirty = true;
  renderColumnMap();
  updateCSVReview();
});
$("clearColumnMap").addEventListener("click", () => {
  columnMap = [];
  columnMapDirty = true;
  renderColumnMap();
  updateCSVReview();
});
$("applyColumnMap").addEventListener("click", applyColumnMap);
for (const id of URL_STATE_CONTROL_IDS) {
  const control = $(id);
  if (!control) continue;
  control.addEventListener("input", scheduleComparisonURLReplace);
  control.addEventListener("change", scheduleComparisonURLReplace);
}
for (const id of ["base", "old", "new", "hasHeader", "alignColumns", "leftFormat", "rightFormat", "leftParser", "rightParser", "leftDelimiter", "rightDelimiter", "lazyQuotes", "trimLeadingSpace"]) {
	$(id).addEventListener("change", () => { csvInspection = null; $("inspection").textContent = ""; $("keySetup").hidden = true; renderColumnMap(); });
}
function applyScratch() {
  const on = $("scratch").checked;
  syncLaunchPathsVisibility();
  $("scratchArea").hidden = !on;
  // Pasting text is a two-file comparison; there is no shape to pick.
  $("inputShapeGroup").hidden = on;
  syncCopyComparisonURLVisibility();
}
$("scratch").addEventListener("change", async () => {
  if (editingEnabled() && !(await guardUnsavedEdits())) {
    $("scratch").checked = !$("scratch").checked;
    applyScratch();
    await compare();
    return;
  }
  applyScratch();
});
applyScratch();
applyScheme(localStorage.getItem("ayame-scheme") || "default");
applyTheme(localStorage.getItem("ayame-theme") || "system");
applyWrap(localStorage.getItem("ayame-wrap") !== "0");
applyViewMode(localStorage.getItem("ayame-view") || "side");
applyTabSize(localStorage.getItem("ayame-tab-size") || 8);
applyAmbiguousWide(localStorage.getItem("ayame-ambiguous-wide") === "1");
$("showWs").checked = localStorage.getItem("ayame-showws") === "1";
$("syntax").checked = localStorage.getItem("ayame-syntax") !== "0";
$("word").checked = localStorage.getItem("ayame-word") !== "0";
applyDisplayPreferences();
applyDirFlat(localStorage.getItem("ayame-dirflat") === "1", false);
$("lang").addEventListener("change", () => { applyLang($("lang").value); renderColumnMap(); });
$("stopServer").addEventListener("click", stopServer);
syncModeOpts();
syncPatchOpts();
applyLang(lang);
renderColumnMap();
startBrowserSession();

const launch = new URLSearchParams(location.search);
const launchTabs = parseTabDoc(readTabState(location.href));
if (launchTabs) { tabDoc = launchTabs; renderTabs(); }
const launchState = readComparisonState(location.href);
if (comparisonURLHasState()) {
  if (launchState) {
    queueMicrotask(() => {
      restoreComparisonFromURL(launchState).catch(() => setStatus(t("urlStateInvalid"), "warning"));
    });
  } else {
    setStatus(t("urlStateInvalid"), "warning");
  }
} else {
  // Backward-compatible command launch parameters. A successful autorun
  // replaces these with the versioned fragment, leaving only the API token and
  // unrelated query parameters in the live URL.
  if (launch.has("base")) $("base").value = launch.get("base");
  if (launch.has("old")) $("old").value = launch.get("old");
  if (launch.has("new")) $("new").value = launch.get("new");
  if (launch.has("output")) $("mergeOutput").value = launch.get("output");
  launchLabels = {
    base: launch.get("baseLabel") || "",
    old: launch.get("oldLabel") || "",
    new: launch.get("newLabel") || "",
  };
  if (URL_STATE_MODES.has(launch.get("mode"))) setMode(launch.get("mode"));
  if (launch.has("base") || launch.has("old") || launch.has("new")) csvInspection = null;
  // Reflect whichever mode arrived (or the default) into both axes.
  syncModeOpts();
  syncCompareReady();
  syncLaunchPathsVisibility();
  const launchReady = $("old").value && $("new").value && (!$("basePathRow").hidden ? $("base").value : true);
  if (launch.get("autorun") === "1" && launchReady) {
    queueMicrotask(() => compare({ urlHistory: "replace" }));
  }
}

window.addEventListener("popstate", () => {
  clearTimeout(comparisonURLReplaceTimer);
  comparisonURLReplaceTimer = 0;
  // Each history entry carries its own tab set, so Back returns to the tabs
  // that belonged with that comparison (#281).
  const entryTabs = parseTabDoc(readTabState(location.href));
  if (entryTabs) { tabDoc = entryTabs; renderTabs(); }
  const state = readComparisonState(location.href);
  if (!state) {
    comparisonURLRestoreGeneration++;
    cancelCurrentOperation();
    if (comparisonURLHasState()) setStatus(t("urlStateInvalid"), "warning");
    else location.reload();
    return;
  }
  restoreComparisonFromURL(state).catch(() => setStatus(t("urlStateInvalid"), "warning"));
});

apiFetch("/api/health")
  .then((r) => r.json())
  .then((d) => { if (d.version) $("version").textContent = d.version; })
  .catch(() => {});
