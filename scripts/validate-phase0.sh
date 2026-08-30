#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"

for schema in api/schemas/*.json; do
  jq empty "$schema"
done

npx --no-install redocly lint api/openapi/thinkpixelmp.yaml
npx --no-install redocly bundle api/openapi/thinkpixelmp.yaml --output /tmp/thinkpixelmp-openapi-bundle.yaml

git diff --check

printf '%s\n' 'Phase 0 schema, OpenAPI, and whitespace validation passed.'
