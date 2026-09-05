# DB-004 Artifact persistence

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The Artifact domain models the tenant-scoped logical resource defined by the V1
schema: UUIDv7 tenant, Artifact, and Namespace identifiers; canonical
`{namespace}/{name}` identity; one of the five closed Artifact kinds; bounded
display metadata, absolute URIs, and labels; and a UTC creation timestamp. Labels
are defensively copied so callers cannot mutate validated domain state. The
repository boundary exposes tenant-scoped create, read by ID or logical identity,
and bounded UUID-ordered list operations.

The PostgreSQL schema references Namespace through a tenant-consistent composite
foreign key and makes `(tenant_id, namespace_id, name)` unique. The canonical
identity is derived by joining the authoritative Namespace path rather than stored
as an independently mutable duplicate. A creation query also matches the domain's
namespace path to its Namespace ID, failing closed for missing, cross-tenant, or
mismatched references. Database constraints and triggers enforce canonical names,
the V1 kind vocabulary, bounded scalar metadata and label maps, immutable identity
and kind, and no deletion. Discovery metadata remains structurally updateable for
future application work without permitting identity changes.

Every adapter operation uses a transaction, validates the tenant UUID, sets the
tenant through bound transaction-local `set_config`, and repeats it in predicates.
Forced RLS and composite keys provide database defense in depth. Duplicate logical
identities, missing Namespaces, invalid input, and unavailable persistence map to
stable redacted typed errors.

DB-004 does not create publication authority merely from Namespace membership.
Namespace delegation and longest-prefix authorization remain PUB-003. Semantic
version, digest, source, descriptor, delivery model, and lifecycle persistence
remain DB-005 through DB-010. Audit/outbox coupling and idempotency remain DB-011,
DB-012, and DB-015. No public schema or cross-component contract changed, and no
dependency was added.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db004-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/artifact ./internal/adapters/postgres/artifact ./internal/adapters/postgres/migration ./cmd/migrate` passed.
- `GOCACHE=/tmp/thinkpixelmp-db004-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against the suite-owned PostgreSQL 18.6 container. It covered all four
  migrations plus Artifact create/read/list, cross-tenant same-name isolation,
  duplicate mapping, Namespace/path consistency, RLS denial, label checks, and
  immutable identity/kind/deletion behavior.
- `GOCACHE=/tmp/thinkpixelmp-db004-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed.
- `git diff --check` passed.
