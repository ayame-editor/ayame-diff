#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
failed=0

require_literal() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq "$expected" "$file"; then
    echo "error: $file is missing required contributor policy: $expected" >&2
    failed=1
  fi
}

template="$root_dir/.github/pull_request_template.md"
require_literal "$template" "<!-- ui-regression-checklist -->"
require_literal "$template" "docs/ui-regression-checklist.ja.md"
require_literal "$template" "Compare/再比較"
require_literal "$template" "クリック数"
require_literal "$template" "キーボード"
require_literal "$template" "結果領域"
require_literal "$template" "100% / 200%"
require_literal "$template" "Click counts:"
require_literal "$template" "Keyboard path:"
require_literal "$template" "Viewport / scale:"

require_literal "$root_dir/CONTRIBUTING.md" "docs/ui-regression-checklist.ja.md"
require_literal "$root_dir/docs/gui-setup-parity.md" "ui-regression-checklist.md"
require_literal "$root_dir/docs/gui-setup-parity.ja.md" "ui-regression-checklist.ja.md"
require_literal "$root_dir/mkdocs.yml" "ui-regression-checklist.md"
require_literal "$root_dir/mkdocs.yml" "ui-regression-checklist.ja.md"

exit "$failed"
