# Feature 2 — Resolve merge conflicts (JetBrains-style)

Instructions for Claude Code. Read this file together with `CLAUDE.md`; everything in `CLAUDE.md` still applies. Where this file and `CLAUDE.md` disagree about this feature, this file wins, and you update `CLAUDE.md` to match as part of the work (see Phase 4).

## Goal

After a `git pull`, `merge`, `rebase`, `cherry-pick`, `revert` or `stash pop` that stops with conflicts, `mergediff resolve` opens a **Conflicts window** listing every conflicted file with its status on each side. From there the user can:

- **Merge…** a file in the **existing** 3-way merge view (in its own window),
- **Accept Yours** or **Accept Theirs** for one or more files,
- and see the list update until nothing is left.

This mirrors the JetBrains IDEs' Conflicts dialog.

### Why a new command and not `git mergetool`

`git mergetool` has no directory mode: it launches the tool once per conflicted file, one after another, so it can never feed a list window. JetBrains does not use `git mergetool` either: its Conflicts dialog talks to Git directly. `mergediff resolve` does the same. `git mergetool` with mergediff keeps working exactly as today.

### How JetBrains does it (reference, from `intellij-community`, `plugins/git4idea/backend/src/`)

| Step | JetBrains | mergediff |
|---|---|---|
| List conflicts | `git ls-files --exclude-standard --unmerged -t -z` (`merge/GitMergeProvider.java`) | `git ls-files --unmerged -z` |
| Status per side | from which index stages exist: `MODIFIED`, `ADDED`, `DELETED` (`repo/GitConflict.java`) | same |
| Read base / yours / theirs | `git cat-file -p :1:path`, `:2:`, `:3:` (optionally `--filters`) (`merge/GitMergeUtil.java`, `util/GitFileUtils.kt`) | `git cat-file --filters :N:path`, only for stages that exist |
| Accept Yours / Theirs | `git checkout --ours\|--theirs -- path`, skipped when that side deleted the file (`GitMergeUtil.acceptOneVersion`) | same |
| Mark resolved | `git add` (or `git rm` when the accepted side is a deletion) (`GitMergeUtil.markConflictResolved`) | same |
| Rebase | yours/theirs swapped while rebasing (`GitMergeUtil.isReverseRoot`) | same |

**Why `git cat-file --filters` and not `git checkout-index --temp`:** `cat-file` writes to stdout and never touches the working tree, so nothing can be left behind if mergediff crashes or is killed. `checkout-index --temp` writes `.merge_file_*` files into the working tree that must be cleaned up. `--filters` applies the same smudge / line-ending conversion a checkout would. Because `ls-files --unmerged` already says which stages exist, never ask `cat-file` for a missing stage and never parse Git's error messages.

## Before you start

1. **Use the Task tool to track progress.** At the start of each phase below, break it into tasks; mark each task in progress when you start it and completed when it is done and verified. Never work on something that is not tracked as a task.
2. **Feature 1 must be merged first.** Verify:
   ```
   git checkout main && git pull --ff-only
   ```
   then check that `main` contains Feature 1 (`internal/dirdiff/`, `frontend/src/filelist.ts` and the per-window session model described in `CLAUDE.md`). If it does not, **stop and ask**; do not start this feature.
3. **Branch:** create a dedicated branch from that up-to-date `main`:
   ```
   git checkout -b feature/resolve-conflicts
   ```
   Never commit to `main` directly, and do not merge the branch yourself (see "Finishing the feature").
4. **Mockups:** check that these files exist in `docs/mockups/`. If any is missing, stop and ask for it:
   - `mergediff-dir-merge-light.png`, `mergediff-dir-merge-dark.png` — flat list
   - `mergediff-dir-merge-grouped-light.png`, `mergediff-dir-merge-grouped-dark.png` — "Group by directory" checked
   - `mergediff-mockup-light.png`, `mergediff-mockup-dark.png` — the existing 3-way merge view that **Merge…** opens
5. Work one phase at a time. At the end of each phase: all tests green, last commit `gofmt`-clean, then stop and give a short summary.

## Design reference

The mockups are a **visual reference**, not a pixel-exact target: follow their layout, colours and wording, but this file is authoritative wherever they differ. **Reuse `frontend/src/filelist.ts` from Feature 1** for the list (path display, filter, chips, grouping, keyboard, virtual scrolling) and the existing colour tokens.

What the mockups show:

- **Title bar:** `mergediff — Resolve conflicts · <repository folder name>`.
- **Operation bar:** a sentence describing what is in progress (see "Labels"), and a red pill `N of M files unresolved` (green `All conflicts resolved` when N is 0).
- **Filter bar:** text filter, chips `All M`, `Unresolved N`, `Resolved N`, and a `Group by directory` checkbox.
- **Table columns:** File, **Yours** and **Theirs** (each with the branch label under the heading), **State**.
  - Yours / Theirs cells: `Modified`, `Added` or `Deleted` tags in the modified / added / deleted colours.
  - State: `Conflict` (both sides changed the content), `Modify/delete` (one side deleted it), `Binary`, `Symlink`, `Submodule`, or `✓ Resolved`.
  - Resolved rows stay in the list, faded.
- **Side panel**, top to bottom: `Merge…` (primary), `Accept Yours`, `Accept Theirs`; below them the selected file's full path (wrapped) and a one-line description (e.g. "Both sides changed this file. 3 conflicts, 5 non-conflicting changes."); `Close` at the bottom.
- **Status bar:** left, keyboard hints; right, `<repo path> · mergediff <version>`.
- **Long paths and Group by directory:** exactly the rules in Feature 1 (directory part truncated with `…`, file name never truncated, tooltip with the full path; grouping flat by parent directory, `(root)` first, file rows show only the name). Group headers show `N files · N unresolved` in the conflict colour, or `all resolved` in green.

The branch names and paths in the mockups are illustrative.

## Behaviour

### Command line

```
mergediff resolve [--auto-apply] [--] [PATH...]
```

- Runs in any directory inside a Git working tree. `PATH...` (relative to the current directory) limits the list to conflicts under those paths.
- `--auto-apply` is passed to every merge window opened from the list (same meaning as today).
- No conflicts: print `mergediff: no conflicts to resolve` and exit `0` without opening a window.
- Not in a working tree, `git` not found, or Git older than **2.11** (needed for `cat-file --filters`): clear message, exit `2`.
- Exit codes: `0` if no conflicts remain (within the given paths) when the window closes; `1` if some remain; `2` for errors.
- Update the usage text in `cli.go`.
- README: document the command and the optional alias `git config --global alias.resolve '!mergediff resolve'` (note: Git runs `!` aliases from the repository root).

### Git access (`internal/gitconflict`, new package, no Wails imports)

- Run Git with `exec.Command` (never through a shell), always as `git -C <toplevel> -c core.quotepath=off …`, with `LC_ALL=C`, paths after `--`, and `-z` output parsed as NUL-separated records.
- Find the top level with `git rev-parse --show-toplevel`; use `git rev-parse --git-path <name>` to locate state files (works with worktrees).
- **Conflicts:** `git ls-files --unmerged -z` → mode, object, stage and path per record; group by path. Status per side: stage present and stage 1 present → `Modified`; stage present without stage 1 → `Added`; stage absent → `Deleted`.
- **Kind** per file: mode `160000` → Submodule; mode `120000` → Symlink; content with a NUL byte in the first 8 KB (same rule as `textio`) → Binary; otherwise text.
- **Content:** `git cat-file --filters :N:path` for each stage that exists; a missing stage is empty text. You may use `git cat-file --batch` with `--filters` for speed if you verify it works with the minimum Git version; otherwise one call per stage is fine.
- **Accept a side:** if that side's status is not `Deleted` → `git checkout --ours|--theirs -- <paths>` then `git add -- <paths>`; if it is `Deleted` → `git rm --quiet -- <paths>`.
- **Save a merge:** write the result to the working-tree file (with the existing `textio` line-ending handling), then `git add -- <path>`.
- **Allowed write commands are only:** `git checkout --ours/--theirs -- <path>`, `git add -- <path>`, `git rm -- <path>`, and writing the merged file. Never run `reset`, `clean`, `stash`, `commit`, `merge --abort`, `rebase --continue` or any other command that changes history or other files.

### Labels and the rebase swap

- **Operation** from state files: `MERGE_HEAD` → merge; `rebase-merge/` or `rebase-apply/` → rebase; `CHERRY_PICK_HEAD` → cherry-pick; `REVERT_HEAD` → revert; none of these → "Resolving conflicts" (e.g. after `stash pop`).
- **Sentence:** `Merging <theirs> into <current branch>`, `Rebasing <branch> onto <upstream>`, `Cherry-picking <short hash> <subject>`, `Reverting <short hash> <subject>`. Use the branch name where one exists (`git symbolic-ref --short HEAD`, `git name-rev --name-only`, `rebase-merge/head-name`, `rebase-merge/onto`), else the short hash. Best effort: a label that cannot be determined falls back to `HEAD` / the short hash, never an error.
- **Rebase swap:** while rebasing, Git's stage 2 ("ours") is the upstream and stage 3 ("theirs") is the user's own commit. Like JetBrains, **show the user's commit as Yours** and the upstream as Theirs, and swap `--ours`/`--theirs` (and the merge window's Local/Remote) accordingly. Keep this mapping in one place in `internal/gitconflict` with tests.

### Actions in the window

- **Merge…** (also double-click / `Enter`): only for text conflicts where both sides exist. Opens the existing 3-way merge view in a **child window** (per-window sessions from Feature 1) with base / yours / theirs from the stages and the working-tree file as the output. Pane headers show the real branch labels. `Save` writes the file, runs `git add`, closes the child window and refreshes the list. `Cancel` closes the child window with no change. If the file's merge window is already open, focus it.
- **Accept Yours / Accept Theirs** (`Ctrl/Cmd+Shift+Y`, `Ctrl/Cmd+Shift+T`): work on all selected rows (multi-select with `Shift`/`Ctrl/Cmd`+click). Ask for confirmation when more than one file is selected, naming how many.
- **Modify/delete:** `Merge…` is disabled. The buttons read `Keep modified (yours)` / `Delete (theirs)` (or the reverse, depending on the side). A `Show changes` link opens the existing 2-way diff of base vs the modified side in a child window.
- **Binary and Symlink:** `Merge…` disabled; Accept Yours / Theirs available. The description says why.
- **Submodule:** no actions; the description says to resolve it with Git. It still counts as unresolved.
- Rows whose merge window is open cannot be accepted until it closes.
- **Refresh:** after every action, and on `F5`, re-run `ls-files --unmerged`, so conflicts resolved in a terminal meanwhile also show as resolved. Files that were conflicted when the window opened stay listed; those no longer unmerged are `✓ Resolved`.
- **All resolved:** the pill turns green and a banner shows the next command to run (`git commit`, `git merge --continue`, `git rebase --continue`, `git cherry-pick --continue` or `git revert --continue`, matching the operation) with a Copy button. mergediff **does not run it**.
- **Close** (or the window's close button): if child merge windows have unsaved edits, ask first; closing the list closes all child windows. Then exit with `0` or `1` as described above.

## Phases

### Phase 1 — `internal/gitconflict`

- Repository detection, Git version check, conflict listing, kinds, stage content, accept / add / rm, operation detection, labels, rebase swap.
- Tests build **real temporary repositories** with `git` for each case: both modified, both added, modify/delete in both directions, binary, symlink, submodule (skip if not possible in CI), CRLF with `.gitattributes eol=crlf` (content must match what a checkout produces), path with spaces and unicode, running from a subdirectory, `PATH...` limits, merge, rebase (swap), cherry-pick, revert, stash pop, and a rename that Git reports as conflicted (list whatever paths Git reports; no rename pairing).
- **Accept:** `go test ./...` passes; a test asserts that no file is created in the working tree while reading conflicts.

### Phase 2 — Command and Conflicts window

- `mergediff resolve` parsing and exit codes; session mode `resolve`; the Conflicts window using `filelist.ts`, flat and grouped, as in `mergediff-dir-merge-*.png` and `mergediff-dir-merge-grouped-*.png`; Accept Yours / Theirs with multi-select; refresh.
- **Accept:** in a conflicted test repo the window matches the mockups; accepting files updates the list and the index (`git status` agrees).

### Phase 3 — Merge, modify/delete, special kinds

- Merge… in child windows with save → `git add` → refresh; Show changes for modify/delete; Binary / Symlink / Submodule handling; open-window locking; the all-resolved banner.
- **Accept:** a full session in a test repo resolves every kind of conflict; the final exit code is `0`, or `1` when one is left.

### Phase 4 — End-to-end, docs, finish

- `scripts/e2e`: create a repo with several conflicts, run `mergediff resolve`, merge one file through the UI, accept one each way, close; check file bytes, `git status` and the exit code.
- README: the command, the alias, what each conflict kind allows, a screenshot placeholder (mockups may stand in).
- `CLAUDE.md`: add the command, Bound API, UI and Phases; update "Out of scope" ("VCS features beyond mergetool/difftool" now has this exception).
- **Accept:** `make check` passes (and `make e2e` where a display is available).

## Finishing the feature

1. Make sure the branch is up to date with `main` and `make check` passes.
2. Push the branch and open a pull request into `main` (`gh pr create` if `gh` is available; otherwise print the compare URL). Use the summary of all phases as the description.
3. **Stop.** Do not merge the PR yourself; the maintainer reviews and merges it.

## Out of scope

Running `commit` / `--continue` / `--abort`, undoing a resolution ("revert conflict resolution"), rename pairing, resolving submodules, Git LFS-specific handling beyond what `--filters` provides, repositories without a working tree.
