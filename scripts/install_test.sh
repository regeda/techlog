#!/bin/sh
# Tests install.sh against a fake release served over file://.
set -u

ROOT=$(cd "$(dirname "$0")/.." && pwd)
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
FAILED=0

case $(uname -s) in Linux) OS=linux ;; Darwin) OS=darwin ;; *) echo "unsupported host"; exit 1 ;; esac
case $(uname -m) in x86_64 | amd64) ARCH=amd64 ;; arm64 | aarch64) ARCH=arm64 ;; *) echo "unsupported host"; exit 1 ;; esac

if command -v sha256sum >/dev/null 2>&1; then
	sha() { sha256sum "$1" | cut -d' ' -f1; }
else
	sha() { shasum -a 256 "$1" | cut -d' ' -f1; }
fi

ARCHIVE="techlog_1.2.3_${OS}_${ARCH}.tar.gz"

# make_release builds a fresh fake release in $TMP/release and an empty $TMP/bin.
make_release() {
	rm -rf "${TMP:?}/release" "${TMP:?}/bin" "${TMP:?}/pkg"
	mkdir -p "$TMP/release/download/v1.2.3" "$TMP/pkg"
	printf '#!/bin/sh\necho techlog fake\n' >"$TMP/pkg/techlog"
	chmod +x "$TMP/pkg/techlog"
	echo license >"$TMP/pkg/LICENSE"
	(cd "$TMP/pkg" && tar -czf "$TMP/release/download/v1.2.3/$ARCHIVE" techlog LICENSE)
	printf '%s  %s\n' "$(sha "$TMP/release/download/v1.2.3/$ARCHIVE")" "$ARCHIVE" \
		>"$TMP/release/download/v1.2.3/checksums.txt"
}

# run_install runs install.sh with the fake release; extra env goes in "$@".
run_install() {
	env TECHLOG_BASE_URL="file://$TMP/release" INSTALL_DIR="$TMP/bin" VERSION="${VERSION:-v1.2.3}" "$@" \
		sh "$ROOT/install.sh" >"$TMP/out" 2>&1
}

pass() { echo "ok   $1"; }
fail() { echo "FAIL $1"; sed 's/^/     /' "$TMP/out" 2>/dev/null; FAILED=1; }

make_release
if run_install && [ -x "$TMP/bin/techlog" ] && [ "$("$TMP/bin/techlog")" = "techlog fake" ]; then
	pass installs_pinned_version
else fail installs_pinned_version; fi

echo old >"$TMP/bin/techlog"
if run_install && [ "$("$TMP/bin/techlog")" = "techlog fake" ]; then
	pass reinstall_replaces
else fail reinstall_replaces; fi

make_release
echo tampered >>"$TMP/release/download/v1.2.3/$ARCHIVE"
if ! run_install && grep -qi checksum "$TMP/out" && [ ! -e "$TMP/bin/techlog" ]; then
	pass tampered_archive_rejected
else fail tampered_archive_rejected; fi

make_release
: >"$TMP/release/download/v1.2.3/checksums.txt"
if ! run_install && [ ! -e "$TMP/bin/techlog" ]; then
	pass missing_checksum_line_rejected
else fail missing_checksum_line_rejected; fi

make_release
if ! VERSION=v9.9.9 run_install && [ ! -e "$TMP/bin/techlog" ]; then
	pass download_failure
else fail download_failure; fi

make_release
mkdir -p "$TMP/shim"
# shellcheck disable=SC2016 # the shim body must stay unexpanded
printf '#!/bin/sh\n[ "$1" = "-m" ] && echo riscv64 || exec /usr/bin/uname "$@"\n' >"$TMP/shim/uname"
chmod +x "$TMP/shim/uname"
if ! run_install PATH="$TMP/shim:$PATH" && grep -q riscv64 "$TMP/out" && [ ! -e "$TMP/bin/techlog" ]; then
	pass unsupported_arch
else fail unsupported_arch; fi

exit "$FAILED"
