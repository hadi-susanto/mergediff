#!/usr/bin/env bash
# Responsiveness on a 10,000-line merge with ~430 changes, in the real UI.
# Budgets are generous; they catch "the window froze", not small slowdowns.
set -u
source "$(dirname "$0")/lib.sh"
python3 "$E2E_DIR/gen-large.py" "$WORK/large"
L=$WORK/large
now() { date +%s.%N; }
secs() { awk -v a="$1" -v b="$2" 'BEGIN { printf "%.2f", b - a }'; }
within() { awk -v a="$1" -v b="$2" -v max="$3" 'BEGIN { exit !(b - a <= max) }'; }

section "10,000-line merge"
t0=$(now)
start_app "$L/base.go" "$L/local.go" "$L/remote.go" -o "$L/out.go"
ui wait
t1=$(now)
check "interactive within 10 s ($(secs "$t0" "$t1") s)" within "$t0" "$t1" 10
focus "mergediff — out.go"
read -r X Y W H <<<"$(wmctrl -lG | grep 'mergediff — out.go' | awk '{print $3, $4, $5, $6}')"

# 200 wheel notches 30 ms apart (~6 s of input) through the Result pane,
# then the UI must answer promptly: no backlog of unprocessed scrolling.
ui scroll $((X + W / 2)) $((Y + H / 2)) 200
t0=$(now)
ui dump >/dev/null
t1=$(now)
check "responsive right after fast scrolling ($(secs "$t0" "$t1") s)" within "$t0" "$t1" 2
ui scroll $((X + W / 6)) $((Y + H / 2)) -100
t0=$(now)
ui dump >/dev/null
t1=$(now)
check "responsive after scrolling the Local pane back ($(secs "$t0" "$t1") s)" within "$t0" "$t1" 2

t0=$(now)
ui type "typing check " click "✦ Apply non-conflicting changes"
STATE=$(state "✦ Apply non-conflicting changes")
t1=$(now)
check "typing + apply all within 3 s ($(secs "$t0" "$t1") s)" within "$t0" "$t1" 3
check "apply all applied everything" test "$STATE" = disabled
ui click Save click "Save anyway"
finish_app
check "exit 1 (conflicts left)" test $RC -eq 1
check "result saved with the typing" test "$(head -c 13 "$L/out.go")" = "typing check "
summary
