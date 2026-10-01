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
[ADR 0004](adr/0004-git-repository-boundary.md) for that boundary. The
`difftool` command accepts Git's two-file form; `mergetool` accepts the
three-way form described below.

Register a terminal diff:

```bash
git config --global diff.tool ayame-diff
git config --global difftool.ayame-diff.cmd \
  'ayame-diff difftool "$LOCAL" "$REMOTE"'
git config --global difftool.prompt false

git difftool --tool=ayame-diff HEAD~1 HEAD -- path/to/file
```

For a browser comparison, add `--wait`. The process then blocks until the tab
closes, so `git difftool` runs one file at a time and waits for each:

```bash
git config --global difftool.ayame-diff.cmd \
  'ayame-diff difftool --wait "$LOCAL" "$REMOTE"'
```

Git supplies temporary files through `$LOCAL` and `$REMOTE`. Those temp paths
are what the panes would otherwise show, so `--label` overrides them with
logical names:

```bash
git config --global difftool.ayame-diff.cmd \
  'ayame-diff difftool --label "$LOCAL" --label "$REMOTE" "$LOCAL" "$REMOTE"'
```

Git does not hand a custom tool the revision expressions, so a static label or a
small wrapper that maps `$LOCAL`/`$REMOTE` to names such as `HEAD~1:foo.txt` is
what makes the heading meaningful. Labels follow the positional order: the first
names LEFT, the second RIGHT.

## Git mergetool

Register the three-way merge and let Git trust its exit code:

```bash
git config --global merge.tool ayame-diff
git config --global mergetool.ayame-diff.cmd \
  'ayame-diff mergetool --output "$MERGED" "$BASE" "$LOCAL" "$REMOTE"'
git config --global mergetool.ayame-diff.trustExitCode true

git mergetool --tool=ayame-diff -- path/to/file
```

Git defines `$BASE`, `$LOCAL`, `$REMOTE`, and `$MERGED` for a custom merge
tool. The positional order is `BASE LOCAL REMOTE` (P4Merge/Git); Meld's
`LOCAL BASE REMOTE` order is available with `--order local-base-remote`. Add
`--label` three times to name BASE, LOCAL, and REMOTE, and `--gui` to resolve in
the browser (it blocks until the tab closes):

```bash
git config --global mergetool.ayame-diff.cmd \
  'ayame-diff mergetool --gui --order base-local-remote --label BASE --label LOCAL --label REMOTE --output "$MERGED" "$BASE" "$LOCAL" "$REMOTE"'
```

`mergetool` returns 0 only when `$MERGED` is written with no unresolved
conflicts. It returns 1 when a saved output still carries standard conflict
markers, and 130 when the GUI session ended without saving anything (aborted),
so Git cannot confuse “saved” with “resolved”. The terminal path is the same
engine as `ayame-diff 3way text --merge-exit-code --output "$MERGED"`; the GUI
path receives the save's own unresolved count from the server. With
`trustExitCode=true`, Git keeps marker-bearing output unresolved instead of
accepting it. Git may restore its pre-tool worktree content after the nonzero
exit; resolve the unmerged path manually or with another interactive tool, then
`git add` it.

## SVN

SVN and TortoiseSVN can point their external diff and merge tools at the same
commands. TortoiseSVN uses `%mine` / `%yours` for a two-file diff and
`%base` / `%mine` / `%theirs` / `%merged` for a merge:

```text
Diff:  ayame-diff difftool "%mine" "%yours"
Merge: ayame-diff mergetool --output "%merged" "%base" "%mine" "%theirs"
```

The command-line client's `--diff-cmd` passes its own `-u -L …` arguments, which
`difftool` does not consume, so wrap it in a script that forwards only the two
paths when using `svn diff --diff-cmd`.

## IDE external tools

IDEs that accept a configured external diff or merge program use the same
contract even though their placeholder syntax differs (JetBrains `$1`, Visual
Studio `%1`, and so on). Set the program to `ayame-diff` and the arguments to:

```text
Diff:  difftool <left> <right>
Merge: mergetool --output <merged> <base> <local> <remote>
```

Map the IDE's left/right or base/local/remote/output placeholders onto those
positions, and add `--gui` when you want the browser view. The exit code carries
the resolution state, so enable any “trust the tool's exit code” option to let an
interrupted or still-conflicting merge stay unresolved.

## Repeated invocation

`git difftool` starts the tool once per file and waits for each run. Every
invocation starts and stops its own short-lived local server; sessions stay
sequential, and there is no cross-invocation server reuse. Set
`difftool.prompt false` to skip the per-file prompt. Use the terminal form when
you only need the text output.
