#!/usr/bin/env bash
# mergediff as a real `git mergetool`: exit codes, result files, line endings.
set -u
source "$(dirname "$0")/lib.sh"

# new_repo DIR: a repo with a conflicting merge in main.go (LF) and crlf.txt
# (CRLF, stored byte-exact). LOCAL is branch feature, REMOTE is main.
new_repo() {
  rm -rf "$1" && mkdir -p "$1" && cd "$1" || exit 1
  git init -q -b main
  git config user.email e2e@example.invalid
  git config user.name e2e
  git config commit.gpgsign false
  git config merge.tool mergediff
  git config mergetool.mergediff.cmd "timeout $APP_TIMEOUT $BIN \"\$BASE\" \"\$LOCAL\" \"\$REMOTE\" -o \"\$MERGED\""
  git config mergetool.mergediff.trustExitCode true
  git config mergetool.keepBackup false
  git config mergetool.prompt false
  echo '*.txt -text' >.gitattributes
  cp "$ROOT/testdata/dev/base.go" main.go
  printf 'a\r\nb\r\nc\r\nd\r\ne\r\n' >crlf.txt
  git add -A && git commit -qm base
  git checkout -q -b feature
  cp "$ROOT/testdata/dev/local.go" main.go
  printf 'A\r\nb\r\nL\r\nd\r\ne\r\n' >crlf.txt
  git commit -qam local
  git checkout -q main
  cp "$ROOT/testdata/dev/remote.go" main.go
  printf 'a\r\nb\r\nR\r\nd\r\nE\r\n' >crlf.txt
  git commit -qam remote
  git checkout -q feature
  git merge -q main >/dev/null 2>&1
}

# mergetool FILE STEP...: run git mergetool on FILE and drive the window.
mergetool() {
  local file=$1
  shift
  git mergetool "$file" >"$WORK/mergetool.log" 2>&1 &
  local pid=$!
  ui wait sleep 0.5 "$@"
  wait $pid
  RC=$?
}

bytes() { od -An -c "$1" | tr -s ' ' | tr -d '\n'; }

EXPECT_MAIN=$(sed -e 's/builds/returns/' -e 's/"Hello, "/"Hi, "/' -e 's/"world"/"Go"/' "$ROOT/testdata/dev/base.go")

section "resolve main.go: apply all, take local for the conflict, save"
new_repo "$WORK/repo1"
check "both files conflicted" test "$(git ls-files -u | cut -f2 | sort -u | tr '\n' ' ')" = "crlf.txt main.go "
# Strip buttons in DOM order after apply-all: local strip (conflict × »),
# remote strip (conflict « ×).
mergetool main.go \
  click "✦ Apply non-conflicting changes" \
  click "Apply this change to the result#0" \
  click "Ignore this change#0" \
  click Save
check "git mergetool exit 0" test $RC -eq 0
check "main.go no longer unmerged" test -z "$(git ls-files -u main.go)"
check "main.go staged" grep -qx main.go <(git diff --cached --name-only)
check "main.go content: local conflict side + both non-conflicting" test "$(cat main.go)" = "$EXPECT_MAIN"
check "main.go keeps LF" test "$(grep -c $'\r' main.go)" -eq 0

section "resolve crlf.txt: From Remote only, take remote for the conflict, save"
# Buttons: local strip (A: ×#0 »#0, conflict: ×#1 »#1), remote strip (conflict: «#2 ×#2).
mergetool crlf.txt \
  click "Apply from one side only" \
  click "From Remote only (1)" \
  click "Apply this change to the result#2" \
  click "Ignore this change#1" \
  click Save
check "git mergetool exit 0" test $RC -eq 0
check "crlf.txt no longer unmerged" test -z "$(git ls-files -u crlf.txt)"
printf 'a\r\nb\r\nR\r\nd\r\nE\r\n' >"$WORK/want.txt"
check "crlf.txt bytes: CRLF kept, local-only A not applied, remote E applied, conflict = R" \
  test "$(bytes crlf.txt)" = "$(bytes "$WORK/want.txt")"
git commit -qm merged
check "merge commits" test -z "$(git ls-files -u)"

section "cancel: git keeps the conflict"
new_repo "$WORK/repo2"
mergetool main.go click Cancel
check "git mergetool exit non-zero" test $RC -ne 0
check "git reports the failure" grep -q "merge of main.go failed" "$WORK/mergetool.log"
check "main.go still unmerged" test -n "$(git ls-files -u main.go)"
check "main.go still has conflict markers" grep -q '^<<<<<<<' main.go

section "save with an unresolved conflict: confirm, exit 1, git keeps the conflict"
mergetool main.go click "✦ Apply non-conflicting changes" click Save click "Save anyway"
check "git mergetool exit non-zero" test $RC -ne 0
check "main.go still unmerged" test -n "$(git ls-files -u main.go)"

section "--auto-apply matches the engine's helper"
cd "$ROOT" || exit 1
start_app testdata/dev/base.go testdata/dev/local.go testdata/dev/remote.go -o "$WORK/auto.go" --auto-apply
ui wait sleep 0.5
STATE=$(state "✦ Apply non-conflicting changes")
ui click Save click "Save anyway"
finish_app
check "apply button disabled at start" test "$STATE" = disabled
check "exit 1 (conflict left)" test $RC -eq 1
check "result = engine helper output" cmp -s "$WORK/auto.go" internal/merge/testdata/mockup_main_go/expected

summary
