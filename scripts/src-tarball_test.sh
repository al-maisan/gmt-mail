#!/bin/bash
# Tests for src-tarball.sh: make test-ppa
# Uses the newest stable tag; go mod vendor needs the module cache or network.
set -uo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}
builds() { if "$SRC/scripts/src-tarball.sh" "$@" >/dev/null 2>&1; then echo yes; else echo no; fi; }

TAG=$(git -C "$SRC" tag -l 'v*' --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)
if [ -z "$TAG" ]; then
    echo "skip - src-tarball tests: no stable tag (fetch the tags)"
    exit 0
fi
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

"$SRC/scripts/src-tarball.sh" "$TAG" "$TMP/a.tar.gz" >/dev/null || { echo "FAIL - build"; exit 1; }
sleep 1 # a later run must not change the times
(umask 077 && "$SRC/scripts/src-tarball.sh" "$TAG" "$TMP/b.tar.gz" >/dev/null)
check "same tag, same tarball (later, umask 077)" "$(sha256sum <"$TMP/a.tar.gz")" "$(sha256sum <"$TMP/b.tar.gz")"

NAME=gmt-mail-${TAG#v}
LIST=$(tar -tvzf "$TMP/a.tar.gz" --numeric-owner)
check "one top dir" "$NAME/" "$(tar -tzf "$TMP/a.tar.gz" | cut -d/ -f1 | sort -u)/"
check "vendored modules" yes "$(tar -tzf "$TMP/a.tar.gz" | grep -qx "$NAME/vendor/modules.txt" && echo yes || echo no)"
check "owner root" "0/0" "$(awk '{ print $2 }' <<<"$LIST" | sort -u)"
check "times: the tag's commit time" "$(TZ=UTC git -C "$SRC" log -1 --format=%cd --date=format-local:'%Y-%m-%d %H:%M' "$TAG^{commit}")" \
    "$(TZ=UTC tar -tvzf "$TMP/a.tar.gz" | awk '{ print $4, $5 }' | sort -u)"
check "gzip: no timestamp" 0 "$(od -An -tu4 -j4 -N4 "$TMP/a.tar.gz" | tr -d ' ')"

check "rejects a prerelease tag" no "$(builds v1.2.3-rc1 "$TMP/c.tar.gz")"
check "rejects an unknown tag" no "$(builds v99.0.0 "$TMP/c.tar.gz")"
check "rejects a non-.tar.gz output" no "$(builds "$TAG" "$TMP/c.tar")"
check "rejects a missing output dir" no "$(builds "$TAG" "$TMP/nope/c.tar.gz")"

exit "$fail"
