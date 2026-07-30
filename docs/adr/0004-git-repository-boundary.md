<!-- i18n: language-switcher -->
[English](0004-git-repository-boundary.md) | [日本語](0004-git-repository-boundary.ja.md)

# ADR 0004: Keep Git repository management outside ayame-diff

- Status: Accepted (2026-07-30)
- Related Issue: hjosugi/ayame-diff#290
- Related Work: #280 (local architecture), #295 (external tool invocation)

## Context

Ayame-diff primarily compares files that are not managed together by a version
control system. Adding repository awareness would bring a second product scope:
object and revision lookup, index state, branches, remotes, authentication,
ignore rules, staging, and commits. Mature editors and Git clients already
serve that workflow.

Some of their interaction patterns remain valuable outside Git. VS Code Source
Control is explicitly a design reference, not a feature boundary: a folder
comparison benefits from a change list and continuous multi-file view (#291);
direct editing benefits from gutter change markers (#292) and hunk-local
actions (#293); temporary inputs benefit from meaningful logical labels (#295).

Git can also invoke an external diff or merge tool after materializing files.
In that direction, Git owns repository semantics and ayame-diff receives only
ordinary paths. This composes with the product without adding repository
management.

## Decision

Ayame-diff will not inspect or manage Git repositories.

- It will not read `.git`, resolve revisions such as `HEAD~1`, show history,
  branches, remotes, staged/unstaged state, or perform stage, commit, fetch,
  pull, push, or authentication operations.
- It will not give `.gitignore` special meaning. Folder comparison continues
  to use its own explicit filters and projects.
- CLI and GUI comparisons start from explicit paths, pasted content, or an
  ayame-diff project file.
- Being invoked as a custom `git difftool` or `git mergetool` is allowed. Git
  supplies `$LOCAL`, `$REMOTE`, `$BASE`, and `$MERGED`; ayame-diff does not
  discover repository state. The supported terminal setup is documented in
  [File-manager and quick launch](../shell-integration.md).
- Git-independent UX patterns may be adopted when they improve general
  comparison work. This includes multi-file result browsing, gutter change
  markers, hunk-local actions, and logical pane labels.

## Consequences

- The comparison engine remains usable for arbitrary files and automation
  without repository-specific state or authentication.
- Repository operations stay in Git, editors, and dedicated Git clients.
- Requests that require Git's object model should be rejected or reframed as
  explicit-path input. Requests about generic comparison interaction may still
  use Git clients as design references.
- External-tool improvements must preserve the direction of dependency:
  Git calls ayame-diff; ayame-diff does not become a Git client.

## Rejected alternatives

- **Embed a read-only Git browser:** Even without writes, revision resolution,
  worktree/index state, submodules, and authentication create a large ongoing
  compatibility surface.
- **Add staging and commit operations after direct editing:** This couples
  comparison safety to repository mutation and duplicates established clients.
- **Reject all Git-related workflows:** Custom difftool/mergetool invocation
  keeps repository ownership in Git and is therefore compatible with this
  boundary.
