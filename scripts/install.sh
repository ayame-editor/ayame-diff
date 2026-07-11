#!/usr/bin/env bash
#
# ayame-diff installer for Linux and macOS.
#
# Downloads a release archive from GitHub, verifies its SHA-256 checksum,
# and installs the "ayame-diff" binary into a bin directory.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/hjosugi/ayame-diff/main/scripts/install.sh | bash
#   ./scripts/install.sh                 # install the latest release
#   ./scripts/install.sh v0.5.1          # pin a release tag (positional)
#   VERSION=v0.5.1 ./scripts/install.sh  # pin a release tag (environment)
#   PREFIX="$HOME/.local" ./scripts/install.sh   # choose the install prefix
#
set -euo pipefail

REPO="hjosugi/ayame-diff"
API="https://api.github.com/repos/${REPO}/releases/latest"
BINARY="ayame-diff"

info() { printf '==> %s\n' "$*"; }
warn() { printf 'warning: %s\n' "$*" >&2; }
err()  { printf 'error: %s\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 \
    || err "required command not found: $1 (please install it and re-run)"
}

# --- prerequisites -----------------------------------------------------------
need curl
need tar

# a sha256 tool: sha256sum (Linux) or shasum (macOS)
if command -v sha256sum >/dev/null 2>&1; then
  SHA_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHA_CMD="shasum -a 256"
else
  err "no sha256 tool found: install coreutils (sha256sum) or shasum"
fi

# --- detect platform ---------------------------------------------------------
os=$(uname -s)
case "$os" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)      err "unsupported operating system: $os (only linux and darwin are supported)" ;;
esac

machine=$(uname -m)
case "$machine" in
  x86_64 | amd64)  ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)               err "unsupported architecture: $machine" ;;
esac
info "detected platform: ${OS}/${ARCH}"

# --- resolve version ---------------------------------------------------------
VERSION="${VERSION:-${1:-}}"
if [ -z "$VERSION" ]; then
  info "resolving latest release tag from GitHub"
  VERSION=$(curl -fsSL --proto '=https' "$API" \
    | grep -m1 '"tag_name"' \
    | sed -e 's/.*"tag_name"[[:space:]]*:[[:space:]]*"//' -e 's/".*//') \
    || err "could not query the GitHub releases API"
  [ -n "$VERSION" ] || err "could not determine the latest release tag"
fi
info "installing ${BINARY} ${VERSION}"

ASSET="${BINARY}-${VERSION}-${OS}-${ARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"

# --- workspace ---------------------------------------------------------------
TMP=$(mktemp -d "${TMPDIR:-/tmp}/ayame-diff-install.XXXXXX")
trap 'rm -rf "$TMP"' EXIT INT TERM

# --- download ----------------------------------------------------------------
info "downloading ${ASSET}"
curl -fsSL --proto '=https' "${BASE}/${ASSET}" -o "$TMP/$ASSET" \
  || err "download failed: ${BASE}/${ASSET}"
info "downloading SHA256SUMS"
curl -fsSL --proto '=https' "${BASE}/SHA256SUMS" -o "$TMP/SHA256SUMS" \
  || err "download failed: ${BASE}/SHA256SUMS"

# --- verify checksum ---------------------------------------------------------
info "verifying sha256 checksum"
# SHA256SUMS lines look like: "<hash>  ./ayame-diff-<tag>-<os>-<arch>.tar.gz"
expected=$(grep -E "(^|[[:space:]]|/)${ASSET}\$" "$TMP/SHA256SUMS" \
  | awk '{print $1}' | head -n1) || true
[ -n "$expected" ] || err "no checksum for ${ASSET} in SHA256SUMS"
actual=$(cd "$TMP" && $SHA_CMD "$ASSET" | awk '{print $1}')
if [ "$expected" != "$actual" ]; then
  err "checksum mismatch for ${ASSET}: expected ${expected}, got ${actual}"
fi
info "checksum OK"

# --- extract -----------------------------------------------------------------
info "extracting archive"
tar -xzf "$TMP/$ASSET" -C "$TMP"
extracted="$TMP/${BINARY}-${VERSION}-${OS}-${ARCH}/${BINARY}"
[ -f "$extracted" ] || err "expected binary not found in archive: ${extracted}"
chmod 0755 "$extracted"

# --- choose install directory ------------------------------------------------
PREFIX="${PREFIX:-/usr/local}"
BIN_DIR="${PREFIX}/bin"

usable_dir() {
  # succeed if the directory exists and is writable, or can be created
  if [ -d "$1" ]; then
    [ -w "$1" ]
  else
    mkdir -p "$1" 2>/dev/null
  fi
}

if ! usable_dir "$BIN_DIR"; then
  fallback="${HOME}/.local/bin"
  warn "${BIN_DIR} is not writable; falling back to ${fallback}"
  BIN_DIR="$fallback"
  usable_dir "$BIN_DIR" || err "cannot create install directory: ${BIN_DIR}"
fi

# --- install -----------------------------------------------------------------
DEST="${BIN_DIR}/${BINARY}"
info "installing to ${DEST}"
cp "$extracted" "$DEST"
chmod 0755 "$DEST"

info "installed ${BINARY} ${VERSION} to ${DEST}"

case ":${PATH}:" in
  *":${BIN_DIR}:"*) info "run '${BINARY} --version' to verify" ;;
  *) warn "${BIN_DIR} is not on your PATH; add it, e.g.: export PATH=\"${BIN_DIR}:\$PATH\"" ;;
esac
