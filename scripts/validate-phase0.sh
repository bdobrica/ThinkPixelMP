#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

for schema in api/schemas/*.json; do
  jq empty "$schema"
done

./scripts/openapi.sh check

git diff --check

printf '%s\n' 'Phase 0 schema, OpenAPI, and whitespace validation passed.'
