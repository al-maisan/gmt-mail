# Sourced by the image scripts: helpers for the container image. On failure
# they say why on stderr and return 1.
# shellcheck shell=bash

# highest_stable_tag: read tag names on stdin; print the highest stable one
# (vMAJOR.MINOR.PATCH), or nothing.
highest_stable_tag() {
    { grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' || true; } | sort -V | tail -n 1
}

# ghcr_login: print the user logged in to ghcr.io.
ghcr_login() {
    if ! command -v skopeo >/dev/null; then
        echo "skopeo not found (Fedora: sudo dnf install skopeo)" >&2
        return 1
    fi
    if ! skopeo login --get-login ghcr.io 2>/dev/null; then
        echo "not logged in to ghcr.io; run 'podman login ghcr.io' with a GitHub token (classic) that has write:packages" >&2
        return 1
    fi
}

# oci_digest DIR: print the digest of the one image in the OCI layout DIR.
oci_digest() {
    local -a digests
    mapfile -t digests < <(grep -o '"digest": *"sha256:[0-9a-f]\{64\}"' "$1/index.json" 2>/dev/null | grep -o 'sha256:[0-9a-f]*')
    if ((${#digests[@]} != 1)); then
        echo "$1/index.json does not list exactly one image" >&2
        return 1
    fi
    echo "${digests[0]}"
}
