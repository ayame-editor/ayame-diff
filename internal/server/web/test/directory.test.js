"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const {
  buildDirTree,
  formatBytes,
  formatStamp,
  dirEntrySize,
  dirEntryStamp,
  directoryEntryRequest,
  filterDirectoryEntries,
  sortDirectoryEntries,
  locationOf,
  formatEpochNanos,
  rememberPlace,
} = require("../directory.js");

test("flat entries become a counted folder hierarchy", () => {
  const tree = buildDirTree([
    { path: "src/a.txt", status: "changed" },
    { path: "src/nested/b.txt", status: "added" },
    { path: "README.md", status: "same" },
  ]);

  assert.equal(tree.total, 3);
  assert.deepEqual(tree.counts, { added: 1, removed: 0, changed: 1, same: 1 });
  assert.equal(tree.files[0].name, "README.md");

  const src = tree.dirs.get("src");
  assert.equal(src.total, 2);
  assert.deepEqual(src.counts, { added: 1, removed: 0, changed: 1, same: 0 });
  assert.equal(src.files[0].name, "a.txt");
  assert.equal(src.dirs.get("nested").files[0].name, "b.txt");
});

test("byte columns use compact binary units and missing metadata stays blank", () => {
  assert.equal(formatBytes(0), "0 B");
  assert.equal(formatBytes(1536), "1.5 KB");
  assert.equal(formatBytes(10 * 1024), "10 KB");
  assert.equal(formatBytes(-1), "");
  assert.equal(formatBytes(undefined), "");

  assert.equal(dirEntrySize({ status: "added", old_size: -1, new_size: 1536 }), "1.5 KB");
  assert.equal(dirEntrySize({ status: "removed", old_size: 12, new_size: -1 }), "12 B");
  assert.equal(dirEntrySize({ status: "changed", old_size: 12, new_size: 20 }), "12 B → 20 B");
});

test("timestamps are list-friendly and use the existing side", () => {
  const iso = "2026-07-19T12:34:00Z";
  const at = new Date(iso);
  const pad = (value) => String(value).padStart(2, "0");
  const expected = `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())} ${pad(at.getHours())}:${pad(at.getMinutes())}`;

  assert.equal(formatStamp(iso), expected);
  assert.equal(formatStamp("not-a-date"), "");
  assert.equal(dirEntryStamp({ status: "removed", old_mtime: iso, new_mtime: "" }), expected);
  assert.equal(dirEntryStamp({ status: "added", old_mtime: "", new_mtime: iso }), expected);
});

test("directory rows produce explicit one-sided diff requests", () => {
  assert.deepEqual(
    directoryEntryRequest({ path: "new/file.txt", status: "added" }, "/old/", "/new"),
    {
      old: "/old/new/file.txt",
      new: "/new/new/file.txt",
      oldAbsent: true,
      newAbsent: false,
    },
  );
  assert.deepEqual(
    directoryEntryRequest({ path: "gone.txt", status: "removed" }, "C:\\old\\", "C:\\new\\"),
    {
      old: "C:\\old/gone.txt",
      new: "C:\\new/gone.txt",
      oldAbsent: false,
      newAbsent: true,
    },
  );
});

test("status and path search filter the flat entries before building the tree", () => {
  const entries = [
    { path: "src/NewFile.go", status: "added" },
    { path: "src/changed.go", status: "changed" },
    { path: "docs/guide.md", status: "same" },
  ];
  assert.deepEqual(
    filterDirectoryEntries(entries, "different", "SRC/").map((entry) => entry.path),
    ["src/NewFile.go", "src/changed.go"],
  );
  assert.deepEqual(
    filterDirectoryEntries(entries, "added", "file").map((entry) => entry.path),
    ["src/NewFile.go"],
  );
  assert.deepEqual(
    filterDirectoryEntries(entries, "all", "guide").map((entry) => entry.path),
    ["docs/guide.md"],
  );
});

test("the flat view lists files in path order without mutating the input", () => {
  const entries = [
    { path: "src/z.go", status: "changed" },
    { path: "README.md", status: "same" },
    { path: "src/a.go", status: "added" },
  ];
  const sorted = sortDirectoryEntries(entries);
  assert.deepEqual(sorted.map((entry) => entry.path), ["README.md", "src/a.go", "src/z.go"]);
  assert.deepEqual(entries.map((entry) => entry.path), ["src/z.go", "README.md", "src/a.go"]);
  assert.deepEqual(sortDirectoryEntries([]), []);
});

test("the Location column holds the parent folder, empty at the root", () => {
  assert.equal(locationOf("src/nested/a.go"), "src/nested");
  assert.equal(locationOf("src/a.go"), "src");
  assert.equal(locationOf("README.md"), "");
  assert.equal(locationOf(""), "");
});

test("nanosecond stamps become readable local times", () => {
  const stamp = formatEpochNanos("1700000000000000000");
  assert.match(stamp, /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/);
  assert.equal(new Date(1700000000000).getHours(), Number(stamp.slice(11, 13)));
  assert.equal(formatEpochNanos(""), "");
  assert.equal(formatEpochNanos("nope"), "");
  assert.equal(formatEpochNanos("0"), "");
});

test("recent places are newest-first, deduplicated and bounded", () => {
  assert.deepEqual(rememberPlace([], "/a"), ["/a"]);
  assert.deepEqual(rememberPlace(["/a", "/b"], "/c"), ["/c", "/a", "/b"]);
  assert.deepEqual(rememberPlace(["/a", "/b"], "/b"), ["/b", "/a"]);
  assert.deepEqual(rememberPlace(["/a", "/b", "/c"], "/d", 2), ["/d", "/a"]);
  // An empty path does not add an entry, and a non-array is tolerated.
  assert.deepEqual(rememberPlace(["/a"], ""), ["/a"]);
  assert.deepEqual(rememberPlace(null, "/a"), ["/a"]);
});
