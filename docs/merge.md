<!-- i18n: language-switcher -->
[English](merge.md) | [日本語](merge.ja.md)

# Merge and reconcile

The local GUI can turn a comparison into a new merged file without modifying
either input by default.

## Text

After a text comparison, each hunk has **Use left** and **Use right** toggles.
Clicking a side you already chose clears it, so selecting and deselecting are
the same gesture, and the pressed state shows what each hunk currently adopts.
Turning on both sides adopts both contributions, concatenated left then right;
**All left** or **All right** set every hunk at once. Undo and redo store only
hunk-selection snapshots, so large source files are not copied into browser
history. `Alt+Left` and `Alt+Right` toggle the side for the current hunk; the
existing `Alt+Up` / `Alt+Down` shortcuts navigate and `Ctrl+Shift+S` saves the
merge. In a three-way result `Alt+B` chooses BASE and `Alt+A` keeps both sides;
`F8` / `Shift+F8` walk the conflicts only, and `Alt+Shift+A` turns on
auto-advance to the next unresolved conflict after each choice (#277).

A three-way comparison offers **Use left**, **Use base**, and **Use right**
toggles on each conflict. The adopted contributions are concatenated in the
fixed order base, left, right, so "left + right" reads as the left side followed
by the right side (the order the toggles were clicked does not matter).

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

Each logical keyed difference has a stable content-derived ID. A CHANGED pair
offers left and right toggles; turning on both keeps both rows, left then right.
A LEFT_ONLY or RIGHT_ONLY row has content on only one side, so its toggles keep
or drop that row instead of concatenating (the same keep/drop decision as
before). Saving reruns the memory-bounded partition/sort pipeline and emits one
complete, reconciled, key-sorted CSV or TSV including equal rows. Only the
stable choice map is held in memory.

The output delimiter follows the filename: `.csv` / `.csv.gz` uses comma;
other names use tab. Quoting is written with the standard CSV rules.

## Safety rules

- Unresolved differences block saving by default. The merge panel lists them
  first; accepting the warning saves them using the selected **Unresolved
  target** (left, right, or conflict markers) instead of always the left side.
- A new output path is the default and is written atomically.
- An output path matching either input is rejected unless **overwrite input**
  is enabled and the second destructive confirmation is accepted. **Preview
  impact** (or the save confirmation) lists the output that will be written and
  every compared input it would overwrite before the step proceeds.
- Rejected, cancelled, or failed operations leave both inputs unchanged and do
  not publish a partial output.
