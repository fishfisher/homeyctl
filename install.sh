#!/bin/sh
# Install or upgrade homeyctl from GitHub Releases.
#
#   curl -fsSL https://raw.githubusercontent.com/fishfisher/homeyctl/main/install.sh | sh
#
# Options (environment variables):
#   HOMEYCTL_VERSION   Tag to install, e.g. v1.4.0. Defaults to the latest release.
#   HOMEYCTL_BIN_DIR   Install directory. Defaults to the first writable of
#                      /usr/local/bin, /opt/homebrew/bin, $HOME/.local/bin.
#
# The release assets are public, so no token or authentication is needed.
# Every download is verified against the release's checksums.txt.

set -eu

REPO="fishfisher/homeyctl"
BIN_NAME="homeyctl"

die() {
	echo "install.sh: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

need uname
need mkdir
need install

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL "$1" -o "$2"; }
	fetch_stdout() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -qO "$2" "$1"; }
	fetch_stdout() { wget -qO- "$1"; }
else
	die "need curl or wget"
fi

os=$(uname -s)
[ "$os" = "Darwin" ] || die "homeyctl currently ships macOS builds only (detected $os)"

case $(uname -m) in
arm64 | aarch64) arch="arm64" ;;
x86_64 | amd64) arch="amd64" ;;
*) die "unsupported architecture: $(uname -m)" ;;
esac

asset="${BIN_NAME}-darwin-${arch}"

version="${HOMEYCTL_VERSION:-}"
if [ -z "$version" ]; then
	# The GitHub API returns the latest published release for a public repo
	# without authentication.
	version=$(fetch_stdout "https://api.github.com/repos/${REPO}/releases/latest" |
		sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
	[ -n "$version" ] || die "could not resolve the latest release tag; set HOMEYCTL_VERSION"
fi

bin_dir="${HOMEYCTL_BIN_DIR:-}"
if [ -z "$bin_dir" ]; then
	for candidate in /usr/local/bin /opt/homebrew/bin "$HOME/.local/bin"; do
		if [ -d "$candidate" ] && [ -w "$candidate" ]; then
			bin_dir="$candidate"
			break
		fi
	done
fi
if [ -z "$bin_dir" ]; then
	bin_dir="$HOME/.local/bin"
	mkdir -p "$bin_dir"
fi
[ -d "$bin_dir" ] || mkdir -p "$bin_dir"
[ -w "$bin_dir" ] || die "$bin_dir is not writable; set HOMEYCTL_BIN_DIR to a directory you own"

base="https://github.com/${REPO}/releases/download/${version}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

echo "Downloading ${BIN_NAME} ${version} (darwin/${arch})..."
fetch "${base}/${asset}" "${tmp}/${asset}" || die "download failed: ${base}/${asset}"

# Verify against the release checksums rather than trusting the transfer alone.
if fetch "${base}/checksums.txt" "${tmp}/checksums.txt" 2>/dev/null; then
	expected=$(grep " ${asset}\$" "${tmp}/checksums.txt" | awk '{print $1}' | head -1)
	if [ -n "$expected" ]; then
		if command -v shasum >/dev/null 2>&1; then
			actual=$(shasum -a 256 "${tmp}/${asset}" | awk '{print $1}')
		elif command -v sha256sum >/dev/null 2>&1; then
			actual=$(sha256sum "${tmp}/${asset}" | awk '{print $1}')
		else
			actual=""
		fi
		if [ -n "$actual" ] && [ "$actual" != "$expected" ]; then
			die "checksum mismatch for ${asset}: expected ${expected}, got ${actual}"
		fi
		[ -n "$actual" ] && echo "Checksum verified."
	else
		echo "install.sh: warning: ${asset} not listed in checksums.txt" >&2
	fi
else
	echo "install.sh: warning: could not fetch checksums.txt; skipping verification" >&2
fi

install -m 0755 "${tmp}/${asset}" "${bin_dir}/${BIN_NAME}"

# The binaries are ad-hoc signed by the Go toolchain, not notarized. Clearing the
# quarantine attribute avoids Gatekeeper blocking a binary fetched by a browser.
if command -v xattr >/dev/null 2>&1; then
	xattr -d com.apple.quarantine "${bin_dir}/${BIN_NAME}" 2>/dev/null || true
fi

echo "Installed ${BIN_NAME} ${version} to ${bin_dir}/${BIN_NAME}"

case ":${PATH}:" in
*":${bin_dir}:"*) ;;
*) echo "Note: ${bin_dir} is not on your PATH. Add it to your shell profile." >&2 ;;
esac

"${bin_dir}/${BIN_NAME}" --version || true
