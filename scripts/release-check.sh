#!/bin/bash
# Checks that TAG may be released; `make release` runs it before it builds or
# uploads anything.
# Usage: release-check.sh TAG
#
# Stops unless the tree is clean, HEAD is TAG, TAG is vMAJOR.MINOR.PATCH[-PRE],
# signed by the release key (RELEASE_KEY) and the same tag object on GitHub, and
# HEAD is on master of GitHub.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=release-key.sh
source scripts/release-key.sh

REMOTE=https://github.com/al-maisan/gmt-mail.git

die() {
    echo "release-check: $*" >&2
    exit 1
}

[ $# -eq 1 ] || die "usage: $0 TAG"
TAG=$1
[[ $TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || die "tag '$TAG' is not vMAJOR.MINOR.PATCH[-PRE]"
[ -z "$(git status --porcelain)" ] || die "working tree not clean"
[ "$(git rev-parse -q --verify "refs/tags/$TAG^{commit}")" == "$(git rev-parse HEAD)" ] \
    || die "HEAD is not $TAG; run: git checkout $TAG"
tag_signed "$TAG" || die "tag '$TAG' is not signed by $release_key"

# The tag on GitHub must be this very tag object, not just point to HEAD.
REFS=$(git ls-remote "$REMOTE" "refs/tags/$TAG" "refs/tags/$TAG^{}") || die "cannot list the tags of $REMOTE"
[ "$(awk -v r="refs/tags/$TAG" '$2 == r { print $1 }' <<<"$REFS")" == "$(git rev-parse "refs/tags/$TAG")" ] \
    || die "tag '$TAG' on GitHub is missing or is another tag object"

MASTER=$(git ls-remote "$REMOTE" refs/heads/master | cut -f1)
[ -n "$MASTER" ] || die "cannot read master of $REMOTE"
git cat-file -e "$MASTER^{commit}" 2>/dev/null || die "master of GitHub ($MASTER) is not local; run: git fetch origin"
git merge-base --is-ancestor HEAD "$MASTER" || die "HEAD is not on master of GitHub"
echo "release-check: $TAG ok"
