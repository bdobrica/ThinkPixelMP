# DB-010 ArtifactDependency persistence

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The ArtifactDependency domain accepts normalized declaration bytes from the
trusted descriptor-validation boundary. It enforces the descriptor's 1 MiB
compiled ceiling and the published closed V1 schema, including duplicate-key and
unknown-field rejection, canonical artifact identities, bounded optional
source/catalog context, required/optional semantics, and exactly one digest,
strict-version, or bounded-range selector. Mutable `latest` and unconstrained `*`
ranges are rejected. The separately supplied SHA-256 digest is verified over the
exact bytes, which are returned only through defensive copies.

The tenth forward migration creates immutable ordered dependency rows under each
tenant-local persisted ArtifactDescriptor. The `(ArtifactVersion, declaration
index)` key preserves source order, and dependency names are unique within the
parent as required by the dependency contract. Each row stores the exact bytes,
digest, equivalent JSONB projection, and queryable declaration fields. A database
trigger requires the projection to equal the descriptor dependency at that exact
array index. Forced RLS, explicit tenant predicates, composite references, and
update/delete guards provide defense in depth.

The PostgreSQL repository exposes tenant-scoped create, get, and ordered list
operations. Duplicate index/name and missing or descriptor-mismatched cases map
to stable redacted typed errors. Identical declarations and digests may recur in
different parents or tenants.

This slice persists publisher declarations only. It does not select candidates,
evaluate catalog eligibility or lifecycle, expand optional dependencies, detect
cycles, build locks, or grant the parent/child any runtime capability. Those
responsibilities remain in Phase 5/6 and the DB-011 through DB-013 audit,
idempotency, and outbox work. No public API/schema, accepted ADR, dependency, or
cross-component boundary changed. `PLAN.md` remains unchanged because sequencing
and implementation intent did not change.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db010-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/artifactdependency ./internal/adapters/postgres/artifactdependency ./internal/adapters/postgres/migration ./cmd/migrate`
  passed.
- `GOCACHE=/tmp/thinkpixelmp-db010-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against the suite-owned PostgreSQL 18.6 container. It covered all ten
  migrations plus dependency create/read/ordered-list operations, exact
  normalized-byte and digest preservation, descriptor-index binding, duplicate
  rejection, cross-tenant denial, and immutable update/delete guards.
- `GOCACHE=/tmp/thinkpixelmp-db010-go-cache GOTOOLCHAIN=go1.26.7 make verify`
  passed.
- `git diff --check` passed.
