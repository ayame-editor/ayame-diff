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
  --summary-json "$TMP/summary.json" \
  --out "$TMP/diff.tsv"

sed '1d' "$TMP/diff.tsv" | sort > "$TMP/actual-rows.tsv"
cat > "$TMP/expected-rows.tsv" <<'DATA'
CHANGED	left	3	US	30
CHANGED	right	3	US	31
LEFT_ONLY	left	1	JP	10
RIGHT_ONLY	right	4	CA	40
DATA
sort "$TMP/expected-rows.tsv" -o "$TMP/expected-rows.tsv"
cmp "$TMP/expected-rows.tsv" "$TMP/actual-rows.tsv"

for expected in \
  '"left_rows": 3' '"right_rows": 3' '"equal_rows": 1' \
  '"left_only": 1' '"right_only": 1' \
  '"changed_left": 1' '"changed_right": 1' '"diff_rows": 4'
do
  grep -Fq "$expected" "$TMP/summary.json" || {
    echo "summary assertion failed: $expected" >&2
    cat "$TMP/summary.json" >&2
    exit 1
  }
done

echo "smoke test passed"
