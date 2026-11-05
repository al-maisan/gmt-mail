#!/bin/bash
# Tests for the helpers in copr-build.sh: make test-copr
# No network: feeds releases.json-, Bodhi- and COPR-shaped JSON on stdin.
set -uo pipefail

# shellcheck source=copr-build.sh
source "$(dirname "$0")/copr-build.sh"
set +e # copr-build.sh sets -e; the checks below must keep going after a failure

fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}
flat() { paste -sd' '; }

# Mirrors 2026-10-31: 42 released but no longer maintained, 45 maintained but beta only.
RELEASES_JSON='[{"version": "45 Beta", "variant": "Workstation"}, {"version": "44", "variant": "Workstation"},
 {"version": "44", "variant": "Server"}, {"version": "43", "variant": "Workstation"}, {"version": "42", "variant": "KDE"}]'
BODHI='{"releases": [
 {"name": "EPEL-9", "id_prefix": "FEDORA-EPEL", "version": "9", "state": "current"},
 {"name": "F43", "id_prefix": "FEDORA", "version": "43", "state": "current"},
 {"name": "F43C", "id_prefix": "FEDORA-CONTAINER", "version": "43", "state": "current"},
 {"name": "F44", "id_prefix": "FEDORA", "version": "44", "state": "current"},
 {"name": "F44F", "id_prefix": "FEDORA-FLATPAK", "version": "44", "state": "current"},
 {"name": "F45", "id_prefix": "FEDORA", "version": "45", "state": "current"},
 {"name": "ELN", "id_prefix": "FEDORA", "version": "eln", "state": "pending"}]}'
BUILDS='{"items": [
 {"id": 6, "state": "failed",    "chroots": ["fedora-44-x86_64"], "source_package": {"version": "0.8.0-1"}},
 {"id": 5, "state": "succeeded", "chroots": ["fedora-43-x86_64"], "source_package": {"version": "0.8.0-1"}},
 {"id": 4, "state": "running",   "chroots": ["fedora-44-x86_64"], "source_package": {"version": "0.8.1-1"}},
 {"id": 3, "state": "succeeded", "chroots": ["fedora-44-x86_64"], "source_package": {"version": "0.8.0-10"}},
 {"id": 2, "state": "importing", "chroots": [], "source_package": null}]}'

RELEASED=$(released_versions <<<"$RELEASES_JSON")
check "released: final images only, no beta" "42 43 44" "$(flat <<<"$RELEASED")"
check "supported: released and maintained, every arch" \
    "fedora-43-x86_64 fedora-43-aarch64 fedora-44-x86_64 fedora-44-aarch64" \
    "$(supported_chroots "$RELEASED" "${ARCHES[@]}" <<<"$BODHI" | flat)"
check "arches: x86_64 and aarch64" "x86_64 aarch64" "${ARCHES[*]}"
TARGETS=$(supported_chroots "$RELEASED" x86_64 <<<"$BODHI")
check "supported: one arch" "fedora-43-x86_64 fedora-44-x86_64" "$(flat <<<"$TARGETS")"
check "supported: picks up 45 once released" "fedora-43-x86_64 fedora-44-x86_64 fedora-45-x86_64" \
    "$(supported_chroots "$(printf '43\n44\n45')" x86_64 <<<"$BODHI" | flat)"

check "builds of a version: failed ones too" "6 5" "$(nvr_builds 0.8.0-1 <<<"$BUILDS" | flat)"
check "builds: 0.8.0-1 does not match 0.8.0-10" "" "$(nvr_builds 0.8.0-1 <<<'{"items":[{"id":1,"state":"succeeded","source_package":{"version":"0.8.0-10"}}]}')"
check "builds: unknown version" "" "$(nvr_builds 1.0.0-1 <<<"$BUILDS")"
check "builds: fails on a non-JSON answer" no "$(nvr_builds 0.8.0-1 <<<'<html>' >/dev/null 2>&1 && echo yes || echo no)"
check "builds: fails on a build without id" no \
    "$(nvr_builds 0.8.0-1 <<<'{"items":[{"state":"failed","source_package":{"version":"0.8.0-1"}}]}' >/dev/null 2>&1 && echo yes || echo no)"
check "chroots: fails on a non-JSON answer" no "$(done_chroots <<<'' >/dev/null 2>&1 && echo yes || echo no)"
# A failed build: aarch64 failed, x86_64 succeeded; another still running.
CHROOT_JSON='{"items": [{"name": "fedora-44-x86_64", "state": "succeeded"}, {"name": "fedora-44-aarch64", "state": "failed"},
 {"name": "fedora-43-x86_64", "state": "running"}, {"name": "fedora-43-aarch64", "state": "canceled"}]}'
check "chroots: succeeded and running count, failed and canceled do not" "fedora-43-x86_64 fedora-44-x86_64" \
    "$(done_chroots <<<"$CHROOT_JSON" | sort | flat)"

check "todo: only the failed chroot of a failed build" "fedora-44-aarch64" \
    "$(missing "$(printf 'fedora-44-x86_64\nfedora-44-aarch64')" "$(done_chroots <<<"$CHROOT_JSON")" | flat)"
check "todo: nothing when all are built" "" "$(missing "$TARGETS" "$(printf 'fedora-43-x86_64\nfedora-44-x86_64\nfedora-42-x86_64')")"
check "todo: everything when nothing is built" "fedora-43-x86_64 fedora-44-x86_64" "$(missing "$TARGETS" "" | flat)"

check "project chroots" "fedora-43-x86_64" "$(project_chroots <<<'{"chroot_repos": {"fedora-43-x86_64": "https://x"}}')"
check "release from spec template" 1 "$(spec_release)"

# A dirty tree stops the build before any lookup or upload: run a copy of the
# script in a scratch repo with an untracked file.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
git init -q "$TMP/r" && mkdir "$TMP/r/scripts" && cp "$SRC_DIR/scripts/copr-build.sh" "$TMP/r/scripts/" && cp "$SRC_DIR/gmt-mail.spec" "$TMP/r/"
git -C "$TMP/r" add . && git -C "$TMP/r" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm init
touch "$TMP/r/.env"
check "dirty tree refused" "Error: working tree not clean; commit or stash first" "$("$TMP/r/scripts/copr-build.sh" 1.2.3 "$TMP" 2>&1)"

exit "$fail"
