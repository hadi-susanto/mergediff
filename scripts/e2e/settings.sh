#!/usr/bin/env bash
# Settings: the `mergediff settings` command, the config file, --theme and
# --window-state, in the real binary and window.
set -u
source "$(dirname "$0")/lib.sh"
CONF=$XDG_CONFIG_HOME/mergediff/config.toml
printf 'one\ntwo\n' >"$WORK/a.txt"
printf 'one\nTWO\n' >"$WORK/b.txt"
TITLE="mergediff — a.txt ↔ b.txt"

section "settings command"
check "path is under XDG_CONFIG_HOME" test "$("$BIN" settings path)" = "$CONF"
check "list shows the defaults" bash -c "'$BIN' settings list | grep -Eq '^theme +system +default'"
mkdir -p "$(dirname "$CONF")"
printf '# my settings\n' >"$CONF"
"$BIN" settings set theme dark >/dev/null
"$BIN" settings set window-state maximized >/dev/null
check "set keeps comments" test "$(cat "$CONF")" = $'# my settings\ntheme = "dark"\nwindow-state = "maximized"'
check "get" test "$("$BIN" settings get theme)" = dark
before=$(cat "$CONF")
"$BIN" settings set theme blue 2>"$WORK/err"
RC=$?
check "invalid value: exit 2" test $RC -eq 2
check "invalid value: message lists allowed values" grep -q 'allowed: system, light, dark' "$WORK/err"
check "invalid value: file untouched" test "$(cat "$CONF")" = "$before"

# toolbar_brightness: brightness of the toolbar background of the window.
toolbar_brightness() {
  local x y w h
  read -r x y w h <<<"$(geometry "$TITLE")"
  brightness "$(ui pixel $((x + w / 2)) $((y + 20)))"
}
maximized() { xprop -id "$(window_id "$TITLE")" _NET_WM_STATE | grep -q _NET_WM_STATE_MAXIMIZED_VERT; }

# samples: toolbar brightness of every frame in the first 3 s of the window.
samples() { ui samples "$TITLE" 3 >"$WORK/samples"; }
# brightest / darkest: of the painted frames (0 = not drawn yet, skipped).
brightest() { awk '$1 > 0' "$WORK/samples" | sort -n | tail -1; }
darkest() { awk '$1 > 0' "$WORK/samples" | sort -n | head -1; }

section "config: dark theme, maximized window"
start_app "$WORK/a.txt" "$WORK/b.txt"
samples
check "no light frame while starting ($(wc -l <"$WORK/samples") frames, brightest $(brightest))" test "$(brightest)" -lt 96
ui wait sleep 1.5
focus "$TITLE"
check "window starts maximized" maximized
B=$(toolbar_brightness)
check "toolbar is dark (brightness $B)" test "$B" -lt 96
ui click Close
finish_app
check "exit 0" test $RC -eq 0

section "flags override the config: --theme light --window-state normal"
start_app --theme light "$WORK/a.txt" "$WORK/b.txt" --window-state=normal
samples
# Frames before the page draws show the GTK theme's window background, so
# only the toolbar's dark colour (#262624, brightness 37) counts as a flash.
check "no dark-theme frame while starting (darkest $(darkest))" bash -c "! awk '\$1 >= 30 && \$1 <= 44' '$WORK/samples' | grep -q ."
ui wait sleep 1.5
focus "$TITLE"
check "window is not maximized" bash -c "! xprop -id '$(window_id "$TITLE")' _NET_WM_STATE | grep -q MAXIMIZED"
B=$(toolbar_brightness)
check "toolbar is light (brightness $B)" test "$B" -gt 192
ui click Close
finish_app
check "exit 0" test $RC -eq 0

section "broken config"
printf 'theme = "blue"\n' >"$CONF"
timeout 20 "$BIN" "$WORK/a.txt" "$WORK/b.txt" 2>"$WORK/err"
RC=$?
check "exit 2 without a window" test $RC -eq 2
check "message names the config file" grep -qF "mergediff: config $CONF: " "$WORK/err"
"$BIN" settings unset theme >/dev/null
check "unset repairs it" test "$("$BIN" settings get theme)" = system
summary
