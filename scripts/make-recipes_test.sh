#!/bin/bash
# Syntax check of the Makefile's release recipes: make test-release
# .SHELLFLAGS=-nc makes the shell only parse each recipe line (sh -n), so
# nothing runs: no tag, build, upload or push. GH=true keeps the gh calls in
# variables offline.
set -uo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
fail=0
for v in 9.9.9 9.9.9-rc1; do
    for t in tag dist image push-image release publish; do
        if err=$(make -C "$SRC" -s --no-print-directory "$t" VERSION="$v" GH=true .SHELLFLAGS=-nc 2>&1 >/dev/null); then
            echo "ok   - make $t VERSION=$v parses"
        else
            echo "FAIL - make $t VERSION=$v: $err"
            fail=1
        fi
    done
done
exit "$fail"
