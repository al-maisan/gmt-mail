#!/bin/bash
# Tests for image.sh and the image: make test-image
# Builds the image from test binaries with podman or docker (CONTAINER_ENGINE
# picks one) and runs it without network; skipped if neither is installed.
# Needs network for the CA bundle unless it is cached.
set -uo pipefail

SRC=$(cd "$(dirname "$0")/.." && pwd)
ENGINE=${CONTAINER_ENGINE:-}
if [ -z "$ENGINE" ]; then
    if command -v podman >/dev/null; then ENGINE=podman; elif command -v docker >/dev/null; then ENGINE=docker; fi
fi
if [ -z "$ENGINE" ]; then
    echo "skip - image tests: neither podman nor docker is installed"
    exit 0
fi
export CONTAINER_ENGINE=$ENGINE
fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}
builds() { if "$SRC/scripts/image.sh" "$@" >/dev/null 2>&1; then echo yes; else echo no; fi; }
# shellcheck source=release-lib.sh
source "$SRC/scripts/release-lib.sh"

TAG=v0.0.0-imagetest
VERSION=${TAG#v}
REF=localhost/gmt-mail:$TAG
TMP=$(mktemp -d)
# shellcheck disable=SC2329 # run by the EXIT trap
cleanup() {
    if [ "$ENGINE" == podman ]; then podman manifest rm "$REF" >/dev/null 2>&1; else docker rmi "$REF" >/dev/null 2>&1; fi
    # The images of the platforms, now untagged.
    "$ENGINE" image prune -a -f --filter "label=org.opencontainers.image.version=$TAG" >/dev/null 2>&1
    rm -rf "$TMP"
}
trap cleanup EXIT

# Release archives as build-dist.sh names them, with only the binary.
DIST=$TMP/dist
mkdir "$DIST"
for a in amd64 arm64 armv6; do
    name=gmt-mail-${VERSION}-linux-$a goarch=$a goarm=
    if [ "$a" == armv6 ]; then goarch=arm goarm=6; fi
    (cd "$SRC" && CGO_ENABLED=0 GOOS=linux GOARCH=$goarch GOARM=$goarm go build -trimpath \
        -ldflags "-X main.appVersion=$VERSION -X main.gitCommit=abc1234 -X main.buildDate=2026-01-02" \
        -o "$DIST/$name/gmt-mail" .) || { echo "FAIL - go build $a"; exit 1; }
    tar -C "$DIST" -czf "$DIST/$name.tar.gz" "$name"
    rm -rf "${DIST:?}/$name"
done
(cd "$DIST" && sha256sum -- *.tar.gz >SHA256SUMS)

for d in a b; do
    "$SRC/scripts/image.sh" "$TAG" "$DIST" "$TMP/$d" >/dev/null || { echo "FAIL - build"; exit 1; }
done
check "same inputs, same digest" "$(oci_digest "$TMP/a")" "$(oci_digest "$TMP/b")"
check "rebuild replaces the layout" yes "$(builds "$TAG" "$DIST" "$TMP/a")"
mkdir "$TMP/other"
check "rejects an output that is not an OCI layout" no "$(builds "$TAG" "$DIST" "$TMP/other")"
check "rejects bad tag" no "$(builds 'v1;rm' "$DIST" "$TMP/c")"
check "rejects tag without v" no "$(builds 1.2.3 "$DIST" "$TMP/c")"
check "rejects missing archives" no "$(builds v9.9.9 "$DIST" "$TMP/c")"
cp -r "$DIST" "$TMP/bad"
printf x >>"$TMP/bad/gmt-mail-${VERSION}-linux-arm64.tar.gz"
check "rejects an archive that does not match SHA256SUMS" no "$(builds "$TAG" "$TMP/bad" "$TMP/c")"
"$SRC/scripts/image.sh" "$TAG" "$DIST" "$TMP/a" >/dev/null # the local image of these archives

INDEX=$(cat "$TMP/a/blobs/sha256/$(oci_digest "$TMP/a" | cut -d: -f2)")
check "platforms" "amd64 arm64 arm" "$(grep -o '"architecture": *"[a-z0-9]*"' <<<"$INDEX" | cut -d'"' -f4 | paste -sd' ')"
check "arm is v6" 1 "$(grep -c '"variant": *"v6"' <<<"$INDEX")"
# Each platform has its binary: the last layer of each image.
sha() { grep -o 'sha256:[0-9a-f]\{64\}' "$@" | cut -d: -f2; }
for m in $(sha <<<"$INDEX"); do
    layer=$(sha "$TMP/a/blobs/sha256/$m" | tail -1)
    tar -xzOf "$TMP/a/blobs/sha256/$layer" gmt-mail >"$TMP/bin"
    go version -m "$TMP/bin" | awk '$1 == "build" && $2 ~ /^GOARCH=|^GOARM=/ { print $2 }' | paste -sd' '
done >"$TMP/goarch"
check "binaries" "GOARCH=amd64
GOARCH=arm64
GOARCH=arm GOARM=6" "$(cat "$TMP/goarch")"
inspect() { "$ENGINE" image inspect --format "$1" "$REF"; }
check "non-root user" "65532:65532" "$(inspect '{{.Config.User}}')"
check "workdir" "/work" "$(inspect '{{.Config.WorkingDir}}')"
check "version label" "$TAG" "$(inspect '{{index .Config.Labels "org.opencontainers.image.version"}}')"
check "source label" "https://github.com/al-maisan/gmt-mail" "$(inspect '{{index .Config.Labels "org.opencontainers.image.source"}}')"
check "revision label" "$(git -C "$SRC" rev-parse HEAD)" "$(inspect '{{index .Config.Labels "org.opencontainers.image.revision"}}')"
check "created label is the commit time" "$(date -u -d "@$(git -C "$SRC" log -1 --format=%ct)" +%Y-%m-%dT%H:%M:%SZ)" \
    "$(inspect '{{index .Config.Labels "org.opencontainers.image.created"}}')"

# The files of the image for this host.
c=$("$ENGINE" create "$REF")
FILES=$("$ENGINE" export "$c" | tar -tvf - --numeric-owner | awk '{ print $1, $2, $6 }' | LC_ALL=C sort -k3)
CA=$("$ENGINE" export "$c" | tar -xOf - etc/ssl/certs/ca-certificates.crt | sha256sum | cut -d' ' -f1)
"$ENGINE" rm -v "$c" >/dev/null
check "files" "drwxr-xr-x 0/0 etc/
drwxr-xr-x 0/0 etc/ssl/
drwxr-xr-x 0/0 etc/ssl/certs/
-rw-r--r-- 0/0 etc/ssl/certs/ca-certificates.crt
-rwxr-xr-x 0/0 gmt-mail" "$(grep -E ' (etc/(ssl/(certs/(ca-certificates.crt)?)?)?|gmt-mail)$' <<<"$FILES")"
check "pinned CA bundle" "$(sed -n 's/^CA_SHA256=//p' "$SRC/scripts/image.sh")" "$CA"

run() { "$ENGINE" run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges "$@"; }
check "version" "$VERSION-abc1234 (2026-01-02)" "$(run "$REF" -version)"

# The files in /work, as the docs show it: the caller's user, so it can read them.
WORK=$TMP/work
mkdir "$WORK"
run "$REF" -sample-config >"$WORK/c.toml"
run "$REF" -sample-template >"$WORK/t.eml"
USERNS=()
if [ "$ENGINE" == podman ]; then USERNS=(--userns=keep-id); fi
mount=(--user "$(id -u):$(id -g)" "${USERNS[@]}" -v "$WORK:/work:Z")
check "validate" "Config and template are valid: 5 recipient(s)" \
    "$(run "${mount[@]}" "$REF" -validate -config-path c.toml -template-path t.eml 2>&1)"
printf 'SMTP_HOST=127.0.0.1\nSMTP_PORT=1\nSENDER_EMAIL=a@b.example\nSENDER_PASSWORD=x\n' >"$WORK/.env"
run "${mount[@]}" "$REF" -config-path c.toml -template-path t.eml >"$TMP/send" 2>&1
check "send: .env in /work loaded, no network" 3 "$?"
check "send: tries the .env host" 1 "$(grep -c 'failed to connect to 127.0.0.1:1' "$TMP/send")"

exit "$fail"
