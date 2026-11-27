# Shared helpers for the end-to-end scripts. Source it; do not run it.
# shellcheck shell=bash

# Decimal points in timings regardless of the user's locale.
export LC_NUMERIC=C

E2E_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$E2E_DIR/../.." && pwd)
BIN=${MERGEDIFF_BIN:-$ROOT/bin/mergediff}
WORK=$(mktemp -d "${TMPDIR:-/tmp}/mergediff-e2e.XXXXXX")
APP_PID=
trap '[ -n "$APP_PID" ] && kill "$APP_PID" 2>/dev/null; rm -rf "$WORK"' EXIT
# Settings come from a config file of our own, never the user's.
export XDG_CONFIG_HOME=$WORK/config

# Every app run and UI step is time-limited so a failing step reports FAIL
# instead of leaving a window open forever.
APP_TIMEOUT=${APP_TIMEOUT:-60}
UI="timeout 30 python3 $E2E_DIR/ui.py"

PASS=${PASS:-0}
FAIL=${FAIL:-0}

# check DESCRIPTION COMMAND...
check() {
  local d=$1
  shift
  if "$@"; then
    echo "  ok   $d"
    PASS=$((PASS + 1))
  else
    echo "  FAIL $d"
    FAIL=$((FAIL + 1))
  fi
}

section() { echo "== $*"; }

# ui STEP... : drive the window (see ui.py); warnings from AT-SPI are dropped.
ui() { $UI "$@" 2>/dev/null; }

# state NAME: "enabled" or "disabled" for the first button named NAME.
state() { ui dump | grep -F "'$1'" | head -1 | sed 's/.*(\(.*\))/\1/'; }

# has NAME: a button named NAME exists.
has() { ui dump | grep -qF "'$1'"; }

# focus TITLE: raise the window so XTest key presses reach it. ui.py
# refuses to send input while another window is active.
focus() {
  wmctrl -a "$1"
  sleep 0.3
}

# geometry TITLE: prints "X Y W H" of the window.
geometry() { wmctrl -lG | grep -F "$1" | head -1 | awk '{print $3, $4, $5, $6}'; }

# app_windows: number of open mergediff windows, i.e. titles starting with
# "mergediff — ". Matching anywhere in the title would also count other
# applications showing the name, e.g. a browser tab on the GitHub repository.
app_windows() {
  wmctrl -l | awk '{ t = $0; for (i = 0; i < 3; i++) sub(/^[^ ]+ +/, "", t); if (index(t, "mergediff — ") == 1) n++ } END { print n + 0 }'
}

# window_id TITLE: prints the X window id.
window_id() { wmctrl -l | grep -F "$1" | head -1 | awk '{print $1}'; }

# brightness "#rrggbb": prints 0..255 (average of the channels).
brightness() { printf '%d\n' $(( (16#${1:1:2} + 16#${1:3:2} + 16#${1:5:2}) / 3 )); }

# start_app ARGS...: launch mergediff in the background; sets APP_PID.
start_app() {
  timeout "$APP_TIMEOUT" "$BIN" "$@" >"$WORK/app.log" 2>&1 &
  APP_PID=$!
}

# finish_app: wait for the app; sets RC (124 if it had to be killed).
finish_app() {
  wait "$APP_PID"
  RC=$?
}

summary() {
  echo
  echo "passed $PASS, failed $FAIL"
  [ "$FAIL" -eq 0 ]
}
