# ENG-009 OpenAPI generation and drift-check evidence

Completed: 2026-08-31

Implementation commit: `1b8a8ed` (`Add OpenAPI generation and drift checks`)

## Acceptance evidence

The canonical OpenAPI 3.1 source and versioned JSON Schemas now produce the committed, self-contained `api/openapi/generated/thinkpixelmp.bundle.yaml` through the repository-pinned Redocly CLI. `make openapi-generate` is the explicit update path; the generated bundle is documented as derived output and is not an independent contract source.

`make openapi-check` lints the canonical source, resolves and bundles every reference into a temporary candidate, and byte-compares that candidate with the committed bundle. The check reports a missing or stale bundle and is part of `make verify` through the existing contract gate. The equivalent npm scripts delegate to the same repository script so the two entry points cannot drift.

The drift failure path was exercised by adding a temporary difference to the generated bundle. `./scripts/openapi.sh check` failed with a stale-bundle diagnostic and unified diff; the generated bundle was then restored and the check passed.

No public API meaning, JSON Schema meaning, runtime ownership boundary, or dependency version changed.

## Verification

The following commands passed from the repository root:

```text
./scripts/openapi.sh check
npm run validate:openapi
git diff --check
GOCACHE=/tmp/thinkpixelmp-eng009-go-cache GOTOOLCHAIN=go1.26.7 make verify
```

The aggregate verification gate was rerun with public Go module proxy access because the dependency-policy and architecture tests intentionally query module retractions and use isolated module caches. Redocly retained the pre-existing non-fatal duplicate schema-name bundling warnings.
