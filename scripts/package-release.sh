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

# Reproducible archives (#172): pin entry order, timestamps, and ownership so a
# given tag always checksums identically (downstream Homebrew/Scoop/WinGet hash
# these). tar entries are sorted with epoch mtime and zeroed ownership; gzip -n
# drops the mtime/name from the gzip header. zip has a 1980 epoch floor, so the
# staged tree is stamped to 1980-01-01 and fed in sorted order with -X (no uid/
# gid/extra-timestamp fields). Staging dirs are temporary, so stamping is safe.
REPRO_MTIME='@0'
REPRO_ZIP_STAMP='198001010000'

make_targz() {
  # <parent-dir> <entry> <output>
  tar --sort=name --mtime="$REPRO_MTIME" --owner=0 --group=0 --numeric-owner \
    -C "$1" -cf - "$2" | gzip -n -9 > "$3"
}

make_zip() {
  # <parent-dir> <entry> <output-absolute>
  rm -f "$3"
  (
    cd "$1"
    find "$2" -exec touch -t "$REPRO_ZIP_STAMP" {} +
    find "$2" | LC_ALL=C sort | zip -X -q "$3" -@
  )
}

while read -r os arch ext; do
  case "$os" in ''|'#'*) continue ;; esac
  [ "$ext" = "-" ] && ext=""
  require_file "$DIST/ayame-diff-${os}-${arch}${ext}"
done < "$TARGETS"

rm -rf "$RELEASE"
mkdir -p "$RELEASE"
ICONS="$RELEASE/.icons"
(cd "$ROOT" && go run ./cmd/ayame-icon -out "$ICONS")

package_unix() {
  os=$1
  arch=$2
  name="ayame-diff-${VERSION}-${os}-${arch}"
  stage="$RELEASE/$name"

  mkdir -p "$stage"
  cp "$DIST/ayame-diff-${os}-${arch}" "$stage/ayame-diff"
  chmod 0755 "$stage/ayame-diff"
  cp "$ROOT/README.md" "$ROOT/LICENSE" "$ROOT/THIRD_PARTY_NOTICES.md" "$stage/"
  if [ "$os" = "linux" ]; then
    mkdir -p "$stage/share/applications" "$stage/share/icons/hicolor/256x256/apps"
    cp "$ROOT/packaging/linux/ayame-diff.desktop" "$stage/share/applications/"
    cp "$ICONS/ayame-diff.png" "$stage/share/icons/hicolor/256x256/apps/ayame-diff.png"
  fi
  make_targz "$RELEASE" "$name" "$RELEASE/$name.tar.gz"
  rm -rf "$stage"

  if [ "$os" = "darwin" ]; then
    "$ROOT/packaging/macos/build-app.sh" \
      "$DIST/ayame-diff-${os}-${arch}" "$VERSION" "$stage" "$ICONS/ayame-diff.icns"
    make_zip "$stage" "Ayame Diff.app" "$RELEASE/${name}-app.zip"
    rm -rf "$stage"
  fi
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
cp "$ROOT/packaging/windows/install-shell.cmd" "$ROOT/packaging/windows/uninstall-shell.cmd" "$windows_stage/"
cp "$ICONS/ayame-diff.ico" "$windows_stage/"
cp "$ROOT/README_WINDOWS.md" "$ROOT/LICENSE" "$ROOT/THIRD_PARTY_NOTICES.md" "$windows_stage/"
make_zip "$RELEASE" "$windows_name" "$RELEASE/$windows_name.zip"
rm -rf "$windows_stage"
rm -rf "$ICONS"

(
  cd "$RELEASE"
  "$ROOT/scripts/checksum.sh" SHA256SUMS ./*.tar.gz ./*.zip
)

# Generate package-manager metadata from the exact archives above. The WinGet
# tree is ready to copy into microsoft/winget-pkgs; compact artifacts are also
# attached to the GitHub release for automated downstream updates.
PACKAGE_META="$DIST/packaging"
rm -rf "$PACKAGE_META"
(cd "$ROOT" && go run ./cmd/packaging-gen -version "$VERSION" -checksums "$RELEASE/SHA256SUMS" -out "$PACKAGE_META")
make_zip "$PACKAGE_META/winget" "manifests" "$RELEASE/ayame-diff-${VERSION}-winget-manifests.zip"
cp "$PACKAGE_META/scoop/ayame-diff.json" "$RELEASE/ayame-diff-${VERSION}-scoop.json"
cp "$PACKAGE_META/homebrew/ayame-diff.rb" "$RELEASE/ayame-diff-${VERSION}-homebrew.rb"
rm -rf "$PACKAGE_META"
(
  cd "$RELEASE"
  "$ROOT/scripts/checksum.sh" SHA256SUMS ./*.tar.gz ./*.zip ./*.json ./*.rb
)
