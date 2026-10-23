#!/usr/bin/env bash
# install.sh — download, verify and install the latest yt-manage release.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/alimtvnetwork/yt-manage/main/install.sh | sh
#
# Installs to ${PREFIX:-$HOME/.local}/bin. The SHA256 of the downloaded
# archive is verified against the release checksums.txt before
# anything is installed; a mismatch aborts the install.

set -euo pipefail

REPO="alimtvnetwork/yt-manage"

fail() {
    echo "install.sh: $*" >&2
    exit 1
}

# --- Detect OS -------------------------------------------------------

case "$(uname -s)" in
    Linux)
        OS="linux"
        ;;
    Darwin)
        OS="darwin"
        ;;
    *)
        fail "unsupported operating system '$(uname -s)'. Windows users: download yt-manage_windows_amd64.zip from the releases page (https://github.com/${REPO}/releases), unpack it, and place yt-manage.exe on your PATH."
        ;;
esac

# --- Detect architecture ---------------------------------------------

case "$(uname -m)" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        fail "unsupported architecture '$(uname -m)'."
        ;;
esac

# --- Resolve the latest release tag ----------------------------------
# No jq dependency: tag_name is pulled out of the API JSON with
# grep/sed.

API_URL="https://api.github.com/repos/${REPO}/releases/latest"
RELEASE_JSON="$(curl -fsSL "$API_URL")" || fail "could not query ${API_URL}"

TAG="$(printf '%s\n' "$RELEASE_JSON" \
    | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' \
    | head -n 1 \
    | sed 's/.*"\([^"]*\)"$/\1/')"

[ -n "$TAG" ] || fail "could not determine the latest release tag from ${API_URL}"

ARCHIVE="yt-manage_${OS}_${ARCH}.tar.gz"
DOWNLOAD_BASE="https://github.com/${REPO}/releases/download/${TAG}"

# --- Download into a scratch directory -------------------------------

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

echo "Downloading ${ARCHIVE} (${TAG})..."
curl -fsSL -o "${WORK_DIR}/${ARCHIVE}" "${DOWNLOAD_BASE}/${ARCHIVE}" \
    || fail "download failed: ${DOWNLOAD_BASE}/${ARCHIVE}"
curl -fsSL -o "${WORK_DIR}/checksums.txt" "${DOWNLOAD_BASE}/checksums.txt" \
    || fail "download failed: ${DOWNLOAD_BASE}/checksums.txt"

# --- Verify the checksum BEFORE installing anything ------------------

sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

EXPECTED="$(awk -v f="$ARCHIVE" '{name = $2; sub(/^\*/, "", name); if (name == f) print $1}' "${WORK_DIR}/checksums.txt")"
[ -n "$EXPECTED" ] || fail "checksums.txt has no entry for ${ARCHIVE}"

ACTUAL="$(sha256_of "${WORK_DIR}/${ARCHIVE}")"

if [ "$ACTUAL" != "$EXPECTED" ]; then
    echo "checksum mismatch for ${ARCHIVE}" >&2
    echo "  expected: ${EXPECTED}" >&2
    echo "  actual:   ${ACTUAL}" >&2
    exit 1
fi
echo "Checksum verified: ${ARCHIVE}"

# --- Extract and install ----------------------------------------------

mkdir -p "${WORK_DIR}/extract"
tar -xzf "${WORK_DIR}/${ARCHIVE}" -C "${WORK_DIR}/extract"

[ -f "${WORK_DIR}/extract/yt-manage" ] \
    || fail "archive ${ARCHIVE} did not contain a yt-manage binary"

DEST="${PREFIX:-$HOME/.local}/bin"
mkdir -p "$DEST"
install -m 0755 "${WORK_DIR}/extract/yt-manage" "${DEST}/yt-manage"

echo "Installed yt-manage ${TAG} to ${DEST}/yt-manage"

case ":${PATH}:" in
    *":${DEST}:"*)
        ;;
    *)
        echo "Note: ${DEST} is not on your PATH. Add it with:"
        echo "  export PATH=\"${DEST}:\$PATH\""
        ;;
esac
