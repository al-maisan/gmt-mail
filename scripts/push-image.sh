#!/bin/bash
# Push the container image of the stable release TAG to ghcr.io as TAG, and as
# latest if TAG is the highest stable tag on GitHub. The image is rebuilt from
# the linux archives of the GitHub release, checked against its SHA256SUMS
# signed by the release key, so this can be rerun at any time. `make release`
# runs it; by hand: `git checkout vX.Y.Z && make push-image VERSION=X.Y.Z`.
# Needs gh, gpg, skopeo, podman or docker, and a login to ghcr.io.
# Usage: push-image.sh TAG OUT
set -euo pipefail

die() {
    echo "push-image: $*" >&2
    exit 1
}

[ $# -eq 2 ] || die "usage: $0 TAG OUT"
TAG=$1
OUT=$(realpath -m -- "$2")
cd "$(dirname "$0")/.."
# shellcheck source=release-key.sh
source scripts/release-key.sh
# shellcheck source=release-lib.sh
source scripts/release-lib.sh

REPO=al-maisan/gmt-mail
IMAGE=ghcr.io/$REPO
[[ $TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "'$TAG' is not a stable tag vMAJOR.MINOR.PATCH"
[ -z "$(git status --porcelain)" ] || die "working tree not clean"
# The image's revision and timestamps come from HEAD.
[ "$(git rev-parse -q --verify "refs/tags/$TAG^{commit}")" == "$(git rev-parse HEAD)" ] ||
    die "HEAD is not $TAG; run: git checkout $TAG"
tag_signed "$TAG" || die "tag '$TAG' is not signed by $release_key"
LOGIN=$(ghcr_login) || exit 1

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
# Without an ambient GITHUB_TOKEN, as in the Makefile: a narrow-scoped one may fail.
env -u GITHUB_TOKEN gh release download "$TAG" --repo "$REPO" --dir "$TMP" --pattern SHA256SUMS \
    --pattern SHA256SUMS.asc --pattern "gmt-mail-${TAG#v}-linux-*.tar.gz" ||
    die "downloading the assets of release $TAG failed"
sums_signed "$TMP/SHA256SUMS" || die "SHA256SUMS of release $TAG is not signed by $release_key"
scripts/image.sh "$TAG" "$TMP" "$OUT"
DIGEST=$(oci_digest "$OUT") || exit 1

# latest never moves back, e.g. for a fix release of an older version.
TAGS=("$TAG")
HIGHEST=$(git ls-remote --tags --refs "https://github.com/$REPO.git" 'v*' | sed 's,.*refs/tags/,,' | highest_stable_tag) ||
    die "listing the tags of $REPO failed"
if [ "$HIGHEST" == "$TAG" ]; then
    TAGS+=(latest)
else
    echo "push-image: $TAG is not the highest stable tag ($HIGHEST), so latest stays"
fi
for t in "${TAGS[@]}"; do
    skopeo copy --quiet --all --preserve-digests --digestfile "$TMP/digest" "oci:$OUT" "docker://$IMAGE:$t" ||
        die "pushing $IMAGE:$t as $LOGIN failed"
    [ "$(cat "$TMP/digest")" == "$DIGEST" ] || die "$IMAGE:$t is $(cat "$TMP/digest"), but $OUT is $DIGEST"
    echo "push-image: pushed $IMAGE:$t, digest $DIGEST"
done
