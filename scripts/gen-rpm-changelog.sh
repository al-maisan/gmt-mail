#!/bin/bash
# Generate RPM %changelog from GitHub releases.
# Each release produces one changelog entry with the commits since the next lower release.
# Usage: gen-rpm-changelog.sh [TAG]   (with TAG: only releases up to TAG's version)
set -eo pipefail

MAINTAINER="Muharem Hrnjadovic <muharem@linux.com>"

# rpm_escape: escape '%' on stdin as '%%'. rpm expands macros in %changelog, so
# a commit subject like "%(cmd)" would otherwise run cmd during the build.
rpm_escape() { sed 's/%/%%/g'; }

# newest_first [MAX]: read tags on stdin; print the stable ones (vMAJOR.MINOR.PATCH)
# up to MAX, highest version first. Release creation order would pair a
# maintenance release of an older minor (v0.10.1 after v0.11.0) with the wrong
# neighbour, and a v0.10.1 package must not list v0.11.0.
newest_first() {
    local t
    { grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' || true; } | sort -rV | while read -r t; do
        if [ -z "${1:-}" ] || [ "$(printf '%s\n' "$t" "$1" | sort -V | tail -1)" = "$1" ]; then echo "$t"; fi
    done
}

# rpm_date TAG: the date of TAG's commit as %changelog needs it ("Mon Nov 02
# 2026"). git localizes day and month names, which rpmbuild rejects, so use C.
rpm_date() { LC_ALL=C git log -1 --format='%ad' --date=format:'%a %b %d %Y' "$1"; }

# The functions above are sourced by gen-rpm-changelog_test.sh; stop here in that case.
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then return 0; fi

# List released tags newest first (only tags with actual GitHub releases).
# Drop any ambient GITHUB_TOKEN so gh uses the keyring login; a narrow-scoped
# token can make this return nothing and silently truncate the changelog.
# `|| true`: under set -e a failing gh (e.g. not logged in, as in CI) would exit here.
TAGS=($(env -u GITHUB_TOKEN gh release list --exclude-drafts --exclude-pre-releases --json tagName --jq '.[].tagName' 2>/dev/null | newest_first "${1:-}" || true))

if [ ${#TAGS[@]} -eq 0 ]; then
    # Fallback to git tags if gh is unavailable
    TAGS=($(git tag -l 'v*' | newest_first "${1:-}"))
fi

for i in "${!TAGS[@]}"; do
    TAG="${TAGS[$i]}"
    VER="${TAG#v}"

    DATE=$(rpm_date "$TAG")

    echo "* ${DATE} ${MAINTAINER} - ${VER}-1"

    # Get commits between this release and the next older one
    if [ $((i + 1)) -lt ${#TAGS[@]} ]; then
        PREV="${TAGS[$((i + 1))]}"
        git log --format="- %s" "${PREV}..${TAG}" --no-merges
    else
        git log --format="- %s" "${TAG}" --no-merges --max-count=10
    fi
    echo ""
done | rpm_escape
