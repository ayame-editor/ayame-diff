// Pure folder-result helpers shared by the browser and the dependency-free
// Node test harness. DOM construction stays in app.js; hierarchy, metadata
// formatting, and one-sided file requests are deterministic here.
(function (root) {
  "use strict";

  const DIR_MARKERS = { added: "+", removed: "−", changed: "~", same: "=" };
  const DIR_AUTO_EXPAND_LIMIT = 200;

  function dirNode(name = "", path = "") {
    return {
      name,
      path,
      dirs: new Map(),
      files: [],
      total: 0,
      counts: { added: 0, removed: 0, changed: 0, same: 0 },
    };
  }

  // The directory API returns a flat entry list. Folder nodes are the path
  // prefixes shared by those entries, with counts accumulated at every level
  // so a closed folder still says what it contains.
  function buildDirTree(entries) {
    const rootNode = dirNode();
    for (const entry of entries) {
      const parts = entry.path.split("/");
      const fileName = parts.pop();
      let node = rootNode;
      node.total++;
      if (Object.hasOwn(node.counts, entry.status)) node.counts[entry.status]++;
      for (const part of parts) {
        let child = node.dirs.get(part);
        if (!child) {
          child = dirNode(part, node.path ? `${node.path}/${part}` : part);
          node.dirs.set(part, child);
        }
        node = child;
        node.total++;
        if (Object.hasOwn(node.counts, entry.status)) node.counts[entry.status]++;
      }
      node.files.push({ ...entry, name: fileName });
    }
    return rootNode;
  }

  function formatBytes(n) {
    if (typeof n !== "number" || !Number.isFinite(n) || n < 0) return "";
    if (n < 1024) return `${n} B`;
    const units = ["KB", "MB", "GB", "TB"];
    let value = n / 1024;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
      value /= 1024;
      unit++;
    }
    return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[unit]}`;
  }

  function formatStamp(iso) {
    if (!iso) return "";
    const at = new Date(iso);
    if (Number.isNaN(at.getTime())) return "";
    const pad = (value) => String(value).padStart(2, "0");
    return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())} ${pad(at.getHours())}:${pad(at.getMinutes())}`;
  }

  function dirEntrySize(entry) {
    if (entry.status === "removed") return formatBytes(entry.old_size);
    if (entry.status === "changed" && entry.old_size !== entry.new_size) {
      return `${formatBytes(entry.old_size)} → ${formatBytes(entry.new_size)}`;
    }
    return formatBytes(entry.new_size);
  }

  function dirEntryStamp(entry) {
    return formatStamp(entry.status === "removed" ? entry.old_mtime : entry.new_mtime || entry.old_mtime);
  }

  function joinDirectoryPath(rootPath, relativePath) {
    return `${rootPath.replace(/[\\/]$/, "")}/${relativePath}`;
  }

  // Added and removed entries have no file on one side. The nominal path is
  // retained as the diff label while the explicit absent flag tells the server
  // to compare an empty source instead of trying to open that path.
  function directoryEntryRequest(entry, oldRoot, newRoot) {
    return {
      old: joinDirectoryPath(oldRoot, entry.path),
      new: joinDirectoryPath(newRoot, entry.path),
      oldAbsent: entry.status === "added",
      newAbsent: entry.status === "removed",
    };
  }

  function filterDirectoryEntries(entries, statusFilter, query) {
    const needle = query.trim().toLocaleLowerCase();
    return entries.filter((entry) => {
      const statusMatches = statusFilter === "all"
        || (statusFilter === "different" ? entry.status !== "same" : entry.status === statusFilter);
      return statusMatches && (!needle || entry.path.toLocaleLowerCase().includes(needle));
    });
  }

  const api = {
    DIR_MARKERS,
    DIR_AUTO_EXPAND_LIMIT,
    buildDirTree,
    formatBytes,
    formatStamp,
    dirEntrySize,
    dirEntryStamp,
    directoryEntryRequest,
    filterDirectoryEntries,
  };
  root.AyameDirectory = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})(typeof globalThis !== "undefined" ? globalThis : window);
