#!/usr/bin/env bash
# Derive the release version from a vYYYY-MM-DD tag (release workflow).
#
# Usage: version-from-tag.sh TAG
# Prints VERSION_DATE=YYYY-MM-DD and COMMIT=<short HEAD>, and appends them to
# $GITHUB_ENV when set. Exits 1 if TAG is not v + a real calendar date.
# Plain bash arithmetic, so it behaves the same on Linux, macOS and Git Bash.
set -euo pipefail

tag=${1:?usage: version-from-tag.sh TAG}
date=${tag#v}
fail() { echo "version-from-tag: $*" >&2; exit 1; }

[[ $tag == v* && $date =~ ^([0-9]{4})-([0-9]{2})-([0-9]{2})$ ]] ||
	fail "tag $tag is not vYYYY-MM-DD"
y=$((10#${BASH_REMATCH[1]})) m=$((10#${BASH_REMATCH[2]})) d=$((10#${BASH_REMATCH[3]}))
days=(0 31 28 31 30 31 30 31 31 30 31 30 31)
if ((y % 4 == 0 && (y % 100 != 0 || y % 400 == 0))); then days[2]=29; fi
((m >= 1 && m <= 12)) || fail "tag $tag has no month $m"
((d >= 1 && d <= days[m])) || fail "tag $tag is not a real date"

commit=$(git rev-parse --short=7 HEAD) || fail "cannot read HEAD"
echo "VERSION_DATE=$date"
echo "COMMIT=$commit"
if [[ -n ${GITHUB_ENV:-} ]]; then
	{ echo "VERSION_DATE=$date"; echo "COMMIT=$commit"; } >>"$GITHUB_ENV"
fi
