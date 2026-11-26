#!/usr/bin/env bash
# Command line: help and version (flags and subcommands), usage errors and
# exit codes, the settings subcommand's help and the flags it rejects,
# reserved words used as file names, and options before "--". Most checks
# need no window; the file-name checks open one and close it again.
set -u
source "$(dirname "$0")/lib.sh"

# cli ARGS...: run mergediff without waiting for a window; sets RC and
# writes $WORK/out and $WORK/err. The timeout catches a window that opens
# by mistake.
cli() {
  timeout 20 "$BIN" "$@" >"$WORK/out" 2>"$WORK/err"
  RC=$?
}
out_has() { grep -qF -- "$1" "$WORK/out"; }
err_has() { grep -qF -- "$1" "$WORK/err"; }
out_empty() { test ! -s "$WORK/out"; }

# wait_window TITLE: wait up to 10 s for a window with TITLE.
wait_window() {
  local i
  for i in $(seq 50); do
    [ -n "$(window_id "$1" 2>/dev/null)" ] && return 0
    sleep 0.2
  done
  return 1
}

# open_and_close TITLE ARGS...: start mergediff with ARGS, check a window
# called "mergediff — TITLE" opens, close it with its close button; sets RC.
open_and_close() {
  local title="mergediff — $1"
  shift
  start_app "$@"
  check "window \"$title\" opens" wait_window "$title"
  ui wait sleep 0.5
  wmctrl -c "$title"
  finish_app
}

cd "$WORK" || exit 1
printf 'one\ntwo\n' >a.txt
printf 'one\nTWO\n' >b.txt
for name in help version settings resolve; do printf '%s\n' "$name" >"$name"; done
mkdir dir

section "help and version"
cli --help
check "--help: exit 0" test $RC -eq 0
check "--help: usage on stdout" out_has "Usage:"
cli -h
check "-h: usage, exit 0" bash -c "test $RC -eq 0 && grep -qF Usage: '$WORK/out'"
cli a.txt b.txt -h
check "-h after paths wins: usage, exit 0, no window" bash -c "test $RC -eq 0 && grep -qF Usage: '$WORK/out'"
cli --version
check "--version: exit 0" test $RC -eq 0
check "--version: 'mergediff DATE (COMMIT)'" grep -qE '^mergediff [^ ]+ \([^)]+\)$' "$WORK/out"
cli -v
check "-v: version, exit 0" bash -c "test $RC -eq 0 && grep -q '^mergediff ' '$WORK/out'"
cli resolve --help
check "resolve --help: general usage, exit 0" bash -c "test $RC -eq 0 && grep -qF 'mergediff resolve' '$WORK/out'"

section "usage errors (exit 2, message and usage on stderr)"
expect_usage() { # MESSAGE ARGS...
  local msg=$1
  shift
  cli "$@"
  check "$* → exit 2" test $RC -eq 2
  check "$* → '$msg'" err_has "mergediff: $msg"
  check "$* → usage shown" err_has "Usage:"
  check "$* → nothing on stdout" out_empty
}
expect_usage "expected 2 files or directories" a.txt
expect_usage "unknown option --nope" a.txt b.txt --nope
expect_usage "-o is only valid for a 3-way merge" a.txt b.txt -o m.txt
expect_usage "--auto-apply is only valid for a 3-way merge" a.txt b.txt --auto-apply
expect_usage "a 3-way merge needs -o MERGED" a.txt a.txt b.txt
expect_usage "-o needs a file name" a.txt a.txt b.txt -o
expect_usage "-o given more than once" a.txt a.txt b.txt -o m -o n
expect_usage "--theme: invalid value" a.txt b.txt --theme blue
expect_usage "--theme given more than once" --theme dark a.txt b.txt --theme=light
expect_usage "--auto-apply does not take a value" a.txt a.txt b.txt -o m --auto-apply=yes
expect_usage "-o is not valid with resolve" resolve -o m.txt
expect_usage "LEFT dir is a directory but RIGHT a.txt is a file" dir a.txt
expect_usage "LEFT a.txt is a file but RIGHT dir is a directory" a.txt dir

section "settings subcommand"
cli settings --help
check "settings --help: settings usage, exit 0" bash -c "test $RC -eq 0 && grep -qF 'mergediff settings get KEY' '$WORK/out'"
cli settings path
check "settings path" bash -c "test $RC -eq 0 && test \"\$(cat '$WORK/out')\" = '$XDG_CONFIG_HOME/mergediff/config.toml'"
cli settings nope
check "settings nope: exit 2" test $RC -eq 2
check "settings nope: settings usage" err_has "mergediff settings get KEY"

section "reserved words as file names"
open_and_close "settings ↔ b.txt" -- settings b.txt
check "-- settings b.txt: diff closes with exit 0" test $RC -eq 0
open_and_close "resolve ↔ b.txt" ./resolve b.txt
check "./resolve b.txt: diff closes with exit 0" test $RC -eq 0
open_and_close "help ↔ version" -- help version
check "-- help version: diff closes with exit 0" test $RC -eq 0
open_and_close "b.txt ↔ settings" b.txt settings
check "settings not first: diff closes with exit 0" test $RC -eq 0

section "options before --"
open_and_close "m.txt" -o m.txt -- help a.txt b.txt
check "-o M -- B L R: merge closed unsaved exits 1" test $RC -eq 1
check "-o M -- B L R: nothing written" test ! -e m.txt

section "help and version subcommands"
cli help
check "help: general usage, exit 0" bash -c "test $RC -eq 0 && grep -qF 'mergediff resolve' '$WORK/out'"
cli help settings
check "help settings: settings usage, exit 0" bash -c "test $RC -eq 0 && grep -qF 'mergediff settings get KEY' '$WORK/out' && ! grep -qF 'mergediff resolve' '$WORK/out'"
cli help resolve
check "help resolve: general usage, exit 0" bash -c "test $RC -eq 0 && grep -qF 'mergediff resolve' '$WORK/out'"
cli version
check "version: exit 0" test $RC -eq 0
check "version: 'mergediff DATE (COMMIT)'" grep -qE '^mergediff [^ ]+ \([^)]+\)$' "$WORK/out"
cli version --help
check "version --help: general usage, exit 0" bash -c "test $RC -eq 0 && grep -qF Usage: '$WORK/out'"
expect_usage "version takes no arguments" version x
expect_usage 'unknown help topic "nope"' help nope
expect_usage "-o is not valid with help" help -o m.txt

section "settings rejects window flags"
expect_settings_usage() { # MESSAGE ARGS...
  local msg=$1
  shift
  cli "$@"
  check "$* → exit 2" test $RC -eq 2
  check "$* → '$msg'" err_has "mergediff: $msg"
  check "$* → settings usage" err_has "mergediff settings get KEY"
}
expect_settings_usage "--theme is not valid with settings; to save it, use: mergediff settings set theme dark" settings list --theme dark
expect_settings_usage "--theme is not valid with settings" --theme dark settings list
expect_settings_usage "-o is not valid with settings" settings -o m.txt
expect_settings_usage "--auto-apply is not valid with settings" settings path --auto-apply
check "config file untouched" test ! -e "$XDG_CONFIG_HOME/mergediff/config.toml"
summary
