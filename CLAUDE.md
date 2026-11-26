# mergediff — Project Instructions for Claude Code

## Goal

Build **mergediff**: a lightweight, cross-platform, open-source **3-way merge** and **2-way diff** GUI tool, inspired by the JetBrains merge tool. It must ship as a small native binary for Windows, macOS and Linux, and work as a Git `mergetool` / `difftool`, including `git difftool -d` (directory diff). `mergediff resolve` lists a stopped merge's conflicts in one window (Feature 2).

## Working rules

- **Always use the Task tool to track progress.** At the start of each phase, break it into tasks; mark each task in progress when you start it and completed when it is done and verified. Never work on something that is not tracked as a task.
- Work **one phase at a time** (see "Phases"). Do not start the next phase until the current phase's acceptance criteria are met and all tests pass. At the end of each phase, stop and give a short summary of what was done and anything left open.
- Ask before adding any dependency not listed under "Stack".
- Adding tests is pre-approved and expected: when behaviour changes, add or extend Go tests, frontend `node --test` tests and `scripts/e2e` checks without asking, so dependency upgrades that change behaviour are caught. (New test *dependencies* still need approval.)
- Before using a third-party library, check its current API (docs or source) rather than guessing.
- Never edit generated files (`frontend/bindings/`, `frontend/dist/`, `bin/`).
- Keep code formatted (`gofmt`, Prettier defaults) and free of vet / type errors.
- Make small, focused commits, one per step/completed task, with clear messages. Intermediate commits may have a broken build or failing tests; the **last commit of a phase** must be `gofmt`-clean with all tests green.

## Prerequisites

- Go (latest stable), Node.js LTS + npm
- Wails v3 CLI, pinned to the same version as go.mod:
  `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`, then `wails3 doctor`
- C toolchain for cgo: Xcode command-line tools (macOS), gcc + pkg-config (Linux).
  Windows needs none (Wails on Windows does not use cgo).
- Linux: GTK4 and WebKitGTK 6.0 dev packages (Ubuntu/Debian:
  `libgtk-4-dev libwebkitgtk-6.0-dev`; confirm with `wails3 doctor`)
- `git` 2.11+: used at runtime by `mergediff resolve` (only), and by tests

## Stack

- **Backend:** Go + Wails **v3.1.0** (pinned; upgrade only on request).
  Default cgo backend on macOS and Linux. The `purego` tag (CGO-free builds) is
  experimental in v3.1.0 and not used; revisit when Wails marks it stable.
- **Frontend:** Vite + TypeScript 5.x, **no UI framework** (vanilla TS)
- **Editor:** CodeMirror 6 (`@codemirror/state`, `@codemirror/view`, `@codemirror/language`, `@codemirror/commands`, plus language packages for syntax highlighting). `@codemirror/merge` may be used for the 2-way view or as reference, but the 3-pane merge view is custom.
- **Frontend tests:** Node's built-in runner (`node --test`, Node 24 type stripping), no test framework dependency. Logic worth testing (chunk tracking, apply/undo) lives in DOM-free modules that import each other with explicit `.ts` extensions and use no TypeScript enums (Node only strips types); they may import types, not values, from `frontend/bindings/`. `@types/node` (pinned to Node 24's major) lets `tsc` type-check the test files.
- **End-to-end tests:** `scripts/e2e/` drives the real window (Linux/X11 only) through AT-SPI (`python3-pyatspi`), XTest (`python3-xlib`) and `wmctrl`: Git mergetool runs, exit codes, file bytes, shortcuts and dialogs. Run with `make e2e`; not part of `make check` or CI (no display). See `scripts/e2e/README.md`.
- **Line and character diff:** `github.com/sergi/go-diff` (diffmatchpatch). Line diff uses its line mode (`DiffLinesToRunes` + `DiffMainRunes`) with `DiffTimeout = 0` so results are exact (a timeout also enables a non-optimal "half-match" shortcut). `github.com/aymanbagabas/go-udiff` was evaluated and rejected: its LCS search is capped at ~50 edits, after which the rest of the file collapses into one huge hunk.

## Repository layout

```
mergediff/
  go.mod                  # module github.com/hadi-susanto/mergediff
  main.go                 # Wiring only: cliargs.Parse → command.Select → Run(env) → os.Exit(code);
                          #   report (errors → stderr, exit 2); runGUI (the window, Env.OpenWindow);
                          #   //go:embed all:frontend/dist
  version.go              # versionDate / commit vars set via -ldflags, Version() string
  app.go                  # App: one window's state (session, exit code, close guard); thin adapters
  windows.go              # Windows: the bound Wails service; one App per window, child windows
  crash.go                # Crash report to stderr + crash log (see "Panics")
  console_windows.go      # Windows GUI binary: attach to the parent console for CLI output
  release_test.go         # Release workflow tag filter + scripts/ci checks
  devargs.go              # Dev builds only (!production): sample file args for wails3 dev,
                          #   MERGEDIFF_DEV_DIR to run in a directory (make dev-resolve)
  testdata/dev/           # Sample files opened by `make dev`; dirdiff/{left,right} for the directory
                          #   diff (exact bytes kept: -text in .gitattributes)
  Taskfile.yml            # Wails build entry point (generated by wails3 init, user-owned)
  build/                  # Wails build assets (user-owned): config.yml, icons, per-OS Taskfiles,
                          #   Info.plist, Windows manifest. Local edits listed in "Wails v3 setup".
  internal/
    cliargs/              # Command-line tokenizer: flags (one table) and positionals → immutable Option
    command/              # The commands: Select (reserved words, --), one file per command
                          #   (help, version, settings, diff, scratch, merge, resolve), Env, usage text,
                          #   exit codes; window.go loads settings and opens the window via Env
    testrepo/             # Tests only: a Git repository stopped by a conflicted merge
    config/               # Settings registry, TOML config file, flag/file/default layering
    session/              # Builds a Session from CLI paths (load, detect, merge); scratch Compare;
                          #   directory comparison in the background (StartDirDiff, LoadPair);
                          #   resolve sessions (resolve.go: StartResolve, merge / changes windows)
    gitconflict/          # Git access for `resolve`: repo, unmerged list, stages, accept, operation,
                          #   labels, the rebase swap (sides.go), edited-since-stopped check
    textio/               # Reading/writing files, line endings, binary detection
    diff/                 # Line diff wrapper + character-level diff
    dirdiff/              # Directory comparison: walk, pair, compare, line stats, progress
    merge/                # diff3 merge engine
      testdata/           # Merge cases: <name>/base, local, remote, expected, chunks (format in cases_test.go)
  frontend/
    bindings/             # Generated by wails3 generate bindings (committed, do not edit)
    dist/                 # Vite output (gitignored except .gitkeep, see "Wails v3 setup")
    public/.gitkeep       # Copied into dist/ by Vite so dist/.gitkeep survives builds
    src/
      main.ts             # Bootstraps the session, builds layout
      panes.ts            # Creates the CodeMirror editors
      chunks.ts           # Chunk state, tracking ranges through edits
      highlight.ts        # Line + character decorations
      gutter.ts           # Accept / ignore buttons
      scrollsync.ts       # Chunk-aligned scrolling
      connectors.ts       # SVG curves between panes
      toolbar.ts          # Toolbar, navigation, conflict counter, shortcuts
      filelist-core.ts    # DOM-free file list logic: paths, filter, chips, grouping, selection,
                          #   virtual scrolling window (shared with Feature 2's conflicts list)
      filelist.ts         # Generic file list renderer (columns, group headers, keys, virtual rows)
      dirview.ts          # Directory diff window
      filterbar.ts        # Filter bar (text, chips, checkboxes) shared by dirview and resolveview
      resolve-core.ts     # DOM-free Conflicts window logic: chips, texts, actions, confirmations
      resolveview.ts      # Conflicts window (mergediff resolve)
      styles.css
  bin/                    # Build output (gitignored)
  scripts/e2e/            # End-to-end UI tests (make e2e; Linux/X11)
  scripts/dev/            # conflict-repo.sh: the mockups' conflicted sample repository
  scripts/ci/             # Release workflow helpers (version-from-tag.sh)
  .github/workflows/release.yml   # Triggered by pushing a vYYYY-MM-DD tag
  docs/mockups/           # UI design reference (see "Design reference")
    mergediff-mockup-light.png
    mergediff-mockup-dark.png
    mergediff-dir-diff{,-grouped}-{light,dark}.png    # directory diff (01-directory-diff.md)
    mergediff-dir-merge{,-grouped}-{light,dark}.png   # conflicts window (02-resolve-conflicts.md)
  docs/screenshots/       # Screenshots of the real app (shown in SCREENSHOT.md)
    mergediff-{light,dark}.png                        # testdata/dev merge, 1280×400
    mergediff-resolve{,-grouped}-{light,dark}.png     # conflict-repo.sh, 1400×860
  .gitignore              # Must cover all build/dev output (see "Makefile")
  Makefile                # Dev, build, test and release automation
  LICENSE                 # MIT
  README.md
  SCREENSHOT.md           # All screenshots (README shows one and links here)
  GIT.md                  # Git integration: mergetool, difftool, difftool -d, resolve (linked from README)
  01-directory-diff.md    # Feature 1 instructions (done)
  02-resolve-conflicts.md # Feature 2 instructions (done)
  03-refactor.md          # Refactor 1 instructions: command line (done)
```

### Wails v3 setup

Standard Wails v3 project layout: `package main` at the repo root, built by
`wails3 build` (Taskfile.yml + build/). The Makefile wraps the wails3 commands.

- **Production build** (`make build`): `wails3 build EXTRA_LDFLAGS="$(LDFLAGS)"`
  → `bin/mergediff`. macOS universal: `wails3 task darwin:build:universal
  EXTRA_LDFLAGS="$(LDFLAGS)"` (CI on macOS).
- **Dev mode** (`make dev`): `wails3 dev`, which starts the app without
  arguments. In builds without the `production` tag, `devargs.go` then reads
  space-separated arguments from `MERGEDIFF_DEV_ARGS`; `make dev` sets it from
  `ARGS` (default: the `testdata/dev/` pair, e.g.
  `make dev ARGS="testdata/dev/base.go testdata/dev/local.go testdata/dev/remote.go -o /tmp/out.go"`).
  Production builds never read the variable.
- **Local edits to build assets** (re-apply after `wails3 update build-assets`):
  - `build/{linux,darwin,windows}/Taskfile.yml`: production `BUILD_FLAGS` use
    `-ldflags="-w -s {{.EXTRA_LDFLAGS}}"` so the version can be injected.
  - `build/config.yml`: app info (name, identifier, description, copyright)
    and dev-mode sample arguments.
  - `build/Taskfile.yml` `generate:icons`: `windows/icon.ico` from
    `appicon.png`, `darwin/icons.icns` from `darwin/appicon.png` (padded to
    Apple's icon grid). On macOS with Xcode 26+ the Icon Composer file
    `appicon.icon` (foreground layer `Assets/mergediff.svg`, background from
    its `fill`) produces `darwin/Assets.car` and `icons.icns` instead;
    `Assets.car` is build output (gitignored).
- **App icon:** `build/appicon.svg` is the source. `build/appicon.png` and
  `build/darwin/appicon.png` are 1024×1024 transparent renders of it (the
  macOS one scaled to an 824 px body); re-render both and update
  `appicon.icon/Assets/mergediff.svg` when the SVG changes.
- **`frontend/dist/.gitkeep`** is committed (and copied from
  `frontend/public/.gitkeep` by Vite) so `//go:embed all:frontend/dist`
  compiles before the frontend is built; `go vet ./...` and `go test ./...`
  must work on a fresh clone.
- **Exit codes:** Wails has no "quit with code" API. `App.Quit(code)` stores
  the code and calls `app.Quit()`; `main` calls `os.Exit(code)` after
  `app.Run()` returns.
- Phase 2 starts by proving this setup (a minimal window, a version-injected
  build, `go test ./...` on a fresh clone). If any part doesn't work, stop and
  explain the problem and options.

Rules for the root `main` package:

- Only startup and wiring: `run(args, env)` parses the arguments
  (`cliargs.Parse`), picks the command (`command.Select`), runs it with the
  real `command.Env` and exits with its code; `report` maps errors to output
  and exit `2`. `runGUI` configures and runs Wails and is the Env's
  `OpenWindow`. Command logic lives in `internal/command`, not here.
- `App` methods are thin adapters that call `internal/...` packages; no
  diff/merge logic lives in the root package.
- `internal/` packages must not import Wails, so they stay testable with plain
  `go test`.

## Command-line interface

```
mergediff BASE LOCAL REMOTE -o MERGED    # 3-way merge mode
mergediff BASE LOCAL REMOTE -o MERGED --auto-apply
                                       # same, but non-conflicting changes are applied at startup
mergediff LEFT RIGHT                     # 2-way diff mode (read-only)
mergediff LEFTDIR RIGHTDIR               # directory diff (both directories; git difftool -d)
mergediff                                # scratch mode: editable 2-way diff of pasted text
mergediff resolve [--auto-apply] [--] [PATH...]
                                       # Conflicts window for the Git working tree containing the cwd
mergediff settings [list | get KEY | set KEY VALUE | unset KEY | path]
                                       # view or change the config file (no window)
mergediff help [settings | resolve]      # general usage, or the settings usage
mergediff version                        # same as --version
mergediff --help | --version            # --version prints e.g. "mergediff 2026-11-19 (a1b2c3d)"
```

Every setting with `Flag: true` in the registry is also a flag, valid with any
window mode: `--theme system|light|dark`, `--window-state normal|maximized`
(`--KEY VALUE` or `--KEY=VALUE`).

**Reserved words** — `help`, `version`, `settings`, `resolve` — are
subcommands when they are the first positional argument **before `--`**
(options may come first: `--theme dark settings list` is the settings
command). For a file with such a name, put `--` before the paths (preferred,
options go before it: `mergediff -o M -- help L R`) or write `./help`. A
subcommand handles `--help` / `--version` itself: `settings --help` is the
settings usage, `resolve --help` the general usage, `resolve --version` the
version. Without a subcommand `--help` wins, then `--version`, then the
number of paths (3 merge, 2 diff, 0 scratch).

- `resolve` takes `--auto-apply` (passed to every merge window it opens),
  settings flags and `PATH...` limits relative to the cwd; `-o` is a usage
  error.
- `settings` rejects `-o`, `--auto-apply`, `--version` and settings flags
  (pointing at `settings set`). `help` takes one topic at most and no window
  flags; `version` takes no arguments.

Arguments are parsed in two steps, both **before** starting Wails, so bad
arguments print usage and exit without opening a window:

- `internal/cliargs` tokenizes: one flag table (`-o`/`--output`,
  `--auto-apply`, `-h`/`--help`, `-v`/`--version`, the registry's settings
  flags; unknown flags are errors, a new flag is a new table entry), the
  positionals in order, and where `--` was. `Option` is immutable
  (unexported fields, getters return copies). It knows nothing about
  commands.
- `internal/command.Select` picks the command and each command's
  constructor checks its arguments; `Run(env)` does the work. Commands use
  `Env` (stdout/stderr, Getwd, ConfigPath, Version, OpenWindow) instead of
  globals, never import Wails, and live one per file.

Exit codes:

- `0` — merge saved and all conflicts resolved (or diff / directory / scratch window closed normally, or a `settings` command succeeded). `resolve`: no conflicts left within its paths when the window closes (asked from Git, not the UI), or none at start (`mergediff: no conflicts to resolve` on stdout, no window)
- `1` — user cancelled, or closed with unresolved conflicts / unsaved changes; `resolve`: closed with conflicts left
- `2` — usage error or I/O error (message on stderr); `resolve`: not in a working tree, `git` missing or older than 2.11

Two paths that are both directories mean a directory diff; one directory and
one file is a usage error naming which is which (exit `2`). A path that
cannot be read counts as a file, so it is reported as an I/O error.

Git configuration to document (README gives the setup; GIT.md explains
mergetool, difftool and `difftool -d` in detail):

```
git config --global merge.tool mergediff
git config --global mergetool.mergediff.cmd 'mergediff "$BASE" "$LOCAL" "$REMOTE" -o "$MERGED"'
git config --global mergetool.mergediff.trustExitCode true
git config --global diff.tool mergediff
git config --global difftool.mergediff.cmd 'mergediff "$LOCAL" "$REMOTE"'
```

## Versioning and releases

### Version format

The version is `YYYY-MM-DD (short-commit)`, for example `2026-11-19 (a1b2c3d)`.

- **Release builds:** the date comes from the release tag (`v2026-11-19` → `2026-11-19`); the commit is the 7-character short hash of the tagged commit.
- **Local / untagged builds:** the date is today's date in **UTC**, and the commit is the short hash of `HEAD`. If the working tree is not clean, append `-dirty` to the hash: `2026-11-19 (a1b2c3d-dirty)`. "Not clean" means `git status --porcelain` prints anything: modified or staged tracked files **and untracked files** (an untracked `.go` file is still compiled into the binary). Files matched by `.gitignore` do not count.
- Before the first commit (no `HEAD`), the commit part is `unknown` (plus `-dirty` if applicable), never an empty string or a git error.
- Builds without ldflags (e.g. plain `go run`) report `dev (unknown)`.

Inject the two parts separately to avoid quoting problems with the space and parentheses:

```go
// version.go
package main

var (
	versionDate = "dev"     // set via -ldflags "-X main.versionDate=YYYY-MM-DD"
	commit      = "unknown" // set via -ldflags "-X main.commit=a1b2c3d"
)

func Version() string { return versionDate + " (" + commit + ")" }
```

Show the version in `--version` output and at the right end of the window's status bar (`mergediff 2026-11-19 (a1b2c3d)`, see the mockups).

### Tag format and release flow

- Release tags are `vYYYY-MM-DD` (e.g. `v2026-11-19`), annotated, on a commit already pushed to `main`.
- Only **one release per day** is supported by this format. If today's tag already exists (locally or on the remote), `make release` must fail with a clear message and must never move or overwrite an existing tag.
- Pushing the tag triggers the GitHub Actions release workflow. Nothing else triggers a release.

### Makefile

Use GNU Make with POSIX shell commands (works on Linux, macOS, and Windows via Git Bash). Compute version values once at the top:

```make
COMMIT       := $(shell git rev-parse --short=7 HEAD 2>/dev/null || echo unknown)
DIRTY        := $(shell test -z "$$(git status --porcelain 2>/dev/null)" || echo -dirty)
DATE         := $(shell date -u +%Y-%m-%d)
VERSION_DATE ?= $(DATE)
LDFLAGS      := -X main.versionDate=$(VERSION_DATE) -X main.commit=$(COMMIT)$(DIRTY)
TAG          := v$(DATE)
```

Because untracked files count as dirty, `.gitignore` must cover everything that dev and build steps produce (e.g. `bin/`, `frontend/dist/` except its `.gitkeep`, `frontend/node_modules/`, coverage/test output), so a normal build never marks the tree dirty.

Required targets (each with a one-line `## description` shown by `make help`):

- `help` (default) — list targets
- `deps` — `go mod download`, `npm ci` in `frontend/`, `wails3 doctor`
- `dev` — `wails3 dev`
- `build` — `wails3 build EXTRA_LDFLAGS="$(LDFLAGS)"` (output `bin/mergediff`), then fail if `frontend/dist/index.html` is missing (the binary would have no UI)
- `test` — full test from scratch: `clean`, then `check` with uncached Go tests (`GOTESTFLAGS=-count=1`), then `e2e`. Needs the e2e prerequisites; where there is no display (CI), use `check`.
- `test-backend` — `go test $(GOTESTFLAGS) ./...`
- `vet` — `go vet ./...` and `gofmt -l` (fail if any file listed)
- `typecheck` — `npx tsc --noEmit` in `frontend/`
- `test-frontend` — `node --test` on `frontend/src/**/*.test.ts`
- `check` — `vet`, `typecheck`, `test-backend`, `test-frontend`
- `e2e` — `build`, then `scripts/e2e/run.sh` (Linux/X11 only; not part of `check`)
- `version` — print the version string the build would embed
- `clean` — remove `bin/`, built files in `frontend/dist` (keep `.gitkeep`) and Wails' task cache `.task/`. The cache must go too: its checksums would otherwise call the frontend build up to date (the committed `dist/.gitkeep` satisfies its output check) and an empty `dist` would be embedded.
- `release` — create and push today's tag. It must, in order, and stop at the first failure:
  1. confirm the current branch is `main`
  2. confirm the working tree is clean: `git status --porcelain` is empty (no `-dirty`; untracked files also fail this step, and the message lists them)
  3. `git fetch origin --tags` and confirm local `main` equals `origin/main`
  4. confirm tag `$(TAG)` does not exist locally or on `origin`
  5. run `make check`
  6. print the tag and commit, then ask `Create and push $(TAG)? [y/N]` (skip the prompt when `YES=1` is set)
  7. `git tag -a $(TAG) -m "Release $(TAG)"` and `git push origin $(TAG)`
  8. print the URL of the Actions run page: `https://github.com/hadi-susanto/mergediff/actions`
- `release-dry-run` — run steps 1–5 and print what would be tagged, without tagging or pushing

### GitHub Actions release workflow (`.github/workflows/release.yml`)

- Trigger: `on: push: tags: ['v[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]']`. Permissions: `contents: write`.
- First step in every job: derive `VERSION_DATE` by stripping the leading `v` from the tag, validate it is a real `YYYY-MM-DD` date, and fail otherwise. Derive `COMMIT` with `git rev-parse --short=7 HEAD`.
- **test** job (ubuntu-latest): Go tests, vet, gofmt check, frontend type-check and frontend tests. The root package imports Wails (cgo), so this job also installs `libgtk-4-dev libwebkitgtk-6.0-dev`; bindings are committed, so it does not need `wails3`.
- Every job that builds or generates bindings installs the `wails3` CLI at the
  version in go.mod.
- **build** job, needs `test`, matrix:
  - `windows-latest` → `windows/amd64`
  - `macos-latest` → `darwin/universal`
  - `ubuntu-latest` → `linux/amd64` (install `libgtk-4-dev libwebkitgtk-6.0-dev` first)
  - Each runs `make build VERSION_DATE=$VERSION_DATE` (so builds use the same Makefile logic as local), then packages the binary as `mergediff_<YYYY-MM-DD>_<os>_<arch>.zip` (`.tar.gz` on Linux) and uploads it as an artifact. The macOS zip holds `mergediff.app` (`wails3 task darwin:create:app:bundle`, `CFBundleShortVersionString`/`CFBundleVersion` set to `YYYY.MM.DD`, ad-hoc signed) instead of the bare binary. Linux also builds `mergediff_<YYYY-MM-DD>_linux_amd64.deb` with `wails3 tool package -format deb` from `build/linux/nfpm/nfpm.yaml` (version `<YYYY-MM-DD>-1`, `/usr/bin/mergediff`, `build/linux/nfpm/mergediff.desktop`, `build/appicon.svg`; `depends` lists the runtime libraries the binary links and must change with them).
- **release** job, needs `build`: downloads all artifacts, generates `SHA256SUMS.txt`, and creates a GitHub Release named `mergediff YYYY-MM-DD (a1b2c3d)` with tag-derived notes (commits since the previous `v*` tag) and all archives attached.
- Pin every third-party action to a specific major version (e.g. `actions/checkout@v4`); verify the current versions before use.
- Code signing / notarization is out of scope for now; mention in the README that macOS users may need to allow the app in Privacy & Security.

## Text handling

- Treat input as UTF-8. If a file contains a NUL byte in its first 8 KB, treat it as binary: show an error and exit `2`. Invalid UTF-8 is also an error (exit `2`), because it cannot round-trip through the UI without being corrupted.
- Detect line endings (LF vs CRLF) from LOCAL (fall back to BASE). Normalize to LF internally; write MERGED using the detected style.
- Preserve whether the file ends with a trailing newline.
- A missing BASE (Git passes an empty/nonexistent file for add/add conflicts) is treated as empty.
- `mergediff resolve` lists binary and invalid-UTF-8 files (and symlinks, submodules) instead of failing: they can only be accepted whole. Stage texts are read with `git cat-file --filters`, so they carry the checkout's line endings; a merge window writes the working-tree file in that style.

## Error handling

Use the standard library only (`errors`, `fmt`). No error or stack-trace
libraries; adding one needs approval like any other dependency.

### Returning errors

- Wrap errors when they cross a function or package boundary, saying what was
  being done and with which input:
  `fmt.Errorf("read LOCAL %s: %w", path, err)`. Always use `%w`, never `%v`,
  so `errors.Is` / `errors.As` keep working.
- Don't repeat what the wrapped error already says (`os` errors already
  include the path). Don't start messages with a capital letter or end them
  with punctuation, and don't use words like "failed to" or "error:".
- Name an error (`var ErrBinary = errors.New(...)`) when a caller needs to
  react to it; only then. Callers check it with `errors.Is`, never by matching
  message text.
- Handle each error once: return it **or** log it, not both.
- `internal/` packages return errors and never print, log, call `os.Exit`, or
  panic on bad input. (Exception: `internal/command` writes a command's
  normal output, such as help text or the settings table, to `Env.Stdout`.) Panics are only for programmer bugs (broken invariants).

### Exit codes and user-facing messages

- Commands return `(code, err)`. Only the root `main` package (`report`)
  turns errors into output and exit code `2`:
  - usage errors (`*cliargs.UsageError`, `*command.UsageError`) →
    `mergediff: <message>`, a blank line and the usage text (the command's
    own, e.g. the settings usage, or the general one) + exit `2`
  - I/O, binary (`textio.ErrBinary`), invalid UTF-8 (`textio.ErrNotUTF8`)
    → `mergediff: <wrapped message>` on stderr, exit `2`
- Errors from bound `App` methods reach the frontend as rejected promises.
  The frontend shows the message in a dialog and never fails silently.
  A failed `Save` keeps the window open with the result untouched, so the
  user can retry or cancel.

### Panics

- `main()` defers a recover handler, and `application.Options.PanicHandler` is
  set for panics in bound methods. (Wails v3.1.0 recovers those itself and
  turns them into a rejected promise; without our handler the app would keep
  running.) Both write the panic value and stack trace to stderr **and** to a
  crash log (`<os.UserCacheDir()>/mergediff/crash-<timestamp>.log`, because a
  GUI app may have no visible stderr), then exit `2`.
- Never recover a panic just to continue; it always ends the program after
  reporting.

### Tests

- Test error paths with `errors.Is` / `errors.As`, not message strings,
  except where the message itself is the contract (e.g. that it includes the
  file path).

## Merge engine (Go)

Data model, serialized to the frontend as JSON:

```go
type ChunkKind string

const (
	Unchanged  ChunkKind = "unchanged"
	LocalOnly  ChunkKind = "local"
	RemoteOnly ChunkKind = "remote"
	BothSame   ChunkKind = "both"
	Conflict   ChunkKind = "conflict"
)

type Range struct {
	Start int `json:"start"` // 0-based line index
	End   int `json:"end"`   // exclusive
}

type Chunk struct {
	Kind   ChunkKind `json:"kind"`
	Base   Range     `json:"base"`
	Local  Range     `json:"local"`
	Remote Range     `json:"remote"`
}
```

Algorithm (classic diff3):

1. Compute line diffs base→local and base→remote.
2. Walk the base and find stable regions where neither side changed the base lines.
3. Between stable regions, classify the unstable region:
   - only local changed → `LocalOnly`
   - only remote changed → `RemoteOnly`
   - both changed to identical text → `BothSame`
   - otherwise → `Conflict`
4. Return chunks covering all three files completely and in order (no gaps, no overlaps).

Initial result text: the **base** text, with nothing applied (as in JetBrains). Non-conflicting changes are applied by the user with the "Apply non-conflicting changes" button (see UI specification), or automatically at startup when `--auto-apply` is passed. Conflict regions always start with the base text.

The engine also exposes a helper that returns the text produced by applying every `LocalOnly`, `RemoteOnly` and `BothSame` chunk (optionally only one side's), used by tests and by `--auto-apply`.

Character-level diff: for paired changed lines, return spans of inserted/deleted characters; merge tiny spans (< 3 chars apart) to avoid noisy highlighting.

## Settings (`internal/config`)

- One registry, `config.Registry`: each `Setting` has a key, a one-line doc,
  a default, its allowed values and whether it is also a command-line flag.
  Loading, validation, `mergediff settings`, the flags, the usage text and
  `App.Settings` all follow it. **Adding a setting = adding a registry entry**
  (plus a typed accessor on `Config` if Go code reads it).
- Current settings: `theme` (`system` | `light` | `dark`) and `window-state`
  (`normal` | `maximized`).
- Config file: TOML (`github.com/BurntSushi/toml`) at
  `<os.UserConfigDir()>/mergediff/config.toml`, flat top-level keys equal to
  the flag names. A missing file means defaults. Unknown keys, bad values and
  syntax errors are errors (exit `2`, message names the file).
- Precedence: flag, then file, then default. The config is read only when a
  window opens, so `--help` / `--version` work with a broken file.
- `settings set` / `unset` edit the file line by line (comments and other
  lines are kept), validate the result, and write atomically.
- Theme: the frontend sets `data-theme` on `<html>` (`theme.ts`); `styles.css`
  keys dark colours on `:root[data-theme="dark"]`. The native title bar
  follows on Windows and macOS; Wails v3.1.0 has no Linux option.

## Bound API (`windows.go`, `app.go`)

`Windows` is registered as the Wails v3 service
(`application.NewService(wins)` in `application.Options.Services`). Its exported
methods are available to the frontend through the generated `frontend/bindings/`.

### One session per window

Every window has its own `App` (session, exit code, close guard). `Windows`
keeps them by **window name** (`main`, `child-1`, …) and registers a window
before creating it, so its frontend can never call in first. Each bound
method takes `ctx context.Context` first (Wails supplies it and leaves it out
of the generated bindings) and forwards to the **calling window's** `App`,
found with `ctx.Value(application.WindowKey)`. (01-directory-diff.md proposed
an id in the window URL; the calling window was chosen instead: no id to
pass around, and a window cannot reach another window's session.)

- The **main window** (opened from the command line) decides the process exit
  code, exactly as before; closing it closes every child window and ends the
  application.
- **Child windows** (`Windows.openChild(parent, key, sess)`) have their own
  `App` whose `Quit` closes only that window; their exit code is ignored.
  One child per `key` and parent: opening it again focuses the open window.
  Closing a window closes the windows it opened.
- Wails delivers custom events to **every** window, so per-window events
  carry the window name: `mergediff:close-requested:<name>`; the frontend
  gets its name with `Window.Name()`.

```go
type Session struct {
	Mode          string   `json:"mode"` // "merge" | "diff" | "scratch" | "dirdiff" | "resolve"
	Paths         []string `json:"paths"` // dirdiff: absolute LEFT, RIGHT
	Base          string   `json:"base"`
	Local         string   `json:"local"`
	Remote        string   `json:"remote"`
	Chunks        []Chunk  `json:"chunks"`
	InitialResult string   `json:"initialResult"`
	Language      string   `json:"language"` // derived from file extension
	Title         string   `json:"title"`    // file name(s) for the window title
	EOL           string   `json:"eol"`      // "LF" | "CRLF", for the status bar
	AutoApplied   bool     `json:"autoApplied"` // --auto-apply: InitialResult already has the non-conflicting chunks
	LocalLabel    string   `json:"localLabel"`  // resolve windows: branch names (merge) or "Base" (Show changes)
	RemoteLabel   string   `json:"remoteLabel"` //   for the pane headers; "" = file names
	EditedOnDisk  bool     `json:"editedOnDisk"` // resolve merge window: the file was edited since Git stopped
}

// All on *Windows; ctx is the calling window.
LoadSession(ctx) (Session, error)
CharDiff(left, right string) []Span       // Span is defined in internal/diff
Save(ctx, result string) error            // writes MERGED with original line endings
Quit(ctx, code int) error                 // closes the window; the main window exits with code
Version() string                          // build version for the status bar
GuardClose(ctx, on bool) error            // merge mode: route the window's close button to the frontend
CloseRequestSeen(ctx) error               // frontend acknowledges a close request
Settings(ctx) (map[string]string, error)  // effective settings by key, e.g. "theme": "dark"
Compare(left, right string) ([]Chunk, error) // scratch mode: diff two texts
DirEntries(ctx) ([]dirdiff.Entry, error)  // dirdiff: waits for the comparison
DirProgress(ctx) (dirdiff.Snapshot, error) // dirdiff: files found / compared so far, done
DirStats(ctx) (dirdiff.StatsBatch, error) // dirdiff: line counts computed since the last call
OpenEntry(ctx, path string) error         // dirdiff: open a file in a child diff window
Conflicts(ctx) (session.ConflictList, error) // resolve: re-read Git; Open = files in merge windows
AcceptSide(ctx, paths, side) (session.ConflictList, error) // resolve: refused for files in merge windows
MergeCounts(ctx, path) (session.MergeCounts, error) // resolve: conflicts / non-conflicting, for the panel
Edited(ctx, paths) ([]string, error)      // resolve: those edited since Git stopped
OpenMerge(ctx, path) error                // resolve: merge window (child key "merge:"+path), or focus it
ShowChanges(ctx, path) error              // resolve: modify/delete diff window ("changes:"+path)
UnsavedMerges(ctx) ([]string, error)      // resolve: merge windows with unsaved edits
SetDirty(ctx, dirty bool) error           // merge window: result edited since load
```

**Resolve sessions** (`session.StartResolve`, `internal/session/resolve.go`):
the window's main session holds the `gitconflict.Repo`, the `PATH` limits
and the rows. Every `Conflicts` call re-runs `ls-files --unmerged`: rows
listed before stay (no longer unmerged = `Resolved`), new ones are added.
`MergeSession(path)` builds a merge session from the stages (yours / theirs
through `Sides`, so rebases are swapped) whose `Save` calls
`Repo.SaveMerged` (write + `git add`); `ChangesSession(path)` a diff of base
against the side that kept a modify/delete file. When a merge window opens
or closes, `Windows` emits `mergediff:conflicts-changed:<parent>` and the
list refreshes (locks, resolved rows). The main window's exit code is asked
from Git when it closes (`Session.Remaining`).

**`internal/gitconflict`** (no Wails imports): every call is
`git -C <top> -c core.quotepath=off --literal-pathspecs …` with `LC_ALL=C`.
Allowed write commands: `checkout --ours|--theirs`, `add`, `rm`, and writing
a merged file; nothing else changes the repository. `Sides` is the only
place that knows the rebase swap. `Repo.Edited` re-merges the stages with
`git merge-file -p` in temporary files (never the working tree) and compares
with the file, marker labels and CRs ignored; a file equal to a stage is
not edited.

**Close guard:** in merge mode the frontend calls `GuardClose(true)` once it
listens for its `mergediff:close-requested:<name>` event. The window's close button
then cancels the close and emits that event, and the frontend runs the
Cancel flow (confirms if the result was edited). If a request is not
acknowledged within 2 s, the next close goes through, so the window can never
get stuck behind a broken frontend.

`Session` lives in `internal/session`. In diff mode `Local` is LEFT, `Remote`
is RIGHT and `Base` is empty; chunks are `unchanged` or `remote`. Closing the
window without `Quit` exits `0` for a diff or directory diff and `1`
(cancelled) for a merge.

**Directory diff** (`session.StartDirDiff`): before the window opens only the
roots are checked (they exist and are directories; made absolute). The
comparison (`dirdiff.Compare`) then runs in the background, followed by the
line counts (`dirdiff.StatsAll`), so the window opens at once. A file opened
from the list is a diff-mode child session (`Session.EntrySession` →
`LoadPair`): a missing side is empty text titled `(missing)`; errors, symlinks
to directories and files over 5 MB are refused, binary and invalid UTF-8 fail
with textio's errors.

**`internal/dirdiff`** (no Wails imports): walks both trees (`filepath.WalkDir`,
both at once), pairs files by relative path with `/` separators, sorted by
path. Status: Added (right only), Deleted (left only), Modified, Unchanged
(equal bytes), Error (unreadable, broken symlink; the message is in `Note`).
Symlinks to files are followed (Git's `--dir-diff` symlinks the working-tree
side); a symlink to a directory is one entry compared by its target and never
entered (`Note` says so). `.git` directories and files are skipped, empty
directories ignored; a file on one side and a directory on the other lists the
file as Added/Deleted and the directory's files normally. Equal sizes are
compared in 64 KB chunks with buffers reused per worker (a worker pool);
other files are opened once so unreadable ones show as Error. `Stats`:
`+N −N` with `internal/diff`, or a label: `binary` / `binary, differs`,
`not UTF-8`, `line endings` (only CRLF/LF differ), the size change over 5 MB.
`Progress` counts files found and entries settled for the "Comparing…"
message. Budget: 20,000 files with 500 changed in under 1 s with a warm cache
(`TestCompareLargeTreeIsFast`; ~220 ms today).

**Scratch mode** (no arguments): two editable panes, "Left" and "Right",
empty. **⟳ Refresh** (also `F5`, `Ctrl/Cmd+Enter`) sends both texts to
`Compare` and shows the diff like diff mode. Any edit clears the highlights
and the status bar says the result is out of date until the next refresh.
Pasted CRLF becomes LF. `Esc` does not close (it would lose pasted text);
closing exits `0` without asking.

## UI specification

### Design reference

`docs/mockups/mergediff-mockup-light.png` and `docs/mockups/mergediff-mockup-dark.png` are a **visual reference** for the 3-way merge window, not a pixel-exact implementation target: follow their overall layout and colour scheme, but the text in this file is authoritative wherever they differ. The mockups show:

- **Title bar:** `mergediff — <file name>`.
- **Toolbar**, left to right: "✦ Apply non-conflicting changes" with a `▾` dropdown in the same control, `↑` / `↓` previous / next change, previous / next conflict buttons (**required**, though not drawn in the mockup), a red "N conflict(s) left" pill; on the far right, `Cancel` (outlined) and `Save` (filled, primary).
- **Pane headers:** `Local (yours) · <label>`, `Result · editable`, `Remote (theirs) · <label>`. The branch names in the mockup are illustrative: Git's mergetool does not pass branch names, so `<label>` is the file name. A merge window opened from `mergediff resolve` knows them and shows the branch names.
- **Gutter strips** between the panes hold the chunk buttons, aligned with the chunk's first line: `× »` on the Local→Result strip, `« ×` on the Result→Remote strip. The same strips host the SVG connectors (Phase 5).
- **Highlighting:** whole-line background in the chunk colour (blue = modified, red = conflict) in all three panes, with character-level changes in a stronger shade of the same colour. Line numbers in every pane.
- **Status bar:** left `N changes · N non-conflicting · N conflict(s)`; right `<Language> · <LF|CRLF> · UTF-8 · mergediff <version>`.
- **Themes:** light (warm off-white chrome, white editors) and dark (near-black chrome and editors), selected from the system theme unless the `theme` setting overrides it.

### Layout and behaviour

Layout (merge mode): **Local (read-only) | Result (editable) | Remote (read-only)**, with titles showing file names. Diff mode: two read-only panes. Scratch mode: two editable panes (see "Bound API").

Colors (light and dark variants, follow the system theme or the `theme` setting):

- added: green, modified: blue, deleted: gray, conflict: red
- character-level changes: a stronger shade of the chunk color
- resolved chunks: faded

Gutter actions on each non-unchanged chunk:

- Local pane: `>>` apply local change to result, `×` ignore
- Remote pane: `<<` apply remote change to result, `×` ignore
- For conflicts, applying one side then the other inserts both (local first)
- Once a chunk is applied or ignored on both sides, it is resolved and its buttons disappear

Toolbar:

- **"Apply non-conflicting changes"** button (merge mode only), shown prominently at the left of the toolbar:
  - Main click applies every unresolved `LocalOnly`, `RemoteOnly` and `BothSame` chunk to the result in one step.
  - A small dropdown arrow beside it offers "From Local only" and "From Remote only", which apply just that side's non-conflicting chunks.
  - Conflicts are never touched. Chunks the user already applied or ignored are skipped.
  - The whole action is a **single undo step** (one CodeMirror transaction), so `Ctrl/Cmd+Z` reverts it at once.
  - Applied chunks become resolved: highlights fade and their gutter buttons disappear.
  - The button is disabled when no unresolved non-conflicting chunks remain, and its tooltip shows how many it would apply (e.g. "Apply 12 non-conflicting changes").
  - Applying chunks must use their **current** mapped positions in the result, so it works correctly after the user has edited the result.
- Previous / next change, previous / next conflict
- Counter: "N conflicts left"
- Save, Cancel

Keyboard shortcuts:

- `Ctrl/Cmd+S` save
- `F7` / `Shift+F7` next / previous change
- `Ctrl/Cmd+Shift+A` apply all non-conflicting changes
- `Esc` cancel (with confirmation if there are unsaved edits); not in scratch mode
- `F5` / `Ctrl/Cmd+Enter` refresh (scratch mode)

Behavior:

- **Chunk tracking:** store chunk positions in the Result editor in a CodeMirror `StateField` and map them through every transaction (`ChangeSet.mapPos` / `RangeSet.map`) so they stay correct while the user types.
- **Scroll sync:** chunk-aligned. Inside an unchanged region scroll 1:1; inside a changed region scroll proportionally to each side's chunk height, so matching chunks stay aligned.
- **Connectors:** an SVG overlay between panes draws a filled curve from each chunk on the side to its position in the Result. Redraw on scroll, resize and edit, throttled with `requestAnimationFrame`.
- Saving with unresolved conflicts asks for confirmation. Closing with unsaved changes asks for confirmation.
- Syntax highlighting chosen by file extension; plain text if unknown.
- Must stay responsive for files up to ~10,000 lines.

### Directory diff window (`dirview.ts`)

`docs/mockups/mergediff-dir-diff-*.png` (flat) and
`mergediff-dir-diff-grouped-*.png` are the visual reference; colours follow
the existing theme tokens (deleted is gray, as everywhere), not the mockups.

- Title `mergediff — Compare directories`. Path bar: `Left <path> ↔ Right
  <path>`, monospace, each shortened in the middle with `…` when too long,
  full path in a tooltip.
- Filter bar: text filter (case-insensitive substring of the path), chips with
  counts (`All changes`, `Modified`, `Added`, `Deleted`; one active; counts
  follow the text filter), `Show unchanged (N)` (hidden when N is 0; with it,
  "All changes" includes the unchanged files), `Group by directory`. The
  filter state lasts for the window only (no setting).
- Columns: status badge (`M` / `A` / `D`, `!` for Error), File (muted
  directory part truncated first with `…`, bold file name never truncated
  unless alone too wide, full path in a tooltip), Lines (`+N −N` in the added
  / deleted colours, or a muted label; `…` until counted), Status tag.
- Group by directory: flat by parent directory, `(root)` first, others sorted
  by path; header rows with `▾`/`▸`, a folder icon, the path and per-status
  tags; file rows show only the name, indented; groups emptied by the filter
  disappear.
- Status bar: left, the keyboard hint (or why a file cannot be opened);
  right, `N changed · N unchanged [· N errors] · mergediff <version>`, counts
  with thousands separators.
- While the comparison runs (after 150 ms, so a fast one never flashes it):
  `Comparing… N files found, N compared`, polled every 200 ms.
- Opening: double-click (two presses on the same row within 500 ms; the
  browser's dblclick is lost when a redraw replaces the clicked cell) or
  `Enter`; a file that cannot be opened says why in the status bar.
- Keys: `↑`/`↓`, `Home`/`End`, `Enter`, `←`/`→` (fold / unfold; `←` on a file
  goes to its header), `F7` / `Shift+F7` (next / previous file that can be
  opened, and open it), `Ctrl/Cmd+F` (filter), `Esc` (clear the filter, or
  close the window when it is empty).
- The list (`filelist.ts`) renders only the rows in view (fixed 32 px rows on
  a canvas as tall as the list) and keeps row elements in place while the
  selection changes: replacing the element under the mouse would stop the
  click from focusing the list.

### Conflicts window (`resolveview.ts`)

`docs/mockups/mergediff-dir-merge{,-grouped}-*.png` are the visual
reference; 02-resolve-conflicts.md has the full behaviour.

- Title `mergediff — Resolve conflicts · <repo>`. Toolbar: the operation
  sentence (names in bold) and the pill `N of M files unresolved`, green
  `All conflicts resolved`.
- Filter bar (`filterbar.ts`, shared with dirview): text, chips All /
  Unresolved / Resolved, Group by directory (headers `N files · N
  unresolved` / `all resolved`).
- Columns File / Yours / Theirs (branch label under the heading) / State;
  multi-select (`Ctrl`/`Shift`+click); resolved rows faded, rows open in a
  merge window italic and locked.
- Side panel: Merge… / Accept Yours / Accept Theirs (modify/delete with one
  row: "Keep modified (yours)", "Delete (theirs)", "Keep added (…)"), the
  selected path, a description (with conflict counts), the "Edited since
  Git stopped" warning, Show changes (modify/delete text), Close.
- Accepting several files, or a file edited since Git stopped, asks first.
  Saving a merge of an edited file asks too.
- All resolved: a banner with the next command and Copy (Wails clipboard).
- Close, `Esc` (empty filter) and the window's close button (close guard)
  ask first when a merge window has unsaved edits (`SetDirty`), then close
  all children.
- Keys: `Enter` / double-click merge, `Ctrl/Cmd+Shift+Y` / `+T` accept,
  `F5` refresh, `Ctrl/Cmd+F` filter, `Esc`.

## Phases

Each phase ends with its acceptance criteria met, tests green, and a summary.

### Phase 1 — Merge engine (Go only, no GUI)

- `internal/textio`, `internal/diff`, `internal/merge` with table-driven tests.
- At least 15 test cases in `testdata/`: no changes, one-side edits, identical edits on both sides, overlapping conflicts, adjacent edits, insert at start/end, delete everything, empty base, CRLF input, no trailing newline.
- A comparison test that runs `git merge-file -p --diff3` on each case and checks the conflict regions agree with our chunk classification (skip the test if `git` is not installed).
- **Accept:** `go test ./...` and `go vet ./...` pass.

### Phase 2 — Wails skeleton, CLI and 2-way diff

- Wails v3 project in the root layout described in "Wails v3 setup", with the vanilla-ts template's frontend setup; CLI parsing and exit codes as specified.
- 2-way read-only diff view with line highlighting.
- Basic Makefile (`help`, `deps`, `dev`, `build`, `test`, `vet`, `typecheck`, `check`, `version`, `clean`) and `version.go` with ldflags injection, so `--version` works from the start.
- **Accept:** `mergediff a.txt b.txt` opens a window showing highlighted differences; bad arguments print usage and exit `2`; `make dev` and `make build` both work; `go test ./...` and `go vet ./...` pass from the repo root, including on a fresh clone before the frontend is built.

### Phase 3 — 3-pane merge view

- Three panes, chunk highlighting, gutter accept/ignore, chunk tracking through edits, conflict counter, save and cancel with correct exit codes, line ending preservation.
- "Apply non-conflicting changes" button with its Local-only / Remote-only dropdown, and the `--auto-apply` flag.
- **Accept:** works end-to-end as a Git mergetool on a real conflicting merge; the result file is correct and Git sees the right exit code. Clicking "Apply non-conflicting changes" produces the same text as the engine's helper, leaves conflicts as base text, and is fully reverted by a single undo — including after the user has edited the result first.

### Phase 4 — Navigation and precision

- Chunk-aligned scroll sync, character-level highlights, previous/next navigation, keyboard shortcuts.
- Character spans are widened to whole words when they cut through a word (e.g. `[sampl]e` → `[sample]`, `[aft]er` → `[after]`), in `internal/diff`, with tests.
- **Accept:** scrolling keeps matching chunks aligned on a file with chunks of very different heights; all shortcuts work.

### Phase 5 — Polish

- SVG connectors, light/dark themes, syntax highlighting by extension, confirmation dialogs, performance check on a 10,000-line file.
- **Accept:** no visible lag when scrolling or editing the large file.

### Phase 6 — Packaging and docs

- `version.go` and ldflags injection as described in "Versioning and releases".
- Makefile with all required targets, and the release workflow as specified.
- README with installation, usage, Git configuration, the release process (`make release`), and a screenshots placeholder (the mockups can stand in until real screenshots exist). License: MIT (`LICENSE` at the repo root); mention it in the README.
- **Accept:** `make build` produces a binary whose `--version` prints `YYYY-MM-DD (short-commit)`; `make release-dry-run` passes on a clean `main` and fails with clear messages on a dirty tree, another branch, or an existing tag; the workflow file passes `actionlint` (if available) and its tag filter matches `v2026-11-19` but not `v1.0.0`.

Note: the Makefile's `build`, `test`, `check` and `version` targets are useful earlier, so create a basic Makefile in Phase 2 and extend it here.

### Feature 1 — Directory diff (01-directory-diff.md, done)

1. Per-window sessions (`Windows`, child windows), no visible change.
2. `internal/dirdiff` and the CLI (`mergediff LEFTDIR RIGHTDIR`).
3. Directory window (flat list), opening files in child windows, background
   comparison with progress, line counts after the list.
4. Filter, chips, Show unchanged, Group by directory, keys, virtual
   scrolling, the 20,000-file budget.
5. e2e (`scripts/e2e/dirdiff.sh`, including `git difftool -d`; the
   20,000-file timing in `perf.sh`), README, GIT.md, this file.

### Feature 2 — Resolve conflicts (02-resolve-conflicts.md, done)

1. `internal/gitconflict`: repository, unmerged list, kinds and states,
   stages, accept, operation and labels, the rebase swap.
2. `mergediff resolve`, the resolve session and the Conflicts window (list,
   filter, grouping, multi-select, Accept with confirmation), `make
   dev-resolve`, `scripts/e2e/resolve.sh`.
3. Merge… and Show changes in child windows, locks, the unsaved-edits
   guard, "edited since Git stopped", the all-resolved banner.
4. Full e2e, README / SCREENSHOT.md, GIT.md, this file.

### Refactor 1 — Command line (03-refactor.md, done)

1. `scripts/e2e/cli.sh` pinning the command line before the change;
   `internal/cliargs` (tokenizer, immutable `Option`).
2. `internal/command`: `Command`, `Env`, `UsageError`, exit codes; Help,
   Version, Settings, Diff (files or directories), Scratch, Merge, Resolve;
   `Select`. `internal/testrepo`.
3. `main.go` runs through cliargs and command; `cli.go` and
   `settings_cmd.go` removed; `help` / `version` subcommands and reserved
   words in cli.sh, README, the usage text and this file. Next: moving the
   GUI (`app.go`, `windows.go`) to `internal/gui`.

## Useful commands

```
make dev                 # run with hot reload
make dev-resolve         # the Conflicts window on a fresh conflicted sample repository
make build               # production binary in bin/
make check               # vet + gofmt + type-check + Go and frontend tests
make test                # clean, check (uncached), then e2e — the full run
make version             # show the version string that would be embedded
make release-dry-run     # verify everything is ready for today's release
make release             # tag vYYYY-MM-DD and push → triggers the release workflow
```

## Out of scope (for now)

Binary/image diff, ignore-whitespace options, encodings other than UTF-8, editing the side panes (except scratch mode), VCS features beyond mergetool/difftool (exception: `mergediff resolve` reads Git's conflicts and runs only `checkout --ours/--theirs`, `add` and `rm`; it never commits, continues, aborts or resolves submodules). In the directory diff: editing files, copying between sides, ignore patterns, a nested tree view, three-way directory merge.
