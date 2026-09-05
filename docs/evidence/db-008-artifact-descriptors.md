# DB-008 ArtifactDescriptor persistence

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The ArtifactDescriptor domain accepts normalized V1 descriptor bytes from the
trusted validation boundary, enforces a 1 MiB compiled ceiling, rejects malformed
JSON, duplicate keys, trailing content, unknown common-envelope fields, invalid
coordinates, unsupported kind/media-type pairs, and more than 256 dependency
entries. It verifies the separately supplied SHA-256 descriptor digest over the
exact normalized bytes and returns defensive copies.

The eighth forward migration creates one descriptor per tenant-local
ArtifactVersion. It persists both the exact digest-bearing bytes and their JSONB
equivalent, while composite foreign keys and JSON checks bind the schema version,
kind, media type, and repeated namespace/name/version coordinates to the parent
Artifact and ArtifactVersion. Forced RLS and explicit tenant predicates provide
defense in depth. A database trigger rejects every update and deletion.

The registration-only PostgreSQL repository exposes tenant-scoped create and read
operations. It derives stored coordinates from the authoritative parent rows,
requires them to match the normalized descriptor envelope, maps duplicate and
missing/mismatched parent cases to stable redacted typed errors, and exposes no
mutation operation.

This persistence slice does not fetch descriptors, implement RFC 8785
canonicalization, perform kind-specific schema/package validation, extract
requirements or dependencies, or couple registration to audit/outbox records.
Those responsibilities remain in DSC-001 through DSC-007 and DB-009 through
DB-013. No public API/schema, accepted ADR, dependency, or cross-component
boundary changed.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db008-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/artifactdescriptor ./internal/adapters/postgres/artifactdescriptor ./internal/domain/artifactversion ./internal/adapters/postgres/artifactversion ./internal/adapters/postgres/migration ./cmd/migrate` passed.
- `GOCACHE=/tmp/thinkpixelmp-db008-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against the suite-owned PostgreSQL 18.6 container. It covered all eight
  migrations plus descriptor create/read, exact normalized-byte round trip,
  digest verification, parent-coordinate binding, duplicate rejection, tenant
  isolation, and immutable update/delete guards.
- `GOCACHE=/tmp/thinkpixelmp-db008-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed.
- `git diff --check` passed.
