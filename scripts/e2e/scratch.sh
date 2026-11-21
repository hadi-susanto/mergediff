#!/usr/bin/env bash
# Scratch mode: mergediff without arguments, text typed into both panes,
# Refresh (button, F5, Ctrl+Enter) and the out-of-date state.
set -u
source "$(dirname "$0")/lib.sh"
TITLE="mergediff — scratch"

section "scratch diff"
start_app
ui wait sleep 1
focus "$TITLE"
read -r X Y W H <<<"$(geometry "$TITLE")"
check "Refresh button" has "⟳ Refresh"
check "next change disabled before comparing" test "$(state 'Next change (F7)')" = disabled
check "hint in the status bar" ui find "then Refresh (F5)"

ui clickat $((X + W / 4)) $((Y + H / 2)) type "alpha" key Return type "beta" key Return
ui clickat $((X + W * 3 / 4)) $((Y + H / 2)) type "alpha" key Return type "BETA" key Return
ui key F5
check "F5 compares" ui find "1 change · 1 modified"
check "next change enabled" test "$(state 'Next change (F7)')" = enabled

ui type "gamma"
check "editing marks the result out of date" ui find "Edited — Refresh (F5)"
check "next change disabled while out of date" test "$(state 'Next change (F7)')" = disabled
ui key ctrl+Return
check "Ctrl+Enter compares again" ui find "1 change · 1 modified"

ui clickat $((X + W / 4)) $((Y + H / 2)) key ctrl+a type "alpha" key Return type "BETA" key Return type "gamma"
ui click "⟳ Refresh"
check "Refresh button: identical texts" ui find "No differences"

ui key Escape
check "Esc does not close the scratch window" kill -0 "$APP_PID"
ui click Close
finish_app
check "Close exits 0" test $RC -eq 0
summary
