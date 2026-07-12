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
