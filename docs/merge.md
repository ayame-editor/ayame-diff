<!-- i18n: language-switcher -->
[English](merge.md) | [日本語](merge.ja.md)

# Merge and reconcile

The local GUI can turn a comparison into a new merged file without modifying
either input by default.

## Text

After a text comparison, each hunk has **Use left** and **Use right** actions.
Use **All left** or **All right** for the whole result. Undo and redo store only
hunk-choice maps, so large source files are not copied into browser history.
`Alt+Left` and `Alt+Right` choose the side for the current hunk; the existing
`Alt+Up` / `Alt+Down` shortcuts navigate.

Saving recomputes the complete diff and streams unchanged/chosen ranges into a
temporary sibling file. The temporary file is flushed before atomic rename.
Original LF/CRLF and final-newline state follow the source selected for each
range. Decoded non-UTF-8 input is saved as UTF-8.

Before a save with unresolved differences, the merge panel lists each one with
its kind and location, and **Go to unresolved** jumps to the first. An
**Unresolved target** control chooses what happens when you continue anyway:

- **left** / **right** resolves each remaining difference to that side.
- **Keep conflict markers** writes standard `<<<<<<< LEFT` / `=======` /
  `>>>>>>> RIGHT` blocks so a later pass can resolve them.

The save result is honest about which happened: a file written with implicit
side choices or remaining markers reports that, and does not read as a clean,
conflict-free merge.

## CSV / TSV

Each logical keyed difference has a stable content-derived ID. Choose left or
right for CHANGED pairs, LEFT_ONLY rows (keep/drop), and RIGHT_ONLY rows
(drop/keep). Saving reruns the memory-bounded partition/sort pipeline and emits
one complete, reconciled, key-sorted CSV or TSV including equal rows. Only the
stable choice map is held in memory.

The output delimiter follows the filename: `.csv` / `.csv.gz` uses comma;
other names use tab. Quoting is written with the standard CSV rules.

## Safety rules

- Unresolved differences block saving by default. The merge panel lists them
  first; accepting the warning saves them using the selected **Unresolved
  target** (left, right, or conflict markers) instead of always the left side.
- A new output path is the default and is written atomically.
- An output path matching either input is rejected unless **overwrite input**
  is enabled and the second destructive confirmation is accepted.
- Rejected, cancelled, or failed operations leave both inputs unchanged and do
  not publish a partial output.
