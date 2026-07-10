#!/usr/bin/env sh
set -eu

BINARY=${1:-./dist/ayame-diff}
TMP=${TMPDIR:-/tmp}/ayame-diff-smoke-$$
trap 'rm -rf "$TMP"' EXIT INT TERM
mkdir -p "$TMP"

cat > "$TMP/left.tsv" <<'DATA'
id	region	value
1	JP	10
2	JP	20
3	US	30
DATA

cat > "$TMP/right.csv" <<'DATA'
region,id,value
US,3,31
JP,2,20
CA,4,40
DATA

"$BINARY" \
  --left "$TMP/left.tsv" \
  --right "$TMP/right.csv" \
  --key id --key region \
  --partitions 8 --workers 2 --parse-workers 2 \
  --memory 64MiB --partition-buffer 16KiB \
  --progress=false \
  --out "$TMP/diff.tsv"

cat "$TMP/diff.tsv"
