#!/bin/bash
# Tests for release-check.sh and release-key.sh: make test-release
# No network, no key: only the checks before the first remote call.
set -uo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
# shellcheck source=release-key.sh
source "$SRC/scripts/release-key.sh"
fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}
signed() { if printf '%s\n' "$@" | signed_by_release_key; then echo yes; else echo no; fi; }

K=$release_key
SUB=0123456789ABCDEF0123456789ABCDEF01234567
check "primary key signed" yes "$(signed "[GNUPG:] VALIDSIG $K 2026-01-01 1 0 4 0 1 10 01 $K")"
check "subkey of the release key signed" yes "$(signed "[GNUPG:] VALIDSIG $SUB 2026-01-01 1 0 4 0 1 10 01 $K")"
check "another key" no "$(signed "[GNUPG:] VALIDSIG $SUB 2026-01-01 1 0 4 0 1 10 01 $SUB")"
check "bad signature" no "$(signed "[GNUPG:] BADSIG ${K:24} Muharem")"
check "key in another line" no "$(signed "[GNUPG:] GOODSIG ${K:24} $K")"
check "no status" no "$(signed "")"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
# A copy of the scripts in a scratch repo, so the tree checks see a known state.
git init -q "$TMP/r" && mkdir "$TMP/r/scripts" && cp "$SRC"/scripts/release-{check,key}.sh "$TMP/r/scripts/"
git -C "$TMP/r" add . && git -C "$TMP/r" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm init
git -C "$TMP/r" tag v1.2.3
rc() { "$TMP/r/scripts/release-check.sh" "$@" 2>&1; }
check "needs a tag" "release-check: usage: $TMP/r/scripts/release-check.sh [pre|post] TAG" "$(rc)"
check "rejects an unknown phase" "release-check: usage: $TMP/r/scripts/release-check.sh [pre|post] TAG" "$(rc later v1.2.3)"
check "rejects a bad tag" "release-check: tag 'v1.2' is not vMAJOR.MINOR.PATCH[-PRE]" "$(rc v1.2)"
check "rejects an unknown tag" "release-check: HEAD is not v9.9.9; run: git checkout v9.9.9" "$(rc v9.9.9)"
check "rejects an unsigned tag" "release-check: tag 'v1.2.3' is not signed by $K" "$(rc v1.2.3)"
check "pre: rejects an unsigned tag" "release-check: tag 'v1.2.3' is not signed by $K" "$(rc pre v1.2.3)"
touch "$TMP/r/x"
check "rejects a dirty tree" "release-check: working tree not clean" "$(rc v1.2.3)"
check "pre: rejects a dirty tree" "release-check: working tree not clean" "$(rc pre v1.2.3)"

exit "$fail"
