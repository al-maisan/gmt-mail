#!/bin/bash
# Write the source tarball of the PPA and RPM packages: the tree of a stable release tag
# plus the vendored Go modules (Launchpad builds without network), as
# gmt-mail-VERSION/. The same tag gives the same tarball byte for byte: fixed
# file order, owner, modes and times (the tag's commit time), and no gzip
# timestamp. So a rebuild that regenerates it matches Launchpad's copy.
# Usage: src-tarball.sh TAG OUT.tar.gz   (called by ppa/build-ppa.sh and make srpm)
set -euo pipefail

die() {
    echo "src-tarball: $*" >&2
    exit 1
}

[ $# -eq 2 ] || die "usage: $0 TAG OUT.tar.gz"
TAG=$1
OUT=$(realpath -m -- "$2")
cd "$(dirname "$0")/.."
[[ $TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "tag '$TAG' is not vMAJOR.MINOR.PATCH"
git rev-parse -q --verify "refs/tags/$TAG^{commit}" >/dev/null || die "tag '$TAG' not found"
[[ $OUT == *.tar.gz && -d $(dirname "$OUT") ]] || die "'$OUT' is not a .tar.gz in an existing directory"

NAME=gmt-mail-${TAG#v}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
git archive --prefix="$NAME/" "$TAG" | tar -xf - -C "$TMP"
# go.sum pins every module, so the vendored tree is the same on every run.
(cd "$TMP/$NAME" && { GOFLAGS=-mod=mod go mod vendor 2>/dev/null || GOFLAGS=-mod=mod GOTOOLCHAIN=local go mod vendor; })
tar --sort=name --owner=0 --group=0 --numeric-owner --mode=go-w,a+rX \
    --mtime="@$(git log -1 --format=%ct "$TAG^{commit}")" -C "$TMP" -cf - "$NAME" | gzip -9n >"$TMP/out.tar.gz"
mv "$TMP/out.tar.gz" "$OUT"
echo "src-tarball: $OUT"
