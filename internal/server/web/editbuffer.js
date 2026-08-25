// The buffer behind an editable diff pane (#255).
//
// The diff response only carries hunk slices, so editing works on the whole
// file the server hands over separately: the buffer holds those lines, records
// which ones the user changed, and hands back the text the comparison runs on.
// Nothing is written until a save, and the save sends the file's own
// conventions back with it, so an edited file keeps its encoding, BOM and line
// endings.
//
// DOM-free on purpose, so it runs under node --test (#139).
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.AyameEditBuffer = api;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  function createEditBuffer(source) {
    const path = String(source?.path || "");
    const readOnly = Boolean(source?.readOnly);
    let profile = { ...(source?.profile || {}) };
    let stamp = { ...(source?.stamp || {}) };
    let original = [...(source?.lines || [])];
    let lines = [...original];
    const changed = new Set();

    function line(index) {
      return lines[index];
    }

    // setLine reports whether anything moved, so a caller can skip the
    // re-comparison an unchanged keystroke would otherwise trigger.
    function setLine(index, text) {
      if (readOnly) return false;
      if (!Number.isInteger(index) || index < 0 || index >= lines.length) return false;
      const value = String(text ?? "");
      if (lines[index] === value) return false;
      lines[index] = value;
      if (value === original[index]) changed.delete(index);
      else changed.add(index);
      return true;
    }

    function changedLines() {
      return [...changed].sort((a, b) => a - b);
    }

    // text() joins with "\n" regardless of the file's own terminator: the
    // comparison is about content, and the terminator is restored on save.
    function text() {
      return lines.join("\n");
    }

    // accept() takes what the server confirmed it wrote, so a saved buffer is
    // clean and the next stale-write check compares against the new stamp.
    function accept(next) {
      if (next?.lines) {
        original = [...next.lines];
        lines = [...next.lines];
      } else {
        original = [...lines];
      }
      if (next?.stamp) stamp = { ...next.stamp };
      if (next?.profile) profile = { ...next.profile };
      changed.clear();
    }

    function revert() {
      lines = [...original];
      changed.clear();
    }

    return {
      path: () => path,
      readOnly: () => readOnly,
      profile: () => ({ ...profile }),
      stamp: () => ({ ...stamp }),
      count: () => lines.length,
      line,
      lines: () => [...lines],
      original: () => [...original],
      setLine,
      isDirty: () => changed.size > 0,
      changedLines,
      text,
      accept,
      revert,
    };
  }

  // A comparison can only be edited when both sides are real files whose lines
  // map back to something writable. Pasted text has nowhere to save to, sorted
  // and CSV views reorder or reshape rows so a line number no longer addresses
  // a file line, and a folder comparison has no single file at all.
  function editableComparison(mode, scratch) {
    return !scratch && mode === "text";
  }

  return { createEditBuffer, editableComparison };
});
