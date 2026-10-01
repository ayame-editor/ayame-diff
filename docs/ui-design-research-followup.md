<!-- i18n: language-switcher -->
[English](ui-design-research-followup.md) | [日本語](ui-design-research-followup.ja.md)

# Competitor UI research: unverified items (#284)

The 2026-07 UI design research left a set of behaviours unverified because they
belong to commercial or unreleased tools that were not available for hands-on
testing. This page is a **documentary follow-up** to issue
[#284](https://github.com/ayame-editor/ayame-diff/issues/284): it records what
official documentation, public source, and public trackers establish, what they
do not, and why.

!!! warning "Nothing here is hands-on"
    No item below was confirmed by running the application. Araxis Merge and
    Kaleidoscope require a licence; the WinXMerge desktop build and the
    WinXMerge WASM build could not be launched in this environment. Findings
    are marked **Documented** (stated by an official/source reference) or
    **Unverified** (not established). The checklist at the end lists what still
    needs a hands-on pass.

## Method and limitations

- Sources are the vendors' own user guides, release notes, help pages,
  repository READMEs/source, and public issue trackers (URLs inline and per
  section).
- Araxis Merge documentation was read for **Merge for Windows 2026.1**
  (released 2026-09-23). The macOS build has a different chrome (toolbar
  instead of a ribbon) and was not reviewed in the same depth.
- WinXMerge findings come from its **v0.51.0** README and source
  (`src/lib.rs`, `src/wasm.rs`, `Cargo.toml`). The live web build at
  <https://winxmerge.app> is a WASM/`<canvas>` application; the built-in
  browser was disconnected, so it was not loaded, and the WASM findings below
  are read from source rather than observed.
- Meld's undo behaviour is not in its help; it was read from the Meld source
  tree (`meld/filediff.py`, `meld/undo.py`).

## Araxis Merge

- Product/documentation: <https://www.araxis.com/merge/windows/index.en>
- Text comparison: <https://www.araxis.com/merge/windows/comparing-text-files.en>
- Folder comparison: <https://www.araxis.com/merge/windows/comparing-folders.en>
- Folder overview: <https://www.araxis.com/merge/windows/folder-comparison-overview.en>
- File overview: <https://www.araxis.com/merge/windows/file-comparison-overview.en>
- Ribbon: <https://www.araxis.com/merge/windows/merge-ribbon-interface.en>
- Three-way: <https://www.araxis.com/merge/windows/three-way-file-comparison-and-merging.en>
- FAQ: <https://www.araxis.com/merge/windows/frequently-asked-questions.en>
- Release notes: <https://www.araxis.com/merge/release-notes-2026.en>

| Question from #284 | What the public sources establish | Status |
|---|---|---|
| Setup shape (is there a start screen?) | **No separate start screen.** "By default, an empty new text comparison tab is automatically opened when you start Merge." New comparisons are created from the leftmost ribbon item's drop-down (New text comparison, `Ctrl+L`; New folder comparison, `Ctrl+D`) or the equivalent ribbon buttons. A fresh folder comparison defaults to the **Two-way with file comparison** split layout. | Documented |
| Where options live | **Two layers.** (1) Quick toggles in the ribbon **Options** menu, e.g. folder comparison `Show changes column`, `Show timestamps and sizes`, `Show executable file and product versions`, `Show Unicode code points`; text comparison `Add vertical padding to align changes`, line-wrapping, `Show line-detail panel`. (2) A full **Options…** dialog with pages for Application, File Comparisons, Text Comparisons (Display, Expressions, Line expressions, Line pairing, Editing, Syntax highlighting) and Folder Comparisons (Method, Launch behaviour, Filters, Method…). Some settings are registry-only (`MRUTabOrder`, `IOThrottleTime`). | Documented |
| Editability and recomputation | Either file can be edited **in place**; "The file comparison dynamically updates as you make changes." A modified-file indicator appears in the pane. Editing read-only files is disabled by default but can be enabled. The FAQ phrases large-file performance advice as "as I edit files", confirming an edit → recompute loop. | Documented |
| Three-way placement | Three panes side by side; "Merge is designed to be most effective when the **common ancestor file is used as the centre file**", with the two modified versions on either side. Switch two-way/three-way from the ribbon; a **Three-way with file comparison** split view also exists. | Documented |
| Per-pane path display | Each pane has an entry field above it showing the file/folder path, with Browse and Show-history buttons; a **Versions** button lists other available revisions. | Documented |
| Folder results: tree or flat list | A **flat results list** with background colours per status, not a tree; folder rows expand/collapse and double-clicking a folder row opens a separate comparison window. The split view shows the selected row's files underneath. | Documented |
| Windows ribbon vs legacy menus | Ribbon since Merge 2020; the guide keeps a separate "Merge 2019 and earlier" topic for the legacy menu/toolbar UI. | Documented |
| Large-file hangs | The docs describe many comparison-performance options but do not reproduce or acknowledge a specific hang. Community reports are not resolvable from documentation alone. | Unverified |

**Still unverified (Araxis):** exact layout geometry (pane proportions, centre-marker behaviour), whether every documented options page is reachable in the current UI, the exact read-only-edit toggle, whether inline edits and merge-button operations share one undo stack (undo is not documented), and a reproducible large-file hang. **Why:** commercial product; a licence and Windows/macOS install are required.

## WinXMerge

- Repository / README (v0.51.0): <https://github.com/masak1yu/winxmerge>
- Docs: <https://winxmerge-site.pages.dev/en/guides/introduction/>
- Quick start: <https://winxmerge-site.pages.dev/en/guides/quickstart/>
- Inline editing: <https://winxmerge-site.pages.dev/en/features/inline-editing/>
- Keyboard shortcuts: <https://winxmerge-site.pages.dev/en/reference/keyboard-shortcuts/>
- Web build: <https://winxmerge.app>
- Source used: [`src/wasm.rs`](https://github.com/masak1yu/winxmerge/blob/main/src/wasm.rs), [`src/lib.rs`](https://github.com/masak1yu/winxmerge/blob/main/src/lib.rs), [`Cargo.toml`](https://github.com/masak1yu/winxmerge/blob/main/Cargo.toml), [`index.html`](https://github.com/masak1yu/winxmerge/blob/main/index.html)

| Question from #284 | What the public sources establish | Status |
|---|---|---|
| UI shape (toolbar, setup) | Native menu bar; a **WinMerge-style single-row icon toolbar** (New, Open, Save, Undo/Redo, Rescan, Options, Navigation, Copy, Copy & Advance, Copy All, Ignore WS/Case); a tab bar; a status bar with a proportional diff bar graph; a bottom diff-detail pane; and a **location pane (minimap of diff positions)**. On launch a **WinMerge-style file-selection dialog with a recent-files list** appears (also `File → New → Text / Table / 3-way`). | Documented |
| WASM constraints / File System Access API | The web build "supports text input, file upload, clipboard paste, and diff navigation with stats"; desktop-only features (native file dialogs, syntax highlighting, folder compare, inline editing, archives, Excel, images) are excluded with `cfg(not(target_arch = "wasm32"))`. File opening creates a hidden `<input type="file" accept="…">` element and reads `Blob.text()` (`open_file_picker` in `src/wasm.rs`). `Cargo.toml` enables only `HtmlInputElement`, `File`, `FileList`, `Blob`, `Document`, `Element`, `EventTarget`, `Event`, `Window`, `console` for `web-sys`. **`showOpenFilePicker` / `showSaveFilePicker` / `FileSystemHandle` are not used**, so the web build does **not** use the File System Access API and cannot read or write local paths in place. | Documented (source) |
| Large-input behaviour | README claims "Performance optimizations for large files" on the desktop, but gives no size or timing. In the WASM build `on_compare` calls `compute_diff_with_options()` **synchronously on the main thread**; there is no Web Worker. A very large paste would therefore block the UI, but this was not measured. | Unverified |
| WinMerge keybinding compatibility | README claims "Same keyboard shortcuts" and the CLI uses WinMerge-compatible slash syntax (`/ignorews`, `/m`, `/t`, `/dl` …). The documented in-app defaults are `Alt+↑`/`Alt+↓` (previous/next diff), `Alt+Home`/`Alt+End`, `F2` bookmark, `F5` rescan, `Cmd/Ctrl+S` save, `Ctrl+Z`/`Ctrl+Shift+Z`, `Cmd+N`/`Cmd+T`/`Cmd+W`, `Cmd+F`, `Cmd+G`, `Cmd+M`. WinMerge's own manual lists `F7`/`F8` as the primary previous/next difference and `Shift+F7` for conflicts; those are not used here. Compatibility is therefore **partial** (shared `Alt+↑/↓`, save, undo, find) but differs on `F7`/`F8` and conflict navigation. | Partially documented |
| Undo/redo | The desktop README lists undo/redo for merge operations and inline edits. `src/wasm.rs` registers no `on_undo`/`on_redo` callback, so the web build appears to have no undo at all. | Documented (source) / needs confirmation in-browser |

**Still unverified (WinXMerge):** the rendered desktop UI and setup dialog, the actual WASM UI/undo state, large-input timing on both builds, drag-and-drop onto the web app, and a line-by-line comparison against WinMerge's default shortcuts. **Why:** the desktop build is not installed here and the in-app browser was disconnected, so <https://winxmerge.app> could not be exercised; those items require actually launching the build.

## Meld

- Help home: <https://help.gnome.org/meld/>
- Getting started comparing files: <https://help.gnome.org/meld/file-mode.html>
- Dealing with changes: <https://help.gnome.org/meld/file-changes.html>
- Things Meld doesn't do: <https://help.gnome.org/meld/missing-functionality.html>
- Source: <https://gitlab.gnome.org/GNOME/meld> (`meld/filediff.py`, `meld/undo.py`)

| Question from #284 | What the official sources say | Status |
|---|---|---|
| Is the setup page replaced by the result? | "Once you've selected your files, Meld will show them side-by-side." The selection view is replaced by the comparison editors rather than staying as a persistent panel. | Documented |
| Undo semantics (do change-bar operations and typed edits share a stack?) | **The help does not mention undo or redo at all.** Source inspection answers it: `Filediff` creates one `UndoSequence(self.textbuffer)` shared by all panes (`meld/filediff.py`); `on_text_insert_text`/`on_text_delete_range` add `BufferInsertionAction`/`BufferDeletionAction` for **every** buffer change, and change-bar merge actions (`copy_chunk`, pull-all, merge-all) modify the same buffers inside `begin_user_action()`/`end_user_action()`. `UndoSequence.begin_group()`/`end_group()` collapse multi-buffer edits into one logical `GroupAction`. So change-bar operations and typed edits **do share one undo stack per comparison**, with per-buffer checkpoints and grouped multi-file steps. | Unverified in docs; Documented in source |
| Live recompute | "Editing the files will cause the comparison to update on-the-fly." | Documented |
| Change-bar actions | Replace (default), delete (Shift), insert (Ctrl) via the arrows/cross icons in the central change bar. | Documented |

**Still unverified (Meld):** the exact undo granularity a user perceives (e.g. whether a single hammered keypress groups with the next change-bar action) and whether any change-bar action is excluded from undo. **Why:** source describes the mechanism, but the user-visible grouping and any GTK/GtkSourceView interference need a running Meld to confirm.

## KDiff3

- Handbook (SourceForge): <https://kdiff3.sourceforge.net/doc/index.html>
- Merging and the output window: <https://kdiff3.sourceforge.net/doc/merging.html>
- Interpreting the input windows: <https://kdiff3.sourceforge.net/doc/interpretinginformation.html>
- Navigation and editing: <https://kdiff3.sourceforge.net/doc/navigation.html>
- FAQ: <https://kdiff3.sourceforge.net/doc/faq.html>
- Current KDE docs: <https://docs.kde.org/stable_kf6/en/kdiff3/kdiff3/documentation.html>

| Question from #284 | What the official sources say | Status |
|---|---|---|
| Are the A/B/C input panes editable? | The docs describe the input windows for navigation, selection, copy/paste, manual diff alignment, and splitting/joining sections. Editing is only ever described for the **merge output editor**: "It is often helpful directly edit the merge output." and "In the merge output editor you can also use the other keys for editing." The input info line is described as containing "the editable filename" (the path, not the body). Input-body editing is **not documented**, and community answers state editing is only available in the output pane. | Unverified (docs silent) |
| Does output editing trigger a live recompute? | The output is a separate merge target below the diff; the only "immediately recalculate" behaviour documented is for **manual diff alignment** in the input panes. Output editing is not described as re-running the diff. | Unverified (docs silent) |
| Undo | **Explicitly absent.** FAQ 5.7: "Why does the editor in the merge result window not have an 'undo'-function?" — "This was too much effort until now. You can always restore a version from one source (A, B or C) by clicking the respective button." | Documented |
| Other constraints | Saving is disabled until all conflicts are resolved; the output remembers section boundaries, so it cannot be saved and resumed. | Documented |

**Still unverified (KDiff3):** whether the current KDE-Frameworks-6 build still refuses input-pane edits and still has no undo, and whether output edits ever re-run the diff. The FAQ and SourceForge handbook are old; confirm against the current release. **Why:** no KDiff3 install here.

## P4Merge (Helix Core Visual Merge Tool)

- P4Merge User Guide (r18.4 PDF): <https://ftp.perforce.com/perforce/r18.4/doc/manuals/p4merge.pdf>
- Product page: <https://www.perforce.com/products/helix-core-apps/merge-diff-tool-p4merge>

The r18.4 guide describes diff, edit, image, and merge workflows, but is silent on several #284 questions.

| Question from #284 | What the guide says | Status |
|---|---|---|
| Edit model | Editing is **not in-place**. In diff mode, toolbar buttons open a **separate edit pane at the bottom** for a writable workspace file in double-pane layout. In merge mode, "To edit the text in the merge result file, click on the text in the **bottom pane**". | Documented |
| Three-way layout | Top half: base file in the middle with the two changed versions either side; bottom half: the merge result. | Documented |
| Minimap / overview | **Nothing in the guide mentions a minimap, overview ruler, or scrollbar change markers.** Per #284, do not assume one exists. | Unverified (not documented) |
| Live recomputation | Not described. The guide only documents choosing chunks and editing the result. | Unverified (not documented) |
| Undo / redo | Not described anywhere in the guide. | Unverified (not documented) |
| Per-pane path display | The guide says "each file in its own pane" but does not document a per-pane path/filename label. | Unverified (not documented) |
| Current-version performance | The guide is r18.4 (2018); known hang/crash reports cited in #284 are from the macOS Mavericks/Qt4 era. Whether the current Qt5/current build still exhibits them is not established. | Unverified |

**Still unverified (P4Merge):** minimap/overview, live recompute, undo, per-pane path display, and current-version performance. **Why:** the official guide is silent and the tool is not installed; a current-version build and its release notes must be checked by hand.

## JetBrains IDEs (IntelliJ IDEA 2026.2)

- Diff Viewer: <https://www.jetbrains.com/help/idea/differences-viewer.html>
- Compare file/folder versions: <https://www.jetbrains.com/help/idea/comparing-file-versions.html>
- Resolve Git conflicts: <https://www.jetbrains.com/help/idea/resolve-conflicts.html>
- Tracker: <https://youtrack.jetbrains.com/issue/IJPL-241701> (public REST record also checked)

| Question from #284 | What the public sources establish | Status |
|---|---|---|
| Changing the compared revision inside the diff viewer | **Not available.** Revision selection happens outside the viewer: `Git | Compare With Revision` / `Git | Compare With Branch or Tag` from the Project view, or opening a diff from the Commit tool window. There is an **open feature request** to add an in-viewer revision dropdown — IJPL-241701, "Allow to select different file revision directly from the diff viewer" (State: **Open**, unassigned to a fix; the request explicitly says "there is no direct way to switch one of the compared versions … within the Diff viewer itself"). | Documented |
| Confirmed in-viewer actions | Docs confirm **Compare with Clipboard** (per-pane context menu) and **Switch to Three-Side Viewer**; the earlier survey's "Swap Sides" is a further in-viewer action. A **Compare Contents** button appears in the conflict-resolution merge actions. | Documented |
| Conflict numeric counter | The docs describe a **Conflicts dialog** that lists conflicting files and a resolved/unresolved progress state, and a three-pane resolution tool with per-conflict Accept/Ignore. **No `n of N conflicts` counter is documented** (unlike VS Code's merge editor). | Unverified (not documented) |
| Undo semantics (contrast) | "Ctrl+Z/Ctrl+Shift+Z — undo/redo a merge operation. Conflicts will be kept in sync with the text." Merge and text undo share the gesture and keep conflicts in sync. | Documented |

**Still unverified (JetBrains):** the exact conflict counter UI (likely absent) and whether any hidden in-viewer revision switch exists. **Why:** needs a running IDE; the docs and the open tracker together make absence the working assumption.

## Kaleidoscope 7

- Help home: <https://kaleidoscope.app/help>
- Text diffs and colors: <https://kaleidoscope.app/help/docs/text-diffs-and-colors>
- Text comparison views: <https://kaleidoscope.app/help/docs/text-comparison-views>
- Text compare settings: <https://kaleidoscope.app/help/docs/text-compare-settings>
- Blog, "Navigating Changes and Conflicts": <https://blog.kaleidoscope.app/2024/12/30/navigating-changes-and-conflicts/>

| Question from #284 | What the public sources establish | Status |
|---|---|---|
| Scroll-bar change markers vs a minimap | #284 confirmed change markers on the scroll bar by observation. The help and blog document a **bottom-toolbar change stepper** and a label that "outlines how many changes there are and where within those changes you are currently located" — i.e. a **numeric change counter**. **No minimap / code map is documented**, and the text views (Fluid, Blocks, Unified) are layout modes, not maps. The scroll-bar marks are the closest documented feature; whether they render a miniature text map is not stated. | Partially documented |
| Change detection and navigation | Added (green), deleted (red), changed (blue with deeper-blue words); next/previous change via stepper, `View > Next/Previous Change`, `⌘▼`/`⌘▲`, or a three-finger swipe. | Documented |
| Merge/conflict navigation | In merge mode the stepper navigates conflicts instead of changes; Option+`⌘▼/▲` switches back to change navigation; unresolved conflicts show a warning icon. | Documented |

**Still unverified (Kaleidoscope):** whether the scroll-bar markers are merely tick marks or a true minimap, and the exact counter format. **Why:** Kaleidoscope is a paid macOS app; no licence/install here.

## Background: DiffMerge (already confirmed)

The issue's appendix facts about SourceGear DiffMerge were verified separately and are not re-investigated here: the webhelp TLS issue (`curl -k` needed), the 2024-12 GPL-3.0 open-sourcing that stalled after three commits, the Homebrew cask `fails_gatekeeper_check` deprecation with disabling on 2026-09-01, and release notes stopping at 4.2.0 (2013-10).

## Hands-on verification checklist

Run these with the licensed/installed builds. Each item is one observable check; record the build/version with the result.

### Araxis Merge (needs a licence; Windows 2026.1 and current macOS)

- [ ] Launch with no arguments: confirm an empty comparison tab opens (no start/welcome screen) and note which comparison type appears.
- [ ] Record the exact geometry: pane widths, centre-point marker, and per-pane path labels.
- [ ] Locate the read-only-edit toggle and confirm in-place editing of a read-only file when enabled.
- [ ] Confirm the edit → recompute delay on a large file, and try to reproduce the reported hang.
- [ ] Confirm whether inline edits and merge-button operations undo together (one stack) or separately.
- [ ] Compare the macOS toolbar chrome with the Windows ribbon.

### WinXMerge (build v0.51.0 desktop; open <https://winxmerge.app> in a browser)

- [ ] Desktop: capture the file-selection dialog, toolbar, status bar, detail pane, and location pane (minimap).
- [ ] Web: confirm whether undo/redo is available at all, and that file open uses an upload picker only.
- [ ] Web: measure a large paste (e.g. multi-MB) for UI freeze; confirm there is no worker.
- [ ] Compare the default shortcuts against WinMerge's manual (F7/F8, Shift+F7, Alt+↑/↓, save, undo, find).
- [ ] Try drag-and-drop of a file onto the web app.

### Meld (desktop)

- [ ] Confirm that a change-bar copy/delete is undone by `Ctrl+Z` and that mixed typed + change-bar edits form a sensible single sequence.
- [ ] Confirm whether any change-bar action is excluded from undo.

### KDiff3 (current KDE build)

- [ ] Confirm the A/B/C input panes reject body edits.
- [ ] Confirm the merge output has no undo (FAQ 5.7) in the current release.
- [ ] Edit the output and check whether the diff re-runs (expected: no).

### P4Merge (current Helix Core build)

- [ ] Confirm whether a minimap/overview exists (default assumption: no).
- [ ] Confirm undo/redo and whether editing the result re-runs the diff.
- [ ] Capture per-pane path/filename labelling.
- [ ] Test large files on the current build (Qt5-era behaviour).

### JetBrains IDEs (current release)

- [ ] Confirm there is no in-viewer revision dropdown (see IJPL-241701) and re-check the tracker state.
- [ ] Confirm the conflict-resolution UI has no numeric `n of N` conflict counter.
- [ ] Confirm `Ctrl+Z`/`Ctrl+Shift+Z` keeps conflicts in sync.

### Kaleidoscope 7 (needs a subscription)

- [ ] Confirm whether the scroll-bar marks are tick marks or a minimap.
- [ ] Record the change-counter format and whether a map view can be enabled.
