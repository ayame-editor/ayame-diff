#!/usr/bin/env bash
set -euo pipefail

adr_dir="${1:-docs/adr}"
failed=0

shopt -s nullglob

for legacy in "$adr_dir"/*.en.md; do
  echo "error: legacy English ADR filename: $legacy" >&2
  failed=1
done

for english in "$adr_dir"/*.md; do
  if [[ "$english" == *.ja.md || "$english" == *.en.md ]]; then
    continue
  fi

  japanese="${english%.md}.ja.md"
  if [[ ! -f "$japanese" ]]; then
    echo "error: missing Japanese ADR counterpart: $japanese" >&2
    failed=1
    continue
  fi

  base="$(basename "${english%.md}")"
  expected="[English](${base}.md) | [日本語](${base}.ja.md)"
  for document in "$english" "$japanese"; do
    actual="$(sed -n '2p' "$document")"
    if [[ "$actual" != "$expected" ]]; then
      echo "error: invalid language switcher in $document" >&2
      echo "  expected: $expected" >&2
      echo "  actual:   $actual" >&2
      failed=1
    fi
  done
done

for japanese in "$adr_dir"/*.ja.md; do
  english="${japanese%.ja.md}.md"
  if [[ ! -f "$english" ]]; then
    echo "error: missing English ADR counterpart: $english" >&2
    failed=1
  fi
done

english_index="$adr_dir/README.md"
japanese_index="$adr_dir/README.ja.md"
for english in "$adr_dir"/[0-9][0-9][0-9][0-9]-*.md; do
  if [[ "$english" == *.ja.md ]]; then
    continue
  fi

  base="$(basename "${english%.md}")"
  number="${base%%-*}"
  if ! grep -Fq "[$number]($base.md)" "$english_index"; then
    echo "error: $english_index does not index $english" >&2
    failed=1
  fi
  if ! grep -Fq "[$number]($base.ja.md)" "$japanese_index"; then
    echo "error: $japanese_index does not index ${english%.md}.ja.md" >&2
    failed=1
  fi
done

exit "$failed"
