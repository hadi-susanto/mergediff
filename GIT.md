# Using mergediff with Git

mergediff works as Git's merge tool and diff tool. Git starts it with the
right files and reads its exit code, so you keep using the usual commands:

| Command           | What mergediff shows                                      |
|-------------------|-----------------------------------------------------------|
| `git mergetool`   | a 3-way merge for each conflicted file, one after another |
| `git difftool`    | a 2-way diff for each changed file, one after another     |
| `git difftool -d` | one window listing every changed file (directory diff)    |
| `mergediff resolve` | one window listing every conflicted file (no Git setup needed) |

Make sure `mergediff` is on your `PATH` first (see
[Installation](README.md#installation)).

## Setup

Run once; this writes to your global `~/.gitconfig`:

```sh
git config --global merge.tool mergediff
git config --global mergetool.mergediff.cmd 'mergediff "$BASE" "$LOCAL" "$REMOTE" -o "$MERGED"'
git config --global mergetool.mergediff.trustExitCode true
git config --global diff.tool mergediff
git config --global difftool.mergediff.cmd 'mergediff "$LOCAL" "$REMOTE"'
```

Leave out `--global` to configure only the current repository.

Optional:

```sh
# Start every merge with the non-conflicting changes already applied.
git config --global mergetool.mergediff.cmd 'mergediff "$BASE" "$LOCAL" "$REMOTE" -o "$MERGED" --auto-apply'

# Do not ask "Launch 'mergediff'?" before each file of git difftool.
git config --global difftool.prompt false

# Do not keep the conflicted file as FILE.orig after a successful merge.
git config --global mergetool.keepBackup false
```

## Resolving conflicts: `git mergetool`

When a `merge`, `rebase`, `cherry-pick`, `revert` or `stash pop` stops with
conflicts, run:

```sh
git mergetool                 # every conflicted file, one after another
git mergetool path/to/file    # just this file
```

For each file mergediff opens **Local | Result | Remote**. Git passes four
paths: `$BASE` (the common ancestor), `$LOCAL` (your side), `$REMOTE` (the
other side) and `$MERGED` (the working-tree file the result is written to).
Git does not pass branch names, so the panes are labelled with file names.

With `trustExitCode`, Git uses mergediff's exit code to decide whether the
file is resolved:

| Exit | When                                                        | Git                                 |
|------|-------------------------------------------------------------|-------------------------------------|
| `0`  | *Save* with no conflict left                                | marks the file resolved (`git add`) |
| `1`  | *Cancel*, closing the window, or saving with conflicts left | keeps the conflict                  |
| `2`  | the files could not be read (binary, not UTF-8, I/O error)  | keeps the conflict                  |

During a **rebase**, Git's "local" side is the branch being rebased onto
and "remote" is your own commit being replayed; mergediff shows what Git
passes.

## All conflicts in one window: `mergediff resolve`

`git mergetool` starts its tool once per file, so it cannot show a list.
`mergediff resolve` talks to Git itself, like the JetBrains IDEs' Conflicts
dialog, and needs no configuration. Run it anywhere in the working tree
after Git stopped with conflicts, or later, after resolving some by hand:

```sh
mergediff resolve                  # every conflicted file of the repository
mergediff resolve src/ pom.xml     # only these (relative to the current directory)
mergediff resolve --auto-apply     # merge windows start with non-conflicting changes applied
```

To run it as `git resolve` (the `cd` keeps `PATH` arguments relative to
where you are; Git runs aliases from the top level):

```sh
git config --global alias.resolve '!f() { cd "${GIT_PREFIX:-.}" && mergediff resolve "$@"; }; f'
```

|                  | `git mergetool`                         | `mergediff resolve`                                       |
|------------------|-----------------------------------------|-----------------------------------------------------------|
| Files            | one window after another                | one list; open, accept or skip them in any order          |
| Whole-file sides | not offered                             | *Accept Yours* / *Accept Theirs*, for several files at once |
| Pane labels      | file names                              | branch names                                              |
| Binary, symlink, modify/delete | Git's own prompts         | accepted from the list                                    |
| During a rebase  | Git's sides: "local" is the upstream    | swapped: *Yours* is your commit                           |
| Setup            | `merge.tool` and `mergetool.*`          | none                                                      |

What it reads and runs:

- `git ls-files --unmerged` lists the conflicts; `git cat-file --filters`
  reads base / yours / theirs with the checkout's line endings. A file's
  status on each side (Modified, Added, Deleted) comes from which stages
  exist.
- **Merge…** opens the 3-way view on those texts and writes the result to
  the working-tree file, then `git add`.
- **Accept** runs `git checkout --ours|--theirs -- FILE` and `git add`, or
  `git rm` when the accepted side deleted the file. While rebasing it passes
  the opposite flag, so *Yours* always means your work.
- These are the only commands that change anything. mergediff never commits,
  continues, aborts, resets or stashes: when all are resolved it shows the
  command to run next (`git commit`, `git merge --continue`,
  `git rebase --continue`, `git cherry-pick --continue` or
  `git revert --continue`) with a *Copy* button.

**Edited since Git stopped:** if you have already worked on a conflicted
file by hand (and not `git add`ed it), mergediff notices by merging the
stages again (`git merge-file` on temporary copies, in your
`merge.conflictStyle`) and comparing. The file is flagged, and *Accept* or
saving a merge over it asks first, because both replace the whole file.

Exit codes: `0` when no conflicts are left within the given paths as the
window closes (or there were none: "mergediff: no conflicts to resolve"),
`1` when some are left, `2` outside a Git working tree or with a `git`
older than 2.11.

## Viewing changes file by file: `git difftool`

```sh
git difftool                  # working tree vs the index
git difftool HEAD             # working tree vs the last commit
git difftool --cached         # staged changes
git difftool main..feature    # between two commits or branches
git difftool -- src/          # limited to some paths
```

Each changed file opens in a read-only 2-way diff; close the window (or
press `Esc`) to go to the next one. Closing exits `0`.

## Viewing all changes at once: `git difftool -d`

`-d` (`--dir-diff`) shows every changed file in **one** window. It uses the
same `difftool.mergediff.cmd`; no extra setup is needed.

```sh
git difftool -d                   # working tree vs the index
git difftool -d HEAD              # working tree vs the last commit
git difftool -d --cached          # staged changes
git difftool -d main...feature    # what feature changed since it left main
git difftool -d HEAD~3 -- src/    # last three commits, under src/
```

Git copies the changed files of each side into two temporary directories
and starts mergediff once with them, so mergediff compares two directories.
Files on the working-tree side are symlinks to your real files; mergediff
follows them. Git copies only changed files, so the list normally has no
unchanged files.

In the window:

- Files are listed by path with their status (**M**odified, **A**dded,
  **D**eleted) and the number of lines added and removed.
- Double-click a file, or select it and press `Enter`, to open it in a
  2-way diff window of its own. An added or deleted file opens with the
  missing side empty, titled `(missing)`. Opening a file that is already
  open brings its window to the front.
- Binary files, files that are not valid UTF-8 and files over 5 MB are
  listed but cannot be opened; the status bar says why.
- Filter by typing part of a path (`Ctrl+F`), pick a status with the chips,
  or tick **Group by directory**.
- Closing the list closes the files opened from it. mergediff exits `0`;
  Git then deletes the temporary directories.

The list is read-only: nothing you do in it changes your files.

## Comparing two directories without Git

The same window works on any two directories:

```sh
mergediff path/to/old path/to/new
```

There every file is compared, so unchanged files are counted too; tick
**Show unchanged** to list them. `.git` directories are skipped.

## Troubleshooting

- **`mergediff: command not found`, or Git says the tool is not
  available:** mergediff is not on the `PATH` Git sees. Use the full path in
  the `cmd` settings, e.g. `/usr/local/bin/mergediff "$LOCAL" "$REMOTE"`.
- **macOS:** link the app's binary onto your `PATH` as described in
  [Installation](README.md#installation); Git cannot start `mergediff.app`
  itself.
- **Check what Git will run:** `git config --get-regexp '^(merge|diff)tool'`.
- **A merge shows "file looks binary" or "not valid UTF-8" (exit `2`):**
  mergediff merges UTF-8 text only; accept one side in `mergediff resolve`,
  or use `git checkout --ours` / `--theirs` or another tool.
- **`mergediff resolve` says "not inside a Git working tree", "git not
  found" or "git is too old":** run it inside the repository; it needs `git`
  2.11 or later on the `PATH`.
- **A submodule conflict:** mergediff lists it but leaves it to you: check
  out the commit you want inside the submodule, then `git add` it.
