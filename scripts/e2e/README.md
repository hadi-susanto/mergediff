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
- `perf.sh` — a 10,000-line merge with ~430 changes: time to interactive,
  responsiveness right after fast scrolling, typing and "apply all", saving
- `gen-large.py` — generates that merge (`gen-large.py DIR [LINES]`), also
  handy for manual checks
- `lib.sh` — shared helpers (`check`, `ui`, `start_app`, …)
- `ui.py` — the driver: `wait`, `dump`, `click NAME[#N]`, `key COMBO`,
  `type TEXT`, `scroll X Y N`, `clickat X Y`, `pixel X Y`, `find TEXT`,
  `samples TITLE SECS`

The scripts use their own config file (`XDG_CONFIG_HOME` points into a
temporary directory), so your settings never affect them. `ui.py` refuses
to send keys, clicks or wheel events unless a mergediff window is active,
so input can never land in another application.

Every app run and UI step is time-limited (`APP_TIMEOUT`, default 60 s), so
a step that fails reports `FAIL` instead of leaving a window open.

## Writing checks

Buttons are found by their accessible name, which is their visible label
(`Save`, `↓ Conflict`) or, for symbol-only buttons, their tooltip
(`Next change (F7)`, `Ignore this change`). Several buttons can share a
name; `NAME#N` picks the N-th in page order: toolbar, then the Local→Result
strip, then the Result→Remote strip. `ui.py dump` lists them. Keep the
windows on screen and avoid typing while the scripts run.
