<div class="doc-family">
  <span>Need to open and edit a huge file?</span>
  <a href="https://ayame-editor.github.io/ayame-editor/">Go to Ayame Editor →</a>
</div>

<section class="doc-hero">
  <div class="doc-hero-copy">
    <p class="doc-eyebrow">ayame-diff Docs</p>
    <h1>ayame-diff</h1>
    <p>See what changed, even when files are huge. Compare text, CSV / TSV, folders, archives, and three-way changes.</p>
    <div class="doc-actions">
      <a class="doc-action doc-action-primary" href="gui/">Use the GUI</a>
      <a class="doc-action" href="usage/">CLI usage</a>
    </div>
  </div>
  <figure class="doc-preview">
    <img src="assets/screenshot-folder.png" alt="ayame-diff folder comparison result" loading="eager">
    <figcaption>A folder comparison shows added, removed, and changed files at a glance.</figcaption>
  </figure>
</section>

## Choose a comparison

<div class="doc-card-grid">
  <a class="doc-card" href="gui/">
    <strong>Visual file comparison</strong>
    <span>Pick two files in the local web UI, navigate each difference, and export a patch.</span>
  </a>
  <a class="doc-card" href="usage/">
    <strong>Text or structured data</strong>
    <span>Compare lines, sorted text, or CSV / TSV rows by key from the terminal.</span>
  </a>
  <a class="doc-card" href="shell-integration/">
    <strong>Folders and archives</strong>
    <span>Compare directory trees, compressed inputs, or launch from your file manager.</span>
  </a>
  <a class="doc-card" href="three-way/">
    <strong>Three-way merge</strong>
    <span>Review BASE / LEFT / RIGHT, resolve conflicts, and save a merged result.</span>
  </a>
</div>

## Quick start

Install a binary for your OS from the [latest release](https://github.com/hjosugi/ayame-diff/releases/latest), then choose the shortest command for the job:

```bash
ayame-diff gui                              # choose files in the browser
ayame-diff text old.txt new.txt             # compare text
ayame-diff csv --left old.csv --right new.csv --key id --out diff.tsv
ayame-diff dir old-folder new-folder        # compare folder trees
```

## Subcommands at a glance

```text
ayame-diff csv    [flags] --left A --right B --out D   # CSV/TSV key comparison
ayame-diff text   [flags] LEFT RIGHT                      # line-oriented text diff
ayame-diff sorted [flags] LEFT RIGHT                      # sort both sides, then diff
ayame-diff dir    [flags] LEFT RIGHT                      # directory/archive comparison
ayame-diff bin    [flags] LEFT RIGHT                      # binary/hex comparison
ayame-diff 3way   [text|csv] [flags]                   # three-way comparison
ayame-diff serve  [--addr host:port]                   # local web UI
ayame-diff gui    [flags] [LEFT [RIGHT]]                  # open the web UI in a browser
ayame-diff update [--check]                            # check for or install the latest release
ayame-diff remove [--yes]                              # uninstall a standalone binary
ayame-diff shell-install                               # register file-manager integration
ayame-diff shell-uninstall                             # remove file-manager integration
ayame-diff shell-select PATH                           # Windows Explorer integration helper
```

Use [Comparison options](comparison-options.md) for whitespace, case, line filters, word highlights, and resync controls. For repeatable jobs, save the full setup as a [comparison project](projects.md).

<div class="doc-link-row">
  <a href="https://github.com/hjosugi/ayame-diff">GitHub</a>
  <a href="https://github.com/hjosugi/ayame-diff/releases/latest">Latest release</a>
  <a href="https://github.com/hjosugi/ayame-diff/blob/main/CHANGELOG.md">Changelog</a>
  <a href="https://github.com/hjosugi/ayame-diff/blob/main/CONTRIBUTING.md">Contributing</a>
</div>
