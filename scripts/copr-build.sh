#!/bin/bash
# Build the tagged version in Fedora COPR for every supported Fedora release that
# does not have it yet.
# Usage: copr-build.sh VERSION SRPM_DIR   (called by `make copr`)
#
# Supported releases = released (final images listed in fedoraproject.org/releases.json)
# and still maintained (state "current" in Fedora's Bodhi, which also marks betas as
# current), for every arch in ARCHES. New releases are picked up automatically;
# COPR_CHROOTS="fedora-44-x86_64 ..." overrides the list.
# A chroot counts as done if it succeeded or is still in progress in a build of this
# version; failed/canceled chroots do not, so a retry builds only those.
# COPR_FORCE=1 builds for all target chroots regardless.
set -euo pipefail

OWNER="al-maisan"
PROJECT="gmt-mail"
PACKAGE="gmt-mail"
ARCHES=(x86_64 aarch64)
SRC_DIR="$(cd "$(dirname "$0")/.." && pwd)"
COPR_API="https://copr.fedorainfracloud.org/api_3"

# released_versions: read fedoraproject.org/releases.json on stdin, print the Fedora
# versions that have final (non-beta) images, one per line.
released_versions() {
    python3 -I -c '
import json, sys
print("\n".join(sorted({r["version"] for r in json.load(sys.stdin) if str(r.get("version", "")).isdigit()}, key=int)))'
}

# supported_chroots RELEASED ARCH...: read Bodhi's releases JSON on stdin, print
# fedora-N-ARCH for every current Fedora release that is also in RELEASED
# (newline-separated versions) and every ARCH.
# EPEL, ELN, container and flatpak releases are skipped.
supported_chroots() {
    python3 -I -c '
import json, sys
released = set(sys.argv[1].split())
vers = {r["version"] for r in json.load(sys.stdin).get("releases", [])
        if r.get("id_prefix") == "FEDORA" and r.get("state") == "current" and r.get("version") in released}
for v in sorted(vers, key=int):
    for a in sys.argv[2:]:
        print(f"fedora-{v}-{a}")
' "$@"
}

# nvr_builds NVR: read COPR's build/list JSON on stdin, print the ids of the builds
# of NVR (e.g. 0.8.0-1), whatever their state: a failed build can hold chroots that
# succeeded.
nvr_builds() {
    python3 -I -c '
import json, sys
for b in json.load(sys.stdin).get("items", []):
    if (b.get("source_package") or {}).get("version") == sys.argv[1]:
        print(b["id"])
' "$1"
}

# done_chroots: read COPR's build-chroot/list JSON of one build on stdin, print the
# chroots that succeeded or are still in progress. A build fails as a whole if one
# chroot fails, so a retry must look at each chroot.
done_chroots() {
    python3 -I -c '
import json, sys
for c in json.load(sys.stdin).get("items", []):
    if c.get("state") not in ("failed", "canceled"):
        print(c["name"])
'
}

# project_chroots: read COPR's project JSON on stdin, print the chroots enabled in it.
project_chroots() {
    python3 -I -c 'import json, sys; print("\n".join(sorted(json.load(sys.stdin).get("chroot_repos", {}))))'
}

# missing WANTED HAVE: lines of WANTED (newline-separated) that are not in HAVE.
missing() {
    comm -23 <(sort -u <<<"$1" | sed '/^$/d') <(sort -u <<<"$2" | sed '/^$/d')
}

# spec_release: the Release number from the spec template (Release: 1%{?dist} -> 1).
spec_release() {
    awk '/^Release:/ { sub(/%.*/, "", $2); print $2; exit }' "$SRC_DIR/gmt-mail.spec"
}

# The functions above are sourced by copr-build_test.sh; stop here in that case.
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then return 0; fi

VERSION="${1:?usage: copr-build.sh VERSION SRPM_DIR}"
SRPM_DIR="${2:?usage: copr-build.sh VERSION SRPM_DIR}"
NVR="${VERSION}-$(spec_release)"
# The spec comes from the working tree; a dirty tree means it is not what is committed.
[ -z "$(git -C "$SRC_DIR" status --porcelain)" ] || { echo "Error: working tree not clean; commit or stash first" >&2; exit 1; }

# Fail loudly on any lookup error: guessing would build for the wrong releases or
# submit duplicate builds.
fetch() { curl -fsS "$1" || { echo "Error: cannot fetch $1" >&2; exit 1; }; }

if [ -n "${COPR_CHROOTS:-}" ]; then
    TARGETS=$(tr ' ' '\n' <<<"$COPR_CHROOTS" | sed '/^$/d')
else
    RELEASES_JSON=$(fetch "https://fedoraproject.org/releases.json")
    BODHI=$(fetch "https://bodhi.fedoraproject.org/releases/?state=current&rows_per_page=100")
    TARGETS=$(supported_chroots "$(released_versions <<<"$RELEASES_JSON")" "${ARCHES[@]}" <<<"$BODHI")
    [ -n "$TARGETS" ] || { echo "Error: found no released and maintained Fedora release" >&2; exit 1; }
fi

if [ -n "${COPR_FORCE:-}" ]; then
    TODO="$TARGETS"
else
    BUILDS=$(fetch "${COPR_API}/build/list?ownername=${OWNER}&projectname=${PROJECT}&packagename=${PACKAGE}&limit=100&order=id&order_type=DESC")
    # An assignment, not the for list: set -e ignores a failed $(...) there, and an
    # empty list would rebuild every chroot.
    IDS=$(nvr_builds "$NVR" <<<"$BUILDS")
    DONE=
    for ID in $IDS; do
        DONE+=$(done_chroots <<<"$(fetch "${COPR_API}/build-chroot/list?build_id=${ID}")")$'\n'
    done
    TODO=$(missing "$TARGETS" "$DONE")
    if [ -z "$TODO" ]; then
        echo "COPR already has ${PACKAGE}-${NVR} for: $(paste -sd' ' <<<"$TARGETS"). Nothing to do."
        echo "To force a rebuild: make copr COPR_FORCE=1"
        exit 0
    fi
fi
echo "Building ${PACKAGE}-${NVR} for: $(paste -sd' ' <<<"$TODO")"

# COPR only builds for chroots enabled in the project; enable new ones, keep the rest.
PROJECT_JSON=$(fetch "${COPR_API}/project?ownername=${OWNER}&projectname=${PROJECT}")
ENABLED=$(project_chroots <<<"$PROJECT_JSON")
NEW=$(missing "$TODO" "$ENABLED")
if [ -n "$NEW" ]; then
    echo "Enabling in COPR project ${OWNER}/${PROJECT}: $(paste -sd' ' <<<"$NEW")"
    mapfile -t ALL < <(printf '%s\n%s\n' "$ENABLED" "$NEW" | sed '/^$/d' | sort -u)
    copr-cli modify "$PROJECT" "${ALL[@]/#/--chroot=}"
fi

"${MAKE:-make}" -C "$SRC_DIR" srpm
mapfile -t CHROOTS <<<"$TODO"
copr-cli build "$PACKAGE" "${CHROOTS[@]/#/--chroot=}" "$(ls "$SRPM_DIR"/gmt-mail-"$VERSION"-*.src.rpm)"
