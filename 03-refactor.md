# Refactor 1 — Command line (`internal/cliargs`, `internal/command`)

Instructions for Claude Code. Read this file together with `CLAUDE.md`; everything in `CLAUDE.md` (working rules, error handling, tests, versioning) still applies. Where this file and `CLAUDE.md` disagree, this file wins, and you update `CLAUDE.md` to match as part of the work (see Phase 3).

## Goal

The root `package main` holds the command line (`cli.go`, `settings_cmd.go`, the dispatch in `main.go`) next to the GUI (`app.go`, `windows.go`). The command line was written when mergediff had one command; it has since grown a `switch` on argument counts plus special cases for `settings` and `resolve`. This refactor splits it into:

- **`internal/cliargs`** — a pure tokenizer. It turns the arguments into an immutable `cliargs.Option` and knows nothing about commands.
- **`internal/command`** — one type per command. `command.Select(opt)` decides which command runs; each command checks its own arguments; `Run(env)` executes it. No Wails import.
- **`main.go`** — the orchestrator: parse → select → run → exit code.

The GUI (`app.go`, `windows.go`) stays in the root package for now; moving it to `internal/gui` is a separate, later refactor.

User-visible behaviour stays the same except for the changes listed in "Behaviour changes".

## Before you start

1. **Use the Task tool to track progress.** At the start of each phase below, break it into tasks; mark each task in progress when you start it and completed when it is done and verified. Never work on something that is not tracked as a task.
2. **Branch:** work only on `refactor/cli`, created from an up-to-date `main`:
   ```
   git checkout main && git pull --ff-only
   git checkout -b refactor/cli
   ```
   Never commit to `main` directly, and do not merge the branch yourself (see "Finishing").
3. **Commits:** one small commit per step in "Phases". Intermediate commits may fail to build or test; the **last commit of each phase** must be `gofmt`-clean with `make check` green. The last commit of Phase 3 also needs `make e2e` green.
4. No new dependencies.
5. Work one phase at a time. At the end of each phase, stop and give a short summary.

## Behaviour changes (accepted)

- `help` and `version` become subcommands: `mergediff help`, `mergediff help settings`, `mergediff version`. `--help` / `-h` / `--version` / `-v` keep working.
- **Reserved words** — `help`, `version`, `settings`, `resolve` — are subcommands when they are the **first positional argument and come before `--`**. Today a subcommand must be literally the first argument; now `mergediff --theme dark settings list` is the settings command (which then rejects `--theme`).
- To use a file whose name is a reserved word, write `--` before the paths (preferred) or `./name`: `mergediff -- help other.txt`. Everything after `--` is a path, so options go before it: `mergediff -o MERGED -- help local remote`.
- A subcommand wins over `--help` / `--version`: `mergediff settings --help` prints the settings help (as today); `mergediff resolve --help` prints the general usage (as today).
- `settings` rejects `-o`, `--auto-apply` and settings flags (`settings list --theme dark` is a usage error, exit 2).
- `version` with any argument, and `help` with an unknown topic or more than one argument, are usage errors.

## Design

### `internal/cliargs`

- `Parse(args []string) (Option, error)`.
- **One flag table:** `-o` / `--output VALUE`, `--auto-apply`, `-h` / `--help`, `-v` / `--version`, and every `config.Registry` setting with `Flag: true` (`--KEY VALUE` or `--KEY=VALUE`, value checked with `Setting.Validate`). An unknown flag is an error; a new command-specific flag means a new table entry.
- Errors: unknown flag, missing or empty value, a value on a switch (`--auto-apply=yes`), a repeated `-o` or a repeated `--KEY`. Repeating `--auto-apply`, `--help` or `--version` is allowed.
- `--` ends the options: everything after it is positional. `-` alone is positional.
- Positional arguments keep their order. Flags become fields; their relative order to the positionals is not kept (no command needs it). Settings keep their given order.
- **`Option` is immutable:** unexported fields, a value type, getters; slice getters return **copies**.
  `Positionals() []string`, `DashDash() int` (index into `Positionals()` of the first argument after `--`, or `-1` without `--`), `Output() string`, `HasOutput() bool`, `AutoApply() bool`, `Help() bool`, `Version() bool`, `Settings() []Setting` (`Setting{Key, Value string}`).
- Errors are `*cliargs.UsageError`. The package imports `internal/config` (the registry) and nothing about commands.

Sketch (guidance, not final code):

```go
type Option struct {
	positionals              []string
	dashDash                 int // positionals[dashDash:] came after "--"; -1 = no "--"
	output                   string // "" = not given (Parse rejects an empty value)
	autoApply, help, version bool
	settings                 []Setting
}

func (o Option) Positionals() []string { return slices.Clone(o.positionals) }
func (o Option) Settings() []Setting   { return slices.Clone(o.settings) }
// … plain getters for the rest

type flag struct {
	takesValue bool
	set        func(o *Option, name, value string) error // checks repeats, validates
}

// lookup: -o/--output, --auto-apply, -h/--help, -v/--version, then the
// config.Registry settings with Flag: true.
func lookup(name string) (flag, bool)

func Parse(args []string) (Option, error) {
	o := Option{dashDash: -1}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			o.dashDash = len(o.positionals)
			o.positionals = append(o.positionals, args[i+1:]...)
			return o, nil
		case arg == "-" || !strings.HasPrefix(arg, "-"):
			o.positionals = append(o.positionals, arg)
			continue
		}
		name, value, inline := strings.Cut(arg, "=")
		f, ok := lookup(name)
		switch {
		case !ok:
			return Option{}, usagef("unknown option %s", arg)
		case inline && !f.takesValue:
			return Option{}, usagef("%s does not take a value", name)
		case !inline && f.takesValue:
			if i+1 >= len(args) {
				return Option{}, usagef("%s needs a value", name) // keep today's per-flag hints
			}
			i++
			value = args[i]
		}
		if err := f.set(&o, name, value); err != nil {
			return Option{}, err
		}
	}
	return o, nil
}
```

Keep today's error messages (`-o needs a file name`, `--theme needs a value (system, light, dark)`, `--theme given more than once`, …); tests and users rely on them.

Tokenizing examples:

| args | positionals | dashDash | flags | selected |
|---|---|---|---|---|
| `B L R -o M` | B L R | -1 | output=M | Merge |
| `settings --help` | settings | -1 | help | Settings help |
| `settings list --theme dark` | settings list | -1 | theme=dark | Settings → usage error |
| `--theme dark settings list` | settings list | -1 | theme=dark | Settings → usage error |
| `-- help b` | help b | 0 | — | Diff of files `help` and `b` |
| `-o M -- help L R` | help L R | 0 | output=M | Merge |
| `a b --help` | a b | -1 | help | Help |

### `internal/command`

```go
type Command interface {
	Run(env Env) (code int, err error)
}

type Env struct {
	Stdout, Stderr io.Writer
	Getwd          func() (string, error)
	ConfigPath     func() (string, error)
	Version        string
	OpenWindow     func(*session.Session, config.Config) int // main supplies runGUI
}
```

- `Env` replaces the `startGUI` and `configPath` package variables: tests pass fakes. `command` never imports Wails, so its tests run without cgo / GTK.
- Exit codes `ExitOK`, `ExitCancel`, `ExitError` move here; the root `App` uses them.
- `*command.UsageError{Msg, Usage string}`: main prints `mergediff: Msg`, a blank line and `Usage` (the general usage text when empty), exit `2`. Any other error: `mergediff: <err>` on stderr, exit `2`. Commands write their normal output (help text, settings table, "no conflicts to resolve") themselves.
- The general usage text (today's `usage` in `cli.go`) and the settings usage move here.
- `Select(opt cliargs.Option) (Command, error)` checks all arguments, so bad arguments are reported before any window opens:

```go
func Select(opt cliargs.Option) (Command, error) {
	pos := opt.Positionals()
	// A reserved word counts only if the first positional came before "--".
	if len(pos) > 0 && opt.DashDash() != 0 {
		if newSub, ok := subcommands[pos[0]]; ok { // help, version, settings, resolve
			return newSub(opt, pos[1:])
		}
	}
	switch {
	case opt.Help():
		return Help{}, nil
	case opt.Version():
		return Version{}, nil
	}
	switch len(pos) {
	case 3:
		return newMerge(opt, pos)
	case 2:
		return newDiff(opt, pos) // file or directory is decided in Run
	case 0:
		return newScratch(opt)
	}
	return nil, usagef("expected 2 files or directories (diff) or 3 files and -o (merge), got %d", len(pos))
}
```

Commands:

| Command | Accepts | Rejects (usage error) |
|---|---|---|
| Help | `help`; `help settings` (settings help); `help resolve` (general usage); `--help` | other topics, more than one argument, `-o`, `--auto-apply`, settings flags |
| Version | `version`, `--version`; `version --help` → general usage | any argument, any other flag |
| Settings | today's `list`, `get`, `set`, `unset`, `path`, `help`; `--help` → settings help | `-o`, `--auto-apply`, settings flags, `--version` |
| Resolve | `[--auto-apply] [--KEY VALUE] [--] [PATH...]`; `--help` → general usage; `--version` → version (as today) | `-o` |
| Merge | 3 paths, `-o` required, `--auto-apply`, settings flags | — |
| Diff | 2 paths, both files or both directories; settings flags | `-o`, `--auto-apply`; one directory and one file (message names which) |
| Scratch | no paths; settings flags | `-o`, `--auto-apply` |

- Diff covers files **and** directories: the choice needs `os.Stat`, both take the same arguments and exit codes, and the "one directory, one file" error belongs here.
- Merge, Diff, Scratch and Resolve share one helper: load the settings (`env.ConfigPath` plus flag overrides; today's `loadSettings`), build the session, `env.OpenWindow`.
- Resolve: `session.ErrNoConflicts` → `mergediff: no conflicts to resolve` on stdout, exit `0` (as today). The working directory comes from `env.Getwd`.

### `main.go`

Only startup and wiring. `main()`, the panic recovery, `devArgs`, `runGUI`, `windowOptions`, `version.go`, `console_windows.go` and `crash.go` stay in the root package.

```go
func main() {
	defer recoverCrash() // today's recover → reportCrash → exit 2
	os.Exit(run(devArgs(os.Args[1:]), command.Env{
		Stdout: os.Stdout, Stderr: os.Stderr,
		Getwd: os.Getwd, ConfigPath: config.Path,
		Version: Version(), OpenWindow: runGUI,
	}))
}

// run is the whole orchestration; tests call it with a fake Env.
func run(args []string, env command.Env) int {
	opt, err := cliargs.Parse(args)
	if err != nil {
		return report(env.Stderr, err)
	}
	cmd, err := command.Select(opt)
	if err != nil {
		return report(env.Stderr, err)
	}
	code, err := cmd.Run(env)
	if err != nil {
		return report(env.Stderr, err)
	}
	return code
}

// report is the only place errors become output and exit code 2.
func report(w io.Writer, err error) int {
	var cu *command.UsageError
	var pu *cliargs.UsageError
	switch {
	case errors.As(err, &cu):
		fmt.Fprintf(w, "mergediff: %s\n\n%s", cu.Msg, cmp.Or(cu.Usage, command.Usage))
	case errors.As(err, &pu):
		fmt.Fprintf(w, "mergediff: %v\n\n%s", err, command.Usage)
	default:
		fmt.Fprintf(w, "mergediff: %v\n", err)
	}
	return command.ExitError
}
```

## Phases

Each numbered step is one commit.

### Phase 1 — Baseline and tokenizer

1. **`e2e: cli.sh pins today's command-line behaviour`** — `scripts/e2e/cli.sh`, added to `run.sh`. On the real binary: `--help` and `-h` (also after paths), `--version` and `-v`, usage errors exit `2` with the usage text, `settings --help`, `settings path`, one directory and one file (exit `2`, message names which), `mergediff -- settings b` and `./resolve b` open a diff window (title checked, closing exits `0`), `mergediff -o M -- B L R` opens a merge window. Run it against the current build: it must pass **before** any code changes.
2. **`cliargs: tokenizer and immutable Option`** — `internal/cliargs` with table tests: the tokenizing cases of `cli_test.go` (`TestParseArgs`, `TestParseArgsErrors`), the `--` index, order kept, getters return copies (changing the returned slice does not change the `Option`), repeated-flag rules. Not used by `main` yet.

**Accept:** `make check` passes; `cli.sh` passes on the current binary.

### Phase 2 — Commands (main still on the old path)

3. **`command: Command, Env, UsageError, exit codes; Help and Version`** — the usage text is copied from `cli.go` (the old one is removed in step 9).
4. **`command: Settings`** — logic from `settings_cmd.go`, tests ported from `settings_cmd_test.go`; rejects settings flags, `-o`, `--auto-apply`.
5. **`command: Diff and Scratch with the shared window helper`** — `bothDirs`, `loadSettings`; tests ported from `main_test.go` (`TestRunOpensWindow`, `TestRunDirectories`, `TestSettingsFromConfigAndFlags`, `TestRunWithoutWindow`) using a fake `OpenWindow`.
6. **`test: share the conflicted-repo helper`** — move `conflictRepo` from `resolve_test.go` to a test-only package `internal/testrepo`, used by the root `Windows` tests and the command tests.
7. **`command: Merge and Resolve`** — tests ported from `TestRunResolve` and `TestRunResolveErrors`.
8. **`command: Select`** — reserved words, `--`, help / version precedence and the command table; the selection cases of `cli_test.go` plus new ones: `help`, `help settings`, `help nope` (error), `version x` (error), `settings list --theme dark` (error), `--theme dark settings list` (settings → error), `-- help b` (diff), `-o M -- help L R` (merge).

**Accept:** `make check` passes; `cliargs` and `command` together cover every case of today's `cli_test.go`, `settings_cmd_test.go` and the `run` tests in `main_test.go` / `resolve_test.go`.

### Phase 3 — Switch, cleanup, docs

9. **`main: run through cliargs and command`** — the new `run(args, env)` and `report`; `App` uses `command.Exit*`. The root tests keep the `App`, crash, close-guard, window-options and `Windows` tests, plus one wiring test of `run` with a fake `Env`.
10. **`main: remove cli.go and settings_cmd.go`** and their tests.
11. **`e2e: cli.sh checks the new commands`** — `mergediff help`, `help settings`, `version`, `version x` (exit `2`), `settings list --theme dark` (exit `2`), `mergediff -- help version` opens a diff of the files `help` and `version`, `mergediff -o M -- help L R` opens a merge.
12. **`docs: README and usage text for reserved words and --`** — the usage text and README list the four reserved words, recommend `--` (`mergediff -- help other.txt`), say that options go before `--` (`mergediff -o MERGED -- help local remote`), and mention `mergediff help` / `mergediff version`. This replaces "write `./settings`".
13. **`docs: CLAUDE.md for cliargs and command; e2e README`**:
    - Repository layout: add `internal/cliargs`, `internal/command`, `internal/testrepo`; remove `cli.go`, `settings_cmd.go`; add this file.
    - "Rules for the root `main` package": main parses, selects, runs and exits; commands live in `internal/command`.
    - Error handling: commands return `(code, err)` and write their own normal output; only `main` turns errors into `mergediff: …` and exit `2`.
    - Command-line interface: `help` / `version` subcommands, the reserved words, `--`.
    - Phases: a "Refactor 1 — Command line" entry.
    - `scripts/e2e/README.md`: describe `cli.sh`.

**Accept:** `make check` and `make build && make e2e` pass (every script, including `mergetool.sh`, `dirdiff.sh` with `git difftool -d`, `resolve.sh`, `settings.sh` and `cli.sh`). Manual check: `bin/mergediff help`, `bin/mergediff version`, `bin/mergediff settings --help`, `bin/mergediff -- help version`.

## Finishing

1. Make sure the branch is up to date with `main` (`git fetch && git rebase origin/main`; the history uses PR merges), and `make check` passes.
2. Push the branch and open a pull request into `main` (`gh pr create` if `gh` is available; otherwise print the compare URL). Use the summary of all phases as the description.
3. **Stop.** Do not merge the PR yourself; the maintainer reviews and merges it. The GUI refactor (`internal/gui`) starts only after this branch is merged.

## Out of scope

Moving the GUI (`app.go`, `windows.go`, `runGUI`) out of the root package, new commands or flags, per-command help texts beyond `help settings`, changes to exit codes or to Git configuration.
