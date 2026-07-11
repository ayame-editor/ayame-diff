#!/usr/bin/env sh
set -eu

VERSION="${VERSION:-v0.3.1}"
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
TARGETS="$ROOT/scripts/targets.txt"

require_file() {
  if [ ! -f "$1" ]; then
    echo "missing release input: $1" >&2
    exit 2
  fi
}

while read -r os arch ext; do
  case "$os" in ''|'#'*) continue ;; esac
  [ "$ext" = "-" ] && ext=""
  require_file "$DIST/ayame-diff-${os}-${arch}${ext}"
done < "$TARGETS"

rm -rf "$RELEASE"
mkdir -p "$RELEASE"

package_unix() {
  os=$1
  arch=$2
  name="ayame-diff-${VERSION}-${os}-${arch}"
  stage="$RELEASE/$name"

  mkdir -p "$stage"
  cp "$DIST/ayame-diff-${os}-${arch}" "$stage/ayame-diff"
  chmod 0755 "$stage/ayame-diff"
  cp "$ROOT/README.md" "$ROOT/LICENSE" "$ROOT/THIRD_PARTY_NOTICES.md" "$stage/"
  tar -C "$RELEASE" -czf "$RELEASE/$name.tar.gz" "$name"
  rm -rf "$stage"
}

while read -r os arch ext; do
  case "$os" in ''|'#'*) continue ;; esac
  case "$os" in linux|darwin) package_unix "$os" "$arch" ;; esac
done < "$TARGETS"

windows_name="ayame-diff-${VERSION}-windows"
windows_stage="$RELEASE/$windows_name"
mkdir -p "$windows_stage"
while read -r os arch ext; do
  case "$os" in ''|'#'*) continue ;; esac
  [ "$os" = "windows" ] || continue
  [ "$ext" = "-" ] && ext=""
  if [ "$arch" = "amd64" ]; then
    destination="$windows_stage/ayame-diff.exe"
  else
    mkdir -p "$windows_stage/$arch"
    destination="$windows_stage/$arch/ayame-diff.exe"
  fi
  cp "$DIST/ayame-diff-${os}-${arch}${ext}" "$destination"
done < "$TARGETS"
cp "$ROOT/packaging/windows/start-gui.cmd" "$windows_stage/"
cp "$ROOT/README_WINDOWS.md" "$ROOT/LICENSE" "$ROOT/THIRD_PARTY_NOTICES.md" "$windows_stage/"
(
  cd "$RELEASE"
  zip -qr "$windows_name.zip" "$windows_name"
)
rm -rf "$windows_stage"

(
  cd "$RELEASE"
  "$ROOT/scripts/checksum.sh" SHA256SUMS ./*.tar.gz ./*.zip
)
