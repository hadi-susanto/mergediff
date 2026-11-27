# Refactor 2 — GUI (`internal/gui`) and typed sessions

Instructions for Claude Code. Read this file together with `CLAUDE.md`; everything in `CLAUDE.md` (working rules, error handling, tests, versioning) still applies. Where this file and `CLAUDE.md` disagree, this file wins, and you update `CLAUDE.md` to match as part of the work (see Phase 3).

## Goal

After Refactor 1 (`03-refactor.md`) the root package still holds the GUI:

- `app.go` — one window's state (session, exit code, close guard, dirty flag) plus about twelve methods that only forward to the session;
- `windows.go` — the window registry and every bound method, each of which forwards to an `App` method (a call goes `Windows` → `App` → `session`);
- `runGUI`, `windowOptions` and the event names in `main.go`.

And `internal/session.Session` is one struct for five modes: fields only one mode uses (`dir`, `res`, `saveTo`, `output`), methods that check `Mode` at run time (`Save`, `DirEntries`, `Conflicts`, …), fields whose meaning changes per mode (`Local` / `Remote` are LEFT / RIGHT in a diff), and `App` branches on `Mode` for the exit code.

This refactor:

1. Moves the GUI to **`internal/gui`**, the only internal package that imports Wails. `main` keeps the decision to exit (crash hooks). The bound methods call the session directly; `App` becomes the unexported per-window state `window`.
2. Splits the session into **typed sessions** behind a small `session.Session` interface, with the frontend's data in a `session.View` whose shared fields use general names.

`internal/session` stays where it is: it does not import Wails, and `internal/command` loads sessions before any window opens.

## Before you start

1. **Use the Task tool to track progress.** At the start of each phase below, break it into tasks; mark each task in progress when you start it and completed when it is done and verified. Never work on something that is not tracked as a task.
2. **Branch:** work only on `refactor/gui`, created from an up-to-date `main`:
   ```
   git checkout main && git pull --ff-only
   git checkout -b refactor/gui
   ```
   Never commit to `main` directly, and do not merge the branch yourself (see "Finishing").
3. **Commits:** one small commit per step in "Phases". Intermediate commits may fail to build or test; the **last commit of each phase** must be `gofmt`-clean with `make check` and `make e2e` green.
4. No new dependencies. Never edit `frontend/bindings/` by hand: regenerate it (`wails3 generate bindings`, as `make build` does) and commit the result.
5. Work one phase at a time. At the end of each phase, stop and give a short summary.

## Behaviour changes

None visible to users: windows, exit codes, the close guard, child windows and the frontend behave exactly as before.

Internal only: a bound call from the wrong kind of window (e.g. `DirEntries` from a merge window, `Save` from a diff window) returns an error wrapping `gui.errWrongMode` instead of the session's run-time errors (`session.ErrNotMerge`, "not a resolve session"). The UI never makes such calls.

## Design

### `internal/gui`

| File | Contents |
|---|---|
| `run.go` | `Options`; `Run(sess session.Session, cfg config.Config, opts Options) int` — the Wails application, its panic / error handlers (calling the hooks), the window factory and close hooks; returns the exit code |
| `options.go` | `windowOptions`: title, size, theme in the URL, maximized, native theme and background colour |
| `registry.go` | the `Windows` registry: `openMain`, `openChild`, `focusChild`, `closed`, `children`, `handle`, `byName`, `caller` |
| `api.go` | every exported (bound) method of `Windows`: the frontend's contract in one place |
| `window.go` | unexported `window`: session, settings, exit code, close guard, dirty flag, quit |
| `events.go` | `mainWindowName`, `closeRequestedEvent`, `conflictsChangedEvent` |

```go
type Options struct {
	Assets  fs.FS  // main embeds frontend/dist
	Version string // for the status bar
	// Called for a panic in a bound method and for a fatal Wails error;
	// main reports the crash and exits 2. Internal packages never exit.
	OnPanic func(value any, stack []byte)
	OnFatal func(err error)
}
```

`main.go` wires it:

```go
OpenWindow: func(s session.Session, c config.Config) int { return gui.Run(s, c, guiOptions) },
```

with `guiOptions` holding the embedded assets, `Version()` and hooks that call `reportCrash` and `os.Exit(command.ExitError)`. `crash.go`, `version.go`, `devargs*.go` and `console_windows.go` stay in the root package.

- The bound type stays **`Windows`**. Its Go path changes, so the bindings move from `frontend/bindings/github.com/hadi-susanto/mergediff/windows.ts` to `…/mergediff/internal/gui/windows.ts`. Bound calls are keyed by the Go path, so the regenerated bindings and the `main.ts` import change go **in the same commit** as the move that changes the path.
- **No double forwarding:** bound methods call the session (or `diff` / `session.Compare` for the stateless ones); `var stateless App` goes away. `window` keeps only state (`Quit`, `SetDirty`, `GuardClose`, `CloseRequestSeen`, `interceptClose`, the exit code).
- The registry stays in `gui` (it is mostly plain Go and tested with fake window handles; no separate Wails-free package).
- Event names are still written by hand on the TypeScript side; the e2e scripts catch a mismatch.

### Typed sessions (`internal/session`)

```go
// View is what the frontend gets from LoadSession. Only the parts a mode
// uses are set; the others are null in JSON.
type View struct {
	Mode  Mode       `json:"mode"`
	Title string     `json:"title"`
	Paths []string   `json:"paths"` // merge: BASE LOCAL REMOTE MERGED; diff, dirdiff: LEFT RIGHT; resolve: top level
	Text  *Texts     `json:"text"`  // diff, scratch, merge: the two side panes
	Merge *MergeView `json:"merge"` // merge only
}

// Texts are the two side panes: LEFT / RIGHT in a diff, LOCAL (yours) /
// REMOTE (theirs) in a merge.
type Texts struct {
	Left       string        `json:"left"`
	Right      string        `json:"right"`
	LeftLabel  string        `json:"leftLabel"`  // pane headers when not file names (branches, "Base")
	RightLabel string        `json:"rightLabel"`
	Chunks     []merge.Chunk `json:"chunks"`     // diff: Unchanged / RemoteOnly
	Language   string        `json:"language"`
	EOL        string        `json:"eol"`
}

// MergeView is what only a merge has.
type MergeView struct {
	Base          string `json:"base"`
	InitialResult string `json:"initialResult"`
	AutoApplied   bool   `json:"autoApplied"`
	EditedOnDisk  bool   `json:"editedOnDisk"`
}

type Session interface {
	View() View
	// ExitCode is the process exit code when the main window showing the
	// session closes; code is what Quit set, quit whether it was called.
	ExitCode(code int, quit bool) int
}

type DiffSession struct{ /* view */ }    // files; a directory diff's entry; Show changes
type ScratchSession struct{ /* view */ }
type MergeSession struct{ /* view, eol, save */ }   // Save
type DirDiffSession struct{ /* view, dir */ }       // DirEntries, DirStats, DirProgress, Entry(path) (*DiffSession, error)
type ResolveSession struct{ /* view, res */ }       // Conflicts, Accept, Remaining, MergeCounts, Edited,
                                                    // Merge(path) (*MergeSession, error), Changes(path) (*DiffSession, error)
```

- **General names:** `Local` / `Remote` (LEFT / RIGHT in a diff) become `Text.Left` / `Text.Right`, `LocalLabel` / `RemoteLabel` become `LeftLabel` / `RightLabel`; merge-only data moves to `View.Merge`; directory diff and resolve windows get neither part. The data stays in `View`, not on the `Session` interface: the frontend reads JSON, and interface methods are not serialised. `merge.Chunk` keeps its Base / Local / Remote ranges (merge engine, out of scope).
- **Type names** carry the `Session` suffix because the `Mode` constants (`Merge`, `Diff`, `Scratch`, `DirDiff`, `Resolve`) keep their names: the generated TypeScript enum uses them (`Mode.DirDiff` in `main.ts`).
- **`ExitCode`** replaces both `Mode` checks in `App`: a merge exits `1` unless Quit was called; a resolve session keeps Quit's `2`, otherwise asks Git (`Remaining`: `0` none left, `1` some left, `2` on error); the others return `code` (default `0`).
- **Constructors return concrete types:** `LoadDiff` / `LoadPair` → `*DiffSession`, `LoadMerge` → `*MergeSession`, `NewScratch` → `*ScratchSession`, `StartDirDiff` → `*DirDiffSession`, `StartResolve` → `*ResolveSession`. `command.Env.OpenWindow` takes a `session.Session`.
- **Bound methods** pick the type they need, e.g.:
  ```go
  d, ok := win.sess.(*session.DirDiffSession)
  if !ok {
  	return nil, fmt.Errorf("DirEntries: %w", errWrongMode)
  }
  ```
  `LoadSession` returns `session.View`.
- **Frontend:** only `main.ts` changes: the type `Session` becomes `View`, and its reads become `session.text.left`, `session.merge.base`, … (about 25 reads).
- `TestDirEntriesOtherModes` and `TestResolveMethodsOtherModes` become compile-time guarantees and are removed; `gui` gets `TestWrongModeCalls`.

## Phases

Each numbered step is one commit.

### Phase 1 — Move the GUI to `internal/gui` (no change)

Before any code: build `main` and run `make e2e` as the baseline; record the results (some checks depend on the machine, e.g. `settings.sh`'s dark-frame check).

1. **`gui: move App and Windows to internal/gui`** — `git mv` `app.go`, `windows.go` and `windows_test.go` (history kept); the App, close-guard and window-options tests from `main_test.go`, and the `Windows` tests and `TestResolveExitCode` from `resolve_test.go`, move too. Package `gui`; the version comes from a field. Does not build yet (`main` still refers to them).
2. **`gui: Run, Options and events`** — `runGUI` → `gui.Run` (`run.go`), `windowOptions` → `options.go`, the event and window names → `events.go`; `main` wires `gui.Options` with the crash hooks; bindings regenerated and the `main.ts` import updated. Builds.
3. **`gui: split windows.go into registry.go and api.go`** — a pure move.
4. **`gui: bound methods call the session; App becomes window`** — the forwarding methods and `var stateless App` removed; `App` renamed to the unexported `window`; `window.go`.

**Accept:** `make check` and `make e2e` pass with the baseline's results; the root package holds only `main.go`, `version.go`, `crash.go`, `devargs*.go`, `console_windows.go` and their tests.

### Phase 2 — Typed sessions

5. **`session: View with Text and Merge parts`** — the JSON struct renamed `View` and restructured (`Text`, `Merge`, general names); Go callers, regenerated bindings and `main.ts` updated. Builds.
6. **`session: Session interface; DiffSession and ScratchSession`** — with their constructors; tests ported from `session_test.go`. Callers do not build from here until step 10.
7. **`session: MergeSession`** — `Save` and the resolve save hook.
8. **`session: DirDiffSession`** — `DirEntries`, `DirStats`, `DirProgress`, `Entry`.
9. **`session: ResolveSession`** — `Conflicts`, `Accept`, `Remaining`, `MergeCounts`, `Edited`, `Merge`, `Changes`; tests ported from `resolve_test.go`.
10. **`command: open typed sessions`** — `Env.OpenWindow(session.Session, config.Config)`; command tests updated.
11. **`gui: typed sessions and ExitCode`** — `window` holds a `session.Session`; `ExitCode` replaces the mode checks; bound methods type-switch with `errWrongMode`; `TestWrongModeCalls` and per-session exit-code tests.

**Accept:** `make check` and `make e2e` pass with the baseline's results; no `Mode` comparison is left in Go outside `internal/session`'s constructors.

### Phase 3 — Docs

12. **`docs: CLAUDE.md for internal/gui and typed sessions`**:
    - Repository layout: root files; `internal/gui` and its files; `app.go` / `windows.go` removed; this file.
    - Rules for the root package and "`internal/` packages must not import Wails — except `internal/gui`, the only one that does".
    - Bound API section: lives in `internal/gui`; `LoadSession` returns `View`; the typed sessions and `ExitCode`; `errWrongMode`; the `Session` struct listing replaced by `View`, `Texts`, `MergeView`.
    - Panics: `Options.OnPanic` / `OnFatal`, main reports and exits.
    - Phases: a "Refactor 2 — GUI and typed sessions" entry; the "Next" note in Refactor 1 removed.

**Accept:** `make check` and `make build && make e2e` pass. Manual check: a file diff, a merge (`make dev`), `mergediff resolve` in `make dev-resolve`'s repository with a merge window, and a directory diff opening a file.

## Finishing

1. Make sure the branch is up to date with `main` (`git fetch && git rebase origin/main`; the history uses PR merges), and `make check` passes.
2. Push the branch and open a pull request into `main` (`gh pr create` if `gh` is available; otherwise print the compare URL). Use the summary of all phases as the description.
3. **Stop.** Do not merge the PR yourself; the maintainer reviews and merges it.

## Out of scope

Changes to the frontend beyond the bindings import and the `View` reads, new bound methods, renaming the bound type `Windows`, the merge engine's `Chunk` ranges, sharing event names between Go and TypeScript, moving `internal/session`.
