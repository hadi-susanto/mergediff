#!/usr/bin/env bash
# Runs every end-to-end script. Linux with an X11 session only: the scripts
# drive the real window through AT-SPI (python3-pyatspi) and XTest
# (python3-xlib), and raise it with wmctrl.
set -u
cd "$(dirname "$0")" || exit 1

missing=()
[ "$(uname -s)" = Linux ] || missing+=("Linux")
[ -n "${DISPLAY:-}" ] || missing+=("an X11 display (DISPLAY)")
python3 -c 'import pyatspi' 2>/dev/null || missing+=("python3-pyatspi")
python3 -c 'import Xlib' 2>/dev/null || missing+=("python3-xlib")
command -v wmctrl >/dev/null || missing+=("wmctrl")
command -v git >/dev/null || missing+=("git")
if [ ${#missing[@]} -gt 0 ]; then
  printf 'e2e: missing %s\n' "${missing[@]}" >&2
  exit 2
fi
[ -x "${MERGEDIFF_BIN:-../../bin/mergediff}" ] || { echo "e2e: build first (make build)" >&2; exit 2; }

status=0
for script in mergetool.sh shortcuts.sh perf.sh; do
  echo "### $script"
  bash "$script" || status=1
  echo
done
[ $status -eq 0 ] && echo "e2e: all passed" || echo "e2e: FAILED"
exit $status
