#!/bin/bash
# Tests for build-dist.sh: make test-dist
# Builds two targets twice into a temp dir; no network, no signing.
set -uo pipefail

fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}

DIST="$(cd "$(dirname "$0")" && pwd)/build-dist.sh"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
export DIST_TARGETS="linux/amd64 linux/armv6 windows/amd64"

"$DIST" 1.2.3 abc1234 2026-01-02 "$TMP/a" >/dev/null || { echo "FAIL - build-dist.sh exited $?"; exit 1; }
cd "$TMP/a" || exit 1

check "archives" "gmt-mail-1.2.3-linux-amd64.tar.gz gmt-mail-1.2.3-linux-armv6.tar.gz gmt-mail-1.2.3-windows-amd64.zip SHA256SUMS" "$(ls | sort -f | paste -sd' ')"
check "checksums verify" "0" "$(sha256sum --quiet -c SHA256SUMS >/dev/null 2>&1; echo $?)"
check "tar contents" "gmt-mail-1.2.3-linux-amd64/ gmt-mail-1.2.3-linux-amd64/LICENSE gmt-mail-1.2.3-linux-amd64/README.md gmt-mail-1.2.3-linux-amd64/assets/ gmt-mail-1.2.3-linux-amd64/assets/logo-dark.svg gmt-mail-1.2.3-linux-amd64/assets/logo.svg gmt-mail-1.2.3-linux-amd64/gmt-mail gmt-mail-1.2.3-linux-amd64/gmt-mail.1" \
    "$(tar tzf gmt-mail-1.2.3-linux-amd64.tar.gz | paste -sd' ')"
check "zip has the logos" "gmt-mail-1.2.3-windows-amd64/assets/logo-dark.svg gmt-mail-1.2.3-windows-amd64/assets/logo.svg" \
    "$(unzip -Z1 gmt-mail-1.2.3-windows-amd64.zip | grep '\.svg$' | LC_ALL=C sort | paste -sd' ')"
check "zip has the .exe" "gmt-mail-1.2.3-windows-amd64/gmt-mail.exe" \
    "$(unzip -Z1 gmt-mail-1.2.3-windows-amd64.zip | grep '\.exe$')"

tar xzf gmt-mail-1.2.3-linux-amd64.tar.gz
check "armv6 binary" "ARM, EABI5" \
    "$(tar xzOf gmt-mail-1.2.3-linux-armv6.tar.gz gmt-mail-1.2.3-linux-armv6/gmt-mail | file -b - | grep -o 'ARM, EABI5')"
check "binary version" "1.2.3-abc1234 (2026-01-02)" "$(./gmt-mail-1.2.3-linux-amd64/gmt-mail -version)"
check "man page version" '.TH GMT\-MAIL 1 "2026-01-02" "gmt-mail 1.2.3" "User Commands"' \
    "$(grep '^\.TH' gmt-mail-1.2.3-linux-amd64/gmt-mail.1)"

"$DIST" 1.2.3 abc1234 2026-01-02 "$TMP/b" >/dev/null
check "reproducible" "$(cat "$TMP/a/SHA256SUMS")" "$(cat "$TMP/b/SHA256SUMS")"

(umask 077 && "$DIST" 1.2.3 abc1234 2026-01-02 "$TMP/c" >/dev/null)
check "reproducible under umask 077" "$(cat "$TMP/a/SHA256SUMS")" "$(cat "$TMP/c/SHA256SUMS")"
check "modes under umask 077" "-rw-r--r-- -rwxr-xr-x drwxr-xr-x" \
    "$(tar tvzf "$TMP/c/gmt-mail-1.2.3-linux-amd64.tar.gz" | awk '{print $1}' | LC_ALL=C sort -u | paste -sd' ')"

touch "$TMP/b/keep.txt"
"$DIST" 1.2.3 abc1234 2026-01-02 "$TMP/b" >/dev/null
check "rebuild keeps unrelated files" "yes" "$([ -e "$TMP/b/keep.txt" ] && echo yes || echo no)"
check "usage error" "1" "$("$DIST" 1.2.3 >/dev/null 2>&1; echo $?)"

exit "$fail"
