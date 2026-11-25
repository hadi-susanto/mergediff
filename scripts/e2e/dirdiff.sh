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
check "the files' windows closed with it" test "$(wmctrl -l | grep -cF 'mergediff —')" -eq 0
summary
