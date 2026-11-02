#!/bin/bash
# Build the release archives and their SHA256SUMS.
# Usage: build-dist.sh VERSION COMMIT DATE OUT_DIR   (called by `make dist`)
#
# One archive per target (tar.gz, zip for Windows) holding the binary, LICENSE,
# README.md with its logos (assets/) and the man page. Binaries use the same
# -ldflags as `make build`.
# Archives are reproducible: the same source, arguments and Go toolchain give the
# same SHA256SUMS (the Go version is embedded in the binaries).
# DIST_TARGETS="linux/amd64 ..." overrides the target list.
set -euo pipefail

# Same output whatever the caller's environment says.
export CGO_ENABLED=0 GOAMD64=v1 GOARM64=v8.0 GOFLAGS='' GOEXPERIMENT=''

# linux/armv6 runs on every Raspberry Pi, Zero and 1 included, with a 32-bit OS.
TARGETS=${DIST_TARGETS:-"linux/amd64 linux/arm64 linux/armv6 darwin/amd64 darwin/arm64 windows/amd64"}

if [ $# -ne 4 ]; then
    echo "usage: $0 VERSION COMMIT DATE OUT_DIR" >&2
    exit 1
fi
VERSION=$1 COMMIT=$2 DATE=$3 OUT=$(realpath -m "$4")
SRC=$(cd "$(dirname "$0")/.." && pwd)
LDFLAGS="-X main.appVersion=$VERSION -X main.gitCommit=$COMMIT -X main.buildDate=$DATE"
# Fixed file times keep the archives reproducible.
EPOCH=$(date -u -d "$DATE" +%s)

# Remove only what this script writes: OUT may hold other files.
rm -rf "$OUT/stage" "$OUT"/gmt-mail-* "$OUT"/SHA256SUMS*
mkdir -p "$OUT/stage"
for target in $TARGETS; do
    os=${target%/*} arch=${target#*/} goarch=${target#*/} goarm=
    if [ "$arch" == armv6 ]; then goarch=arm goarm=6; fi
    name="gmt-mail-${VERSION}-${os}-${arch}"
    dir="$OUT/stage/$name"
    exe=gmt-mail
    [ "$os" == windows ] && exe=gmt-mail.exe
    mkdir -p "$dir"
    (cd "$SRC" && CGO_ENABLED=0 GOOS=$os GOARCH=$goarch GOARM=$goarm go build -mod=readonly -trimpath \
        -ldflags "$LDFLAGS" -o "$dir/$exe" .)
    cp "$SRC/LICENSE" "$SRC/README.md" "$dir/"
    install -Dm644 -t "$dir/assets" "$SRC/assets/logo.svg" "$SRC/assets/logo-dark.svg"
    sed -e "s/@VERSION@/$VERSION/g" -e "s/@DATE@/$DATE/g" "$SRC/gmt-mail.1" >"$dir/gmt-mail.1"
    chmod -R u=rwX,go=rX "$dir" # modes must not depend on the umask
    find "$dir" -exec touch -h -d "@$EPOCH" {} +
    if [ "$os" == windows ]; then
        (cd "$OUT/stage" && find "$name" | LC_ALL=C sort | TZ=UTC zip -qX "$OUT/$name.zip" -@)
    else
        tar -C "$OUT/stage" --sort=name --owner=0 --group=0 --numeric-owner --mtime="@$EPOCH" \
            -cf - "$name" | gzip -n >"$OUT/$name.tar.gz"
    fi
    echo "dist: $name"
done
rm -rf "$OUT/stage"
shopt -s nullglob
(cd "$OUT" && files=(*.tar.gz *.zip) && sha256sum -- "${files[@]}" >SHA256SUMS)
echo "dist: $OUT/SHA256SUMS"
