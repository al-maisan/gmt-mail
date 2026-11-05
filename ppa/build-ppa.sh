#!/bin/bash
set -euo pipefail

# Configuration
PPA="ppa:al-maisan/gmt-mail"
GPG_KEY="753B6ECF2B458FF3D19D568C1E0A288397AE739E"
LP_USER="al-maisan"
LP_PPA="gmt-mail"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC_DIR="$(dirname "$SCRIPT_DIR")"
# Per-user build dir (a fixed /tmp path could be pre-created by another user)
BUILD_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/gmt-mail/ppa-build"

# Derive version from the latest git tag (the single source of truth)
TAG=$(git -C "$SRC_DIR" describe --tags --abbrev=0)
VERSION="${TAG#v}"

# Ubuntu releases to target (add/remove as needed)
RELEASES=("resolute")  # 26.04 LTS

# uploaded_versions VERSION: read Launchpad's getPublishedSources JSON on stdin and
# print the package versions uploaded for upstream VERSION (e.g. 0.8.0-1ppa1~resolute1).
uploaded_versions() {
    local v
    { grep -oP '"source_package_version":\s*"\K[^"]+' || true; } | while read -r v; do
        if [[ "$v" == "$1-1ppa"* ]]; then echo "$v"; fi
    done
}

# uploaded_for RELEASE: true if UPLOADED (newline-separated versions) has one for RELEASE.
uploaded_for() {
    grep -qE "^${VERSION//./\\.}-1ppa[0-9]+~$1[0-9]+$" <<<"$UPLOADED"
}

# next_rev: one above the highest ppa revision in UPLOADED, or 1.
next_rev() {
    local top
    top=$({ grep -oP "^${VERSION//./\\.}-1ppa\K[0-9]+" <<<"$UPLOADED" || true; } | sort -n | tail -1)
    echo $(( ${top:-0} + 1 ))
}

# prev_release TAG: read release tags on stdin; print the highest stable one
# below TAG, or nothing. Versions, not release creation order: v0.10.1 created
# after v0.11.0 still pairs with v0.10.0.
prev_release() {
    { grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' || true; echo "$1"; } | sort -uV | grep -B1 -xF -- "$1" | { grep -vxF -- "$1" || true; }
}

# lp_sources: print Launchpad's JSON list of this PPA's gmt-mail uploads.
# Anonymous API reads go through a cache that can be many minutes old and miss a
# fresh upload, which then looks absent; no-cache asks Launchpad itself.
lp_sources() {
    curl -fsS -H 'Cache-Control: no-cache' \
        "https://api.launchpad.net/1.0/~${LP_USER}/+archive/ubuntu/${LP_PPA}?ws.op=getPublishedSources&source_name=gmt-mail&exact_match=true"
}

# add_debian DIR: write the packaging into DIR/debian from HEAD (never the working
# tree, so no uncommitted or ignored file is uploaded) and fill in the tag's commit
# and date, as the release archives have them (Launchpad builds without .git).
add_debian() {
    local commit date
    commit=$(git -C "$SRC_DIR" rev-parse --short=7 "$TAG^{commit}") || return 1
    date=$(git -C "$SRC_DIR" log -1 --format=%cs "$TAG^{commit}") || return 1
    mkdir "$1/debian" || return 1
    git -C "$SRC_DIR" archive HEAD:ppa/debian | tar -xf - -C "$1/debian" || return 1
    git -C "$SRC_DIR" archive HEAD:scripts deb-layout.sh install-docs.sh | tar -xf - -C "$1/debian" || return 1
    sed -i -e "s/@COMMIT@/$commit/" -e "s/@BUILD_DATE@/$date/" "$1/debian/rules"
}

# The functions above are sourced by build-ppa_test.sh; stop here in that case.
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then return 0; fi

# Ask Launchpad what is already uploaded for this version. Fail loudly: guessing
# "nothing uploaded" would build a fresh orig that Launchpad then rejects.
LP_JSON=$(lp_sources) || { echo "Error: cannot query Launchpad for existing uploads" >&2; exit 1; }
UPLOADED=$(uploaded_versions "$VERSION" <<<"$LP_JSON")

# Without an explicit revision, skip releases that already have this version, so an
# unchanged tag is not re-published. Pass a revision to force a rebuild:
#   make ppa PPA_REV=2   (or: ppa/build-ppa.sh 2)
if [ -n "${1:-}" ]; then
    PPA_REV="$1"
    TODO=("${RELEASES[@]}")
else
    PPA_REV=$(next_rev)
    TODO=()
    for RELEASE in "${RELEASES[@]}"; do
        if uploaded_for "$RELEASE"; then
            echo "Skipping ${RELEASE}: ${VERSION} already uploaded as $(grep "~${RELEASE}" <<<"$UPLOADED" | paste -sd' ')"
        else
            TODO+=("$RELEASE")
        fi
    done
    if [ ${#TODO[@]} -eq 0 ]; then
        echo "Nothing to do. To force a rebuild: make ppa PPA_REV=${PPA_REV}"
        exit 0
    fi
fi
RELEASES=("${TODO[@]}")

cd "$SRC_DIR"
# The packaging comes from HEAD; a dirty tree means it is not what gets uploaded.
[ -z "$(git status --porcelain)" ] || { echo "Error: working tree not clean; commit or stash first" >&2; exit 1; }
# Verify the git tag exists
if ! git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "Error: git tag '$TAG' not found. Create it first:"
    echo "  git tag -s $TAG -m '$TAG'"
    echo "  git push origin $TAG"
    exit 1
fi

echo "=== Building gmt-mail ${VERSION} (ppa${PPA_REV}) from tag ${TAG} for: ${RELEASES[*]} ==="
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

ORIG="gmt-mail_${VERSION}.orig.tar.gz"
cd "$BUILD_DIR"

# Launchpad requires one immutable orig tarball per upstream version, so a rebuild
# reuses the published one. src-tarball.sh gives the same tarball for the same tag,
# so a first upload that missed an earlier one still matches it; origs of tags
# before src-tarball.sh (v0.11.0 and older) were not built that way.
if [ -n "$UPLOADED" ]; then
    echo "Reusing published orig tarball for ${VERSION} (ppa${PPA_REV} rebuild)"
    curl -fSL -o "$ORIG" \
        "https://launchpad.net/~${LP_USER}/+archive/ubuntu/${LP_PPA}/+files/${ORIG}"
    tar xzf "$ORIG"
else
    # First upload of this version: the orig from the tag, with the vendored
    # modules (not in git), so it is self-contained.
    "$SRC_DIR/scripts/src-tarball.sh" "$TAG" "$BUILD_DIR/$ORIG"
    tar xzf "$ORIG"
fi

SIGN_FLAG=""
if [ -n "$GPG_KEY" ]; then
    SIGN_FLAG="-k${GPG_KEY}"
fi

for RELEASE in "${RELEASES[@]}"; do
    FULL_VER="${VERSION}-1ppa${PPA_REV}~${RELEASE}1"
    echo ""
    echo "=== Building source package: ${FULL_VER} ==="
    cd "$BUILD_DIR"
    rm -rf "gmt-mail-${VERSION}-${RELEASE}"
    cp -a "gmt-mail-${VERSION}" "gmt-mail-${VERSION}-${RELEASE}"
    add_debian "gmt-mail-${VERSION}-${RELEASE}"
    cd "gmt-mail-${VERSION}-${RELEASE}"

    # Generate changelog from git log between previous tag and current tag
    # Find previous released tag (not just any tag). Drop any ambient
    # GITHUB_TOKEN so gh uses the keyring login (a narrow-scoped token would
    # make this empty and mis-scope the changelog range).
    PREV_TAG=$(env -u GITHUB_TOKEN gh release list --exclude-drafts --exclude-pre-releases --limit 100 --json tagName --jq '.[].tagName' 2>/dev/null \
        | prev_release "$TAG" || echo "")
    TIMESTAMP=$(date -R)
    {
        echo "gmt-mail (${FULL_VER}) ${RELEASE}; urgency=medium"
        echo ""
        if [ -n "$PREV_TAG" ]; then
            git -C "$SRC_DIR" log --format="  * %s" "${PREV_TAG}..${TAG}" --no-merges
        else
            git -C "$SRC_DIR" log --format="  * %s" "${TAG}" --no-merges --max-count=20
        fi
        echo ""
        echo " -- Muharem Hrnjadovic <muharem@linux.com>  ${TIMESTAMP}"
    } > debian/changelog

    # Build signed source package
    # Use -sd (debian-only) when the orig is already on Launchpad
    if [ -n "$UPLOADED" ]; then
        SA_FLAG="-sd"
    else
        SA_FLAG="-sa"
    fi
    debuild -S $SA_FLAG -d ${SIGN_FLAG}

    echo ""
    echo "=== Uploading to PPA for ${RELEASE} ==="
    cd "$BUILD_DIR"
    dput "$PPA" "gmt-mail_${FULL_VER}_source.changes"
done

echo ""
echo "=== Done ==="
echo "Check build status at: https://launchpad.net/~al-maisan/+archive/ubuntu/gmt-mail"
