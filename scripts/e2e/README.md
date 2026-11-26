# End-to-end tests

These scripts drive the real mergediff window, so they catch behaviour that
unit tests cannot: Git integration, exit codes, line endings on disk,
keyboard handling in the webview (e.g. WebKitGTK sending Ctrl+Z twice) and
dialogs. Run them after UI changes and before upgrading Wails, CodeMirror or
other dependencies.

```
make e2e        # builds, then runs scripts/e2e/run.sh
```

## Requirements

Linux with an X11 session (they open real windows on your desktop), plus:

- `python3-pyatspi` — finds and clicks buttons through the accessibility tree
- `python3-xlib` — XTest key presses and mouse wheel (real modifier keys)
- `wmctrl` — raises the window so key presses reach it
- `xprop` (x11-utils) — reads the window state (maximized)
- `git`

`run.sh` checks these first and exits 2 if something is missing. The
scripts are not part of `make check` or CI, which have no display.

### X11 only, no Wayland yet

Log in to an **X11 session** to run them (tested on Linux Mint 22.3 / X11).
In a Wayland session GTK4 opens mergediff as a native Wayland window, which
`wmctrl`, `xprop`, XTest and the screen-pixel checks cannot see. The scripts
then fail with "window not found" or ui.py's "refusing to send input"; they
never type into another window. Only the AT-SPI parts (finding and clicking
buttons, reading text) work on Wayland.

Probable fix, not yet tried: start the app with `GDK_BACKEND=x11` when
`WAYLAND_DISPLAY` is set (in `start_app`, `lib.sh`), so it runs through
XWayland where the X11 tools work. Whether screen pixels can be read under
XWayland is unverified. The app itself is not X11-specific.

## Files

- `run.sh` — preflight checks, runs every script, overall status
- `mergetool.sh` — `git mergetool` on a real conflicting merge (LF and CRLF
  files): resolve and save (exit 0, file staged), "From Remote only",
  cancel and save-with-conflicts (Git keeps the conflict), `--auto-apply`
  output equals the engine's
- `shortcuts.sh` — F7 / Shift+F7, Ctrl+Shift+A and its single-step undo,
  Ctrl+S with and without conflicts, Esc in dialogs and windows, undo not
  touching earlier typing, the diff window
- `settings.sh` — `mergediff settings` (set keeps comments, bad values
  leave the file alone), a dark theme and maximized window from the config
  file, `--theme` / `--window-state` overriding it, no frame painted in the
  wrong theme at startup, a broken config (exit 2)
- `scratch.sh` — `mergediff` without arguments: typing into both panes,
  Refresh by F5, Ctrl+Enter and the button, the out-of-date state after an
  edit, Esc not closing, Close exiting 0
- `dirdiff.sh` — the directory diff on `testdata/dev/dirdiff`: double-click
  and End + Enter open files in their own windows, a file that cannot be
  opened says why, Esc closes the list and its windows (exit 0); then
  `git difftool -d` on a temporary repository (modified, added, deleted)
  opens the list, a file opens, and Git exits 0 when the list closes
- `resolve.sh` — `mergediff resolve` on the mockups' conflicted repository
  (`scripts/dev/conflict-repo.sh`): rows selected by real clicks (also in
  the grouped list), Accept Theirs on a binary file (bytes checked), Ctrl+click
  multi-select and its confirmation, modify/delete buttons (`git rm`), F5
  after resolving in a terminal, Ctrl+Shift+T; Show changes opens a diff
  window; a file edited since Git stopped is flagged and accepting it asks
  first; double-click opens the merge window (branch names in its headers,
  row locked, a second double-click focuses it), closing the list with an
  unsaved merge asks, Save stages the file and refreshes the list; the
  all-resolved banner and Copy; exit 0 from the window's close button when
  all are resolved, exit 1 with conflicts left, exit 2 outside a
  repository, "no conflicts" with exit 0
- `perf.sh` — a 10,000-line merge with ~430 changes: time to interactive,
  responsiveness right after fast scrolling, typing and "apply all", saving;
  and a 20,000-file directory diff (500 changed) shown within 3 s of starting
- `gen-large.py` — generates that merge (`gen-large.py DIR [LINES]`), also
  handy for manual checks
- `gen-dirs.py` — generates two directory trees for the directory diff
  (`gen-dirs.py DIR [FILES] [CHANGED]`, default 20,000 and 500)
- `lib.sh` — shared helpers (`check`, `ui`, `start_app`, …)
- `ui.py` — the driver: `wait`, `dump`, `click NAME[#N]`, `key COMBO`,
  `type TEXT`, `scroll X Y N`, `clickat X Y`, `clickin TITLE X Y`,
  `modclickin ctrl|shift TITLE X Y`, `dblclickin TITLE X Y`,
  `pixel X Y`, `find TEXT`, `samples TITLE SECS`

The scripts use their own config file (`XDG_CONFIG_HOME` points into a
temporary directory), so your settings never affect them. `ui.py` refuses
to send keys, clicks or wheel events unless a mergediff window is active,
so input can never land in another application.

Every app run and UI step is time-limited (`APP_TIMEOUT`, default 60 s), so
a step that fails reports `FAIL` instead of leaving a window open.

## Writing checks

Buttons (and the directory view's filter chips and checkboxes) are found by
their accessible name, which is their visible label
(`Save`, `↓ Conflict`) or, for symbol-only buttons, their tooltip
(`Next change (F7)`, `Ignore this change`). Several buttons can share a
name; `NAME#N` picks the N-th in page order: toolbar, then the Local→Result
strip, then the Result→Remote strip. `ui.py dump` lists them. While a
dialog is open only its buttons are listed (the page behind it is inert). Keep the
windows on screen and avoid typing while the scripts run.
