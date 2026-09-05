# DB-003 Namespace persistence

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The Namespace domain validates tenant, namespace, and owning Publisher UUIDv7
identifiers; contract-compatible hierarchical paths up to 255 bytes; and UTC
creation timestamps. Namespace identity and original ownership attribution are
immutable. The domain repository boundary exposes tenant-scoped create, read by
ID/path, and bounded ordered list operations.

The PostgreSQL schema stores one owning Publisher for each tenant-local canonical
path. Composite keys and foreign keys prevent cross-tenant ownership references,
while a database trigger locks and verifies the Publisher's current state before
accepting creation. A claimed, suspended, revoked, missing, or cross-tenant
Publisher cannot own a newly created namespace. Tenant A and tenant B may use the
same path without creating global authority. An update/delete trigger protects
identity and historical ownership attribution, and forced RLS fails closed when
tenant context is absent or different.

The PostgreSQL adapter opens a transaction for every operation, applies the
validated tenant UUID through bound transaction-local `set_config`, and repeats
the tenant in all predicates. Expected duplicate paths, invalid owners, missing
records, and invalid input map to stable typed errors; raw PostgreSQL details are
not exposed.

Delegation persistence and longest-prefix publication authorization are not
pulled forward: they remain PUB-003. Audit/outbox coupling and ownership-change
concurrency remain DB-011, DB-015, and their later application work. No public API
or cross-component contract changed, and no dependency was added.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db003-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/namespace ./internal/adapters/postgres/namespace ./internal/adapters/postgres/migration ./cmd/migrate` passed.
- `GOCACHE=/tmp/thinkpixelmp-db003-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against the suite-owned PostgreSQL 18.6 container. It covered migration
  status/up/repeat/concurrency/rollback plus Namespace create/read/list, same-path
  isolation across two tenants, duplicate conflict mapping, verified-owner and
  cross-tenant-owner rejection, RLS denial of a cross-tenant lookup, and database
  rejection of namespace update/delete.
- `GOCACHE=/tmp/thinkpixelmp-db003-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed.
- `git diff --check` passed.

The first sandboxed Docker attempt could not access the Docker socket. The same
documented command was rerun with approved Docker access and passed; no
operator-supplied database or persistent development volume was used.
