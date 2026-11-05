#!/bin/bash
# Checks that TAG may be released; `make release` runs it before it builds or
# uploads anything.
# Usage: release-check.sh [pre|post] TAG
#
# pre (before the tag is pushed): stops unless TAG is vMAJOR.MINOR.PATCH[-PRE],
# the tree is clean, HEAD is TAG and TAG is signed by the release key
# (RELEASE_KEY). A stable TAG also needs the download example of
# docs/install.md at its version (doc-version.sh).
# post (after the push): stops unless TAG on GitHub is the same tag object and
# HEAD is on master of GitHub.
# Without a phase, both run.
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck source=release-key.sh
source scripts/release-key.sh

REMOTE=https://github.com/al-maisan/gmt-mail.git

die() {
    echo "release-check: $*" >&2
    exit 1
}

PHASE=all
if [ $# -eq 2 ]; then PHASE=$1; shift; fi
[ $# -eq 1 ] && [[ $PHASE =~ ^(all|pre|post)$ ]] || die "usage: $0 [pre|post] TAG"
TAG=$1
[[ $TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] || die "tag '$TAG' is not vMAJOR.MINOR.PATCH[-PRE]"

if [ "$PHASE" != post ]; then
    [ -z "$(git status --porcelain)" ] || die "working tree not clean"
    [ "$(git rev-parse -q --verify "refs/tags/$TAG^{commit}")" == "$(git rev-parse HEAD)" ] \
        || die "HEAD is not $TAG; run: git checkout $TAG"
    tag_signed "$TAG" || die "tag '$TAG' is not signed by $release_key"
    # The download example of docs/install.md must show this version.
    if [[ $TAG != *-* ]]; then scripts/doc-version.sh check "${TAG#v}" || die "docs/install.md not at $TAG"; fi
fi
[ "$PHASE" != pre ] || { echo "release-check: $TAG ok before the push"; exit 0; }

# The tag on GitHub must be this very tag object, not just point to HEAD.
REFS=$(git ls-remote "$REMOTE" "refs/tags/$TAG" "refs/tags/$TAG^{}") || die "cannot list the tags of $REMOTE"
[ "$(awk -v r="refs/tags/$TAG" '$2 == r { print $1 }' <<<"$REFS")" == "$(git rev-parse "refs/tags/$TAG")" ] \
    || die "tag '$TAG' on GitHub is missing or is another tag object"

HEADS=$(git ls-remote "$REMOTE" refs/heads/master) || die "cannot read master of $REMOTE"
MASTER=$(cut -f1 <<<"$HEADS")
[ -n "$MASTER" ] || die "no master on $REMOTE"
git cat-file -e "$MASTER^{commit}" 2>/dev/null || die "master of GitHub ($MASTER) is not local; run: git fetch origin"
git merge-base --is-ancestor HEAD "$MASTER" || die "HEAD is not on master of GitHub"
echo "release-check: $TAG ok"
