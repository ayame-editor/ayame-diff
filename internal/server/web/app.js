"use strict";

const $ = (id) => document.getElementById(id);

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

// ---- i18n (JA/EN) ----
const I18N = {
  ja: {
    mode: "モード", encoding: "文字コード", window: "ウィンドウ",
    maxHunks: "最大ハンク数", maxLines: "ハンクあたり最大行",
    word: "ワードハイライト", numeric: "数値", reverse: "逆順", compare: "比較",
	ignoreCase: "大小無視", whitespace: "空白", ignoreEOL: "改行コード無視",
	ignoreTrailingEOL: "末尾改行無視", lineFilters: "行フィルタ", activeFilters: "適用中",
	lineFiltersPlaceholder: "1行に1つの正規表現",
	cancel: "キャンセル",
    cancelled: "キャンセルしました", scheme: "配色", wrap: "折り返し",
    syntax: "シンタックスハイライト", showWs: "空白表示", scratch: "テキスト貼り付け",
    patchFormat: "patch形式", patchContext: "patch文脈行", exportPatch: "patchを書き出す",
    exporting: "patch生成中…", exported: "patchを書き出しました",
    diffCounter: (v) => `差分 ${v.current} / ${v.total}（未読 ${v.unread}）`,
    differenceNavigation: "差分ナビゲーション",
    firstDiff: "最初の差分",
    previousDiff: "前の差分",
    nextDiff: "次の差分",
    lastDiff: "最後の差分",
    navHelpText: "差分移動: Alt+↓/↑、採用: Alt+← 左 / Alt+→ 右 / Alt+B ベース、Alt+Home/End、検索: Ctrl+F（Enter/Shift+Enter で次/前、Esc で閉じる）",
    keyboardShortcuts: "キーボードショートカット", differenceMap: "差分マップ",
    detectMoves: "移動ブロック検出", moveMinLines: "移動の最小行数", moved: "移動",
    addSync: "同期点を追加", clearSync: "同期点を全削除", syncPoints: "同期点",
    ignoreHunk: "この差分を無視", restoreHunk: "無視を解除", ignored: "無視",
    syncSelect: "左右から対応させる行を1行ずつ選択してください。",
    syncOrderError: "同期点は左右とも昇順になるよう選択してください。",
    hunks: "ハンク", added: "追加", deleted: "削除", modified: "変更",
    // mode select + folder status-filter options, translated so JA follows (#125)
    modeText: "テキスト", modeSorted: "ソート済み", modeCsv: "CSV / TSV",
    modeFolder: "フォルダ", modeThreeway: "3-way テキスト", modeThreewayCsv: "3-way CSV",
    statusDifferent: "差分あり", statusAll: "すべて",
    // summary labels shared by the CSV / folder / 3-way renderers (#125)
    changed: "変更", removed: "削除", same: "一致", left: "左", right: "右",
    leftOnly: "左のみ", rightOnly: "右のみ", equalRows: "一致", bytes: "バイト",
    // detected encoding surfaced on file diffs (#130)
    encodingDetected: (v) => `文字コード: OLD=${v.old} / NEW=${v.new}`,
    encodingMismatch: "左右で文字コードが異なります",
    omitted: (n) => `（${n} ハンク省略。最大ハンク数を上げてください）`,
    moveDetectionSkipped: "ハンクが省略されたため、移動検出は実施されませんでした。",
    theme: "テーマ", themeSystem: "OS に合わせる", themeLight: "ライト", themeDark: "ダーク",
    jumpToKind: (v) => `${v.label}の差分へ移動`,
    confirmTitle: "確認", confirmProceed: "実行", cancel: "キャンセル", close: "閉じる",
    shortcutNavigate: "次/前の差分へ移動", shortcutFirstLast: "最初/最後の差分へ",
    shortcutChooseSide: "左/右を採用", shortcutChooseBase: "ベースを採用",
    shortcutSearch: "結果内を検索", shortcutSearchStep: "次/前の一致へ",
    shortcutClose: "検索・ダイアログを閉じる", shortcutCompare: "比較を実行",
    swap: "入れ替え", swapSides: "OLD と NEW を入れ替え",
    engineTuning: "エンジン調整", engineTuningHint: "差分の計算量と表示量に効きます（何を差分と見なすかは変わりません）。結果が打ち切られたときだけ上げてください。",
    csvParsingOptions: "ファイル形式", csvProjectOptions: "保存した比較",
    csvParsingHint: "自動判定されます。読み取りがおかしいときだけ設定してください。",
    csvAdvancedHint: "大きい/遅い比較向けの調整です。ほとんどのファイルは既定のままで動きます。",
    changedSettings: (v) => `${v.count} 件変更`,
    searchResults: "結果内を検索", searchPlaceholder: "結果内を検索", searchCase: "大小区別",
    searchRegex: "正規表現", searchChangedOnly: "変更行のみ", searchPrevious: "前の一致", searchNext: "次の一致",
    searchClose: "検索を閉じる", searchNoMatches: "一致なし",
    searchCounter: (v) => `${v.current} / ${v.total}${v.capped} 件`,
    comparing: "比較中…", rendering: (v) => `描画中… ${v.done} / ${v.total} ハンク`, noDiff: "差分はありません。",
    completeMatch: "✔ 完全一致", filteredMatch: "✔ 比較条件適用後に一致",
    textMatchScope: (v) => `OLD ${v.old} 行 / NEW ${v.new} 行を比較、差分 0`,
    csvMatchScope: (v) => `${v.rows} 行 / ${v.columns} 列を比較、差分 0`,
    threeWayTextMatchScope: (v) => `3入力・${v.lines} 行を比較、差分 0`,
    threeWayCSVMatchScope: (v) => `3入力・${v.columns} 列を比較、差分 0`,
    matchNotVerified: "⚠ 完全一致を確認できません",
    requiredField: (v) => `${v.field}を指定してください。`,
    requiredFields: (v) => `${v.fields.join("、")}を指定してください。`,
    invalidIndex: (v) => `${v.field}のインデックスが不正です。`,
    projectPath: "プロジェクトパス",
    emptyTitle: "比較を始める",
    emptySteps: "1. OLDを指定 → 2. NEWを指定 → 3. 比較",
    emptyDrop: "ファイルをこの画面へドロップしても比較できます。",
    enterPaths: "OLD と NEW のパスを入力してください。",
	csvSetup: "CSV / TSV セットアップ", inspect: "ヘッダー検査", leftFormat: "左形式", rightFormat: "右形式",
	leftParser: "左パーサー", rightParser: "右パーサー", leftDelimiter: "左区切り", rightDelimiter: "右区切り",
	hasHeader: "ヘッダーあり", alignColumns: "列名で整列", lazyQuotes: "不正引用を許可", trimLeadingSpace: "先頭空白を除去",
	keyMode: "キーモード", allColumns: "全列", includeKeys: "選択列をキー", excludeKeys: "選択列をキーから除外",
	searchColumns: "列を検索", selectAll: "全選択", invert: "反転", csvCompareOptions: "比較・性能設定",
	ignoreColumns: "比較から無視する列", tolerance: "数値許容差", columnTolerances: "列別許容差", maxRows: "最大表示行",
	memory: "メモリ", tempDir: "一時ディレクトリ", partitions: "パーティション", parseWorkers: "入力リーダー", workers: "ワーカー",
	mergeFanIn: "マージ fan-in", partitionBuffer: "分割バッファ", maxRecordBytes: "最大レコード", keepTemp: "一時ファイルを保持",
	changedColumnsOnly: "変更列だけ表示", outputPath: "出力パス", outputFormat: "出力形式", outputHeader: "ヘッダーを出力",
	review: "設定レビュー", runExport: "実行して書き出す", chooseFile: "ファイルを選択", open: "開く",
	browseFile: "ファイルを参照", close: "閉じる", parentFolder: "親フォルダ",
	inspectionDone: (v) => `${v.column_count} 列 — 左 ${v.left_format}/${v.left_parser}、右 ${v.right_format}/${v.right_parser}`,
	csvNoDiff: "CSV 差分はありません。", csvTruncated: "表示上限に達しました。全件は書き出しを使用してください。",
	selectKey: "キー列を1つ以上選択してください。",
	previousPage: "前のページ", nextPage: "次のページ", pageInput: (v) => `ページ番号（全 ${v.total} ページ）`,
	pageTotal: (v) => `/ ${v.total} ページ`, exportedCSV: (v) => `${v} に書き出しました`,
	openProject: "プロジェクトを開く", saveProject: "プロジェクト保存", recent: "最近の比較", projectSaved: "プロジェクトを保存しました",
	mergeMode: "マージモード", mergeResult: "マージ結果", chooseLeft: "左を採用", chooseRight: "右を採用", chooseBase: "ベースを採用", allLeft: "すべて左", allRight: "すべて右", allBase: "すべてベース",
	threeWay: "3-way 比較", conflicts: "競合", autoMerged: "自動マージ",
	undo: "元に戻す", redo: "やり直す", unresolved: (n) => `未解決 ${n}`, overwriteInput: "入力を上書き", saveMerge: "マージ保存",
	mergeSaved: (v) => `${v} にマージ結果を保存しました`, unresolvedWarning: (n) => `${n} 件が未解決です。未解決箇所は左を残して保存しますか？`, overwriteWarning: "入力ファイルを上書きします。元に戻せません。続行しますか？",
	folderSetup: "フォルダ比較", includes: "include glob", excludes: "exclude glob", hiddenFiles: "隠しファイル", quickCompare: "サイズ + mtime を信頼", statusFilter: "状態", symlinkPolicy: "シンボリックリンクはスキップ。.gz は展開内容を比較します。", chooseFolder: "このフォルダを選択",
	filterExpression: "フィルタ式", filterFile: "フィルタファイル", filterSet: "フィルタセット", compareBy: "比較方法", filterPreview: "フィルタをプレビュー", filterPreviewResult: (v) => `左 ${v.old_count} / 右 ${v.new_count} / 合計 ${v.union_count}`,
    langButton: "日本語 → EN",
    langSwitchLabel: "言語を英語に切り替え",
  },
  en: {
    mode: "mode", encoding: "encoding", window: "window", maxHunks: "max hunks",
    maxLines: "max lines/hunk", word: "word highlight", numeric: "numeric",
    reverse: "reverse", compare: "Compare",
	ignoreCase: "ignore case", whitespace: "whitespace", ignoreEOL: "ignore EOL",
	ignoreTrailingEOL: "ignore trailing EOL", lineFilters: "line filters", activeFilters: "active",
	lineFiltersPlaceholder: "one regular expression per line",
	cancel: "Cancel",
    cancelled: "Cancelled", scheme: "colors", wrap: "wrap",
    syntax: "syntax highlight", showWs: "show whitespace", scratch: "paste text",
    patchFormat: "patch format", patchContext: "patch context", exportPatch: "Export patch",
    exporting: "Exporting patch…", exported: "Patch exported",
    diffCounter: (v) => `Difference ${v.current} / ${v.total} (${v.unread} unread)`,
    differenceNavigation: "Difference navigation",
    firstDiff: "First difference",
    previousDiff: "Previous difference",
    nextDiff: "Next difference",
    lastDiff: "Last difference",
    navHelpText: "Navigate: Alt+↓/↑; choose: Alt+← left / Alt+→ right / Alt+B base; Alt+Home/End; search: Ctrl+F (Enter/Shift+Enter next/previous, Esc closes)",
    keyboardShortcuts: "Keyboard shortcuts", differenceMap: "Difference map",
    detectMoves: "detect moves", moveMinLines: "move min lines", moved: "moved",
    addSync: "Add sync", clearSync: "Clear sync", syncPoints: "Sync points",
    ignoreHunk: "Ignore this difference", restoreHunk: "Restore difference", ignored: "ignored",
    syncSelect: "Select one corresponding line on each side.",
    syncOrderError: "Sync points must increase on both sides.",
    hunks: "hunks", added: "added", deleted: "deleted", modified: "modified",
    modeText: "text", modeSorted: "sorted", modeCsv: "csv / tsv",
    modeFolder: "folder", modeThreeway: "3-way text", modeThreewayCsv: "3-way csv",
    statusDifferent: "different", statusAll: "all",
    changed: "changed", removed: "removed", same: "same", left: "left", right: "right",
    leftOnly: "left only", rightOnly: "right only", equalRows: "equal", bytes: "bytes",
    encodingDetected: (v) => `encoding: OLD=${v.old} / NEW=${v.new}`,
    encodingMismatch: "left and right encodings differ",
    omitted: (n) => `(${n} hunks omitted; raise max hunks)`,
    moveDetectionSkipped: "Move detection was skipped because hunks were omitted.",
    theme: "Theme", themeSystem: "Match system", themeLight: "Light", themeDark: "Dark",
    jumpToKind: (v) => `Jump to ${v.label}`,
    confirmTitle: "Confirm", confirmProceed: "Proceed", cancel: "Cancel", close: "Close",
    shortcutNavigate: "Next / previous difference", shortcutFirstLast: "First / last difference",
    shortcutChooseSide: "Choose left / right", shortcutChooseBase: "Choose base",
    shortcutSearch: "Search in results", shortcutSearchStep: "Next / previous match",
    shortcutClose: "Close search or dialog", shortcutCompare: "Run the comparison",
    swap: "Swap", swapSides: "Swap OLD and NEW",
    engineTuning: "Engine tuning", engineTuningHint: "Affects how much of a difference is computed and shown, not what counts as one. Raise these only when a result is truncated.",
    csvParsingOptions: "File format", csvProjectOptions: "Saved comparisons",
    csvParsingHint: "Detected automatically. Set these only when a file is misread.",
    csvAdvancedHint: "Tuning for large or slow comparisons; the defaults suit most files.",
    changedSettings: (v) => `${v.count} changed`,
    searchResults: "Search results", searchPlaceholder: "Search in results", searchCase: "Match case",
    searchRegex: "Regex", searchChangedOnly: "Changed lines only", searchPrevious: "Previous match", searchNext: "Next match",
    searchClose: "Close search", searchNoMatches: "No matches",
    searchCounter: (v) => `${v.current} / ${v.total}${v.capped}`,
    comparing: "Comparing…", rendering: (v) => `Rendering… ${v.done} / ${v.total} hunks`, noDiff: "No differences.",
    completeMatch: "✔ Complete match", filteredMatch: "✔ Match under comparison rules",
    textMatchScope: (v) => `Compared ${v.old} OLD / ${v.new} NEW lines; 0 differences`,
    csvMatchScope: (v) => `Compared ${v.rows} rows / ${v.columns} columns; 0 differences`,
    threeWayTextMatchScope: (v) => `Compared 3 inputs / ${v.lines} lines; 0 differences`,
    threeWayCSVMatchScope: (v) => `Compared 3 inputs / ${v.columns} columns; 0 differences`,
    matchNotVerified: "⚠ Complete match not verified",
    requiredField: (v) => `${v.field} is required.`,
    requiredFields: (v) => `${v.fields.join(" and ")} ${v.fields.length === 1 ? "is" : "are"} required.`,
    invalidIndex: (v) => `${v.field} contains an invalid index.`,
    projectPath: "project path",
    emptyTitle: "Start a comparison",
    emptySteps: "1. Choose OLD → 2. Choose NEW → 3. Compare",
    emptyDrop: "You can also drop files anywhere on this screen to compare them.",
    enterPaths: "Enter both OLD and NEW paths.",
	csvSetup: "CSV / TSV setup", inspect: "Inspect headers", leftFormat: "left format", rightFormat: "right format",
	leftParser: "left parser", rightParser: "right parser", leftDelimiter: "left delimiter", rightDelimiter: "right delimiter",
	hasHeader: "header row", alignColumns: "align by name", lazyQuotes: "lazy quotes", trimLeadingSpace: "trim leading space",
	keyMode: "key mode", allColumns: "all columns", includeKeys: "selected keys", excludeKeys: "exclude selected",
	searchColumns: "search columns", selectAll: "Select all", invert: "Invert", csvCompareOptions: "Comparison and performance",
	ignoreColumns: "ignored value columns", tolerance: "numeric tolerance", columnTolerances: "column tolerances", maxRows: "max displayed rows",
	memory: "memory", tempDir: "temp directory", partitions: "partitions", parseWorkers: "input readers", workers: "workers",
	mergeFanIn: "merge fan-in", partitionBuffer: "partition buffer", maxRecordBytes: "max record size", keepTemp: "keep temporary files",
	changedColumnsOnly: "changed columns only", outputPath: "output path", outputFormat: "output format", outputHeader: "output header",
	review: "Review settings", runExport: "Run and export", chooseFile: "Choose a file", open: "Open",
	browseFile: "Browse for a file", close: "Close", parentFolder: "Parent folder",
	inspectionDone: (v) => `${v.column_count} columns — left ${v.left_format}/${v.left_parser}, right ${v.right_format}/${v.right_parser}`,
	csvNoDiff: "No CSV differences.", csvTruncated: "Display limit reached. Use export for the complete result.",
	selectKey: "Select at least one key column.",
	previousPage: "Previous page", nextPage: "Next page", pageInput: (v) => `Page number (${v.total} pages total)`,
	pageTotal: (v) => `of ${v.total} pages`, exportedCSV: (v) => `Exported to ${v}`,
	openProject: "Open project", saveProject: "Save project", recent: "Recent comparisons", projectSaved: "Project saved",
	mergeMode: "Merge mode", mergeResult: "Merge result", chooseLeft: "Use left", chooseRight: "Use right", chooseBase: "Use base", allLeft: "All left", allRight: "All right", allBase: "All base",
	threeWay: "3-way comparison", conflicts: "conflicts", autoMerged: "auto-merged",
	undo: "Undo", redo: "Redo", unresolved: (n) => `${n} unresolved`, overwriteInput: "overwrite input", saveMerge: "Save merge",
	mergeSaved: (v) => `Merged result saved to ${v}`, unresolvedWarning: (n) => `${n} differences are unresolved. Save them using the left side?`, overwriteWarning: "This will overwrite an input file and cannot be undone. Continue?",
	folderSetup: "Folder comparison", includes: "include globs", excludes: "exclude globs", hiddenFiles: "hidden files", quickCompare: "trust size + mtime", statusFilter: "statuses", symlinkPolicy: "Symbolic links are skipped. .gz files compare decompressed content.", chooseFolder: "Choose this folder",
	filterExpression: "filter expression", filterFile: "filter file", filterSet: "filter set", compareBy: "compare by", filterPreview: "Preview filter", filterPreviewResult: (v) => `old ${v.old_count} / new ${v.new_count} / union ${v.union_count}`,
    langButton: "English → 日本語",
    langSwitchLabel: "Switch language to Japanese",
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
	for (const el of document.querySelectorAll("[data-i18n-placeholder]")) el.placeholder = t(el.getAttribute("data-i18n-placeholder"));
  for (const el of document.querySelectorAll("[data-i18n-title]")) el.title = t(el.getAttribute("data-i18n-title"));
  for (const el of document.querySelectorAll("[data-i18n-aria-label]")) el.setAttribute("aria-label", t(el.getAttribute("data-i18n-aria-label")));
  if (lastData) updateCounter();
	if (csvData && $("mode").value === "csv") renderCSV(csvData);
	renderRecentComparisons();
}

// ---- word-level diff (ported from ayame-editor web/src/search.ts) ----
// The word diff lives in worddiff.js so it can be tested without a DOM (#139).
const { inlineWordDiff, inlineTokens, pushPart } = globalThis.AyameWordDiff;

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
  "compare", "exportPatch", "inspectCSV", "exportCSV", "saveMerge",
  "saveProject", "loadProject", "dirPreview", "saveDirProject", "loadDirProject",
  "addSync", "clearSync", "allLeft", "allRight", "allBase",
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
    await fn();
  } finally {
    busyOperation = null;
    unlockExclusiveControls();
  }
  return true;
}

// requestGeneration invalidates the result of a superseded request. Even with
// the lock, an operation that was already in flight when the lock was taken
// (or one aborted and restarted) must not paint over a newer result.
let requestGeneration = 0;
function beginRequest() { return ++requestGeneration; }
function isCurrentRequest(generation) { return generation === requestGeneration; }
let lastData = null; // last diff response, retained for navigation and merge actions
let lastComparedRequest = null;
let currentHunk = -1;
let readHunks = new Set();
let navObserver = null;
let syncSelection = { old: null, new: null };
let syncPoints = [];
let ignoredHunks = new Set();
let csvInspection = null;
let csvData = null;
let csvPage = 0;
const CSV_PAGE_SIZE = 100;
let browserTarget = null;
let directoryData = null, directoryBody = null;
let mergeChoices = new Map(), mergeDefault = null, mergeUndo = [], mergeRedo = [];
// Merge (adopt-left/right) controls are opt-in: most sessions only read diffs,
// so the per-hunk adopt buttons and the merge panel stay hidden until the user
// enters merge mode. setMergeMode syncs the body class the CSS keys off and the
// toggle's aria-pressed; updateMergeUI decides when the toggle is offered (#100).
let mergeMode = false;
function setMergeMode(on) {
  mergeMode = on;
  document.body.classList.toggle("merge-mode", on);
  $("mergeMode").setAttribute("aria-pressed", on ? "true" : "false");
}
let threeWayData = null;

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
function appendText(el, text) {
  if (!showWhitespace()) {
    el.appendChild(document.createTextNode(text));
    return;
  }
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
      visible.textContent = m[1].replace(/ /g, "·").replace(/\t/g, "→");
      s.append(original, visible);
      el.appendChild(s);
    } else {
      el.appendChild(document.createTextNode(m[2]));
    }
  }
}

function showWhitespace() { return $("showWs").checked; }
function syntaxPath(side) {
  if ($("scratch").checked) return "";
  return side === "old" ? $("old").value : $("new").value;
}
function appendSyntax(el, text, path) {
  const spans = globalThis.AyameSyntax?.highlightSpans(text, path);
  if (!spans) { appendText(el, text); return; }
  for (const part of spans) {
    if (part.kind === "plain") { appendText(el, part.text); continue; }
    const token = document.createElement("span");
    token.className = `syn syn-${part.kind}`;
    appendText(token, part.text);
    el.append(token);
  }
}
function textSpan(parts, changedClass, path) {
  const tx = document.createElement("span");
  tx.className = "tx";
  if (!parts) return tx;
  for (const p of parts) {
    const s = document.createElement("span");
    if (p.changed) s.className = changedClass;
    appendSyntax(s, p.text, path);
    tx.append(s);
  }
  return tx;
}

function plainSpan(text, path) {
  const tx = document.createElement("span");
  tx.className = "tx";
  appendSyntax(tx, text, path);
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
    c.tabIndex = 0;
    c.setAttribute("role", "button");
    c.setAttribute("aria-pressed", "false");
    const select = () => selectSyncLine(c);
    c.addEventListener("click", select);
    c.addEventListener("keydown", (event) => {
      if (event.key !== "Enter" && event.key !== " ") return;
      event.preventDefault();
      select();
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

function renderHunk(h, index) {
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
  const mergeActions = document.createElement("span");
  mergeActions.className = "hunk-merge";
  for (const [side, label] of [["left", t("chooseLeft")], ["right", t("chooseRight")]]) {
    const button = document.createElement("button"); button.type = "button"; button.className = `choose-${side}`; button.textContent = label;
    button.addEventListener("click", (event) => { event.stopPropagation(); chooseMerge(index, side); });
    mergeActions.append(button);
  }
  head.append(mergeActions);
  box.append(head);

  const rows = document.createElement("div");
  rows.className = "rows";
  const old = h.old || [], neu = h.new || [];
  const oldPath = syntaxPath("old"), newPath = syntaxPath("new");

  if (h.kind === "insert") {
    for (let k = 0; k < neu.length; k++)
      rows.append(row(cell("empty", null, plainSpan("")), cell("add", h.new_start + k + 1, plainSpan(neu[k], newPath), "new")));
  } else if (h.kind === "delete") {
    for (let k = 0; k < old.length; k++)
      rows.append(row(cell("del", h.old_start + k + 1, plainSpan(old[k], oldPath), "old"), cell("empty", null, plainSpan(""))));
  } else {
    const pairs = Math.min(old.length, neu.length);
    for (let k = 0; k < pairs; k++) {
      // The DP and its per-part spans are pure cost when the highlight is
      // off, since CSS only hid the result (#127).
      const wd = $("word").checked ? inlineWordDiff(old[k], neu[k]) : null;
      const left = cell("chg", h.old_start + k + 1, wd ? textSpan(wd.oldParts, "w-del", oldPath) : plainSpan(old[k], oldPath), "old");
      const right = cell("chg", h.new_start + k + 1, wd ? textSpan(wd.newParts, "w-add", newPath) : plainSpan(neu[k], newPath), "new");
      rows.append(row(left, right));
    }
    for (let k = pairs; k < old.length; k++)
      rows.append(row(cell("del", h.old_start + k + 1, plainSpan(old[k], oldPath), "old"), cell("empty", null, plainSpan(""))));
    for (let k = pairs; k < neu.length; k++)
      rows.append(row(cell("empty", null, plainSpan("")), cell("add", h.new_start + k + 1, plainSpan(neu[k], newPath), "new")));
  }
  box.append(rows);
  return box;
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
    count.textContent = n.toLocaleString();
    s.append(count, ` ${label}`);
    return s;
  };
  el.append(
    stat("", t("hunks"), res.hunk_count),
    stat("add", t("added"), res.added, "insert"),
    stat("del", t("deleted"), res.deleted, "delete"),
    stat("chg", t("modified"), res.modified, "replace"),
  );
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
    n.textContent = t("omitted", res.omitted_hunks.toLocaleString());
    el.append(n);
  }
  // Show what `encoding: auto` decoded each file as, and flag a left/right
  // mismatch — the material clue when output looks garbled (#130). Present only
  // for file inputs; inline text carries no detected encoding.
  if (res.old_encoding || res.new_encoding) {
    const mismatch = res.old_encoding && res.new_encoding && res.old_encoding !== res.new_encoding;
    const enc = document.createElement("span");
    enc.className = mismatch ? "note encoding-mismatch" : "note";
    enc.textContent = t("encodingDetected", { old: res.old_encoding || "—", new: res.new_encoding || "—" });
    if (mismatch) enc.textContent += ` — ${t("encodingMismatch")}`;
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

// renderBudgetMs is how long one slice may build before yielding. Comfortably
// inside a frame, so a slice cannot itself cause a dropped frame.
const RENDER_BUDGET_MS = 8;

// renderToken invalidates an in-progress render when a newer one starts, so a
// superseded render stops appending instead of interleaving with its successor.
let renderToken = 0;

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
// superseded this one, in which case the caller must not run its finishing
// steps.
async function renderInSlices(target, items, build) {
  // The nodes any search hits pointed at are being replaced.
  clearSearchHits();
  const token = ++renderToken;
  let index = 0;
  while (index < items.length) {
    const started = performance.now();
    const frag = document.createDocumentFragment();
    // Always place at least one node, so a single very expensive item cannot
    // stall the loop forever.
    do {
      frag.append(build(items[index], index));
      index++;
    } while (index < items.length && performance.now() - started < RENDER_BUDGET_MS);
    target.append(frag);
    if (index >= items.length) break;
    setStatus(t("rendering", { done: index.toLocaleString(), total: items.length.toLocaleString() }), "busy");
    await yieldToBrowser();
    if (token !== renderToken) return false;
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

// renderResult draws a diff response once. Display preferences only toggle
// classes on the completed DOM via applyDisplayPreferences.
async function renderResult(data) {
  applyDisplayPreferences();
  renderSummary(data);
  const result = $("result");
  result.innerHTML = "";
  setupNavigation(data);
  syncExportPatchVisibility();
  if (!data.hunks.length) {
    const scope = t("textMatchScope", { old: data.old_lines.toLocaleString(), new: data.new_lines.toLocaleString() });
    result.append(resultStateCard(t(comparisonUsesRules() ? "filteredMatch" : "completeMatch"), scope));
    return;
  }
  const complete = await renderInSlices(result, data.hunks, (hunk, index) => renderHunk(hunk, index));
  if (!complete) return;
  setStatus("");
  // Re-apply an open search to the diff that just replaced the old one (#118).
  if (searchOpen()) runSearch();
  updateMergeUI();
  observeHunks();
  updateMinimapViewport();
}

function mutateMerge(mutator) {
  mergeUndo.push({ choices: new Map(mergeChoices), defaultChoice: mergeDefault });
  if (mergeUndo.length > 100) mergeUndo.shift();
  mergeRedo = [];
  mutator();
  updateMergeUI();
}
function chooseMerge(index, side) { mutateMerge(() => mergeChoices.set(index, side)); }
function updateMergeUI() {
	$("mergeMode").hidden = true; // the merge-mode toggle is a text-diff affordance (#100)
	if (threeWayData && ($("mode").value === "threeway" || $("mode").value === "threeway-csv")) { updateThreeWayMergeUI(); return; }
	$("allBase").hidden = true;
	if ($("mode").value === "csv" && csvData) { updateCSVMergeUI(); return; }
  const mergeable = Boolean(lastData?.hunks?.length) && $("mode").value === "text";
  // Offer the toggle whenever a text diff can be merged, but keep the adopt
  // buttons (CSS) and the merge panel hidden until the user opts into merge mode.
  $("mergeMode").hidden = !mergeable;
  $("mergePanel").hidden = !(mergeable && mergeMode);
  if (!mergeable) return;
  lastData.hunks.forEach((_, index) => {
    const box = $(`hunk-${index}`), side = mergeChoices.get(index);
    box?.classList.toggle("merge-left", side === "left"); box?.classList.toggle("merge-right", side === "right");
  });
  $("mergeUnresolved").textContent = t("unresolved", mergeDefault ? 0 : Math.max(0, lastData.hunk_count - mergeChoices.size));
  $("mergeUndo").disabled = mergeUndo.length === 0; $("mergeRedo").disabled = mergeRedo.length === 0;
}
// mergeRowIndex maps a merge id to the row that represents it, filled while
// rendering. Both merge UIs used to find their rows with a document-wide
// attribute selector, once per event, on every merge click. The three-way
// result has no cap on its event count, so a file with thousands of conflicts
// meant thousands of full-document scans per click (#154).
let mergeRowIndex = new Map();

function resetMergeRowIndex() { mergeRowIndex = new Map(); }
function indexMergeRow(id, node) { mergeRowIndex.set(String(id), node); }

function updateCSVMergeUI() {
  $("mergePanel").hidden = false;
  const chosen = new Set([...mergeChoices.keys()].map(String));
  for (const [id, row] of mergeRowIndex) {
    const side = mergeChoices.get(id);
    row.classList.toggle("merge-left", side === "left"); row.classList.toggle("merge-right", side === "right");
  }
  $("mergeUnresolved").textContent = t("unresolved", mergeDefault ? 0 : Math.max(0, (csvData.difference_count || csvData.differences.length) - chosen.size));
  $("mergeUndo").disabled = mergeUndo.length === 0; $("mergeRedo").disabled = mergeRedo.length === 0;
}
function undoMerge() {
  if (!mergeUndo.length) return;
  mergeRedo.push({ choices: new Map(mergeChoices), defaultChoice: mergeDefault });
  const state = mergeUndo.pop(); mergeChoices = state.choices; mergeDefault = state.defaultChoice; updateMergeUI();
}
function redoMerge() {
  if (!mergeRedo.length) return;
  mergeUndo.push({ choices: new Map(mergeChoices), defaultChoice: mergeDefault });
  const state = mergeRedo.pop(); mergeChoices = state.choices; mergeDefault = state.defaultChoice; updateMergeUI();
}
async function saveTextMerge() {
  const output = $("mergeOutput").value.trim();
  if (!output) { setStatus(t("requiredField", { field: t("outputPath") }), "error"); return; }
  const unresolved = mergeDefault ? 0 : Math.max(0, (lastData?.hunk_count || 0) - mergeChoices.size);
  const allowUnresolved = unresolved > 0 && await askConfirm(t("unresolvedWarning", unresolved));
  if (unresolved > 0 && !allowUnresolved) return;
  const overwrite = $("mergeOverwrite").checked;
  const confirmOverwrite = !overwrite || await askConfirm(t("overwriteWarning"));
  if (!confirmOverwrite) return;
  const body = { ...requestBody(), output, choices: Object.fromEntries(mergeChoices), defaultChoice: mergeDefault || "", allowUnresolved, overwrite, confirmOverwrite };
  $("saveMerge").disabled = true;
  try {
    const response = await apiFetch("/api/merge/text", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const data = await response.json(); if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
    setStatus(t("mergeSaved", data.output), "");
  } catch (err) { setStatus(String(err.message || err), "error"); }
  finally { $("saveMerge").disabled = false; }
}
async function saveCSVMerge() {
  const output = $("mergeOutput").value.trim();
  if (!output) { setStatus(t("requiredField", { field: t("outputPath") }), "error"); return; }
  const unresolved = mergeDefault ? 0 : Math.max(0, (csvData?.difference_count || 0) - new Set([...mergeChoices.keys()].map(String)).size);
  const allowUnresolved = unresolved > 0 && await askConfirm(t("unresolvedWarning", unresolved));
  if (unresolved > 0 && !allowUnresolved) return;
  const overwrite = $("mergeOverwrite").checked;
  const confirmOverwrite = !overwrite || await askConfirm(t("overwriteWarning"));
  if (!confirmOverwrite) return;
  const body = { ...csvRequestBody(), output, choices: Object.fromEntries(mergeChoices), defaultChoice: mergeDefault || "", allowUnresolved, overwrite, confirmOverwrite };
  $("saveMerge").disabled = true;
  try {
    const response = await apiFetch("/api/merge/csv", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const data = await response.json(); if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
    setStatus(t("mergeSaved", data.output), "");
  } catch (err) { setStatus(String(err.message || err), "error"); }
  finally { $("saveMerge").disabled = false; }
}
function runSaveMergeResult() { if ($("mode").value === "threeway" || $("mode").value === "threeway-csv") return saveThreeWayMerge(); return $("mode").value === "csv" ? saveCSVMerge() : saveTextMerge(); }
// A merge writes a file from the current inputs, so it must not run while a
// comparison for different inputs is still in flight (#128).
async function saveMergeResult() { return runExclusive("saveMerge", runSaveMergeResult); }

function threeWayRequestBody() { return { ...requestBody(), base: $("base").value.trim() }; }
function threeLines(value, csvMode) { return csvMode ? (value || []).map((row) => row.join("\t")) : (value || []); }
async function renderThreeWay(data, csvMode) {
  threeWayData = { ...data, csvMode }; csvData = null;
  lastComparedRequest = null;
  const summary = $("summary"); summary.innerHTML = "";
  const add = (label, value, cls = "") => { const item = document.createElement("span"); item.className = `stat ${cls}`; const b = document.createElement("b"); b.textContent = value; item.append(b, ` ${label}`); summary.append(item); };
  add(t("conflicts"), data.conflicts, "del"); add(t("left"), data.left_only); add(t("right"), data.right_only); add(t("same"), data.same_change); if (data.merged) add(t("autoMerged"), data.merged, "add"); summary.hidden = false;
  const result = $("result"); result.innerHTML = "";
  resetMergeRowIndex();
  lastData = { old_lines: data.base_lines || data.events.length, new_lines: data.base_lines || data.events.length, hunks: data.events.map((event) => ({ kind: event.kind === "conflict" ? "replace" : "insert", old_start: event.base_start || 0, new_start: event.base_start || 0, old_len: event.base_len || 1, new_len: event.base_len || 1 })) };
  syncExportPatchVisibility();
  setupNavigation(lastData);
  const buildEvent = (event, index) => {
    const box = document.createElement("section");
    box.className = `hunk three-event ${event.kind}`; box.id = `hunk-${index}`; box.dataset.hunk = String(index); box.dataset.mergeId = String(event.id); box.tabIndex = -1;
    indexMergeRow(event.id, box);
    const head = document.createElement("header"); head.className = "hunk-head"; head.append(document.createTextNode(`${event.kind} #${String(event.id).slice(0, 10)} · ${csvMode ? event.key.join(" / ") : `BASE ${event.base_start + 1},${event.base_len}`}`));
    if (event.kind === "conflict") {
      const actions = document.createElement("span"); actions.className = "hunk-merge";
      for (const [side, label] of [["left", t("chooseLeft")], ["base", t("chooseBase")], ["right", t("chooseRight")]]) { const button = document.createElement("button"); button.type = "button"; button.className = `choose-${side}`; button.textContent = label; button.onclick = () => chooseMerge(event.id, side); actions.append(button); }
      head.append(actions);
    }
    const grid = document.createElement("div"); grid.className = "three-grid";
    for (const [name, values] of (event.kind === "merged" ? [["BASE", event.base], ["MERGED", event.combined]] : [["BASE", event.base], ["LEFT", event.left], ["RIGHT", event.right]])) { const pane = document.createElement("section"); pane.className = "three-pane"; const title = document.createElement("h3"); title.textContent = name; pane.append(title); for (const line of threeLines(values, csvMode)) { const row = document.createElement("div"); row.className = "three-line"; row.textContent = line; pane.append(row); } grid.append(pane); }
    box.append(head, grid);
    return box;
  };
  // Sliced like the text path: a three-way result can carry as many events as
  // a diff carries hunks, and this loop appended straight into the live DOM
  // rather than a fragment, so it was the heavier of the two (#127).
  if (data.events.length && !(await renderInSlices(result, data.events, buildEvent))) return;
  if (data.events.length) setStatus("");
  if (!data.events.length) {
    const scope = csvMode
      ? t("threeWayCSVMatchScope", { columns: (data.header || []).length.toLocaleString() })
      : t("threeWayTextMatchScope", { lines: Number(data.base_lines || 0).toLocaleString() });
    result.append(resultStateCard(t(comparisonUsesRules(csvMode) ? "filteredMatch" : "completeMatch"), scope));
  }
  observeHunks(); updateThreeWayMergeUI(); updateMinimapViewport();
}
function updateThreeWayMergeUI() {
	$("allBase").hidden = false;
  $("mergePanel").hidden = !threeWayData;
  if (!threeWayData) return;
  for (const event of threeWayData.events) {
    const row = mergeRowIndex.get(String(event.id)), side = mergeChoices.get(event.id);
    for (const value of ["left", "right", "base"]) row?.classList.toggle(`merge-${value}`, side === value);
  }
  $("mergeUnresolved").textContent = t("unresolved", Math.max(0, threeWayData.conflicts - mergeChoices.size));
  $("mergeUndo").disabled = mergeUndo.length === 0; $("mergeRedo").disabled = mergeRedo.length === 0;
}
async function compareThreeWay(csvMode) {
  if (!$("base").value.trim() || !$("old").value.trim() || !$("new").value.trim()) { setStatus(t("enterPaths"), "error"); return; }
  let body;
  if (csvMode) { if (!csvInspection && !(await inspectCSV())) return; body = { ...csvRequestBody(), base: $("base").value.trim() }; }
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
    const data = await response.json(); if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
    if (!isCurrentRequest(generation)) return;
    threeWayData = null; mergeChoices = new Map(); mergeDefault = null; mergeUndo = []; mergeRedo = [];
    if (!$("mergeOutput").value) { const source = $("base").value.trim(); $("mergeOutput").value = source ? source.replace(/(\.[^./\\]+)?$/, ".merged$1") : (csvMode ? "merged.csv" : "merged.txt"); }
    clearInterval(timer);
    await renderThreeWay(data, csvMode);
  } catch (err) {
    if (err.name === "AbortError") setStatus(t("cancelled"), "");
    else setStatus(String(err.message || err), "error");
  } finally {
    clearInterval(timer);
    $("cancel").hidden = true;
    currentAbort = null;
  }
}
async function saveThreeWayMerge() {
  const output = $("mergeOutput").value.trim(); if (!output) { setStatus(t("requiredField", { field: t("outputPath") }), "error"); return; }
  const unresolved = Math.max(0, (threeWayData?.conflicts || 0) - mergeChoices.size);
  const allowUnresolved = unresolved > 0 && await askConfirm(t("unresolvedWarning", unresolved)); if (unresolved > 0 && !allowUnresolved) return;
  const overwrite = $("mergeOverwrite").checked, confirmOverwrite = !overwrite || await askConfirm(t("overwriteWarning")); if (!confirmOverwrite) return;
  const base = threeWayData.csvMode ? { ...csvRequestBody(), base: $("base").value.trim() } : threeWayRequestBody();
  const body = { ...base, output, choices: Object.fromEntries(mergeChoices), allowUnresolved, overwrite, confirmOverwrite };
  $("saveMerge").disabled = true;
  try { const response = await apiFetch(`/api/merge/three-way/${threeWayData.csvMode ? "csv" : "text"}`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await response.json(); if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`); setStatus(t("mergeSaved", data.output), ""); }
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
}

function activeHunkIndexes() {
  return (lastData?.hunks || []).map((_, index) => index).filter((index) => !ignoredHunks.has(index) && (!threeWayData || threeWayData.events[index]?.kind === "conflict"));
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

// minimapHasMarkers records whether buildMinimap produced anything, so the
// visibility check does not have to re-inspect the DOM on every scroll frame.
let minimapHasMarkers = false;

// updateMinimapViewport decides whether the minimap is worth showing and, when
// it is, positions the indicator over the currently visible slice.
//
// Visibility is decided here rather than at build time for two reasons: it
// needs the populated DOM (setupNavigation runs before the hunks are appended),
// and it has to be revisited whenever the result or the window resizes. Both
// render paths, the scroll handler, and the resize handler already call this.
function updateMinimapViewport() {
  const result = $("result");
  const map = $("minimap");
  const rect = result.getBoundingClientRect();
  // A map for content that already fits navigates nothing: it used to render as
  // a tall empty bar beside a one-line diff (#102). The slack keeps a result
  // only marginally taller than the window from getting one either.
  const scrolls = rect.height > window.innerHeight * 1.15;
  map.hidden = !(minimapHasMarkers && scrolls);
  if (map.hidden || !rect.height) return;
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
  minimapHasMarkers = hasHunks;
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

function setStatus(msg, cls) {
  const el = $("status");
  const error = cls === "error";
  el.setAttribute("role", error ? "alert" : "status");
  el.setAttribute("aria-live", error ? "assertive" : "polite");
  if (!msg) { el.textContent = ""; el.hidden = true; return; }
  el.className = "status " + (cls || "");
  el.textContent = msg;
  el.hidden = false;
}

function splitList(value) {
  return String(value || "").split(",").map((item) => item.trim()).filter(Boolean);
}

function selectedCSVColumns() {
  return [...document.querySelectorAll("#columnList input:checked")].map((input) => ({ name: input.dataset.name, index: Number(input.dataset.index) }));
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
    ignoreCase: $("ignoreCase").checked, whitespace: $("whitespace").value,
    lineFilters: $("lineFilters").value.split(/\r?\n/).map((value) => value.trim()).filter(Boolean),
    ignoreColumnNames: [], ignoreColumnIndexes: [], tolerance: $("tolerance").value === "" ? null : Number($("tolerance").value),
    columnTolerances: [], partitions: Number($("partitions").value), parseWorkers: Number($("parseWorkers").value),
    workers: Number($("workers").value), memory: $("memory").value.trim(), tempDir: $("tempDir").value.trim(),
    partitionBuffer: $("partitionBuffer").value.trim(), mergeFanIn: Number($("mergeFanIn").value),
    maxRecordBytes: $("maxRecordBytes").value.trim(), keepTemp: $("keepTemp").checked,
    maxRows: Number($("csvMaxRows").value), output: $("csvOutput").value.trim(),
    outputFormat: $("csvOutputFormat").value, outputHeader: $("outputHeader").checked,
  };
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
  return body;
}

function updateCSVReview() {
  const body = csvRequestBody();
  const mode = $("keyMode").value;
  const keys = mode === "all" ? t("allColumns") : selectedCSVColumns().map((item) => item.name).join(", ") || "—";
  $("reviewText").textContent = [
    `OLD: ${body.old || "—"}`, `NEW: ${body.new || "—"}`,
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
    const text = document.createElement("span");
    text.textContent = `${index}: ${name}`;
    label.append(input, text); list.append(label);
  });
  $("keySetup").hidden = false;
  // Rebuild the filter cache alongside the list it describes.
  buildColumnFilterIndex();
  applyColumnFilter();
  syncKeyMode();
}

async function inspectCSV() {
  const body = csvRequestBody();
  if (!validateInputs(body, false)) return false;
  $("inspectCSV").disabled = true;
  setStatus(t("comparing"), "busy");
  try {
    const resp = await apiFetch("/api/csv/inspect", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const data = await resp.json();
    if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
    csvInspection = data;
    $("inspection").textContent = t("inspectionDone", data);
    renderColumnSelection(data);
    setStatus("");
    updateCSVReview();
    return true;
  } catch (err) { setStatus(String(err.message || err), "error"); return false; }
  finally { $("inspectCSV").disabled = false; }
}

function renderCSVSummary(data) {
  const summary = data.summary, el = $("summary");
  el.innerHTML = "";
  const add = (label, value, cls = "") => { const item = document.createElement("span"); item.className = `stat ${cls}`; const b = document.createElement("b"); b.textContent = Number(value || 0).toLocaleString(); item.append(b, ` ${label}`); el.append(item); };
  add(t("leftOnly"), summary.left_only, "del"); add(t("rightOnly"), summary.right_only, "add");
  add(t("changed"), Math.max(summary.changed_left || 0, summary.changed_right || 0), "chg"); add(t("equalRows"), summary.equal_rows);
  for (const column of (summary.column_changes || []).slice(0, 8)) add(column.name, column.count, "chg");
  if (data.truncated) { const note = document.createElement("span"); note.className = "note"; note.textContent = t("csvTruncated"); el.append(note); }
  el.hidden = false;
}

function renderCSV(data) {
  csvData = data;
  lastData = null;
  lastComparedRequest = null;
  syncExportPatchVisibility();
  minimapHasMarkers = false; $("diffNav").hidden = true; $("syncPanel").hidden = true; $("minimap").hidden = true;
  // The rows this index pointed at are about to be replaced. Updating the merge
  // UI here would only toggle classes on nodes that are discarded below; the
  // call after the table is built is the one that matters (#154).
  resetMergeRowIndex();
  renderCSVSummary(data);
  const result = $("result"); result.innerHTML = "";
  if (!data.differences.length) {
    if (data.truncated) result.append(resultStateCard(t("matchNotVerified"), t("csvTruncated"), "partial"));
    else {
      const scope = t("csvMatchScope", { rows: Number(data.summary.equal_rows || 0).toLocaleString(), columns: data.header.length.toLocaleString() });
      result.append(resultStateCard(t(comparisonUsesRules(true) ? "filteredMatch" : "completeMatch"), scope));
    }
    return;
  }
  const changedSet = new Set((data.summary.column_changes || []).map((column) => column.index));
  const columns = data.header.map((_, index) => index).filter((index) => !$("changedColumnsOnly").checked || changedSet.has(index));
  const pageCount = Math.max(1, Math.ceil(data.differences.length / CSV_PAGE_SIZE));
  csvPage = Math.max(0, Math.min(csvPage, pageCount - 1));
  const controls = document.createElement("div"); controls.className = "csv-pages";
  const rerenderAndFocus = (selector) => { renderCSV(data); document.querySelector(selector)?.focus(); };
  const prev = document.createElement("button"); prev.type = "button"; prev.className = "csv-page-prev"; prev.textContent = "←"; prev.setAttribute("aria-label", t("previousPage")); prev.title = t("previousPage"); prev.disabled = csvPage === 0; prev.onclick = () => { csvPage--; rerenderAndFocus(".csv-page-prev"); };
  const pageInput = document.createElement("input"); pageInput.type = "number"; pageInput.className = "csv-page-input"; pageInput.min = "1"; pageInput.max = String(pageCount); pageInput.step = "1"; pageInput.value = String(csvPage + 1); pageInput.setAttribute("aria-label", t("pageInput", { total: pageCount }));
  const jumpToPage = (refocus) => {
    const requested = Number(pageInput.value);
    if (!Number.isInteger(requested) || requested < 1 || requested > pageCount) { pageInput.value = String(csvPage + 1); return; }
    if (requested - 1 === csvPage) return;
    csvPage = requested - 1;
    renderCSV(data);
    if (refocus) { const input = document.querySelector(".csv-page-input"); input?.focus(); input?.select(); }
  };
  pageInput.onchange = () => jumpToPage(false);
  pageInput.onkeydown = (event) => { if (event.key === "Enter") { event.preventDefault(); jumpToPage(true); } };
  const total = document.createElement("span"); total.textContent = t("pageTotal", { total: pageCount });
  const next = document.createElement("button"); next.type = "button"; next.className = "csv-page-next"; next.textContent = "→"; next.setAttribute("aria-label", t("nextPage")); next.title = t("nextPage"); next.disabled = csvPage + 1 >= pageCount; next.onclick = () => { csvPage++; rerenderAndFocus(".csv-page-next"); };
  controls.append(prev, pageInput, total, next); result.append(controls);
  const wrap = document.createElement("div"); wrap.className = "csv-table-wrap";
  const table = document.createElement("table"); table.className = "csv-table";
  const head = document.createElement("thead"), headerRow = document.createElement("tr"), sideHead = document.createElement("th"); sideHead.textContent = "_side"; headerRow.append(sideHead);
  const counts = new Map((data.summary.column_changes || []).map((column) => [column.index, column.count]));
  for (const index of columns) { const th = document.createElement("th"); th.textContent = data.header[index]; if (counts.has(index)) { const badge = document.createElement("b"); badge.textContent = counts.get(index); th.append(badge); } headerRow.append(th); }
  head.append(headerRow); table.append(head);
  const tbody = document.createElement("tbody");
  const appendRow = (values, side, kind, changed) => {
    if (!values?.length) return;
    const tr = document.createElement("tr"); tr.className = `csv-${kind.toLowerCase()} csv-${side}`;
    const sideCell = document.createElement("th"); sideCell.textContent = side; tr.append(sideCell);
    for (const index of columns) { const td = document.createElement("td"); td.textContent = values[index] ?? ""; td.title = values[index] ?? ""; if (changed.has(index)) td.classList.add("csv-cell-changed"); tr.append(td); }
    tbody.append(tr);
  };
  for (const diff of data.differences.slice(csvPage * CSV_PAGE_SIZE, (csvPage + 1) * CSV_PAGE_SIZE)) {
    const action = document.createElement("tr"); action.className = "csv-merge-choice"; action.dataset.mergeId = diff.id;
    indexMergeRow(diff.id, action);
    const actionCell = document.createElement("th"); actionCell.colSpan = columns.length + 1;
    const label = document.createElement("span"); label.textContent = `${diff.kind} · ${diff.id.slice(0, 8)}`;
    actionCell.append(label);
    for (const [side, text] of [["left", t("chooseLeft")], ["right", t("chooseRight")]]) {
      const button = document.createElement("button"); button.type = "button"; button.className = `choose-${side}`; button.textContent = text;
      button.onclick = () => chooseMerge(diff.id, side); actionCell.append(button);
    }
    action.append(actionCell); tbody.append(action);
    const changed = new Set((diff.changed_columns || []).map((column) => column.index));
    appendRow(diff.old, "left", diff.kind, changed); appendRow(diff.new, "right", diff.kind, changed);
  }
  table.append(tbody); wrap.append(table); result.append(wrap); updateCSVMergeUI();
}

async function compareCSV() {
	let body = csvRequestBody();
	if (!csvInspection) {
	  if (!validateInputs(body, false) || !(await inspectCSV())) return;
	  body = csvRequestBody();
	}
	if (!validateInputs(body)) return;
  const ac = new AbortController(); currentAbort = ac; $("cancel").hidden = false;
  const generation = beginRequest();
  const started = Date.now(), tick = () => setStatus(t("comparing") + " " + ((Date.now() - started) / 1000).toFixed(1) + "s", "busy"); tick();
  const timer = setInterval(tick, 100);
  try {
    const resp = await apiFetch("/api/csv/diff", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), signal: ac.signal });
    const data = await resp.json(); if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
    if (!isCurrentRequest(generation)) return;
    threeWayData = null; mergeChoices = new Map(); mergeDefault = null; mergeUndo = []; mergeRedo = [];
    if (!$("mergeOutput").value) {
      const source = $("old").value.trim(); $("mergeOutput").value = source ? source.replace(/(\.[^./\\]+)?$/, ".merged$1") : "merged.csv";
    }
    csvPage = 0; renderCSV(data); rememberComparison(body); setStatus("");
  } catch (err) { if (err.name === "AbortError") setStatus(t("cancelled"), ""); else setStatus(String(err.message || err), "error"); }
  finally { clearInterval(timer); $("cancel").hidden = true; currentAbort = null; }
}

async function runExportCSV() {
	let body = csvRequestBody();
	if (!csvInspection) { if (!validateInputs(body, false) || !(await inspectCSV())) return; body = csvRequestBody(); }
	if (!validateInputs(body)) return;
  if (!body.output) { setStatus(t("requiredField", { field: t("outputPath") }), "error"); return; }
  $("exportCSV").disabled = true; setStatus(t("comparing"), "busy");
  try { const resp = await apiFetch("/api/csv/export", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await resp.json(); if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`); setStatus(t("exportedCSV", data.output), ""); }
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
  for (const id of ["leftFormat", "rightFormat", "leftParser", "rightParser", "leftDelimiter", "rightDelimiter", "whitespace", "memory", "tempDir", "partitionBuffer", "maxRecordBytes"]) if (body[id] != null) $(id).value = body[id];
  for (const id of ["hasHeader", "alignColumnsByName", "lazyQuotes", "trimLeadingSpace", "ignoreCase", "keepTemp", "outputHeader"]) {
    const target = id === "alignColumnsByName" ? "alignColumns" : id; if (body[id] != null) $(target).checked = Boolean(body[id]);
  }
  for (const id of ["partitions", "parseWorkers", "workers", "mergeFanIn", "maxRows"]) { const target = id === "maxRows" ? "csvMaxRows" : id; if (body[id]) $(target).value = body[id]; }
  $("lineFilters").value = (body.lineFilters || []).join("\n");
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
  syncKeyMode(); updateCSVReview();
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
  try { const resp = await apiFetch("/api/project/save", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await resp.json(); if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`); rememberComparison(body); setStatus(t("projectSaved"), ""); }
  catch (err) { setStatus(String(err.message || err), "error"); }
}

async function runLoadProject() {
  const path = $("projectPath").value.trim(); if (!path) { setStatus(t("requiredField", { field: t("projectPath") }), "error"); return; }
  try { const resp = await apiFetch("/api/project/load", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path }) }); const data = await resp.json(); if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`); if (data.mode === "dir") applyDirectoryProject(data); else await applyCSVProject(data); rememberComparison(data); setStatus(""); }
  catch (err) { setStatus(String(err.message || err), "error"); }
}

function dirRequestBody() { return { mode: "dir", old: $("old").value.trim(), new: $("new").value.trim(), includes: splitList($("dirIncludes").value), excludes: splitList($("dirExcludes").value), filter: $("dirFilter").value.trim(), filterFile: $("dirFilterFile").value.trim(), filterSets: splitList($("dirFilterSet").value), compareBy: $("dirCompareBy").value, hidden: $("dirHidden").checked, workers: Number($("dirWorkers").value) || 8 }; }

async function runPreviewDirectoryFilter() {
  const body = dirRequestBody(); if (!validateInputs(body)) return;
  $("dirPreview").disabled = true; $("dirPreviewResult").textContent = t("comparing");
  try { const resp = await apiFetch("/api/dir/preview", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await resp.json(); if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`); $("dirPreviewResult").textContent = t("filterPreviewResult", data); $("dirPreviewResult").title = (data.sample || []).join("\n"); }
  catch (err) { $("dirPreviewResult").textContent = String(err.message || err); }
  finally { $("dirPreview").disabled = false; }
}

function applyDirectoryProject(body) {
  $("mode").value = "dir"; syncModeOpts(); $("old").value = body.old || ""; $("new").value = body.new || "";
  $("dirIncludes").value = (body.includes || []).join(", "); $("dirExcludes").value = (body.excludes || []).join(", ");
  $("dirFilter").value = body.filter || ""; $("dirFilterSet").value = (body.filterSets || []).join(", ");
  $("dirCompareBy").value = body.compareBy || "contents"; $("dirHidden").checked = Boolean(body.hidden); $("dirWorkers").value = body.workers || 8;
  if (body.projectPath) $("dirProjectPath").value = body.projectPath;
}

async function runSaveDirectoryProject() {
  const body = dirRequestBody(); body.projectPath = $("dirProjectPath").value.trim();
  if (!validateInputs(body) || !body.projectPath) { if (!body.projectPath) setStatus(t("requiredField", { field: t("projectPath") }), "error"); return; }
  try { const resp = await apiFetch("/api/project/save", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }); const data = await resp.json(); if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`); rememberComparison(body); setStatus(t("projectSaved"), ""); }
  catch (err) { setStatus(String(err.message || err), "error"); }
}

async function runLoadDirectoryProject() {
  const path = $("dirProjectPath").value.trim(); if (!path) { setStatus(t("requiredField", { field: t("projectPath") }), "error"); return; }
  try { const resp = await apiFetch("/api/project/load", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path }) }); const data = await resp.json(); if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`); if (data.mode !== "dir") throw new Error("not a folder project"); applyDirectoryProject(data); rememberComparison(data); setStatus(""); }
  catch (err) { setStatus(String(err.message || err), "error"); }
}

async function renderDirectory(data, body) {
  directoryData = data; directoryBody = body;
  csvData = null; lastData = null; lastComparedRequest = null; minimapHasMarkers = false; $("diffNav").hidden = true; $("minimap").hidden = true; $("syncPanel").hidden = true;
  syncExportPatchVisibility();
  $("mergePanel").hidden = true;
  const summary = $("summary"); summary.innerHTML = "";
  for (const [name, cls] of [["added", "add"], ["removed", "del"], ["changed", "chg"], ["same", ""]]) { const item = document.createElement("span"); item.className = `stat ${cls}`; const b = document.createElement("b"); b.textContent = data[name].toLocaleString(); item.append(b, ` ${t(name)}`); summary.append(item); } summary.hidden = false;
  const result = $("result"); result.innerHTML = ""; const tree = document.createElement("div"); tree.className = "dir-tree";
  const filter = $("dirStatus").value;
  // A folder comparison carries one entry per file with no cap, so a large
  // tree is exactly the case that must not arrive as one blocking loop (#127).
  const visible = data.entries.filter((entry) =>
    !((filter === "different" && entry.status === "same") || (filter !== "all" && filter !== "different" && entry.status !== filter)));
  const buildEntry = (entry) => {
    const row = document.createElement("button"); row.type = "button"; row.className = `dir-entry ${entry.status}`;
    const depth = entry.path.split("/").length - 1; row.style.paddingLeft = `${0.65 + depth * 1.1}rem`;
    const marker = { added: "+", removed: "−", changed: "~", same: "=" }[entry.status];
    row.textContent = `${marker} ${entry.path}`; row.title = `${entry.old_size} → ${entry.new_size} ${t("bytes")}\n${entry.old_mtime || ""} → ${entry.new_mtime || ""}`;
    if (entry.status === "changed") row.addEventListener("click", async () => { $("mode").value = "text"; syncModeOpts(); $("old").value = `${body.old.replace(/[\\/]$/, "")}/${entry.path}`; $("new").value = `${body.new.replace(/[\\/]$/, "")}/${entry.path}`; await compare(); });
    else row.disabled = true;
    return row;
  };
  result.append(tree);
  if (visible.length && !(await renderInSlices(tree, visible, buildEntry))) return;
  setStatus("");
}

async function compareDirectory() {
  const body = dirRequestBody(); if (!validateInputs(body)) return;
  const ac = new AbortController(); currentAbort = ac; $("cancel").hidden = false;
  const generation = beginRequest();
  const started = Date.now();
  const tick = () => setStatus(t("comparing") + " " + ((Date.now() - started) / 1000).toFixed(1) + "s", "busy");
  tick();
  const timer = setInterval(tick, 100);
  try {
    const resp = await apiFetch("/api/dir/diff", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body), signal: ac.signal });
    const data = await resp.json(); if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
    if (!isCurrentRequest(generation)) return;
    clearInterval(timer);
    await renderDirectory(data, body);
  }
  catch (err) { if (err.name === "AbortError") setStatus(t("cancelled"), ""); else setStatus(String(err.message || err), "error"); }
  finally { clearInterval(timer); $("cancel").hidden = true; currentAbort = null; }
}

async function runCompare() {
  if ($("mode").value === "threeway") { await compareThreeWay(false); return; }
  if ($("mode").value === "threeway-csv") { await compareThreeWay(true); return; }
  if ($("mode").value === "csv") { await compareCSV(); return; }
	if ($("mode").value === "dir") { await compareDirectory(); return; }
  const body = requestBody();
  if (!validateInputs(body)) return;
  ignoredHunks = new Set();
  resetSyncSelection();
  const ac = new AbortController();
  currentAbort = ac;
  const generation = beginRequest();
  $("cancel").hidden = false;
  lastData = null;
  lastComparedRequest = null;
  syncExportPatchVisibility();
  $("summary").hidden = true;
  showResultSkeleton();
  const started = Date.now();
  const tick = () => setStatus(t("comparing") + " " + ((Date.now() - started) / 1000).toFixed(1) + "s", "busy");
  tick();
  const timer = setInterval(tick, 100);
  try {
    const resp = await apiFetch("/api/diff", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal: ac.signal,
    });
    const data = await resp.json();
    if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
    // Drag and drop, folder-entry clicks, and sync-point edits all call
    // compare() directly, so a newer comparison can already be running (#128).
    if (!isCurrentRequest(generation)) return;
    lastData = data;
    lastComparedRequest = JSON.stringify(body);
    threeWayData = null;
    mergeChoices = new Map(); mergeDefault = null; mergeUndo = []; mergeRedo = [];
    setMergeMode(false); // every fresh diff opens in reading mode (#100)
    if (!$("mergeOutput").value) {
      const source = $("old").value.trim();
      $("mergeOutput").value = source ? source.replace(/(\.[^./\\]+)?$/, ".merged$1") : "merged.txt";
    }
    // Stop the elapsed ticker before rendering: renderResult yields between
    // slices, so the ticker would otherwise keep overwriting its progress.
    clearInterval(timer);
    await renderResult(data);
  } catch (err) {
    if (err.name === "AbortError") setStatus(t("cancelled"), "");
    else setStatus(String(err.message || err), "error");
  } finally {
    clearInterval(timer);
    $("cancel").hidden = true;
    currentAbort = null;
  }
}

// Public entry points take the exclusion lock; the run* functions hold the
// actual work. Wrapping here rather than at each button covers the callers
// that never touch a button — drag and drop, folder-entry clicks, sync-point
// edits, and the Enter key (#128).
async function compare() { return runExclusive("compare", runCompare); }
async function exportPatch() { return runExclusive("exportPatch", runExportPatch); }
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
  return document.querySelectorAll(selector);
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
  hit.scrollIntoView({ behavior: "smooth", block: "center" });
  setSearchCounter(searchIndex + 1, searchHits.length);
}

function stepSearch(delta) {
  if (!searchHits.length) return;
  searchIndex = (searchIndex + delta + searchHits.length) % searchHits.length;
  focusSearchHit();
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
  for (const control of root.querySelectorAll("input, select")) {
    if (!control.id) continue;
    defaultControlValues.set(control.id, control.type === "checkbox" ? control.checked : control.value);
  }
}

function changedControlCount(root) {
  let changed = 0;
  for (const control of root.querySelectorAll("input, select")) {
    if (!control.id || !defaultControlValues.has(control.id)) continue;
    const initial = defaultControlValues.get(control.id);
    const now = control.type === "checkbox" ? control.checked : control.value;
    if (now !== initial) changed++;
  }
  return changed;
}

// updateDetailsBadges keeps each collapsed group honest about what it hides.
function updateDetailsBadges() {
  for (const [groupID, badgeID] of [["csvAdvanced", "csvAdvancedBadge"], ["csvParsing", "csvParsingBadge"], ["engineTuning", "engineTuningBadge"]]) {
    const group = $(groupID), badge = $(badgeID);
    if (!group || !badge) continue;
    const changed = changedControlCount(group);
    badge.textContent = changed ? t("changedSettings", { count: changed }) : "";
    badge.hidden = changed === 0;
  }
}

// swapSides exchanges the two inputs (#90). Every comparison tool has this and
// ayame-diff did not: reversing a comparison meant retyping both paths.
//
// It swaps whichever pair is actually in use — the paths, or the pasted text
// when scratch mode is on — and re-runs only if a result is already showing, so
// the button never starts work the user did not ask for.
function swapSides() {
  const pairs = $("scratch").checked
    ? [["oldText", "newText"]]
    : [["old", "new"]];
  for (const [left, right] of pairs) {
    const a = $(left), b = $(right);
    [a.value, b.value] = [b.value, a.value];
  }
  // The inspection describes the previous pairing.
  csvInspection = null;
  $("inspection").textContent = "";
  $("keySetup").hidden = true;
  if (lastData || csvData || threeWayData || directoryData) compare();
}

// ---- In-app dialogs (#98, #99) ----
// window.confirm and window.alert cannot be styled or read alongside the UI
// they describe, and the merge path used confirm to ask about writing a file —
// blocking the page on a native prompt at the moment the user most needs to see
// what they are confirming.

// askConfirm resolves true when the user proceeds. It mirrors confirm()'s
// shape so the call sites stay a single awaited expression.
function askConfirm(message) {
  const dialog = $("confirmDialog");
  $("confirmMessage").textContent = message;
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

// SHORTCUTS is the single source for the help dialog, so the list cannot drift
// from what the handlers actually bind.
const SHORTCUTS = [
  ["Alt+↓ / Alt+↑", "shortcutNavigate"],
  ["Alt+Home / Alt+End", "shortcutFirstLast"],
  ["Alt+← / Alt+→", "shortcutChooseSide"],
  ["Alt+B", "shortcutChooseBase"],
  ["Ctrl+F", "shortcutSearch"],
  ["Enter / Shift+Enter", "shortcutSearchStep"],
  ["Esc", "shortcutClose"],
  ["Ctrl+Enter", "shortcutCompare"],
];

function showShortcuts() {
  const list = $("shortcutsList");
  list.innerHTML = "";
  for (const [keys, key] of SHORTCUTS) {
    const term = document.createElement("dt");
    term.textContent = keys;
    const description = document.createElement("dd");
    description.textContent = t(key);
    list.append(term, description);
  }
  const opener = document.activeElement;
  $("shortcutsDialog").addEventListener("close", () => {
    if (opener && typeof opener.focus === "function") opener.focus();
  }, { once: true });
  $("shortcutsDialog").showModal();
}

// jumpToKind moves to the next hunk of one kind, cycling through them so
// repeated clicks walk the group rather than sticking on the first (#110).
let lastJumpKind = "";
let lastJumpIndex = -1;

function jumpToKind(kind) {
  const hunks = lastData?.hunks || [];
  const matches = [];
  for (let i = 0; i < hunks.length; i++) {
    if (hunks[i].kind === kind && !ignoredHunks.has(i)) matches.push(i);
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
	ignoreEOL: $("ignoreEOL").checked,
	ignoreTrailingEOL: $("ignoreTrailingEOL").checked,
	lineFilters: $("lineFilters").value.split(/\r?\n/).map((value) => value.trim()).filter(Boolean),
    whitespace: $("whitespace").value,
    detectMoves: $("detectMoves").checked,
    moveMinLines: Math.max(1, Number($("moveMinLines").value) || 2),
    syncPoints: syncPoints.map((point) => ({ ...point })),
  };
}

function activeFilters() {
	const filters = [];
	if ($("ignoreCase").checked) filters.push(t("ignoreCase"));
	if ($("whitespace").value !== "none") filters.push(`${t("whitespace")}: ${$("whitespace").value}`);
	if ($("ignoreEOL").checked) filters.push(t("ignoreEOL"));
	if ($("ignoreTrailingEOL").checked) filters.push(t("ignoreTrailingEOL"));
	for (const pattern of $("lineFilters").value.split(/\r?\n/).map((value) => value.trim()).filter(Boolean))
	  filters.push(`/${pattern}/`);
	return filters;
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
  body.ignoredHunks = [...ignoredHunks].sort((a, b) => a - b);
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
	const csv = $("mode").value === "csv" || $("mode").value === "threeway-csv";
	const threeway = $("mode").value === "threeway" || $("mode").value === "threeway-csv";
	const directory = $("mode").value === "dir";
	const structured = csv || directory;
  $("numericWrap").hidden = !sorted;
  $("reverseWrap").hidden = !sorted;
	$("csvOptions").hidden = !csv;
	$("exportCSV").hidden = $("mode").value === "threeway-csv";
	$("projectPath").closest(".project-actions").hidden = $("mode").value === "threeway-csv";
	$("basePathRow").hidden = !threeway;
	$("dirOptions").hidden = !directory;
	$("scratch").closest("label").hidden = structured;
	if (structured && $("scratch").checked) { $("scratch").checked = false; applyScratch(); }
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

function syncExportPatchVisibility() {
  const currentRequest = $("mode").value === "text" ? JSON.stringify(requestBody()) : null;
  $("exportPatch").hidden = !lastData || !lastComparedRequest || currentRequest !== lastComparedRequest;
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
  if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
  $("browserPath").value = data.Path || data.path; $("browserUp").dataset.path = data.Parent || data.parent;
  const entries = $("browserEntries"); entries.innerHTML = "";
  for (const item of (data.Entries || data.entries || [])) {
    const button = document.createElement("button"); button.type = "button"; button.className = item.directory ? "directory" : "file";
    button.textContent = `${item.directory ? "📁" : "📄"} ${item.Name || item.name}`;
    const itemPath = item.Path || item.path;
    button.addEventListener("click", async () => { if (item.directory) { await loadBrowser(itemPath); } else { $(browserTarget).value = itemPath; csvInspection = null; $("fileBrowser").close(); updateCSVReview(); } });
    entries.append(button);
  }
}

async function openBrowser(target) {
  browserTarget = target;
  $("fileBrowser").showModal();
	$("chooseFolder").hidden = $("mode").value !== "dir";
  try { await loadBrowser($(target).value ? $(target).value.replace(/[\\/][^\\/]*$/, "") : ""); }
  catch (err) { setStatus(String(err.message || err), "error"); }
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
function rerenderForDisplayChange() {
  applyDisplayPreferences();
  if (threeWayData) renderThreeWay(threeWayData, threeWayData.csvMode);
  else if (lastData) renderResult(lastData);
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
  if (!response.ok) throw new Error(data.error || `HTTP ${response.status}`);
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
$("exportPatch").addEventListener("click", exportPatch);
$("inspectCSV").addEventListener("click", inspectCSV);
$("exportCSV").addEventListener("click", exportCSV);
$("saveProject").addEventListener("click", saveProject);
$("loadProject").addEventListener("click", loadProject);
$("recentProjects").addEventListener("change", async () => { if ($("recentProjects").value !== "") { const body = recentComparisons()[Number($("recentProjects").value)]; if (body.mode === "dir") applyDirectoryProject(body); else await applyCSVProject(body); } });
$("cancel").addEventListener("click", () => { if (currentAbort) currentAbort.abort(); });
$("mode").addEventListener("change", syncModeOpts);
$("detectMoves").addEventListener("change", syncMoveMinLines);
$("setup").addEventListener("input", syncExportPatchVisibility);
$("setup").addEventListener("change", syncExportPatchVisibility);
$("keyMode").addEventListener("change", syncKeyMode);
$("columnSearch").addEventListener("input", filterColumns);
$("selectAllColumns").addEventListener("click", () => { document.querySelectorAll("#columnList .column-choice:not([hidden]) input").forEach((input) => { input.checked = true; }); updateCSVReview(); });
$("invertColumns").addEventListener("click", () => { document.querySelectorAll("#columnList .column-choice:not([hidden]) input").forEach((input) => { input.checked = !input.checked; }); updateCSVReview(); });
$("changedColumnsOnly").addEventListener("change", () => { if (csvData) { csvPage = 0; renderCSV(csvData); } });
document.querySelectorAll(".browse").forEach((button) => button.addEventListener("click", () => openBrowser(button.dataset.target)));
$("browserGo").addEventListener("click", async () => { try { await loadBrowser($("browserPath").value); } catch (err) { setStatus(String(err.message || err), "error"); } });
$("browserUp").addEventListener("click", async () => { try { await loadBrowser($("browserUp").dataset.path); } catch (err) { setStatus(String(err.message || err), "error"); } });
$("chooseFolder").addEventListener("click", () => { if (browserTarget) $(browserTarget).value = $("browserPath").value; $("fileBrowser").close(); });
$("dirStatus").addEventListener("change", () => { if (directoryData) renderDirectory(directoryData, directoryBody); });
$("dirPreview").addEventListener("click", previewDirectoryFilter);
$("saveDirProject").addEventListener("click", saveDirectoryProject);
$("loadDirProject").addEventListener("click", loadDirectoryProject);
$("browserPath").addEventListener("keydown", (event) => { if (event.key === "Enter") { event.preventDefault(); $("browserGo").click(); } });
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
$("firstDiff").addEventListener("click", () => { const active = activeHunkIndexes(); if (active.length) jumpToHunk(active[0]); });
$("prevDiff").addEventListener("click", () => stepHunk(-1));
$("nextDiff").addEventListener("click", () => stepHunk(1));
$("lastDiff").addEventListener("click", () => { const active = activeHunkIndexes(); if (active.length) jumpToHunk(active[active.length - 1]); });
$("addSync").addEventListener("click", addSyncPoint);
$("clearSync").addEventListener("click", clearSyncPoints);
$("allLeft").addEventListener("click", () => mutateMerge(() => { mergeDefault = "left"; if (threeWayData) threeWayData.events.filter((item) => item.kind === "conflict").forEach((item) => mergeChoices.set(item.id, "left")); else if (csvData && $("mode").value === "csv") csvData.differences.forEach((item) => mergeChoices.set(item.id, "left")); else lastData?.hunks.forEach((_, index) => mergeChoices.set(index, "left")); }));
$("allRight").addEventListener("click", () => mutateMerge(() => { mergeDefault = "right"; if (threeWayData) threeWayData.events.filter((item) => item.kind === "conflict").forEach((item) => mergeChoices.set(item.id, "right")); else if (csvData && $("mode").value === "csv") csvData.differences.forEach((item) => mergeChoices.set(item.id, "right")); else lastData?.hunks.forEach((_, index) => mergeChoices.set(index, "right")); }));
$("allBase").addEventListener("click", () => mutateMerge(() => { mergeDefault = "base"; threeWayData?.events.filter((item) => item.kind === "conflict").forEach((item) => mergeChoices.set(item.id, "base")); }));
$("mergeMode").addEventListener("click", () => { setMergeMode(!mergeMode); updateMergeUI(); });
$("mergeUndo").addEventListener("click", undoMerge);
$("mergeRedo").addEventListener("click", redoMerge);
$("saveMerge").addEventListener("click", saveMergeResult);
$("navHelp").addEventListener("click", showShortcuts);
document.addEventListener("keydown", (event) => {
  if (!event.altKey || event.ctrlKey || event.metaKey || !lastData?.hunks?.length) return;
  let target = null;
  const active = activeHunkIndexes();
  if (event.key === "ArrowLeft" || event.key === "ArrowRight" || (threeWayData && event.key.toLowerCase() === "b")) {
    event.preventDefault(); const index = currentHunk >= 0 ? currentHunk : active[0];
    const key = threeWayData?.events?.[index]?.id ?? index;
    if (index != null) chooseMerge(key, event.key === "ArrowLeft" ? "left" : (event.key === "ArrowRight" ? "right" : "base"));
    return;
  } else if (event.key === "ArrowDown") { event.preventDefault(); stepHunk(1); return; }
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
// Throttled like scroll above: a resize fires continuously while dragging, and
// each call forces a layout read (#155).
window.addEventListener("resize", () => {
  if (viewportFrame) return;
  viewportFrame = requestAnimationFrame(() => { viewportFrame = 0; updateMinimapViewport(); });
});
// In-result search (#118). Ctrl+F is intercepted only when there is a result to
// search; otherwise the browser's own find is left alone.
document.addEventListener("keydown", (event) => {
  const findKey = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "f";
  if (findKey && $("result").children.length) {
    event.preventDefault();
    openSearch();
    return;
  }
  if (event.key === "Escape" && searchOpen()) {
    event.preventDefault();
    closeSearch();
    return;
  }
  if (event.key === "Enter" && searchOpen() && document.activeElement === $("searchInput")) {
    event.preventDefault();
    stepSearch(event.shiftKey ? -1 : 1);
  }
});
$("searchInput").addEventListener("input", scheduleSearch);
for (const id of ["searchCase", "searchRegex", "searchChangedOnly"]) $(id).addEventListener("change", runSearch);
$("searchNext").addEventListener("click", () => stepSearch(1));
$("searchPrev").addEventListener("click", () => stepSearch(-1));
$("searchClose").addEventListener("click", closeSearch);
$("swapSides").addEventListener("click", swapSides);
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
updateDetailsBadges();
$("theme").addEventListener("change", () => applyTheme($("theme").value));
$("scheme").addEventListener("change", () => applyScheme($("scheme").value));
$("wrap").addEventListener("change", () => applyWrap($("wrap").checked));
$("showWs").addEventListener("change", () => {
  localStorage.setItem("ayame-showws", $("showWs").checked ? "1" : "0");
  rerenderForDisplayChange();
});
$("syntax").addEventListener("change", () => {
  localStorage.setItem("ayame-syntax", $("syntax").checked ? "1" : "0");
  applyDisplayPreferences();
});
$("word").addEventListener("change", rerenderForDisplayChange);
for (const input of document.querySelectorAll("#csvOptions input, #csvOptions select")) input.addEventListener("change", updateCSVReview);
for (const id of ["base", "old", "new", "hasHeader", "alignColumns", "leftFormat", "rightFormat", "leftParser", "rightParser", "leftDelimiter", "rightDelimiter", "lazyQuotes", "trimLeadingSpace"]) {
	$(id).addEventListener("change", () => { csvInspection = null; $("inspection").textContent = ""; $("keySetup").hidden = true; });
}
function applyScratch() {
  const on = $("scratch").checked;
  $("paths").hidden = on;
  $("scratchArea").hidden = !on;
}
$("scratch").addEventListener("change", applyScratch);
applyScratch();
applyScheme(localStorage.getItem("ayame-scheme") || "default");
applyTheme(localStorage.getItem("ayame-theme") || "system");
applyWrap(localStorage.getItem("ayame-wrap") !== "0");
$("showWs").checked = localStorage.getItem("ayame-showws") === "1";
$("syntax").checked = localStorage.getItem("ayame-syntax") !== "0";
applyDisplayPreferences();
$("lang").addEventListener("click", () => applyLang(lang === "ja" ? "en" : "ja"));
syncModeOpts();
syncPatchOpts();
applyLang(lang);

const launch = new URLSearchParams(location.search);
if (launch.has("base")) $("base").value = launch.get("base");
if (launch.has("old")) $("old").value = launch.get("old");
if (launch.has("new")) $("new").value = launch.get("new");
if (["text", "sorted", "csv", "threeway", "threeway-csv", "dir"].includes(launch.get("mode"))) $("mode").value = launch.get("mode");
if (launch.has("base") || launch.has("old") || launch.has("new")) { csvInspection = null; syncModeOpts(); }
const launchReady = $("old").value && $("new").value && (!$("basePathRow").hidden ? $("base").value : true);
if (launch.get("autorun") === "1" && launchReady) queueMicrotask(compare);

apiFetch("/api/health")
  .then((r) => r.json())
  .then((d) => { if (d.version) $("version").textContent = d.version; })
  .catch(() => {});
