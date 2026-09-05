# DB-007 ArtifactSource persistence

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The ArtifactSource domain preserves the submitted source variant and the resolved
identity used to register an ArtifactVersion. Its closed V1 variants match the
published schema: OCI reference, remote descriptor reference plus HTTPS endpoint,
or UUIDv7 import-record attribution. Every variant carries a bounded immutable
resolved reference and canonical SHA-256 digest. OCI resolved references must end
in the exact `@sha256:...` digest rather than a mutable tag.

The seventh forward migration creates one source value per tenant-local
ArtifactVersion. A composite foreign key makes the source digest equal the
ArtifactVersion's authoritative content digest and also binds source kind to the
parent delivery model (`oci`, `remote`, or `imported-source`). Variant checks
prevent fields from one source shape leaking into another. Forced RLS and explicit
tenant predicates provide defense in depth, while a database trigger rejects all
updates and deletions after registration.

The registration-only PostgreSQL repository exposes tenant-scoped create and read
operations. It sets transaction-local tenant context through a bound parameter,
checks the exact parent version/digest/delivery tuple, maps duplicate and missing
parent cases to stable redacted typed errors, and exposes no mutation operation.

This task does not fetch or resolve untrusted sources, select registry credentials,
create ImportRecord history, evaluate remote endpoints, persist descriptors, or
couple registration to audit/outbox records; those responsibilities remain in
their sequenced tasks. No public API/schema, accepted ADR, dependency, or
cross-component boundary changed.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db007-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/artifactsource ./internal/adapters/postgres/artifactsource ./internal/domain/artifactversion ./internal/adapters/postgres/artifactversion ./internal/adapters/postgres/migration ./cmd/migrate` passed.
- `GOCACHE=/tmp/thinkpixelmp-db007-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against the suite-owned PostgreSQL 18.6 container. It covered all seven
  migrations plus OCI, remote, and import-record source create/read; submitted-versus-resolved
  reference preservation; exact parent-digest and delivery-model binding; tenant
  isolation; duplicate rejection; invalid mutable OCI reference rejection; and
  immutable update/delete guards.
- `GOCACHE=/tmp/thinkpixelmp-db007-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed.
- `git diff --check` passed.
