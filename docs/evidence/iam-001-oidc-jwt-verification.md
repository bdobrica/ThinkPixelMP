# IAM-001 OIDC/JWT verification evidence

Date: 2026-09-06

## Implemented boundary

- Added a provider-neutral identity verifier port returning immutable copies of
  verified JWT claims without assigning tenant, principal, role, or authority.
- Added an OIDC adapter with HTTPS discovery and rotating JWKS support. It
  validates signatures, exact issuer, audience, an explicit asymmetric
  algorithm allowlist, required subject and expiry, optional not-before, bounded
  clock skew, multi-audience authorized party, compact-token shape, and a 64 KiB
  input limit.
- Added typed, stable invalid-token failure and tests for signature, issuer,
  audience, algorithm-substitution, time-window, authorized-party, missing-claim,
  and size failures.
- Added typed OIDC configuration through JSON, environment, and flags. No local
  development authentication path was introduced; that remains IAM-004.

## Dependency review

`github.com/coreos/go-oidc/v3` is pinned at `v3.20.0` and confined to
`internal/adapters/oidc`. It supplies standards-focused provider discovery,
remote JWKS caching/rotation, and JOSE verification rather than moving a vendor
identity model into the application or domain. The release is Apache-2.0,
supports the repository Go baseline, uses the existing public checksum policy,
and adds only `github.com/go-jose/go-jose/v4` plus `golang.org/x/oauth2` to the
selected runtime graph. The wrapper independently enforces literal issuer
matching and the repository's configurable clock-skew bound.

## Verification

The following gates passed for this item:

```text
GOTOOLCHAIN=go1.26.7 go test ./internal/adapters/oidc ./internal/config ./internal/ports/identity
GOTOOLCHAIN=go1.26.7 go test -race ./internal/adapters/oidc ./internal/config ./internal/ports/identity
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

The aggregate gate passed formatting, vet, Staticcheck, unit and race tests,
repository hygiene, dependency policy, govulncheck (`No vulnerabilities
found.`), license classification, OpenAPI/schema drift checks, and both binary
builds. The existing non-fatal Redocly duplicate-schema-name warnings remained
unchanged.
