#!/bin/bash
# Generate RPM %changelog from GitHub releases.
# Each release produces one changelog entry with commits since the previous release.
set -eo pipefail

MAINTAINER="Muharem Hrnjadovic <muharem@linux.com>"

# rpm_escape: escape '%' on stdin as '%%'. rpm expands macros in %changelog, so
# a commit subject like "%(cmd)" would otherwise run cmd during the build.
rpm_escape() { sed 's/%/%%/g'; }

# The function above is sourced by gen-rpm-changelog_test.sh; stop here in that case.
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then return 0; fi

# List released tags newest first (only tags with actual GitHub releases).
# Drop any ambient GITHUB_TOKEN so gh uses the keyring login; a narrow-scoped
# token can make this return nothing and silently truncate the changelog.
# `|| true`: under set -e a failing gh (e.g. not logged in, as in CI) would exit here.
TAGS=($(env -u GITHUB_TOKEN gh release list --exclude-drafts --exclude-pre-releases --json tagName --jq '.[].tagName' 2>/dev/null || true))

if [ ${#TAGS[@]} -eq 0 ]; then
    # Fallback to git tags if gh is unavailable
    TAGS=($(git tag -l 'v*' --sort=-version:refname | grep -vF -- - || true))
fi

for i in "${!TAGS[@]}"; do
    TAG="${TAGS[$i]}"
    VER="${TAG#v}"

    # RPM changelog date format: "Day Mon DD YYYY"
    DATE=$(git log -1 --format='%ad' --date=format:'%a %b %d %Y' "$TAG")

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
