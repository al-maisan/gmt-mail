#!/bin/bash
# Install the files of the deb package under DEST: the binary, the man page and
# the docs (install-docs.sh). Shared by ppa/debian/rules and scripts/deb.sh, so
# the PPA package and the release .deb hold the same files. Run from the source
# tree; build-ppa.sh copies this script and install-docs.sh into debian/.
# Usage: deb-layout.sh DEST BINARY VERSION MAN_DATE
set -euo pipefail

die() {
    echo "deb-layout: $*" >&2
    exit 1
}

[ $# -eq 4 ] || die "usage: $0 DEST BINARY VERSION MAN_DATE"
DEST=$1 BIN=$2 VERSION=$3 MAN_DATE=$4
[[ $DEST == /* ]] || die "DEST must be an absolute path"
[ -f "$BIN" ] || die "no binary '$BIN'"
[[ $VERSION =~ ^[0-9][0-9A-Za-z.+~]*$ ]] || die "invalid version '$VERSION'"
[[ $MAN_DATE =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || die "invalid man date '$MAN_DATE'"
umask 022

DOC=$DEST/usr/share/doc/gmt-mail
MAN=$DEST/usr/share/man/man1/gmt-mail.1
install -Dpm 0755 "$BIN" "$DEST/usr/bin/gmt-mail"
install -d "$(dirname "$MAN")"
sed -e "s/@VERSION@/$VERSION/g" -e "s/@DATE@/$MAN_DATE/g" gmt-mail.1 >"$MAN"
chmod 644 "$MAN"
if grep -n '@[A-Z]*@' "$MAN" >&2; then die "placeholder left in gmt-mail.1"; fi
bash "$(dirname "$0")/install-docs.sh" "$DOC"
