#!/usr/bin/env bash
# mergediff resolve: the Conflicts window on a repository stopped in a merge
# (scripts/dev/conflict-repo.sh, the mockups' nine conflicts). Rows are
# selected with real mouse clicks (XTest), not through the accessibility
# tree, so a regression that stops clicks reaching the list is caught.
set -u
source "$(dirname "$0")/lib.sh"
LIST="mergediff — Resolve conflicts"

# Row i of the flat list (0-based) is at y = 125 + 32 i + 16 in the window:
# operation bar, filter bar and column header above it.
row_y() { echo $((125 + 32 * $1 + 16)); }
# Rows, sorted by path:
#   0 config/application-production.yml       5 …/PaymentGatewayRegistry.java
#   1 …/checkout-banner@2x.png (binary)        6 …/StripePaymentGatewayAdapter.java
#   2 …/CheckoutSummaryPanel.tsx               7 …/V42__add_checkout_session_table.sql (added)
#   3 pom.xml                                  8 …/LegacyCartCalculatorTest.java (yours deleted)
#   4 …/LegacyCartCalculator.java (theirs deleted)
BANNER=frontend/public/images/checkout-banner@2x.png
CALC=src/main/java/com/example/shop/checkout/legacy/LegacyCartCalculator.java
REGISTRY=src/main/java/com/example/shop/payment/gateway/internal/PaymentGatewayRegistry.java

REPO=$WORK/shop-api
"$ROOT/scripts/dev/conflict-repo.sh" "$REPO" >/dev/null
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
G="git -C $REPO"

# unmerged PATH: true while Git still lists PATH as conflicted.
unmerged() { $G diff --name-only --diff-filter=U -- "$1" | grep -q .; }
# staged PATH XY: git status --porcelain shows XY for PATH.
status_is() { [ "$($G status --porcelain=v1 -- "$1" 2>/dev/null | cut -c1-2)" = "$2" ]; }
# same_as PATH REV: the working-tree file equals REV's version byte for byte.
same_as() { $G show "$2:$1" | cmp -s - "$REPO/$1"; }

section "the Conflicts window"
(cd "$REPO" && exec timeout "$APP_TIMEOUT" "$BIN" resolve) >"$WORK/app.log" 2>&1 &
APP_PID=$!
ui wait find "9 of 9 files unresolved"
check "window titled after the repository" test "$(wmctrl -l | grep -cF "$LIST · shop-api")" -eq 1
check "operation sentence names both branches" ui find "Merging " find "origin/main" find "feature/checkout"
check "chips count all, unresolved, resolved" ui find "All 9" find "Unresolved 9" find "Resolved 0"
focus "$LIST"

# A real click selects a row: the side panel describes it.
ui sleep 0.5 clickin "$LIST" 300 "$(row_y 1)"
check "a click selects the row (binary file described)" ui find "Binary file: it cannot be merged line by line"
check "Merge… is disabled for a binary file" test "$(state 'Merge…')" = disabled

ui click "Accept Theirs"
check "Accept Theirs resolves it: 8 of 9 left" ui find "8 of 9 files unresolved"
check "Git no longer lists it as conflicted" bash -c "! git -C '$REPO' diff --name-only --diff-filter=U | grep -qF '$BANNER'"
check "theirs is in the working tree, byte for byte" same_as "$BANNER" origin/main
check "the row stays, resolved" ui find "Resolved 1"

# Multi-select with Ctrl+click, then accept both with one confirmation.
focus "$LIST"
ui clickin "$LIST" 300 "$(row_y 0)" modclickin ctrl "$LIST" 300 "$(row_y 3)"
check "Ctrl+click selects a second row" ui find "2 files selected, 2 unresolved."
ui click "Accept Yours"
check "accepting 2 files asks first, naming how many" ui find "Accept Yours for 2 files?"
ui click "Accept Yours" # the dialog's button: the modal hides the side panel's
check "both resolved: 6 of 9 left" ui find "6 of 9 files unresolved"
check "yours kept (same as HEAD, nothing to commit)" status_is pom.xml ""
check "config resolved too" bash -c "! git -C '$REPO' diff --name-only --diff-filter=U | grep -q config/"

# Modify/delete: the buttons say what they do.
focus "$LIST"
ui clickin "$LIST" 300 "$(row_y 4)"
check "modify/delete buttons are named by their effect" has "Keep modified (yours)"
check "and the other side deletes" has "Delete (theirs)"
ui click "Delete (theirs)"
check "Delete (theirs) removes the file" test ! -e "$REPO/$CALC"
check "and stages the deletion" status_is "$CALC" "D "

# Resolved in a terminal meanwhile: F5 shows it.
$G checkout --theirs -- "$REGISTRY" && $G add -- "$REGISTRY"
focus "$LIST"
ui key F5
check "F5 picks up a resolution made in a terminal" ui find "4 of 9 files unresolved"

# Group by directory, and selecting a row in a group with a real click.
ui click "Group by directory"
check "groups count their files" ui find "1 file · all resolved" find "1 file · 1 unresolved"
focus "$LIST"
# Grouped rows: (root) header, pom.xml, config/ header, application-production.yml,
# frontend/public/images/ header, checkout-banner, frontend/src/… header, panel.
ui clickin "$LIST" 300 "$(row_y 7)"
check "a click in the grouped list selects that row" ui find "Both sides changed this file."
check "keyboard accept works on it (Ctrl+Shift+T)" bash -c "$UI key ctrl+shift+t find '3 of 9 files unresolved' 2>/dev/null"

ui click "Close"
finish_app
check "closed with conflicts left: exit 1" test $RC -eq 1

section "exit codes without a window"
check "outside a repository: exit 2" bash -c "cd '$WORK' && '$BIN' resolve >/dev/null 2>&1; test \$? -eq 2"
$G checkout --ours -- . 2>/dev/null
for p in $($G diff --name-only --diff-filter=U); do
  if [ -e "$REPO/$p" ]; then $G add -- "$p"; else $G rm -q -- "$p"; fi
done
check "nothing left: message and exit 0" bash -c "cd '$REPO' && '$BIN' resolve | grep -qx 'mergediff: no conflicts to resolve'"
summary
