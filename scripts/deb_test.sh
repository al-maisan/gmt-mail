#!/bin/bash
# Tests for deb.sh and deb-layout.sh: make test-dist
# Needs dpkg-deb; no network.
set -uo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}
builds() { if "$SRC/scripts/deb.sh" "$@" >/dev/null 2>&1; then echo yes; else echo no; fi; }

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
printf '#!/bin/sh\n' >"$TMP/bin"
MTIME=1767225600 # 2026-01-01
for d in a b; do
    "$SRC/scripts/deb.sh" "$TMP/bin" 1.2.3 arm64 "$MTIME" "$TMP/$d.deb" >/dev/null || { echo "FAIL - build"; exit 1; }
done
check "same inputs, same .deb" "$(sha256sum <"$TMP/a.deb")" "$(sha256sum <"$TMP/b.deb")"

f() { dpkg-deb -f "$TMP/a.deb" "$1"; }
check "version" "1.2.3-1" "$(f Version)"
check "arch" "arm64" "$(f Architecture)"
check "no depends: a static binary" "" "$(f Depends)"
check "fields from ppa/debian/control" "$(grep -E '^(Maintainer|Section|Homepage):' "$SRC/ppa/debian/control" | sort)" \
    "$(dpkg-deb -f "$TMP/a.deb" Maintainer Section Homepage | sort)"
check "description from ppa/debian/control" "$(sed -n '/^Description:/,$p' "$SRC/ppa/debian/control" | sed 's/^Description: //')" "$(f Description)"

LIST=$(dpkg-deb -c "$TMP/a.deb" | awk '{ print $2, $6 }')
check "owned by root" "root/root" "$(awk '{ print $1 }' <<<"$LIST" | sort -u)"
check "mtime" "$(date -u -d "@$MTIME" '+%Y-%m-%d %H:%M')" "$(TZ=UTC dpkg-deb -c "$TMP/a.deb" | awk '{ print $4, $5 }' | sort -u)"
check "files" "./usr/bin/gmt-mail
./usr/share/doc/gmt-mail/.env.example
./usr/share/doc/gmt-mail/README.md
./usr/share/doc/gmt-mail/assets/logo-dark.svg
./usr/share/doc/gmt-mail/assets/logo.svg
./usr/share/doc/gmt-mail/changelog.Debian.gz
./usr/share/doc/gmt-mail/copyright
./usr/share/lintian/overrides/gmt-mail
./usr/share/man/man1/gmt-mail.1.gz" "$(dpkg-deb -c "$TMP/a.deb" | awk '$1 !~ /^d/ { print $6 }' | LC_ALL=C sort)"

dpkg-deb -R "$TMP/a.deb" "$TMP/x"
check "binary mode" "755" "$(stat -c %a "$TMP/x/usr/bin/gmt-mail")"
check "md5sums match" "" "$(cd "$TMP/x" && md5sum --quiet -c DEBIAN/md5sums 2>&1)"
check "md5sums list every file" "$(cd "$TMP/x" && find . -path ./DEBIAN -prune -o -type f -printf '%P\n' | LC_ALL=C sort)" \
    "$(awk '{ print $2 }' "$TMP/x/DEBIAN/md5sums")"
check "man page filled in" '.TH GMT\-MAIL 1 "2026-01-01" "gmt-mail 1.2.3" "User Commands"' \
    "$(zcat "$TMP/x/usr/share/man/man1/gmt-mail.1.gz" | grep '^\.TH')"
check "changelog version" "gmt-mail (1.2.3-1) stable; urgency=medium" "$(zcat "$TMP/x/usr/share/doc/gmt-mail/changelog.Debian.gz" | head -1)"
# shellcheck disable=SC2012 # plain names
check "no maintainer scripts" "control md5sums" "$(ls "$TMP/x/DEBIAN" | paste -sd' ')"

check "rejects prerelease version" no "$(builds "$TMP/bin" 1.2.3-rc1 arm64 "$MTIME" "$TMP/c.deb")"
check "rejects unknown arch" no "$(builds "$TMP/bin" 1.2.3 i386 "$MTIME" "$TMP/c.deb")"
check "rejects missing binary" no "$(builds "$TMP/nope" 1.2.3 arm64 "$MTIME" "$TMP/c.deb")"
check "rejects bad mtime" no "$(builds "$TMP/bin" 1.2.3 arm64 1e9 "$TMP/c.deb")"
check "rejects non-.deb output" no "$(builds "$TMP/bin" 1.2.3 arm64 "$MTIME" "$TMP/c.tar")"
check "layout rejects relative dest" no \
    "$(cd "$SRC" && scripts/deb-layout.sh rel "$TMP/bin" 1.2.3 2026-01-01 >/dev/null 2>&1 && echo yes || echo no)"
check "layout rejects bad man date" no \
    "$(cd "$SRC" && scripts/deb-layout.sh "$TMP/y" "$TMP/bin" 1.2.3 2026-1-1 >/dev/null 2>&1 && echo yes || echo no)"

exit "$fail"
