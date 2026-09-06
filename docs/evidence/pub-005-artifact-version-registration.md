# PUB-005 ArtifactVersion registration skeleton evidence

- Date: 2026-09-06
- Scope: trusted immutable ArtifactVersion and ArtifactSource persistence seam

## Outcome

The publication application now has a composable registration service for
trusted pipeline output that already contains a canonical semantic version,
authoritative SHA-256 content digest, resolved immutable source reference,
artifact class, and delivery model. The service requires the exact
`publication.publish` action, resolves the logical Artifact in the authenticated
tenant, and rechecks that the attributed Publisher is the live longest-prefix
owner of the Artifact namespace.

Registration rejects a missing or malformed immutable identity, a source digest
that differs from the ArtifactVersion digest, mutable OCI resolved references,
cross-variant source fields, and source-kind/delivery-model disagreement. Within
one tenant transaction it couples the idempotency claim and result with the
immutable ArtifactVersion and ArtifactSource writes. The existing repositories
couple their minimized mutation audit facts to those writes. Identical replay
loads the established version and source; changed content under the same
tenant/principal/action/key tuple conflicts.

This is deliberately an internal trusted persistence seam, not a new public
request shape. Public registration continues to accept submitted source metadata
and optional expected digest under the published asynchronous API contract. OCI
resolution, hostile descriptor inspection and validation, descriptor persistence,
the registration event (which requires a descriptor digest), and the final
immutable commit pipeline remain PUB-006 and its Phase 3 prerequisites. No
caller-supplied source data is treated as proof, catalog eligibility, runtime
authority, or permission to publish.

## Verification

```text
GOTOOLCHAIN=go1.26.7 go test -race ./internal/app/publication ./internal/domain/artifactversion ./internal/domain/artifactsource
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

Focused tests cover exact-role authorization, live namespace ownership,
canonical idempotent replay, changed-request conflict, exact digest equality,
immutable OCI reference enforcement, and audit identity propagation to both
aggregate writes.
