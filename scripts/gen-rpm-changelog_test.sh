#!/bin/bash
# Tests for the helpers in gen-rpm-changelog.sh: make test-copr
set -uo pipefail

# shellcheck source=gen-rpm-changelog.sh
source "$(dirname "$0")/gen-rpm-changelog.sh"
set +e # gen-rpm-changelog.sh sets -e; the checks below must keep going after a failure

fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}

check "escape: plain text unchanged" "- Fix bug (#1)" "$(rpm_escape <<<'- Fix bug (#1)')"
check "escape: percent doubled" "- 100%% done" "$(rpm_escape <<<'- 100% done')"
check "escape: macros neutralised" "- %%(id) %%{version}" "$(rpm_escape <<<'- %(id) %{version}')"

# Release creation order: v0.10.1 came after v0.11.0; prereleases and other tags are dropped.
check "newest first by version" "v0.11.0 v0.10.1 v0.10.0 v0.9.0" "$(printf '%s\n' v0.10.1 v0.11.0 v0.9.0 stable v0.12.0-rc1 v0.10.0 | newest_first | paste -sd' ')"
check "newest first: no tags" "" "$(newest_first </dev/null)"
check "newest first up to v0.10.1" "v0.10.1 v0.10.0 v0.9.0" "$(printf '%s\n' v0.10.1 v0.11.0 v0.9.0 v0.10.0 | newest_first v0.10.1 | paste -sd' ')"
check "newest first up to an unreleased v0.12.0" "v0.11.0 v0.10.1" "$(printf '%s\n' v0.10.1 v0.11.0 | newest_first v0.12.0 | paste -sd' ')"

# %changelog needs English names in any locale (de_DE would give "Mo Nov 02 2026").
TAG=$(git tag -l 'v*' --sort=-version:refname | grep -vF -- - | head -1)
check "date: English under de_DE" yes "$(LC_ALL=de_DE.UTF-8 rpm_date "$TAG" 2>/dev/null | grep -qxE '(Mon|Tue|Wed|Thu|Fri|Sat|Sun) (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) [0-9]{2} [0-9]{4}' && echo yes || echo no)"

# With rpm installed, check that an escaped subject renders literally and runs nothing.
if command -v rpmspec >/dev/null; then
    TMP=$(mktemp -d)
    trap 'rm -rf "$TMP"' EXIT
    {
        printf 'Name: x\nVersion: 1\nRelease: 1\nSummary: x\nLicense: MIT\n%%description\nx\n%%changelog\n'
        printf '* Mon Jan 05 2026 A <a@b.c> - 1-1\n- Fix %%(touch %s/pwned) at 100%%\n' "$TMP" | rpm_escape
    } >"$TMP/x.spec"
    check "rpmspec: renders literally" "- Fix %(touch $TMP/pwned) at 100%" "$(rpmspec -P "$TMP/x.spec" 2>/dev/null | tail -1)"
    check "rpmspec: command not run" "no" "$([ -e "$TMP/pwned" ] && echo yes || echo no)"
fi

exit "$fail"
