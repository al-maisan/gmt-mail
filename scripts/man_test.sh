#!/bin/bash
# Checks that the man page version/date placeholders get filled in everywhere.
# Usage: man_test.sh RENDERED_MAN GENERATED_SPEC VERSION   (run via: make test-man)
set -uo pipefail

MAN="$1" SPEC="$2" VERSION="$3"
SRC_DIR="$(cd "$(dirname "$0")/.." && pwd)"
fail=0
check() { # check NAME EXPECTED ACTUAL
    if [ "$2" == "$3" ]; then echo "ok   - $1"; else echo "FAIL - $1: want '$2', got '$3'"; fail=1; fi
}

check "template header has both placeholders" 1 "$(grep -c -F '.TH GMT\-MAIL 1 "@DATE@" "gmt-mail @VERSION@"' "$SRC_DIR/gmt-mail.1")"
check "make man: no placeholders left" 0 "$(grep -c -E '@(VERSION|DATE)@' "$MAN")"
check "make man: version in header" 1 "$(grep -c -E "^\.TH GMT\\\\-MAIL 1 \"[0-9]{4}-[0-9]{2}-[0-9]{2}\" \"gmt-mail ${VERSION//./\\.}\"" "$MAN")"

# gen-spec replaces @VERSION@ in the spec; the man page sed there must survive it.
check "spec: no placeholder left" 0 "$(grep -cE '@[A-Z_]+@' "$SPEC")"
check "spec: man sed survives gen-spec" 1 "$(grep -c -F "s/@VERSION[@]/%{version}/g" "$SPEC")"

# Run the spec's and debian/rules' sed expressions on the template, as the builds do.
RPM_OUT=$(sed -e "s/@VERSION[@]/$VERSION/g" -e "s/@DATE[@]/2026-01-02/g" "$SRC_DIR/gmt-mail.1" | grep '^\.TH')
check "rpm render" ".TH GMT\\-MAIL 1 \"2026-01-02\" \"gmt-mail $VERSION\" \"User Commands\"" "$RPM_OUT"
# debian/rules installs the man page with deb-layout.sh, as the release .deb does.
# shellcheck disable=SC2016 # literal $(...) and $VAR are looked for
check "debian rules: deb-layout.sh with version and man date" 1 \
    "$(grep -cF 'debian/deb-layout.sh $(CURDIR)/debian/gmt-mail gmt-mail $(VERSION) $(MANDATE)' "$SRC_DIR/ppa/debian/rules")"
# shellcheck disable=SC2016
check "deb-layout.sh: sed present" 1 \
    "$(grep -cF 'sed -e "s/@VERSION@/$VERSION/g" -e "s/@DATE@/$MAN_DATE/g" gmt-mail.1' "$SRC_DIR/scripts/deb-layout.sh")"
DEB_OUT=$(sed -e "s/@VERSION@/$VERSION/g" -e "s/@DATE@/2026-01-02/g" "$SRC_DIR/gmt-mail.1" | grep '^\.TH')
check "debian render" "$RPM_OUT" "$DEB_OUT"

exit "$fail"
