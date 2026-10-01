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
there is no separate "both" button. **All left** or **All right** set every hunk
at once. Undo and redo store only hunk-selection snapshots, so large source
files are not copied into browser history. `Alt+Left` and `Alt+Right` toggle the
side for the current hunk; the existing `Alt+Up` / `Alt+Down` shortcuts navigate.

A three-way comparison offers **Use left**, **Use base**, and **Use right**
toggles on each conflict. The adopted contributions are concatenated in the
fixed order base, left, right, so "left + right" reads as the left side followed
by the right side (the order the toggles were clicked does not matter).

Saving recomputes the complete diff and streams unchanged/chosen ranges into a
temporary sibling file. The temporary file is flushed before atomic rename.
Original LF/CRLF and final-newline state follow the source selected for each
range. Decoded non-UTF-8 input is saved as UTF-8.

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

- Unresolved differences block saving by default. If the warning is accepted,
  unresolved items retain the left side.
- A new output path is the default and is written atomically.
- An output path matching either input is rejected unless **overwrite input**
  is enabled and the second destructive confirmation is accepted. **Preview
  impact** (or the save confirmation) lists the output that will be written and
  every compared input it would overwrite before the step proceeds.
- Rejected, cancelled, or failed operations leave both inputs unchanged and do
  not publish a partial output.
