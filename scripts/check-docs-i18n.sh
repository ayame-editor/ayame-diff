#!/usr/bin/env bash
set -euo pipefail

docs_dir="${1:-docs}"
failed=0

shopt -s nullglob

# Topic pages live side by side so their language switchers stay symmetric.
# The home page is the one intentional exception: Material publishes
# docs/index.md at / and docs/ja/index.md at /ja/.
for english in "$docs_dir"/*.md; do
  if [[ "$english" == *.ja.md || "$(basename "$english")" == "index.md" ]]; then
    continue
  fi

  japanese="${english%.md}.ja.md"
  if [[ ! -f "$japanese" ]]; then
    echo "error: missing Japanese documentation counterpart: $japanese" >&2
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

for japanese in "$docs_dir"/*.ja.md; do
  english="${japanese%.ja.md}.md"
  if [[ ! -f "$english" ]]; then
    echo "error: missing English documentation counterpart: $english" >&2
    failed=1
  fi
done

for home in "$docs_dir/index.md" "$docs_dir/ja/index.md"; do
  if [[ ! -f "$home" ]]; then
    echo "error: missing localized home page: $home" >&2
    failed=1
  fi
done

extract_command_overview() {
  awk '
    $0 == "<!-- i18n: command-overview -->" { marked = 1; next }
    marked && $0 == "```text" { block = 1; next }
    block && $0 == "```" { exit }
    block && $1 == "ayame-diff" { print $2 }
  ' "$1"
}

english_commands="$(extract_command_overview "$docs_dir/usage.md")"
japanese_commands="$(extract_command_overview "$docs_dir/usage.ja.md")"
if [[ -z "$english_commands" || "$english_commands" != "$japanese_commands" ]]; then
  echo "error: English and Japanese command overviews differ" >&2
  diff -u <(printf '%s\n' "$english_commands") <(printf '%s\n' "$japanese_commands") >&2 || true
  failed=1
fi

for document in "$docs_dir/usage.md" "$docs_dir/usage.ja.md"; do
  for pseudo_path in "clip:" "clipboard:"; do
    if ! grep -Fq "$pseudo_path" "$document"; then
      echo "error: $document does not document $pseudo_path" >&2
      failed=1
    fi
  done
done

extract_home_cards() {
  sed -n '/<div class="doc-card-grid">/,/<\/div>/p' "$1" |
    sed -n 's/.*href="\([^"]*\)".*/\1/p' |
    sed -E 's#^\.\./##; s#\.ja/#/#'
}

english_cards="$(extract_home_cards "$docs_dir/index.md")"
japanese_cards="$(extract_home_cards "$docs_dir/ja/index.md")"
if [[ -z "$english_cards" || "$english_cards" != "$japanese_cards" ]]; then
  echo "error: English and Japanese home-page feature cards differ" >&2
  diff -u <(printf '%s\n' "$english_cards") <(printf '%s\n' "$japanese_cards") >&2 || true
  failed=1
fi

exit "$failed"
