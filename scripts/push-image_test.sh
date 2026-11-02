#!/bin/bash
# Tests for push-image.sh and release-lib.sh: make test-release
# No network: only the checks before the first download.
set -uo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
# shellcheck source=release-lib.sh
source "$SRC/scripts/release-lib.sh"
fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

highest() { printf '%s\n' "$@" | highest_stable_tag; }
check "highest stable tag" v1.2.0 "$(highest v1.1.0 v1.2.0 v1.1.5)"
check "backport is not the highest" v1.2.0 "$(highest v1.2.0 v1.1.5)"
check "versions sort as numbers" v1.10.0 "$(highest v1.9.0 v1.10.0 v1.2.0)"
check "prereleases do not count" v1.0.0 "$(highest v2.0.0-rc1 v1.0.0)"
check "other tags do not count" v1.0.0 "$(highest v1.0.0 v9 x1.2.3 v1.2.3.4)"
check "no stable tag" "" "$(highest v1.0.0-rc1)"

D=sha256:$(printf '%064d' 1)
mkdir "$TMP/one" "$TMP/two" "$TMP/pretty" "$TMP/none"
printf '{"manifests":[{"digest":"%s"}]}' "$D" >"$TMP/one/index.json"
printf '{"manifests":[{"digest":"%s"},{"digest":"%s"}]}' "$D" "$D" >"$TMP/two/index.json"
printf '{\n  "manifests": [\n    {\n      "digest": "%s"\n    }\n  ]\n}\n' "$D" >"$TMP/pretty/index.json"
check "digest of the layout" "$D" "$(oci_digest "$TMP/one")"
check "digest of a pretty index" "$D" "$(oci_digest "$TMP/pretty")"
check "rejects two images" no "$(oci_digest "$TMP/two" >/dev/null 2>&1 && echo yes || echo no)"
check "rejects no index" no "$(oci_digest "$TMP/none" >/dev/null 2>&1 && echo yes || echo no)"

pushes() { if "$SRC/scripts/push-image.sh" "$@" >/dev/null 2>&1; then echo yes; else echo no; fi; }
check "push: needs a tag and an output" no "$(pushes v1.2.3)"
check "push: rejects a prerelease" no "$(pushes v1.2.3-rc1 "$TMP/img")"
check "push: rejects a bad tag" no "$(pushes 'v1.2;x' "$TMP/img")"
check "push: says why" "push-image: 'v1.2.3-rc1' is not a stable tag vMAJOR.MINOR.PATCH" \
    "$("$SRC/scripts/push-image.sh" v1.2.3-rc1 "$TMP/img" 2>&1)"

exit "$fail"
