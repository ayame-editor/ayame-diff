#!/usr/bin/env sh
set -eu

if [ "$#" -lt 2 ]; then
  echo "usage: checksum.sh OUTPUT FILE..." >&2
  exit 2
fi

output=$1
shift
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$@" > "$output"
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 "$@" > "$output"
else
  echo "neither sha256sum nor shasum is available" >&2
  exit 2
fi
