#!/bin/bash
# Install the docs every package ships under DIR: README.md and every file its
# relative links reach, so the links work in the installed copy too.
# Used by build-dist.sh (archives), deb-layout.sh (PPA and release .deb) and
# gmt-mail.spec (RPM); build-ppa.sh copies it next to deb-layout.sh.
# LICENSE is not listed: each package ships it its own way.
# Usage: install-docs.sh DIR    (DIR absolute)
#        install-docs.sh --list
set -euo pipefail

DOCS=(
    README.md
    .env.example
    CONTRIBUTING.md
    SECURITY.md
    assets/logo.svg
    assets/logo-dark.svg
    docs/install.md
    docs/usage.md
    docs/comparison.md
    docs/demo/demo.gif
    docs/demo/config.toml
    docs/demo/template.eml
)

die() {
    echo "install-docs: $*" >&2
    exit 1
}

[ $# -eq 1 ] || die "usage: $0 DIR | --list"
if [ "$1" == --list ]; then
    printf '%s\n' "${DOCS[@]}"
    exit 0
fi
DIR=$1
[[ $DIR == /* ]] || die "DIR must be an absolute path"
# The source tree: this script is in scripts/ (or debian/ for the PPA).
cd "$(dirname "$0")/.."
for f in "${DOCS[@]}"; do
    [ -f "$f" ] || die "no '$f'"
    install -Dpm 0644 "$f" "$DIR/$f"
done
