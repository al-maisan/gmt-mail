#!/bin/bash
# The release version in the download example of docs/install.md.
#   doc-version.sh set X.Y.Z    rewrites it; run before tagging vX.Y.Z
#   doc-version.sh check X.Y.Z  fails if it shows another version
# Only the example's "V=<version>"; history ("after v0.10.0") stays. check also
# fails if it finds no example at all, so a rewording can't drop the guard of
# make release.
set -euo pipefail
cd "${DOC_ROOT:-$(dirname "$0")/..}"

FILES=(docs/install.md)
VER='[0-9]+\.[0-9]+\.[0-9]+'
usage() {
    echo "usage: $0 set|check X.Y.Z" >&2
    exit 2
}
[ $# -eq 2 ] && [[ $1 == set || $1 == check ]] && [[ $2 =~ ^$VER$ ]] || usage
V=$2
for f in "${FILES[@]}"; do
    [ -f "$f" ] || { echo "doc-version: $f is missing" >&2; exit 1; }
done
case $1 in
set)
    sed -E -i "s/(^|[^A-Za-z0-9_])V=$VER([^0-9.]|\.?$)/\1V=$V\2/g" "${FILES[@]}"
    ;;
check)
    FOUND=$(grep -nEo "(^|[^A-Za-z0-9_])V=$VER([^0-9.]|\.?$)" "${FILES[@]}" || true)
    if [ -z "$FOUND" ]; then
        echo "doc-version: no download example in ${FILES[*]}; was it reworded?" >&2
        exit 1
    fi
    STALE=$(grep -vE "V=${V//./\\.}([^0-9.]|\.?$)" <<<"$FOUND" || true)
    if [ -n "$STALE" ]; then
        echo "doc-version: the download example shows another version than $V; run: make doc-version V=$V" >&2
        echo "$STALE" >&2
        exit 1
    fi
    ;;
esac
