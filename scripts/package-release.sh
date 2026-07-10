#!/usr/bin/env sh
set -eu

VERSION="${VERSION:-v0.3.0}"
case "$VERSION" in
  v[0-9]*) ;;
  *)
    echo "VERSION must start with v and a digit: $VERSION" >&2
    exit 2
    ;;
esac

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
DIST="$ROOT/dist"
RELEASE="$ROOT/release"

require_file() {
  if [ ! -f "$1" ]; then
    echo "missing release input: $1" >&2
    exit 2
  fi
}

for target in \
  linux-amd64 linux-arm64 \
  darwin-amd64 darwin-arm64 \
  windows-amd64.exe windows-arm64.exe
do
  require_file "$DIST/fcsv-diff-$target"
done

rm -rf "$RELEASE"
mkdir -p "$RELEASE"

package_unix() {
  os=$1
  arch=$2
  name="fcsv-diff-${VERSION}-${os}-${arch}"
  stage="$RELEASE/$name"

  mkdir -p "$stage"
  cp "$DIST/fcsv-diff-${os}-${arch}" "$stage/fcsv-diff"
  chmod 0755 "$stage/fcsv-diff"
  cp "$ROOT/README.md" "$ROOT/LICENSE" "$ROOT/THIRD_PARTY_NOTICES.md" "$stage/"
  tar -C "$RELEASE" -czf "$RELEASE/$name.tar.gz" "$name"
  rm -rf "$stage"
}

package_unix linux amd64
package_unix linux arm64
package_unix darwin amd64
package_unix darwin arm64

windows_name="fcsv-diff-${VERSION}-windows"
windows_stage="$RELEASE/$windows_name"
mkdir -p "$windows_stage/arm64"
cp "$DIST/fcsv-diff-windows-amd64.exe" "$windows_stage/fcsv-diff.exe"
cp "$DIST/fcsv-diff-windows-arm64.exe" "$windows_stage/arm64/fcsv-diff.exe"
cp "$ROOT/packaging/windows/start-interactive.cmd" "$windows_stage/"
cp "$ROOT/README_WINDOWS.md" "$ROOT/LICENSE" "$ROOT/THIRD_PARTY_NOTICES.md" "$windows_stage/"
(
  cd "$RELEASE"
  zip -qr "$windows_name.zip" "$windows_name"
)
rm -rf "$windows_stage"

(
  cd "$RELEASE"
  sha256sum ./*.tar.gz ./*.zip > SHA256SUMS
)
