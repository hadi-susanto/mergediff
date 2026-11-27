# Feature 1 — Directory diff

Instructions for Claude Code. Read this file together with `CLAUDE.md`; everything in `CLAUDE.md` (working rules, error handling, tests, versioning) still applies. Where this file and `CLAUDE.md` disagree about this feature, this file wins, and you update `CLAUDE.md` to match as part of the work (see Phase 5).

## Goal

`mergediff LEFT RIGHT` where both arguments are **directories** opens a directory comparison window: a list of the files that differ, with their status (Modified, Added, Deleted). Opening a file shows it in the **existing** 2-way diff view, in its own window.

Main use: `git difftool -d` (`--dir-diff`). Git creates two temporary directories and calls the configured difftool once with them as `$LOCAL` and `$REMOTE`, so the existing `difftool.mergediff.cmd` works without any configuration change.

This feature also builds the **shared foundation** that Feature 2 (`02-resolve-conflicts.md`) reuses: one session per window, and a reusable file-list component.

## Before you start

1. **Use the Task tool to track progress.** At the start of each phase below, break it into tasks; mark each task in progress when you start it and completed when it is done and verified. Never work on something that is not tracked as a task.
2. **Branch:** work only on a dedicated branch, created from an up-to-date `main`:
   ```
   git checkout main && git pull --ff-only
   git checkout -b feature/directory-diff
   ```
   Never commit to `main` directly, and do not merge the branch yourself (see "Finishing the feature").
3. **Mockups:** check that these files exist in `docs/mockups/`. If any is missing, stop and ask for it:
   - `mergediff-dir-diff-light.png`, `mergediff-dir-diff-dark.png` — flat list
   - `mergediff-dir-diff-grouped-light.png`, `mergediff-dir-diff-grouped-dark.png` — "Group by directory" checked
4. Work one phase at a time. At the end of each phase: all tests green, last commit `gofmt`-clean, then stop and give a short summary.

## Design reference

The mockups are a **visual reference**, not a pixel-exact target: follow their layout, colours and wording, but this file is authoritative wherever they differ. Reuse the existing colour tokens in `frontend/src/styles.css` (`--modified-*`, `--added-*`, `--deleted-*`, `--chrome-*`); add tokens only if needed, for both themes.

What the mockups show:

- **Title bar:** `mergediff — Compare directories`.
- **Path bar:** `Left <full left path> ↔ Right <full right path>`, monospace, each path truncated in the middle with `…` if too long (full path in a tooltip).
- **Filter bar:** a text filter (case-insensitive substring match on the relative path), status chips with counts (`All changes N`, `Modified N`, `Added N`, `Deleted N`; one active at a time), a `Show unchanged (N)` checkbox, and a `Group by directory` checkbox.
- **Table columns:** status letter badge (`M` / `A` / `D`), File, Lines, Status tag.
- **Lines column:** `+N −N` in the added / deleted colours; `binary, differs` for binary files.
- **Status bar:** left, a keyboard hint; right, `N changed · N unchanged · mergediff <version>`.

Differences from the mockups (this file wins):

- The mockups show `Show unchanged (184)` for a `git difftool -d` run. In reality Git copies **only the changed files** into the temporary directories, so unchanged is usually 0. **Hide the checkbox when there are no unchanged files.**
- The left/right paths are illustrative.

### Displaying long paths (flat list)

- The directory part is shown in the muted colour, the file name in bold.
- When space runs out, **truncate the directory part** with a trailing `…`; the **file name is never truncated** unless it alone is wider than the column (then end-ellipsis).
- The full relative path is always in a tooltip.

### Group by directory

See `mergediff-dir-diff-grouped-*.png`.

- Grouping is **flat by parent directory**: one header row per directory that contains changed files, showing the directory's full relative path (truncated with `…` if needed, full path in a tooltip). No nested tree.
- Files directly in the compared root go in a group labelled `(root)`, listed **first**; the other groups are sorted by path.
- The header row shows `▾`/`▸` (collapsible by click or `←`/`→`), a folder glyph, the path, and small tags with the counts per status (`1 modified`, `1 added`, …).
- File rows under a header show **only the file name**, indented.
- Filter and chips apply first; groups with no remaining files are hidden.
- The checkbox state is remembered for the session only (no new setting unless asked).

## Behaviour

### Command line

- `mergediff LEFT RIGHT`: if **both** are directories → directory diff. If both are files → existing file diff (unchanged). If one is a directory and the other a file → usage error, exit `2`, with a message naming which is which.
- Update the usage text in `cli.go` (mention `git difftool -d`).
- Exit code: `0` when the directory window is closed (it is read-only). Closing it closes all file diff windows it opened.

### Comparing the directories (`internal/dirdiff`, new package, no Wails imports)

- Walk both trees with `filepath.WalkDir`; compare **relative paths with `/` separators**; pair entries by path.
- **Follow symlinks** to files (Git's `--dir-diff` symlinks right-side files to the working tree). A broken symlink is listed with an error note, not a crash. Do not follow symlinks to directories (avoid cycles); list them as a single entry with a note.
- Skip `.git` directories (and `.git` files used by worktrees/submodules).
- Ignore empty directories; only files are listed.
- Status: **Added** = right only, **Deleted** = left only, **Modified** = content differs, **Unchanged** = identical bytes.
- Compare by size first, then by streamed byte comparison (do not load whole large files to decide equality).
- A path that is a file on one side and a directory on the other: list the file as Added/Deleted and the directory's files normally.
- Unreadable files: list them with status `Error` and the message; the rest of the comparison continues.
- **Line counts** (`+N −N`) for Modified/Added/Deleted text files, computed with the existing line diff (`internal/diff`, `internal/textio`), only for changed files, using a small worker pool. Binary files (detected the same way as `textio`): `binary, differs`. Files over 5 MB: show the size change instead of line counts. If only line endings differ, show `line endings` in the Lines column.
- Must stay responsive: target under 1 s to show the list for 20,000 files with 500 changed on a local disk.

### Opening a file

- Double-click or `Enter` opens the selected file in the **existing 2-way diff view** (`renderDiffView`) in a **new window**. If that file's window is already open, focus it instead.
- Added / Deleted files open with the missing side as **empty text**, titled `(missing)`.
- Binary, error and too-large rows cannot be opened: the row shows why, and `Enter` does nothing except show the reason in the status bar.
- `F7` / `Shift+F7` in the list: select the next / previous changed file and open it.
- `↑` / `↓` move the selection, `Home` / `End` jump, `Ctrl/Cmd+F` focuses the filter, `Esc` clears the filter (or closes the window when the filter is empty).

## Shared foundation (also used by Feature 2)

### One session per window

Today `App` holds a single `sess *session.Session`, and `Quit`, the close guard and the exit code are application-wide. Refactor so that:

- Each window has its own session, identified by an id passed in the window URL (e.g. `/?session=<id>&theme=…`). `LoadSession(id)`, `Save(id, result)` and the close guard work per window.
- The **main window** (the one opened from the command line) decides the process exit code, exactly as today. Child windows close themselves and report back to their parent (an event or a callback through the backend); they never end the process.
- The existing single-file merge, diff and scratch modes must behave **exactly as before**, including exit codes, the close guard and all existing Go, frontend and e2e tests.
- Before writing code, check the current **Wails v3** API for creating and closing windows and for per-window events (docs or source of the version in `go.mod`); do not guess.

### Reusable file list (`frontend/src/filelist.ts`, new)

A DOM component used by this feature and by Feature 2, configured with columns, row data and callbacks. It provides: the path display rules above, filter text, chips with counts, group-by-directory, selection and keyboard navigation, and per-row "can open / reason" state. Keep the grouping, filtering and path-splitting logic DOM-free so it can be tested with `node --test`. If more than ~2,000 rows are visible, render only the visible rows (virtual scrolling).

## Phases

### Phase 1 — Per-window sessions (no visible change)

- Refactor `App`, `main.go` and `frontend/src/main.ts` to one session per window as described above.
- **Accept:** every existing test passes unchanged, including `make e2e`; a Go test opens a second session and shows that saving or closing it does not affect the main session's exit code.

### Phase 2 — `internal/dirdiff` and CLI

- The comparison package with table-driven tests on temporary directories: added, deleted, modified, unchanged, nested paths, unicode and spaces in names, symlink to file, broken symlink, symlink to directory, binary, empty directory ignored, `.git` skipped, file-vs-directory clash, CRLF-only difference, unreadable file, large file.
- CLI detection of directory arguments; a new session mode `dirdiff` carrying the entries.
- **Accept:** `go test ./...` passes; `mergediff dirA fileB` exits `2` with a clear message.

### Phase 3 — Directory window and opening files

- `filelist.ts`, the directory window (flat list), opening files in child windows.
- **Accept:** `mergediff left/ right/` shows the list as in `mergediff-dir-diff-*.png`; opening Modified, Added and Deleted files works; binary rows show why they cannot be opened; closing the list closes the child windows and exits `0`.

### Phase 4 — Filter, chips, grouping, keyboard, performance

- Filter text, chips, `Show unchanged`, `Group by directory` as in `mergediff-dir-diff-grouped-*.png`, all keyboard shortcuts, virtual scrolling.
- **Accept:** frontend tests cover grouping (root first, sorting, counts, collapse), filtering and path splitting; the 20,000-file target is met (add a generator to `scripts/e2e/gen-large.py` or similar).

### Phase 5 — End-to-end, docs, finish

- `scripts/e2e`: a temporary Git repo with changes, `git difftool -d --tool=mergediff` opens the list; open a file; close; exit code `0`.
- README: directory diff usage, `git difftool -d`, a screenshot placeholder (mockups may stand in).
- `CLAUDE.md`: add the directory diff to the CLI, Bound API, UI specification and Phases sections; remove "Directory comparison" from "Out of scope"; describe the per-window session model.
- **Accept:** `make check` passes (and `make e2e` where a display is available).

## Finishing the feature

1. Make sure the branch is up to date with `main` (`git fetch && git rebase origin/main` or merge, whichever the history already uses), and `make check` passes.
2. Push the branch and open a pull request into `main` (`gh pr create` if `gh` is available; otherwise print the compare URL). Use the summary of all phases as the description.
3. **Stop.** Do not merge the PR yourself; the maintainer reviews and merges it. Feature 2 starts only after this branch is merged into `main`.

## Out of scope

Editing files from the directory view, copying files between sides, three-way directory merge without Git, ignore patterns or whitespace options, a nested tree view.
