#!/bin/bash
# Build the release .deb from a release binary: the PPA package's files
# (deb-layout.sh) plus what debhelper adds there (copyright, changelog,
# lintian overrides, md5sums). Called by build-dist.sh. The same inputs give
# the same .deb. Needs dpkg-deb.
# Usage: deb.sh BINARY VERSION ARCH MTIME OUT.deb
#   (VERSION is MAJOR.MINOR.PATCH, ARCH amd64, arm64 or armhf, MTIME the file
#   time in seconds since the epoch)
set -euo pipefail

die() {
    echo "deb: $*" >&2
    exit 1
}

[ $# -eq 5 ] || die "usage: $0 BINARY VERSION ARCH MTIME OUT.deb"
# Relative to the caller's directory, before the cd.
BIN=$(realpath -e -- "$1") || die "no binary '$1'"
OUT=$(realpath -m -- "$5")
VERSION=$2 ARCH=$3 MTIME=$4
cd "$(dirname "$0")/.."
[[ $VERSION =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "version '$VERSION' is not MAJOR.MINOR.PATCH"
[[ $ARCH =~ ^(amd64|arm64|armhf)$ ]] || die "unsupported arch '$ARCH'"
[[ $MTIME =~ ^[0-9]+$ ]] || die "invalid mtime '$MTIME'"
[[ $OUT == *.deb && -d $(dirname "$OUT") ]] || die "'$OUT' is not a .deb in an existing directory"
command -v dpkg-deb >/dev/null || die "dpkg-deb not found (Fedora: sudo dnf install dpkg)"

# field NAME: print field NAME of the first paragraph in ppa/debian/control that
# has it, with its continuation lines.
field() {
    awk -v f="$1:" '$1 == f { p = 1; print; next } p && /^[ \t]/ { print; next } p { exit }' ppa/debian/control
}
MAINTAINER=$(field Maintainer)
[ -n "$MAINTAINER" ] || die "no Maintainer in ppa/debian/control"

umask 022
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
ROOT=$TMP/root
DOC=$ROOT/usr/share/doc/gmt-mail
scripts/deb-layout.sh "$ROOT" "$BIN" "$VERSION" "$(date -u -d "@$MTIME" +%Y-%m-%d)"
gzip -9n "$ROOT/usr/share/man/man1/gmt-mail.1"
install -pm 0644 ppa/debian/copyright "$DOC/copyright"
install -d "$ROOT/usr/share/lintian/overrides"
{
    echo "# Go with CGO_ENABLED=0: no C libraries are linked, so none can miss a security update."
    echo "gmt-mail: statically-linked-binary [usr/bin/gmt-mail]"
    echo "# The binary of the release archives, so its checksum matches theirs."
    echo "gmt-mail: unstripped-binary-or-object [usr/bin/gmt-mail]"
    echo "# Not a Debian upload, so there is no ITP bug to close."
    echo "gmt-mail: initial-upload-closes-no-bugs"
} >"$ROOT/usr/share/lintian/overrides/gmt-mail"
chmod 644 "$ROOT/usr/share/lintian/overrides/gmt-mail"
{
    echo "gmt-mail ($VERSION-1) stable; urgency=medium"
    echo
    echo "  * Release v$VERSION, see"
    echo "    https://github.com/al-maisan/gmt-mail/releases/tag/v$VERSION"
    echo
    echo " -- ${MAINTAINER#Maintainer: }  $(LC_ALL=C date -u -R -d "@$MTIME")"
} | gzip -9n >"$DOC/changelog.Debian.gz"
chmod 644 "$DOC/changelog.Debian.gz"

install -d "$ROOT/DEBIAN"
(cd "$ROOT" && find . -path ./DEBIAN -prune -o -type f -printf '%P\0' | LC_ALL=C sort -z | xargs -0 md5sum) >"$ROOT/DEBIAN/md5sums"
# As dpkg-gencontrol counts it: KiB per file, rounded up, plus 1 per directory.
SIZE=$(find "$ROOT" -path "$ROOT/DEBIAN" -prune -o -mindepth 1 \( -type d -printf '1\n' -o -printf '%s\n' \) |
    awk '{ n += $1 == 1 ? 1 : int(($1 + 1023) / 1024) } END { print n }')
{
    echo "Package: gmt-mail"
    echo "Version: $VERSION-1"
    echo "Architecture: $ARCH"
    echo "$MAINTAINER"
    echo "Installed-Size: $SIZE"
    field Section
    echo "Priority: optional"
    field Homepage
    field Description
} >"$ROOT/DEBIAN/control"
chmod 644 "$ROOT/DEBIAN/control" "$ROOT/DEBIAN/md5sums"
grep -qx 'Section: [a-z]*' "$ROOT/DEBIAN/control" || die "no Section in ppa/debian/control"
grep -q '^Description: .' "$ROOT/DEBIAN/control" || die "no Description in ppa/debian/control"

find "$ROOT" -exec touch -h -d "@$MTIME" {} +
SOURCE_DATE_EPOCH=$MTIME dpkg-deb --root-owner-group -Zxz --threads-max=1 --build "$ROOT" "$TMP/out.deb" >/dev/null
mv "$TMP/out.deb" "$OUT"
echo "deb: $OUT"
