// Visual filter builder condition model and compiler (#129). Pure: no DOM, no
// network. It mirrors the engine's row-filter semantics so the preview the UI
// shows and the comparison the server runs agree; the node:test suite covers
// both the model→wire compilation and the matching rules.
(function (root) {
  "use strict";

  // The closed operator vocabulary the UI offers, with how many value inputs an
  // operator needs and whether it is a string or an ordering comparison.
  const OPERATORS = [
    { op: "eq", values: 1, kind: "text" },
    { op: "ne", values: 1, kind: "text" },
    { op: "contains", values: 1, kind: "text" },
    { op: "starts", values: 1, kind: "text" },
    { op: "ends", values: 1, kind: "text" },
    { op: "gt", values: 1, kind: "compare" },
    { op: "ge", values: 1, kind: "compare" },
    { op: "lt", values: 1, kind: "compare" },
    { op: "le", values: 1, kind: "compare" },
    { op: "between", values: 2, kind: "compare" },
    { op: "empty", values: 0, kind: "text" },
    { op: "not_empty", values: 0, kind: "text" },
    { op: "regex", values: 1, kind: "text" },
  ];

  function operatorInfo(op) {
    return OPERATORS.find((entry) => entry.op === op) || OPERATORS[0];
  }

  function newCondition(op = "eq") {
    return { column: "", index: 0, byIndex: false, op, value: "", value2: "", not: false };
  }

  function newGroup() {
    return { match: "all", ignoreCase: false, not: false, conditions: [], groups: [] };
  }

  function newFilter() {
    return newGroup();
  }

  function asString(value) {
    return value == null ? "" : String(value);
  }

  // normalizeFilter returns a safe clone of an arbitrary parsed value. Project
  // files and URL state are untrusted, so every field is coerced and arrays are
  // rebuilt instead of trusted by reference.
  function normalizeFilter(raw) {
    const source = raw && typeof raw === "object" ? raw : {};
    const group = {
      match: source.match === "any" ? "any" : "all",
      ignoreCase: Boolean(source.ignoreCase),
      not: Boolean(source.not),
      conditions: [],
      groups: [],
    };
    if (Array.isArray(source.conditions)) {
      for (const item of source.conditions) {
        if (!item || typeof item !== "object") continue;
        const op = operatorInfo(asString(item.op)).op;
        group.conditions.push({
          column: asString(item.column),
          index: Number.isInteger(item.index) && item.index >= 0 ? item.index : 0,
          byIndex: Boolean(item.byIndex),
          op,
          value: asString(item.value),
          value2: asString(item.value2),
          not: Boolean(item.not),
        });
      }
    }
    if (Array.isArray(source.groups)) {
      for (const item of source.groups) {
        if (!item || typeof item !== "object") continue;
        group.groups.push(normalizeFilter(item));
      }
    }
    return group;
  }

  function conditionComplete(condition, nameMode) {
    if (!nameMode && !condition.column && !condition.byIndex) return false;
    const info = operatorInfo(condition.op);
    return info.values === 0 || asString(condition.value).length > 0;
  }

  // pruneFilter removes incomplete conditions and empty groups. A never-touched
  // builder therefore compiles to null rather than an always-true predicate.
  function pruneFilter(filter, nameMode = false) {
    if (!filter) return null;
    const group = normalizeFilter(filter);
    group.conditions = group.conditions.filter((condition) => conditionComplete(condition, nameMode));
    group.groups = group.groups.map((child) => pruneFilter(child, nameMode)).filter(Boolean);
    if (!group.conditions.length && !group.groups.length) return null;
    return group;
  }

  function filterIsEmpty(filter, nameMode = false) {
    return pruneFilter(filter, nameMode) === null;
  }

  function validateFilter(filter, options = {}) {
    const nameMode = Boolean(options.nameMode);
    const columns = Array.isArray(options.columns) ? options.columns : [];
    const hasHeader = Boolean(options.hasHeader);
    const errors = [];
    if (!filter) return errors;

    function walk(group, depth) {
      if (depth > 8) {
        errors.push({ code: "too_deep" });
        return;
      }
      for (const condition of group.conditions) {
        const info = operatorInfo(condition.op);
        if (!nameMode) {
          if (!condition.column && !condition.byIndex) errors.push({ code: "missing_column" });
          else if (hasHeader && condition.column && !columns.includes(condition.column)) {
            errors.push({ code: "unknown_column", detail: condition.column });
          }
        }
        if (info.values >= 1 && asString(condition.value).length === 0) errors.push({ code: "missing_value", detail: condition.op });
        if (info.values === 2 && asString(condition.value2).length === 0) errors.push({ code: "missing_value", detail: condition.op });
        if (condition.op === "regex") {
          try {
            new RegExp(condition.value, group.ignoreCase ? "i" : "");
          } catch (error) {
            errors.push({ code: "invalid_regex", detail: condition.value });
          }
        }
      }
      for (const child of group.groups) walk(child, depth + 1);
    }
    walk(normalizeFilter(filter), 1);
    return errors;
  }

  // compileFilter turns the UI model into the engine's wire shape, resolving
  // headerless conditions to indices and lowercasing nothing (the engine owns
  // case folding via ignore_case). Incomplete conditions are dropped, so the
  // request never carries a half-typed row.
  function compileFilter(filter, options = {}) {
    const nameMode = Boolean(options.nameMode);
    const hasHeader = Boolean(options.hasHeader);
    const pruned = pruneFilter(filter, nameMode);
    if (!pruned) return null;

    function emit(group) {
      const out = { match: group.match };
      if (group.ignoreCase) out.ignore_case = true;
      if (group.not) out.not = true;
      const conditions = [];
      for (const condition of group.conditions) {
        const compiled = { op: condition.op };
        if (!nameMode) {
          if (hasHeader) compiled.column = condition.column;
          else {
            compiled.by_index = true;
            compiled.index = condition.index;
          }
        }
        if (asString(condition.value).length) compiled.value = condition.value;
        if (asString(condition.value2).length) compiled.value2 = condition.value2;
        if (condition.not) compiled.not = true;
        conditions.push(compiled);
      }
      if (conditions.length) out.conditions = conditions;
      const groups = group.groups.map(emit);
      if (groups.length) out.groups = groups;
      return out;
    }
    return emit(pruned);
  }

  function describeCondition(condition, columns, hasHeader) {
    const subject = condition.byIndex || !hasHeader
      ? `#${condition.index}`
      : (condition.column || "?");
    const info = operatorInfo(condition.op);
    if (info.values === 0) return `${subject} ${condition.op}`;
    if (info.values === 2) return `${subject} ${condition.op} ${condition.value}..${condition.value2}`;
    return `${subject} ${condition.op} ${condition.value}`;
  }

  function summarizeFilter(filter, options = {}) {
    const nameMode = Boolean(options.nameMode);
    const columns = Array.isArray(options.columns) ? options.columns : [];
    const hasHeader = Boolean(options.hasHeader);
    const pruned = pruneFilter(filter, nameMode);
    if (!pruned) return "";

    function describe(group) {
      const parts = group.conditions.map((condition) => describeCondition(condition, columns, hasHeader));
      for (const child of group.groups) parts.push(`(${describe(child)})`);
      const joined = parts.join(group.match === "any" ? " | " : " & ");
      return group.not ? `not ${joined}` : joined;
    }
    return describe(pruned);
  }

  function toNumber(value) {
    const text = asString(value).trim();
    if (!text) return NaN;
    const number = Number(text);
    return Number.isFinite(number) ? number : NaN;
  }

  function compareCondition(condition, value, ignoreCase) {
    const literal = ignoreCase ? asString(condition.value).toLowerCase() : asString(condition.value);
    const literal2 = ignoreCase ? asString(condition.value2).toLowerCase() : asString(condition.value2);
    const text = ignoreCase ? asString(value).toLowerCase() : asString(value);
    switch (condition.op) {
      case "gt": case "ge": case "lt": case "le": case "between": {
        const left = toNumber(text);
        const right = toNumber(literal);
        if (!Number.isNaN(left) && !Number.isNaN(right)) return compareNumbers(condition.op, left, right, toNumber(literal2));
        return compareStrings(condition.op, text, literal, literal2);
      }
      default:
        return false;
    }
  }

  function compareNumbers(op, value, a, b) {
    switch (op) {
      case "gt": return value > a;
      case "ge": return value >= a;
      case "lt": return value < a;
      case "le": return value <= a;
      case "between": {
        const [low, high] = a > b ? [b, a] : [a, b];
        return value >= low && value <= high;
      }
      default: return false;
    }
  }

  function compareStrings(op, value, a, b) {
    switch (op) {
      case "gt": return value > a;
      case "ge": return value >= a;
      case "lt": return value < a;
      case "le": return value <= a;
      case "between": {
        const [low, high] = a > b ? [b, a] : [a, b];
        return value >= low && value <= high;
      }
      default: return false;
    }
  }

  function testCondition(condition, value, ignoreCase) {
    const text = ignoreCase ? asString(value).toLowerCase() : asString(value);
    const literal = ignoreCase ? asString(condition.value).toLowerCase() : asString(condition.value);
    let result;
    switch (condition.op) {
      case "eq": result = text === literal; break;
      case "ne": result = text !== literal; break;
      case "contains": result = text.includes(literal); break;
      case "starts": result = text.startsWith(literal); break;
      case "ends": result = text.endsWith(literal); break;
      case "empty": result = asString(value) === ""; break;
      case "not_empty": result = asString(value) !== ""; break;
      case "regex": {
        try {
          result = new RegExp(condition.value, ignoreCase ? "i" : "").test(asString(value));
        } catch { result = false; }
        break;
      }
      default:
        result = compareCondition(condition, value, ignoreCase);
    }
    return condition.not ? !result : result;
  }

  function valueFor(condition, row, columns, hasHeader) {
    if (condition.byIndex || !hasHeader) return row[condition.index];
    const index = columns.indexOf(condition.column);
    return index < 0 ? undefined : row[index];
  }

  function testGroup(group, row, columns, hasHeader, inheritedIgnoreCase) {
    const ignoreCase = inheritedIgnoreCase || Boolean(group.ignoreCase);
    let any = false;
    for (const condition of group.conditions) {
      const matched = testCondition(condition, valueFor(condition, row, columns, hasHeader), ignoreCase);
      if (group.match === "any") {
        if (matched) { any = true; break; }
      } else if (!matched) {
        return group.not ? true : false;
      }
    }
    if (group.match === "all") {
      for (const child of group.groups) {
        if (!testGroup(child, row, columns, hasHeader, ignoreCase)) return group.not ? true : false;
      }
      return group.not ? false : true;
    }
    if (!any) {
      for (const child of group.groups) {
        if (testGroup(child, row, columns, hasHeader, ignoreCase)) { any = true; break; }
      }
    }
    return group.not ? !any : any;
  }

  // matchesRow evaluates a filter against one row. It mirrors the engine,
  // including AND/OR short-circuiting and whole-group negation, so the client
  // preview and the server comparison cannot drift.
  function matchesRow(filter, row, columns = [], hasHeader = true, nameMode = false) {
    const pruned = pruneFilter(filter, nameMode);
    if (!pruned) return true;
    // A column-name filter is evaluated by the server; matching a row against a
    // name filter is not meaningful.
    if (nameMode) return true;
    return testGroup(pruned, Array.isArray(row) ? row : [], columns, hasHeader, false);
  }

  function countConditions(filter) {
    if (!filter) return 0;
    let count = filter.conditions.length;
    for (const group of filter.groups) count += countConditions(group);
    return count;
  }

  const api = {
    OPERATORS,
    operatorInfo,
    newCondition,
    newGroup,
    newFilter,
    normalizeFilter,
    pruneFilter,
    filterIsEmpty,
    validateFilter,
    compileFilter,
    summarizeFilter,
    matchesRow,
    countConditions,
  };
  root.AyameRowFilter = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
