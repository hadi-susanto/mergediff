#!/usr/bin/env bash
# Keyboard shortcuts, undo of "apply all", and the diff window, in the real UI.
set -u
source "$(dirname "$0")/lib.sh"
cd "$ROOT" || exit 1

# Files with chunks of very different heights.
python3 - "$WORK" <<'EOF'
import sys
d = sys.argv[1]
base = [f"u{i:03d} unchanged line\n" for i in range(300)]
local = base[:50] + [f"LOCAL BLOCK {k:02d}\n" for k in range(60)] + base[52:150] + base[190:200] + ["CONFLICT local\n"] + base[201:]
remote = base[:200] + ["CONFLICT remote\n"] + base[201:220] + [f"REMOTE INSERT {k:02d}\n" for k in range(80)] + base[220:]
for name, lines in (("base", base), ("local", local), ("remote", remote)):
    open(f"{d}/{name}.txt", "w").write("".join(lines))
EOF

section "merge window: shortcuts and dialogs"
start_app "$WORK/base.txt" "$WORK/local.txt" "$WORK/remote.txt" -o "$WORK/out.txt"
ui wait sleep 1
focus "mergediff — out.txt"
ui key F7 key F7 key F7 key F7 key shift+F7
check "F7 / Shift+F7 keep the window open" kill -0 "$APP_PID"
check "apply-all enabled before Ctrl+Shift+A" test "$(state '✦ Apply non-conflicting changes')" = enabled
ui key ctrl+shift+a
check "Ctrl+Shift+A applied everything" test "$(state '✦ Apply non-conflicting changes')" = disabled
ui key ctrl+z
check "one Ctrl+Z reverts the whole apply" test "$(state '✦ Apply non-conflicting changes')" = enabled
ui key ctrl+shift+a key ctrl+s
check "Ctrl+S asks because a conflict is left" has "Save anyway"
ui key Escape
check "Esc closes the dialog (keep editing)" bash -c "! $UI dump 2>/dev/null | grep -qF 'Save anyway'"
check "window still open" kill -0 "$APP_PID"
ui key Escape
check "Esc asks before discarding edits" has "Discard"
ui click Discard
finish_app
check "discard exits 1" test $RC -eq 1
check "nothing written" test ! -e "$WORK/out.txt"

section "merge window: undo keeps earlier typing"
start_app testdata/dev/base.go testdata/dev/local.go testdata/dev/remote.go -o "$WORK/typed.go"
ui wait sleep 1 type "XY" click "✦ Apply non-conflicting changes"
focus "mergediff — typed.go"
ui key ctrl+z click Save click "Save anyway"
finish_app
check "exit 1 (conflict left)" test $RC -eq 1
check "saved = base with the typing, apply undone" test "$(cat "$WORK/typed.go")" = "XY$(cat testdata/dev/base.go)"

section "merge window: resolve with the strip buttons, Ctrl+S saves and exits 0"
start_app testdata/dev/base.go testdata/dev/local.go testdata/dev/remote.go -o "$WORK/saved.go"
# Buttons before any action: local strip (A: ×#0 »#0, conflict: ×#1 »#1);
# after applying local to the conflict, the remote strip's conflict ignore is ×#1.
ui wait sleep 1 click "↓ Conflict" click "Apply this change to the result#1" click "Ignore this change#1"
check "conflict navigation disabled once resolved" test "$(state '↓ Conflict')" = disabled
focus "mergediff — saved.go"
ui key ctrl+shift+a key ctrl+s
finish_app
check "exit 0" test $RC -eq 0
EXPECT=$(sed -e 's/builds/returns/' -e 's/"Hello, "/"Hi, "/' -e 's/"world"/"Go"/' testdata/dev/base.go)
check "saved result" test "$(cat "$WORK/saved.go")" = "$EXPECT"

section "diff window"
start_app "$WORK/local.txt" "$WORK/remote.txt"
ui wait sleep 1
focus "mergediff — local.txt"
ui key F7 key F7 key shift+F7
check "F7 / Shift+F7 keep the window open" kill -0 "$APP_PID"
ui key Escape
finish_app
check "Esc closes the diff with exit 0" test $RC -eq 0

summary
