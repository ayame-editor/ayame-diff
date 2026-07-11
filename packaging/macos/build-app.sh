#!/usr/bin/env bash
# Build "Ayame Diff.app", a minimal macOS bundle that launches the ayame-diff
# web GUI (the `gui` subcommand) — so users can double-click instead of using a
# terminal. Icon is optional: pass an .icns path as $4 to embed one.
#
# Usage: build-app.sh <ayame-diff-binary> <version> <output-dir> [icon.icns]
set -euo pipefail

binary=${1:?usage: build-app.sh <binary> <version> <output-dir> [icon.icns]}
version=${2:?missing version}
plist_version=${version#v}
outdir=${3:?missing output dir}
icon=${4:-}

app="$outdir/Ayame Diff.app"
rm -rf "$app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"

# The real executable, plus a launcher that runs it with `gui`.
cp "$binary" "$app/Contents/MacOS/ayame-diff"
chmod 0755 "$app/Contents/MacOS/ayame-diff"
cat > "$app/Contents/MacOS/launcher" <<'LAUNCH'
#!/bin/sh
dir="$(cd "$(dirname "$0")" && pwd)"
exec "$dir/ayame-diff" gui
LAUNCH
chmod 0755 "$app/Contents/MacOS/launcher"

iconkey=""
if [ -n "$icon" ] && [ -f "$icon" ]; then
  cp "$icon" "$app/Contents/Resources/ayame-diff.icns"
  iconkey='<key>CFBundleIconFile</key><string>ayame-diff</string>'
fi

cat > "$app/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>Ayame Diff</string>
  <key>CFBundleDisplayName</key><string>Ayame Diff</string>
  <key>CFBundleIdentifier</key><string>com.hjosugi.ayame-diff</string>
  <key>CFBundleVersion</key><string>${plist_version}</string>
  <key>CFBundleShortVersionString</key><string>${plist_version}</string>
  <key>CFBundleExecutable</key><string>launcher</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  ${iconkey}
</dict>
</plist>
PLIST

echo "built: $app"
