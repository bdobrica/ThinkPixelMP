# IAM-002 claim mapping evidence

Date: 2026-09-06
Implementation commit: `cf049d8`

## Implemented boundary

- Added a provider-neutral mapped identity and mapper port. The mapped result
  contains one UUIDv7 tenant and one opaque principal ID, but no role or
  administrative authority.
- Added an issuer-specific OIDC mapper that reads only configured top-level
  verified claims. Tenant values must match an explicit operator mapping; the
  claim itself never directly supplies a tenant ID.
- Scalar and string-array tenant claims are supported. Missing, malformed,
  unmapped, wrong-issuer, duplicate-name, and multiply mapped claims fail closed
  with the stable `unauthorized:identity.unmapped` classification.
- Principal claim names are configurable. Values are bounded and transformed
  into a stable issuer-scoped SHA-256 identifier so raw identity claims do not
  become persisted audit or idempotency identifiers.
- Added strict JSON, environment, and flag configuration for claim names and
  claim-value-to-UUIDv7 mappings. Configuration rejects missing, duplicate,
  malformed, oversized, and non-UUIDv7 entries. Safe configuration rendering
  exposes only the mapping count, not mapped claim values or tenant IDs.

IAM-002 does not grant roles or administrative actions. That remains IAM-003.
It also does not introduce a local authentication fallback; that remains
IAM-004. No public HTTP or versioned wire schema changed.

## Verification

The following gates passed for this item:

```text
GOTOOLCHAIN=go1.26.7 go test -race ./internal/adapters/oidc ./internal/config ./internal/ports/identity
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

The focused suite covers signed JWT verification followed by mapping, scalar
and array tenant claims, custom principal claims, deterministic opaque
principal IDs, exact issuer binding, ambiguous/unmapped claims, duplicate JSON
claim names, subject consistency, bounded policy input, UUIDv7 validation, and
configuration redaction. The aggregate gate passed formatting, vet,
Staticcheck, unit and race tests, repository hygiene, dependency policy,
govulncheck (`No vulnerabilities found.`), license classification,
OpenAPI/schema drift checks, and both binary builds. Existing non-fatal Redocly
duplicate-schema-name warnings remained unchanged.
