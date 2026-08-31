#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_file="$repo_dir/api/openapi/thinkpixelmp.yaml"
generated_file="$repo_dir/api/openapi/generated/thinkpixelmp.bundle.yaml"

usage() {
  printf 'usage: %s <generate|check|validate>\n' "${0##*/}" >&2
  exit 2
}

command="${1:-}"
case "$command" in
  generate|check|validate) ;;
  *) usage ;;
esac

cd "$repo_dir"
npx --no-install redocly lint "$source_file"

if [[ "$command" == validate ]]; then
  exit 0
fi

if [[ "$command" == generate ]]; then
  mkdir -p "$(dirname "$generated_file")"
  npx --no-install redocly bundle "$source_file" --output "$generated_file"
  exit 0
fi

temporary_dir="$(mktemp -d "${TMPDIR:-/tmp}/thinkpixelmp-openapi.XXXXXX")"
trap 'rm -rf "$temporary_dir"' EXIT
candidate="$temporary_dir/thinkpixelmp.bundle.yaml"
npx --no-install redocly bundle "$source_file" --output "$candidate"

if [[ ! -f "$generated_file" ]]; then
  printf 'generated OpenAPI bundle is missing: %s\n' "$generated_file" >&2
  printf 'run: ./scripts/openapi.sh generate\n' >&2
  exit 1
fi

if ! cmp -s "$generated_file" "$candidate"; then
  printf 'generated OpenAPI bundle is stale: %s\n' "$generated_file" >&2
  printf 'run: ./scripts/openapi.sh generate\n' >&2
  diff -u "$generated_file" "$candidate" || true
  exit 1
fi

printf '%s\n' 'OpenAPI source is valid and generated bundle is current.'
