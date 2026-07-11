#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION="${VERSION:-$(cd "$ROOT" && git describe --tags --always --dirty 2>/dev/null || echo dev)}"
DIST="$ROOT/dist"
TARGETS="$ROOT/scripts/targets.txt"
rm -rf "$DIST"
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

while read -r os arch ext; do
  case "$os" in ''|'#'*) continue ;; esac
  [ "$ext" = "-" ] && ext=""
  build "$os" "$arch" "$ext"
done < "$TARGETS"

(cd "$DIST" && "$ROOT/scripts/checksum.sh" SHA256SUMS ayame-diff-*)
