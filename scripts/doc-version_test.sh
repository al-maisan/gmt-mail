#!/bin/bash
# Tests for doc-version.sh: make test-release
# shellcheck disable=SC2016 # the backticks are Markdown, not commands
set -uo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
dv() { DOC_ROOT=$TMP "$SRC/scripts/doc-version.sh" "$@" 2>/dev/null; }

cat >"$TMP/README.md" <<'X'
Example for Linux amd64 (set `V` to the latest release):
    $ V=0.9.1 A=gmt-mail-$V-linux-amd64
Releases after v0.10.0 also have `.deb` packages.
    $ sudo apt install ./gmt-mail_${V}-1_arm64.deb
X

dv check 0.9.1
check "current version passes" 0 $?
dv check 0.10.0
check "stale version fails" 1 $?
dv set 0.10.0
check "set" 0 $?
dv check 0.10.0
check "set version passes" 0 $?
check "README: example set, history kept" 'Example for Linux amd64 (set `V` to the latest release):
    $ V=0.10.0 A=gmt-mail-$V-linux-amd64
Releases after v0.10.0 also have `.deb` packages.
    $ sudo apt install ./gmt-mail_${V}-1_arm64.deb' "$(cat "$TMP/README.md")"
printf 'V=1.3.2\nV=1.3.1.\nMYV=9.9.9 V=1.3.10\n' >"$TMP/README.md"
dv set 1.3.3
check "set at a line end, before a dot, not in other names" "$(printf 'V=1.3.3\nV=1.3.3.\nMYV=9.9.9 V=1.3.3')" "$(cat "$TMP/README.md")"
dv check 1.3.3
check "check at a line end" 0 $?
printf 'V=1.3.30\n' >"$TMP/README.md"
dv check 1.3.3
check "a longer version is another version" 1 $?
: >"$TMP/README.md"
dv check 1.3.3
check "no example at all fails" 1 $?
rm "$TMP/README.md"
dv check 1.3.3
check "a missing file fails" 1 $?
dv set v1.3.2
check "a v prefix is refused" 2 $?
dv bump 1.3.2
check "an unknown command is refused" 2 $?

exit "$fail"
