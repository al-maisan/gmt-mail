#!/bin/bash
# Tests for install-docs.sh: make test-dist
# Every relative link and image in the installed .md files must reach an
# installed file, so the docs work in every package.
set -uo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

"$SRC/scripts/install-docs.sh" "$TMP/d" || { echo "FAIL - install-docs.sh exited $?"; exit 1; }
check "installs the list" "$("$SRC/scripts/install-docs.sh" --list | LC_ALL=C sort)" \
    "$(cd "$TMP/d" && find . -type f -printf '%P\n' | LC_ALL=C sort)"
check "modes" "644" "$(find "$TMP/d" -type f -printf '%m\n' | sort -u)"

# dead_links DIR: print FILE: TARGET for each relative link or image in the .md
# files under DIR that reaches no file or directory there (LICENSE excepted).
dead_links() {
    local md t path
    while IFS= read -r md; do
        grep -oE '\]\([^)]+\)|src(set)?="[^"]+"' "$md" | sed -E 's/^\]\(//; s/\)$//; s/^src(set)?="//; s/"$//' |
            while IFS= read -r t; do
                path=${t%%#*}
                [[ -z $path || $path == *://* || $path == mailto:* ]] && continue
                path=$(realpath -m "$(dirname "$md")/$path")
                [[ $path == "$1"/* && -e $path ]] || [[ $path == "$1/LICENSE" ]] || echo "${md#"$1"/}: $t"
            done
    done < <(find "$1" -name '*.md' | LC_ALL=C sort)
}
check "no dead links in the installed docs" "" "$(dead_links "$TMP/d")"
mkdir "$TMP/e"
printf '[a](gone.md) [b](#x) [c](https://x.org/y) [d](LICENSE) <img src="nope.svg">\n' >"$TMP/e/README.md"
check "finds dead links and images" "README.md: gone.md
README.md: nope.svg" "$(dead_links "$TMP/e")"

check "rejects a relative DIR" no "$("$SRC/scripts/install-docs.sh" rel >/dev/null 2>&1 && echo yes || echo no)"
check "needs DIR" no "$("$SRC/scripts/install-docs.sh" >/dev/null 2>&1 && echo yes || echo no)"

exit "$fail"
