#!/usr/bin/env sh
set -eu

VERSION="${VERSION:-v0.3.1}"
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
DIST="$ROOT/dist"
mkdir -p "$DIST"

build() {
  os=$1
  arch=$2
  ext=$3
  output="$DIST/ayame-diff-${os}-${arch}${ext}"
  echo "building $output"
  (
    cd "$ROOT"
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
      -o "$output" ./cmd/ayame-diff
  )
}

build linux amd64 ""
build linux arm64 ""
build darwin amd64 ""
build darwin arm64 ""
build windows amd64 ".exe"
build windows arm64 ".exe"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$DIST" && sha256sum ayame-diff-* > SHA256SUMS)
elif command -v shasum >/dev/null 2>&1; then
  (cd "$DIST" && shasum -a 256 ayame-diff-* > SHA256SUMS)
fi
