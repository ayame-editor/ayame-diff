<!-- i18n: language-switcher -->
[English](shell-integration.md) | [日本語](shell-integration.ja.md)

# File-manager and quick launch

Compare two files without spelling out a subcommand:

```bash
ayame-diff old.txt new.txt
ayame-diff old-folder new-folder
ayame-diff --gui old.txt new.txt
```

The first two forms choose text or folder CLI output. `--gui` opens the local
GUI with the paths filled in and starts immediately. In the GUI, drop two files
or folders anywhere to compare them; dropping one fills the first empty side.

## Install file-manager integration

```bash
ayame-diff shell-install
# later:
ayame-diff shell-uninstall
```

Registration is per-user and does not require administrator privileges:

- Windows adds an Explorer **Compare with Ayame Diff** command for files and
  folders. Choose it on the first item and then the second item. It also adds a
  SendTo entry; selecting two items and using SendTo starts the GUI directly.
  Release ZIPs include `install-shell.cmd` and `uninstall-shell.cmd` wrappers.
- macOS installs a Finder Quick Action named **Compare with Ayame Diff** in
  `~/Library/Services`. Select two items and invoke it from Quick Actions.
- Linux installs a desktop entry under `~/.local/share/applications` with file,
  CSV, JSON, and directory MIME types plus a scalable Ayame icon. Select two
  items and use **Open With Ayame Diff** where the file manager supports `%F`.

Re-run `shell-install` after moving the executable because registrations store
its absolute path.

## Git difftool

These commands make ayame-diff a tool called by Git; they do not add repository
inspection or management to ayame-diff. See
[ADR 0004](adr/0004-git-repository-boundary.md) for that boundary.

Register the terminal diff as a custom Git tool:

```bash
git config --global diff.tool ayame-diff
git config --global difftool.ayame-diff.cmd \
  'ayame-diff text "$LOCAL" "$REMOTE"'
git config --global difftool.prompt false
```

Then run:

```bash
git difftool --tool=ayame-diff HEAD~1 HEAD -- path/to/file
```

Git supplies temporary files through `$LOCAL` and `$REMOTE`. This terminal
workflow blocks until the comparison finishes and uses the same text engine as
the ordinary CLI. The browser GUI currently lacks the blocking lifetime and
logical labels needed for a safe repeated `git difftool` workflow; that
follow-up remains tracked in
[#295](https://github.com/ayame-editor/ayame-diff/issues/295).

## Git mergetool

The current non-interactive integration lets Git accept only a clean automatic
merge. If ayame-diff still finds a conflict, it reports failure and Git keeps
the path unresolved:

```bash
git config --global merge.tool ayame-diff
git config --global mergetool.ayame-diff.cmd \
  'ayame-diff 3way text --allow-conflicts --merge-exit-code --output "$MERGED" "$BASE" "$LOCAL" "$REMOTE"'
git config --global mergetool.ayame-diff.trustExitCode true
```

After a conflicted `git merge`, run:

```bash
git mergetool --tool=ayame-diff -- path/to/file
```

Git defines `$BASE`, `$LOCAL`, `$REMOTE`, and `$MERGED` for a custom merge
tool. `--merge-exit-code` requires `--output`: it returns 0 only after writing
an output with no unresolved ayame-diff conflicts, 1 after writing standard
conflict markers, 2 for invalid invocation, and 3 for a runtime or write
failure. With `trustExitCode=true`, Git therefore keeps marker-bearing output
unresolved instead of confusing “saved” with “resolved”. Git may restore its
pre-tool worktree content after the nonzero exit; resolve the unmerged path
manually or with another interactive tool, then `git add` it.

This is deliberately a terminal/automatic baseline. Interactive GUI conflict
resolution, blocking browser lifetime, temporary-file labels, and repeated
session reuse remain in #295.
