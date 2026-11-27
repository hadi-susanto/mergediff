#!/usr/bin/env bash
# Directory diff: the list window, opening files in their own windows by
# double-click and Enter, refusing files that cannot be opened, and closing
# the list (which closes the files it opened) with exit 0.
set -u
source "$(dirname "$0")/lib.sh"
SAMPLES=$ROOT/testdata/dev/dirdiff
LIST="mergediff — Compare directories"

# window_appears TITLE: true once a window titled TITLE exists (5 s at most).
window_appears() {
  for _ in $(seq 1 25); do
    wmctrl -l | grep -qF "$1" && return 0
    sleep 0.2
  done
  return 1
}

# Row i of the list (0-based) is at y = 108 + 32 i + 16 in the window:
# path bar, filter bar and column header above it.
row_y() { echo $((108 + 32 * $1 + 16)); }

section "directory window"
start_app "$SAMPLES/left" "$SAMPLES/right"
ui wait find "All changes 12"
check "list loaded (12 changes)" true
focus "$LIST"

# Rows, sorted by path: 0 docs/latin1.txt (not UTF-8), 1 docs/notes.txt, ...
# The first click right after raising the window is sometimes not
# delivered to the page yet; give it a moment.
ui sleep 0.5 dblclickin "$LIST" 300 "$(row_y 1)"
check "double-click opens the file in its own window" window_appears "mergediff — docs/notes.txt"

focus "$LIST"
ui dblclickin "$LIST" 300 "$(row_y 0)"
check "a file that cannot be opened says why" ui find "docs/latin1.txt cannot be opened: not valid UTF-8"
check "and opens no window" test "$(wmctrl -l | grep -cF 'mergediff — docs/latin1.txt')" -eq 0

focus "$LIST"
ui key End key Return
check "End + Enter opens the last file" window_appears "mergediff — src/test/java/com/example/shop/checkout/legacy/LegacyCartCalculatorTest.java"

focus "$LIST"
ui key Escape
finish_app
check "Esc closes the list: exit 0" test $RC -eq 0
check "the files' windows closed with it" test "$(app_windows)" -eq 0

section "git difftool -d"
# A repository with one modified, one added (staged) and one deleted file.
# Git copies the changed files of both sides into two temporary
# directories (the working-tree side as symlinks) and runs the difftool
# once with them as $LOCAL and $REMOTE.
REPO=$WORK/repo
GIT="git -C $REPO -c user.name=e2e -c user.email=e2e@example.invalid -c commit.gpgsign=false"
git init -q "$REPO"
printf 'one\ntwo\nthree\n' >"$REPO/a.txt"
printf 'keep\n' >"$REPO/c.txt"
printf 'same\n' >"$REPO/same.txt"
$GIT add . && $GIT commit -q -m base
printf 'one\n2\nthree\nfour\n' >"$REPO/a.txt"
printf 'new\n' >"$REPO/b.txt"
$GIT add b.txt
rm "$REPO/c.txt"
timeout "$APP_TIMEOUT" $GIT -c difftool.mergediff.cmd="\"$BIN\" \"\$LOCAL\" \"\$REMOTE\"" \
  difftool -d --tool=mergediff HEAD >"$WORK/difftool.log" 2>&1 &
APP_PID=$!
ui wait find "All changes 3"
check "git difftool -d opens the list (3 changes)" ui find "Modified 1" find "Added 1" find "Deleted 1"
focus "$LIST"
ui key End key Return
check "opening a file from it works" window_appears "mergediff — c.txt"
focus "$LIST"
ui key Escape
finish_app
check "closing the list: git difftool exits 0" test $RC -eq 0
check "no window left" test "$(app_windows)" -eq 0
summary
