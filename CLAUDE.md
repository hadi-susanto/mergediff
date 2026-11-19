# mergediff — Project Instructions for Claude Code

## Goal

Build **mergediff**: a lightweight, cross-platform, open-source **3-way merge** and **2-way diff** GUI tool, inspired by the JetBrains merge tool. It must ship as a small native binary for Windows, macOS and Linux, and work as a Git `mergetool` / `difftool`.

## Working rules

- **Always use the Task tool to track progress.** At the start of each phase, break it into tasks; mark each task in progress when you start it and completed when it is done and verified. Never work on something that is not tracked as a task.
- Work **one phase at a time** (see "Phases"). Do not start the next phase until the current phase's acceptance criteria are met and all tests pass. At the end of each phase, stop and give a short summary of what was done and anything left open.
- Ask before adding any dependency not listed under "Stack".
- Before using a third-party library, check its current API (docs or source) rather than guessing.
- Never edit generated files (`frontend/wailsjs/`, `build/` output).
- Keep code formatted (`gofmt`, Prettier defaults) and free of vet / type errors.
- Make small, focused commits, one per step/completed task, with clear messages. Intermediate commits may have a broken build or failing tests; the **last commit of a phase** must be `gofmt`-clean with all tests green.

## Prerequisites

- Go (latest stable), Node.js LTS + npm
- Wails v2 CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`, then `wails doctor`
- Linux: `webkit2gtk` dev packages (see `wails doctor` output)
- `git` (used only by comparison tests)

## Stack

- **Backend:** Go + Wails v2
- **Frontend:** Vite + TypeScript, **no UI framework** (vanilla TS)
- **Editor:** CodeMirror 6 (`@codemirror/state`, `@codemirror/view`, `@codemirror/language`, `@codemirror/commands`, plus language packages for syntax highlighting). `@codemirror/merge` may be used for the 2-way view or as reference, but the 3-pane merge view is custom.
- **Line and character diff:** `github.com/sergi/go-diff` (diffmatchpatch). Line diff uses its line mode (`DiffLinesToRunes` + `DiffMainRunes`) with `DiffTimeout = 0` so results are exact (a timeout also enables a non-optimal "half-match" shortcut). `github.com/aymanbagabas/go-udiff` was evaluated and rejected: its LCS search is capped at ~50 edits, after which the rest of the file collapses into one huge hunk.

## Repository layout

```
mergediff/
  go.mod                  # module github.com/hadi-susanto/mergediff
  cmd/
    mergediff/            # Binary entry point and all wiring — no business logic
      main.go             # Calls cli.Parse, then starts Wails with options
      cli.go              # Argument parsing, usage text, exit codes
      version.go          # versionDate / commit vars set via -ldflags, Version() string
      app.go              # Wails-bound App struct: thin adapters to internal packages
      wails.json          # Wails project config (see "Wails with cmd/ layout")
      build/              # Wails build assets: icons, Info.plist, manifests
  internal/
    session/              # Builds a Session from CLI paths (load, detect, merge)
    textio/               # Reading/writing files, line endings, binary detection
    diff/                 # Line diff wrapper + character-level diff
    merge/                # diff3 merge engine
      testdata/           # Merge cases: <name>/base, local, remote, expected
  frontend/
    embed.go              # package frontend: //go:embed all:dist → var Assets embed.FS
    wailsjs/              # Generated Wails bindings (do not edit)
    src/
      main.ts             # Bootstraps the session, builds layout
      panes.ts            # Creates the CodeMirror editors
      chunks.ts           # Chunk state, tracking ranges through edits
      highlight.ts        # Line + character decorations
      gutter.ts           # Accept / ignore buttons
      scrollsync.ts       # Chunk-aligned scrolling
      connectors.ts       # SVG curves between panes
      toolbar.ts          # Toolbar, navigation, conflict counter, shortcuts
      styles.css
  .github/workflows/release.yml   # Triggered by pushing a vYYYY-MM-DD tag
  docs/mockups/           # UI design reference (see "Design reference")
    mergediff-mockup-light.png
    mergediff-mockup-dark.png
  .gitignore              # Must cover all build/dev output (see "Makefile")
  Makefile                # Dev, build, test and release automation
  LICENSE                 # MIT
  README.md
```

### Wails with the `cmd/` layout

Wails v2 normally expects `main.go` and `wails.json` at the project root. This project keeps them in `cmd/mergediff/` instead:

- `cmd/mergediff/wails.json` sets `"frontend:dir": "../../frontend"`, `"wailsjsdir": "../../frontend"` and `"outputfilename": "mergediff"`.
- Go's `embed` cannot reach parent directories, so the built assets are embedded by the `frontend` package itself (`frontend/embed.go`), and `cmd/mergediff/main.go` imports `frontend.Assets`.
- Build and run from `cmd/mergediff/` (`wails dev`, `wails build`).
- Before relying on this, **verify these `wails.json` fields against the current Wails v2 docs**. If any part doesn't work, stop and explain the problem and options rather than moving `main.go` back to the root on your own.

Rules for `cmd/mergediff/`:

- Only startup and wiring: parse args, build dependencies, configure and run Wails, map errors to exit codes.
- `App` methods are thin adapters that call `internal/...` packages; no diff/merge logic lives in `cmd/`.
- `internal/` packages must not import Wails, so they stay testable with plain `go test`.

## Command-line interface

```
mergediff BASE LOCAL REMOTE -o MERGED    # 3-way merge mode
mergediff BASE LOCAL REMOTE -o MERGED --auto-apply
                                       # same, but non-conflicting changes are applied at startup
mergediff LEFT RIGHT                     # 2-way diff mode (read-only)
mergediff --help | --version            # --version prints e.g. "mergediff 2026-11-19 (a1b2c3d)"
```

Parse arguments in `cmd/mergediff/cli.go`, called from `main()` **before** starting Wails, so bad arguments print usage and exit without opening a window.

Exit codes:

- `0` — merge saved and all conflicts resolved (or diff window closed normally)
- `1` — user cancelled, or closed with unresolved conflicts / unsaved changes
- `2` — usage error or I/O error (message on stderr)

Git configuration to document in the README:

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
// cmd/mergediff/version.go
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

Because untracked files count as dirty, `.gitignore` must cover everything that dev and build steps produce (e.g. `cmd/mergediff/build/bin/`, `frontend/dist/`, `frontend/node_modules/`, coverage/test output), so a normal build never marks the tree dirty.

Required targets (each with a one-line `## description` shown by `make help`):

- `help` (default) — list targets
- `deps` — `go mod download`, `npm ci` in `frontend/`, `wails doctor`
- `dev` — `wails dev` in `cmd/mergediff/`
- `build` — `wails build -clean -ldflags "$(LDFLAGS)"` in `cmd/mergediff/`
- `test` — `go test ./...`
- `vet` — `go vet ./...` and `gofmt -l` (fail if any file listed)
- `typecheck` — `npx tsc --noEmit` in `frontend/`
- `check` — `vet`, `typecheck`, `test`
- `version` — print the version string the build would embed
- `clean` — remove build output and `frontend/dist`
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
- **test** job (ubuntu-latest): Go tests, vet, gofmt check, frontend type-check.
- **build** job, needs `test`, matrix:
  - `windows-latest` → `windows/amd64`
  - `macos-latest` → `darwin/universal`
  - `ubuntu-latest` → `linux/amd64` (install `webkit2gtk` and GTK dev packages first)
  - Each runs `make build VERSION_DATE=$VERSION_DATE` (so builds use the same Makefile logic as local), then packages the binary as `mergediff_<YYYY-MM-DD>_<os>_<arch>.zip` (`.tar.gz` on Linux) and uploads it as an artifact.
- **release** job, needs `build`: downloads all artifacts, generates `SHA256SUMS.txt`, and creates a GitHub Release named `mergediff YYYY-MM-DD (a1b2c3d)` with tag-derived notes (commits since the previous `v*` tag) and all archives attached.
- Pin every third-party action to a specific major version (e.g. `actions/checkout@v4`); verify the current versions before use.
- Code signing / notarization is out of scope for now; mention in the README that macOS users may need to allow the app in Privacy & Security.

## Text handling

- Treat input as UTF-8. If a file contains a NUL byte in its first 8 KB, treat it as binary: show an error and exit `2`. Invalid UTF-8 is also an error (exit `2`), because it cannot round-trip through the UI without being corrupted.
- Detect line endings (LF vs CRLF) from LOCAL (fall back to BASE). Normalize to LF internally; write MERGED using the detected style.
- Preserve whether the file ends with a trailing newline.
- A missing BASE (Git passes an empty/nonexistent file for add/add conflicts) is treated as empty.

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

## Bound API (`cmd/mergediff/app.go`)

```go
type Session struct {
	Mode          string   `json:"mode"` // "merge" | "diff"
	Paths         []string `json:"paths"`
	Base          string   `json:"base"`
	Local         string   `json:"local"`
	Remote        string   `json:"remote"`
	Chunks        []Chunk  `json:"chunks"`
	InitialResult string   `json:"initialResult"`
	Language      string   `json:"language"` // derived from file extension
}

func (a *App) LoadSession() (Session, error)
func (a *App) CharDiff(left, right string) []Span // Span is defined in internal/diff
func (a *App) Save(result string) error // writes MERGED with original line endings
func (a *App) Quit(code int)            // closes the window, exits with code
```

## UI specification

### Design reference

`docs/mockups/mergediff-mockup-light.png` and `docs/mockups/mergediff-mockup-dark.png` are a **visual reference** for the 3-way merge window, not a pixel-exact implementation target: follow their overall layout and colour scheme, but the text in this file is authoritative wherever they differ. The mockups show:

- **Title bar:** `mergediff — <file name>`.
- **Toolbar**, left to right: "✦ Apply non-conflicting changes" with a `▾` dropdown in the same control, `↑` / `↓` previous / next change, previous / next conflict buttons (**required**, though not drawn in the mockup), a red "N conflict(s) left" pill; on the far right, `Cancel` (outlined) and `Save` (filled, primary).
- **Pane headers:** `Local (yours) · <label>`, `Result · editable`, `Remote (theirs) · <label>`. The branch names in the mockup are illustrative: Git's mergetool does not pass branch names, so `<label>` is the file name.
- **Gutter strips** between the panes hold the chunk buttons, aligned with the chunk's first line: `× »` on the Local→Result strip, `« ×` on the Result→Remote strip. The same strips host the SVG connectors (Phase 5).
- **Highlighting:** whole-line background in the chunk colour (blue = modified, red = conflict) in all three panes, with character-level changes in a stronger shade of the same colour. Line numbers in every pane.
- **Status bar:** left `N changes · N non-conflicting · N conflict(s)`; right `<Language> · <LF|CRLF> · UTF-8 · mergediff <version>`.
- **Themes:** light (warm off-white chrome, white editors) and dark (near-black chrome and editors), selected from the system theme.

### Layout and behaviour

Layout (merge mode): **Local (read-only) | Result (editable) | Remote (read-only)**, with titles showing file names. Diff mode: two read-only panes.

Colors (light and dark variants, follow the system theme):

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
- `Esc` cancel (with confirmation if there are unsaved edits)

Behavior:

- **Chunk tracking:** store chunk positions in the Result editor in a CodeMirror `StateField` and map them through every transaction (`ChangeSet.mapPos` / `RangeSet.map`) so they stay correct while the user types.
- **Scroll sync:** chunk-aligned. Inside an unchanged region scroll 1:1; inside a changed region scroll proportionally to each side's chunk height, so matching chunks stay aligned.
- **Connectors:** an SVG overlay between panes draws a filled curve from each chunk on the side to its position in the Result. Redraw on scroll, resize and edit, throttled with `requestAnimationFrame`.
- Saving with unresolved conflicts asks for confirmation. Closing with unsaved changes asks for confirmation.
- Syntax highlighting chosen by file extension; plain text if unknown.
- Must stay responsive for files up to ~10,000 lines.

## Phases

Each phase ends with its acceptance criteria met, tests green, and a summary.

### Phase 1 — Merge engine (Go only, no GUI)

- `internal/textio`, `internal/diff`, `internal/merge` with table-driven tests.
- At least 15 test cases in `testdata/`: no changes, one-side edits, identical edits on both sides, overlapping conflicts, adjacent edits, insert at start/end, delete everything, empty base, CRLF input, no trailing newline.
- A comparison test that runs `git merge-file -p --diff3` on each case and checks the conflict regions agree with our chunk classification (skip the test if `git` is not installed).
- **Accept:** `go test ./...` and `go vet ./...` pass.

### Phase 2 — Wails skeleton, CLI and 2-way diff

- Wails v2 project with vanilla TS template, restructured into the `cmd/mergediff/` layout described above; CLI parsing and exit codes as specified.
- 2-way read-only diff view with line highlighting.
- Basic Makefile (`help`, `deps`, `dev`, `build`, `test`, `vet`, `typecheck`, `check`, `version`, `clean`) and `version.go` with ldflags injection, so `--version` works from the start.
- **Accept:** `mergediff a.txt b.txt` opens a window showing highlighted differences; bad arguments print usage and exit `2`; `wails dev` and `wails build` both work from `cmd/mergediff/`; `go test ./...` still passes from the repo root.

### Phase 3 — 3-pane merge view

- Three panes, chunk highlighting, gutter accept/ignore, chunk tracking through edits, conflict counter, save and cancel with correct exit codes, line ending preservation.
- "Apply non-conflicting changes" button with its Local-only / Remote-only dropdown, and the `--auto-apply` flag.
- **Accept:** works end-to-end as a Git mergetool on a real conflicting merge; the result file is correct and Git sees the right exit code. Clicking "Apply non-conflicting changes" produces the same text as the engine's helper, leaves conflicts as base text, and is fully reverted by a single undo — including after the user has edited the result first.

### Phase 4 — Navigation and precision

- Chunk-aligned scroll sync, character-level highlights, previous/next navigation, keyboard shortcuts.
- **Accept:** scrolling keeps matching chunks aligned on a file with chunks of very different heights; all shortcuts work.

### Phase 5 — Polish

- SVG connectors, light/dark themes, syntax highlighting by extension, confirmation dialogs, performance check on a 10,000-line file.
- **Accept:** no visible lag when scrolling or editing the large file.

### Phase 6 — Packaging and docs

- `cmd/mergediff/version.go` and ldflags injection as described in "Versioning and releases".
- Makefile with all required targets, and the release workflow as specified.
- README with installation, usage, Git configuration, the release process (`make release`), and a screenshots placeholder (the mockups can stand in until real screenshots exist). License: MIT (`LICENSE` at the repo root); mention it in the README.
- **Accept:** `make build` produces a binary whose `--version` prints `YYYY-MM-DD (short-commit)`; `make release-dry-run` passes on a clean `main` and fails with clear messages on a dirty tree, another branch, or an existing tag; the workflow file passes `actionlint` (if available) and its tag filter matches `v2026-11-19` but not `v1.0.0`.

Note: the Makefile's `build`, `test`, `check` and `version` targets are useful earlier, so create a basic Makefile in Phase 2 and extend it here.

## Useful commands

```
make dev                 # run with hot reload
make build               # production binary in cmd/mergediff/build/bin
make check               # vet + gofmt + type-check + tests
make version             # show the version string that would be embedded
make release-dry-run     # verify everything is ready for today's release
make release             # tag vYYYY-MM-DD and push → triggers the release workflow
```

## Out of scope (for now)

Directory comparison, binary/image diff, ignore-whitespace options, encodings other than UTF-8, editing the side panes, VCS features beyond mergetool/difftool.
