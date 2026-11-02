#!/bin/bash
# Build the container image for linux/amd64, linux/arm64 and linux/arm/v6 from
# the binaries in the release archives of DIST, checked against
# DIST/SHA256SUMS, so the image adds no build of its own. Writes it as an OCI
# layout to OUT and tags it localhost/gmt-mail:TAG in the local image store.
# The same archives and commit give the same image digest. Run via
# `make image`; `make release` pushes it to ghcr.io (push-image.sh).
# Needs podman, or docker with the containerd image store (CONTAINER_ENGINE
# picks one), GNU tar and curl.
# Usage: image.sh TAG DIST OUT
set -euo pipefail

die() {
    echo "image: $*" >&2
    exit 1
}

[ $# -eq 3 ] || die "usage: $0 TAG DIST OUT"
# Relative to the caller's directory, before the cd.
DIST=$(realpath -e -- "$2") || die "no directory '$2'"
OUT=$(realpath -m -- "$3")
TAG=$1
cd "$(dirname "$0")/.."
# shellcheck source=release-lib.sh
source scripts/release-lib.sh
# An image tag: at most 128 of A-Z a-z 0-9 _ . -
[[ $TAG =~ ^v[0-9][0-9A-Za-z._-]{0,126}$ ]] || die "invalid tag '$TAG'"
VERSION=${TAG#v}
[ -f "$DIST/SHA256SUMS" ] || die "no $DIST/SHA256SUMS; run make dist first"
# Never remove anything but an earlier image.
[[ ! -e $OUT || -f $OUT/oci-layout ]] || die "'$OUT' exists and is not an OCI layout"
SOURCE=https://github.com/al-maisan/gmt-mail

ENGINE=${CONTAINER_ENGINE:-}
if [ -z "$ENGINE" ]; then
    if command -v podman >/dev/null; then ENGINE=podman; elif command -v docker >/dev/null; then ENGINE=docker; fi
fi
[[ $ENGINE =~ ^(podman|docker)$ ]] || die "need podman or docker (CONTAINER_ENGINE '$ENGINE')"
command -v "$ENGINE" >/dev/null || die "$ENGINE not found"

# Mozilla's CA certificates as curl extracts them
# (https://curl.se/docs/caextract.html), pinned by date and hash. To update,
# set both to the newest cacert-<date>.pem there and its sha256sum.
CA_URL=https://curl.se/ca/cacert-2026-10-20.pem
CA_SHA256=796f1389bfea13c0ed78f663b765baaff54a508e5483786bd6ece73c5e3e3cbe
CACHE=${XDG_CACHE_HOME:-$HOME/.cache}/gmt-mail
CA=$CACHE/${CA_URL##*/}
sha() { sha256sum <"$1" | cut -d' ' -f1; }
if [[ ! -f $CA || $(sha "$CA") != "$CA_SHA256" ]]; then
    mkdir -p "$CACHE"
    curl -fsSL --proto '=https' --tlsv1.2 -o "$CA.tmp" "$CA_URL" || die "downloading $CA_URL failed"
    [ "$(sha "$CA.tmp")" == "$CA_SHA256" ] || { rm -f "$CA.tmp"; die "$CA_URL does not have sha256 $CA_SHA256"; }
    mv "$CA.tmp" "$CA"
fi

umask 022
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
CTX=$TMP/ctx
install -D -m 644 "$CA" "$CTX/rootfs/etc/ssl/certs/ca-certificates.crt"
# Archive archs and their platforms. The archive arch is also the context
# directory of the platform: its TARGETARCH and TARGETVARIANT, joined.
ARCHS=(amd64 arm64 armv6)
PLATFORMS=(linux/amd64 linux/arm64 linux/arm/v6)
for a in "${ARCHS[@]}"; do
    name=gmt-mail-${VERSION}-linux-$a
    want=$(awk -v n="$name.tar.gz" '$2 == n { print $1 }' "$DIST/SHA256SUMS")
    [ -n "$want" ] || die "$name.tar.gz is not in $DIST/SHA256SUMS"
    [[ -f $DIST/$name.tar.gz && $(sha "$DIST/$name.tar.gz") == "$want" ]] ||
        die "$DIST/$name.tar.gz is missing or does not match SHA256SUMS"
    tar -xzf "$DIST/$name.tar.gz" -C "$TMP" "$name/gmt-mail"
    mkdir "$CTX/$a"
    install -m 755 "$TMP/$name/gmt-mail" "$CTX/$a/gmt-mail"
done

# The commit time: build time, label and timestamps.
MTIME=$(git log -1 --format=%ct)
ARGS=(
    --build-arg "VERSION=$TAG"
    --build-arg "REVISION=$(git rev-parse HEAD)"
    --build-arg "CREATED=$(date -u -d "@$MTIME" +%Y-%m-%dT%H:%M:%SZ)"
    --build-arg "SOURCE=$SOURCE"
    -f Containerfile
)
INDEX=(
    "org.opencontainers.image.source=$SOURCE"
    "org.opencontainers.image.description=Send personalized emails in bulk using templates"
    "org.opencontainers.image.licenses=GPL-3.0-or-later"
)
REF=localhost/gmt-mail:$TAG
rm -rf "$OUT"
echo "image: building $REF with $ENGINE"
if [ "$ENGINE" == podman ]; then
    if podman manifest exists "$REF"; then podman manifest rm "$REF" >/dev/null; fi
    # One platform at a time: a parallel build adds them in random order,
    # which changes the digest.
    for p in "${PLATFORMS[@]}"; do
        podman build --quiet --no-cache --layers=false --platform "$p" --manifest "$REF" --timestamp "$MTIME" \
            --identity-label=false "${ARGS[@]}" "$CTX" >/dev/null
    done
    for a in "${INDEX[@]}"; do podman manifest annotate --index --annotation "$a" "$REF" >/dev/null; done
    podman manifest push --quiet --all --format oci "$REF" "oci:$OUT"
else
    ANNOTATIONS=()
    for a in "${INDEX[@]}"; do ANNOTATIONS+=(--annotation "index:$a"); done
    SOURCE_DATE_EPOCH=$MTIME docker buildx build --quiet --no-cache --provenance=false --sbom=false \
        --platform "$(IFS=,; echo "${PLATFORMS[*]}")" "${ARGS[@]}" "${ANNOTATIONS[@]}" \
        --output "type=oci,dest=$OUT,tar=false,name=$REF,rewrite-timestamp=true" "$CTX" >/dev/null
    # Loaded from the layout: a second --output races on the shared layers.
    tar -C "$OUT" -cf - . | docker load >/dev/null
fi
DIGEST=$(oci_digest "$OUT") || exit 1
echo "image: $OUT ($REF), digest $DIGEST"
