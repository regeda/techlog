#!/bin/sh
# Installs a released techlog binary from GitHub Releases.
#
#   curl -sSfL https://raw.githubusercontent.com/regeda/techlog/main/install.sh | sh
#
# Environment:
#   VERSION      release tag to install, for example v1.2.3 (default: latest)
#   INSTALL_DIR  where to put the binary (default: $HOME/.local/bin)
set -eu

BASE_URL=${TECHLOG_BASE_URL:-https://github.com/regeda/techlog/releases}
INSTALL_DIR=${INSTALL_DIR:-$HOME/.local/bin}

die() {
	echo "install.sh: $*" >&2
	exit 1
}

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -sSfL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -q -O "$2" "$1"; }
else
	die "need curl or wget"
fi

if command -v sha256sum >/dev/null 2>&1; then
	sha256() { sha256sum "$1" | cut -d' ' -f1; }
elif command -v shasum >/dev/null 2>&1; then
	sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }
else
	die "need sha256sum or shasum"
fi

case $(uname -s) in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) die "unsupported OS: $(uname -s)" ;;
esac

case $(uname -m) in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) die "unsupported architecture: $(uname -m)" ;;
esac

# latest_tag prints the tag the releases/latest URL redirects to.
latest_tag() {
	if command -v curl >/dev/null 2>&1; then
		url=$(curl -sSfLI -o /dev/null -w '%{url_effective}' "$BASE_URL/latest") || return 1
	else
		url=$(wget --server-response --spider --max-redirect=0 "$BASE_URL/latest" 2>&1 |
			sed -n 's/^ *[Ll]ocation: *//p' | tr -d '\r' | tail -n 1)
	fi
	tag=${url##*/}
	if [ -z "$tag" ] || [ "$tag" = latest ]; then
		return 1
	fi
	echo "$tag"
}

tag=${VERSION:-}
if [ -z "$tag" ]; then
	tag=$(latest_tag) || die "could not find the latest release"
fi
file="techlog_${tag#v}_${os}_${arch}.tar.gz"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading techlog $tag for $os/$arch"
fetch "$BASE_URL/download/$tag/$file" "$tmp/$file" || die "could not download $file"
fetch "$BASE_URL/download/$tag/checksums.txt" "$tmp/checksums.txt" || die "could not download checksums.txt"

want=$(awk -v f="$file" '$2 == f { print $1 }' "$tmp/checksums.txt")
if [ -z "$want" ] || [ "$(echo "$want" | wc -l)" -ne 1 ]; then
	die "checksums.txt has no single checksum for $file"
fi
[ "$(sha256 "$tmp/$file")" = "$want" ] || die "checksum mismatch for $file"

tar -xzf "$tmp/$file" -C "$tmp" techlog
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/techlog" "$INSTALL_DIR/techlog"
echo "Installed techlog $tag to $INSTALL_DIR/techlog"

case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*) echo "Note: $INSTALL_DIR is not on your PATH" ;;
esac
